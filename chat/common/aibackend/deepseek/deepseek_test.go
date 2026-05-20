package deepseek

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chat/common/aibackend"
)

var _ aibackend.AIBackend = (*Backend)(nil)

func TestFactoryRegistered(t *testing.T) {
	for _, n := range aibackend.RegisteredTypes() {
		if n == TypeName {
			return
		}
	}
	t.Fatalf("deepseek backend not registered")
}

func TestFactoryFromMap(t *testing.T) {
	b, err := aibackend.New(TypeName, map[string]any{
		"Name":          "deepseek-main",
		"BaseURL":       "https://api.deepseek.com",
		"APIKey":        "sk-xxx",
		"Model":         "deepseek-chat",
		"Timeout":       "30s",
		"StreamTimeout": "120s",
		"Temperature":   0.5,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if b.Name() != "deepseek-main" || b.Type() != "deepseek" {
		t.Errorf("Name/Type = %q/%q", b.Name(), b.Type())
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New(Config{BaseURL: "https://x"}); !errors.Is(err, aibackend.ErrUnauthorized) {
		t.Errorf("missing APIKey: err = %v, want ErrUnauthorized", err)
	}
}

func TestOpenConversationIsNoop(t *testing.T) {
	b := mustBackend(t, "")
	resp, err := b.OpenConversation(context.Background(), aibackend.OpenConversationReq{User: "u"})
	if err != nil || resp.ConversationID != "" {
		t.Errorf("OpenConversation = %+v %v, want empty + nil", resp, err)
	}
}

func TestUploadFileNotImplemented(t *testing.T) {
	b := mustBackend(t, "")
	_, err := b.UploadFile(context.Background(), aibackend.UploadFileReq{})
	if !errors.Is(err, aibackend.ErrNotImplemented) {
		t.Errorf("UploadFile err = %v, want ErrNotImplemented", err)
	}
}

// fakeDeepSeek 返回一个 /chat/completions 的 mock：
//   - stream=true 时按 SSE 推 chunks
//   - stream=false 时一次性返回 JSON
func fakeDeepSeek(t *testing.T, chunks []string, blockingAnswer string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if !body.Stream {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, blockingAnswer)
			return
		}
		// stream=true 走 SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for _, c := range chunks {
			io.WriteString(w, "data: "+c+"\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		}
		io.WriteString(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}))
}

func TestChatHappyPath(t *testing.T) {
	srv := fakeDeepSeek(t, nil, "你好！")
	defer srv.Close()

	b := mustBackend(t, srv.URL)
	resp, err := b.Chat(context.Background(), aibackend.ChatReq{
		Messages: []aibackend.Message{
			{Role: "system", Content: "you are helpful"},
			{Role: "user", Content: "你好"},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Answer != "你好！" {
		t.Errorf("Answer = %q, want 你好！", resp.Answer)
	}
}

func TestChatQueryFallback(t *testing.T) {
	// Messages 为空时应用 Query 作为单条 user message 兜底
	var seenRoles []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		for _, m := range body.Messages {
			seenRoles = append(seenRoles, m.Role)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer srv.Close()

	b := mustBackend(t, srv.URL)
	_, err := b.Chat(context.Background(), aibackend.ChatReq{Query: "hi"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(seenRoles) != 1 || seenRoles[0] != "user" {
		t.Errorf("seen roles = %v, want [user]", seenRoles)
	}
}

func TestChatEmptyMessagesAndQuery(t *testing.T) {
	b := mustBackend(t, "")
	_, err := b.Chat(context.Background(), aibackend.ChatReq{})
	if !errors.Is(err, aibackend.ErrBadRequest) {
		t.Errorf("err = %v, want ErrBadRequest", err)
	}
}

func TestChatStreamHappyPath(t *testing.T) {
	chunks := []string{
		`{"choices":[{"delta":{"content":"你"}}]}`,
		`{"choices":[{"delta":{"content":"好"}}]}`,
		`{"choices":[{"delta":{"content":"！"}}]}`,
	}
	srv := fakeDeepSeek(t, chunks, "")
	defer srv.Close()

	b := mustBackend(t, srv.URL)
	ch, err := b.ChatStream(context.Background(), aibackend.ChatReq{
		Messages: []aibackend.Message{{Role: "user", Content: "你好"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var got []aibackend.ChunkType
	var assembled string
	for c := range ch {
		got = append(got, c.Type)
		if c.Type == aibackend.ChunkTypeText {
			assembled += c.Text
		}
	}
	if len(got) == 0 || got[len(got)-1] != aibackend.ChunkTypeDone {
		t.Errorf("last chunk type = %v, want Done (full=%v)", got[len(got)-1], got)
	}
	if assembled == "" {
		t.Errorf("no text chunks assembled; got types %v", got)
	}
}

func TestChatStreamUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	b := mustBackend(t, srv.URL)
	ch, err := b.ChatStream(context.Background(), aibackend.ChatReq{
		Messages: []aibackend.Message{{Role: "user", Content: "x"}},
	})
	if err != nil {
		// 也接受同步错误
		if !errors.Is(err, aibackend.ErrUnauthorized) {
			t.Errorf("sync err = %v, want ErrUnauthorized", err)
		}
		return
	}
	var sawErr bool
	for c := range ch {
		if c.Type == aibackend.ChunkTypeError {
			sawErr = true
			if !errors.Is(c.Err, aibackend.ErrUnauthorized) {
				t.Errorf("error chunk = %v, want ErrUnauthorized", c.Err)
			}
		}
	}
	if !sawErr {
		t.Errorf("expected error chunk; saw none")
	}
}

func TestMapError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"nil", nil, nil},
		{"ctx", context.DeadlineExceeded, aibackend.ErrBackendUnavailable},
		{"401-status", fmt.Errorf("API error: bad, status code: 401"), aibackend.ErrUnauthorized},
		{"403-status", fmt.Errorf("API error: forbidden, status code: 403"), aibackend.ErrUnauthorized},
		{"invalid-key-text", fmt.Errorf("invalid api key"), aibackend.ErrUnauthorized},
		{"429", fmt.Errorf("status code: 429"), aibackend.ErrRateLimited},
		{"rate-limit-text", fmt.Errorf("rate limit exceeded"), aibackend.ErrRateLimited},
		{"400", fmt.Errorf("status code: 400 bad request"), aibackend.ErrBadRequest},
		{"500", fmt.Errorf("status code: 500"), aibackend.ErrBackendUnavailable},
		{"502", fmt.Errorf("status code: 502 bad gateway"), aibackend.ErrBackendUnavailable},
		{"timeout", fmt.Errorf("dial tcp: i/o timeout"), aibackend.ErrBackendUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mapError(c.in)
			if c.want == nil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			if !errors.Is(got, c.want) {
				t.Errorf("mapError(%v) = %v, want errors.Is %v", c.in, got, c.want)
			}
		})
	}
}

func mustBackend(t *testing.T, baseURL string) *Backend {
	t.Helper()
	b, err := New(Config{
		Name:    "deepseek-test",
		BaseURL: baseURL,
		APIKey:  "sk-test",
		Model:   "deepseek-chat",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}
