package openai

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

	copenai "github.com/sashabaranov/go-openai"
)

var _ aibackend.AIBackend = (*Backend)(nil)

func TestFactoryRegistered(t *testing.T) {
	for _, n := range aibackend.RegisteredTypes() {
		if n == TypeName {
			return
		}
	}
	t.Fatalf("openai backend not registered")
}

func TestFactoryFromMap(t *testing.T) {
	b, err := aibackend.New(TypeName, map[string]any{
		"Name":          "openai-main",
		"BaseURL":       "https://api.openai.com",
		"APIKey":        "sk-xxx",
		"Model":         "gpt-4o-mini",
		"Origin":        "open_ai",
		"Temperature":   0.5,
		"MaxToken":      float64(1024),
		"TotalToken":    float64(8000),
		"Timeout":       "30s",
		"StreamTimeout": "120s",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if b.Name() != "openai-main" || b.Type() != "openai" {
		t.Errorf("Name/Type = %q/%q", b.Name(), b.Type())
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New(Config{BaseURL: "https://x"}); !errors.Is(err, aibackend.ErrUnauthorized) {
		t.Errorf("missing APIKey: err = %v, want ErrUnauthorized", err)
	}
}

func TestAzureRequiresEngine(t *testing.T) {
	if _, err := New(Config{APIKey: "k", BaseURL: "https://x.openai.azure.com", Origin: "azure"}); !errors.Is(err, aibackend.ErrBadRequest) {
		t.Errorf("Azure without Engine: err = %v, want ErrBadRequest", err)
	}
}

func TestOpenConversationIsNoop(t *testing.T) {
	b := mustBackend(t, "")
	resp, err := b.OpenConversation(context.Background(), aibackend.OpenConversationReq{User: "u"})
	if err != nil || resp.ConversationID != "" {
		t.Errorf("OpenConversation = %+v %v", resp, err)
	}
}

func TestUploadFileNotImplemented(t *testing.T) {
	b := mustBackend(t, "")
	_, err := b.UploadFile(context.Background(), aibackend.UploadFileReq{})
	if !errors.Is(err, aibackend.ErrNotImplemented) {
		t.Errorf("UploadFile err = %v, want ErrNotImplemented", err)
	}
}

func TestChatHappyPath(t *testing.T) {
	var captured struct {
		path     string
		messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path = r.URL.Path
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		captured.messages = body.Messages
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"msg-1","choices":[{"message":{"role":"assistant","content":"你好！"}}]}`)
	}))
	defer srv.Close()

	b := mustBackend(t, srv.URL)
	resp, err := b.Chat(context.Background(), aibackend.ChatReq{
		Messages: []aibackend.Message{
			{Role: "system", Content: "be brief"},
			{Role: "user", Content: "你好"},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Answer != "你好！" {
		t.Errorf("Answer = %q, want 你好！", resp.Answer)
	}
	if resp.MessageID != "msg-1" {
		t.Errorf("MessageID = %q, want msg-1", resp.MessageID)
	}
	if captured.path != "/v1/chat/completions" {
		t.Errorf("path = %q, want /v1/chat/completions", captured.path)
	}
	if len(captured.messages) != 2 || captured.messages[0].Role != "system" || captured.messages[1].Role != "user" {
		t.Errorf("messages = %+v, want system+user", captured.messages)
	}
}

func TestChatQueryFallback(t *testing.T) {
	var seenRoles []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role string `json:"role"`
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
	if _, err := b.Chat(context.Background(), aibackend.ChatReq{Query: "hi"}); err != nil {
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
		`{"id":"msg-1","choices":[{"delta":{"role":"assistant","content":"你"},"index":0}]}`,
		`{"id":"msg-1","choices":[{"delta":{"content":"好"},"index":0}]}`,
		`{"id":"msg-1","choices":[{"delta":{"content":"！"},"index":0,"finish_reason":"stop"}]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer srv.Close()

	b := mustBackend(t, srv.URL)
	ch, err := b.ChatStream(context.Background(), aibackend.ChatReq{
		Messages: []aibackend.Message{{Role: "user", Content: "你好"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	var got []aibackend.ChunkType
	var text string
	for c := range ch {
		got = append(got, c.Type)
		if c.Type == aibackend.ChunkTypeText {
			text += c.Text
		}
	}
	if text != "你好！" {
		t.Errorf("assembled text = %q, want 你好！", text)
	}
	if len(got) == 0 || got[len(got)-1] != aibackend.ChunkTypeDone {
		t.Errorf("last chunk type = %v, want Done (full=%v)", got[len(got)-1], got)
	}
}

func TestChatStreamUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"bad key","type":"invalid_request_error","code":"invalid_api_key"}}`)
	}))
	defer srv.Close()

	b := mustBackend(t, srv.URL)
	ch, err := b.ChatStream(context.Background(), aibackend.ChatReq{
		Messages: []aibackend.Message{{Role: "user", Content: "x"}},
	})
	if err != nil {
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
				t.Errorf("err chunk = %v, want ErrUnauthorized", c.Err)
			}
		}
	}
	if !sawErr {
		t.Errorf("expected error chunk; saw none")
	}
}

func TestMapErrorTypedAPIError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"nil", nil, nil},
		{"ctx-canceled", context.Canceled, aibackend.ErrBackendUnavailable},
		{"invalid-api-key", &copenai.APIError{HTTPStatusCode: 401, Code: "invalid_api_key", Message: "x"}, aibackend.ErrUnauthorized},
		{"rate-limit", &copenai.APIError{HTTPStatusCode: 429, Code: "rate_limit_exceeded"}, aibackend.ErrRateLimited},
		{"context-length", &copenai.APIError{HTTPStatusCode: 400, Code: "context_length_exceeded"}, aibackend.ErrBadRequest},
		{"status-401", &copenai.APIError{HTTPStatusCode: 401}, aibackend.ErrUnauthorized},
		{"status-429", &copenai.APIError{HTTPStatusCode: 429}, aibackend.ErrRateLimited},
		{"status-500", &copenai.APIError{HTTPStatusCode: 500}, aibackend.ErrBackendUnavailable},
		{"wrapped", fmt.Errorf("upstream: %w", &copenai.APIError{HTTPStatusCode: 401}), aibackend.ErrUnauthorized},
		{"net-timeout", fmt.Errorf("dial tcp: i/o timeout"), aibackend.ErrBackendUnavailable},
		{"refused", fmt.Errorf("connection refused"), aibackend.ErrBackendUnavailable},
		{"unknown", fmt.Errorf("weird"), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mapError(c.err)
			if c.want == nil {
				if c.name == "nil" {
					if got != nil {
						t.Errorf("got %v, want nil", got)
					}
					return
				}
				if got == nil {
					t.Errorf("got nil, want passthrough non-nil")
					return
				}
				for _, std := range []error{
					aibackend.ErrBackendUnavailable, aibackend.ErrBadRequest, aibackend.ErrUnauthorized,
					aibackend.ErrRateLimited, aibackend.ErrConversationNotFound, aibackend.ErrNotImplemented,
				} {
					if errors.Is(got, std) {
						t.Errorf("unknown err mis-mapped to %v", std)
					}
				}
				return
			}
			if !errors.Is(got, c.want) {
				t.Errorf("mapError(%v) = %v, want errors.Is %v", c.err, got, c.want)
			}
		})
	}
}

func mustBackend(t *testing.T, baseURL string) *Backend {
	t.Helper()
	b, err := New(Config{
		Name:    "openai-test",
		BaseURL: baseURL,
		APIKey:  "sk-test",
		Model:   "gpt-3.5-turbo",
		Origin:  "open_ai",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}
