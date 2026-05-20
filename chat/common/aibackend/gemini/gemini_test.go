package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	t.Fatalf("gemini backend not registered")
}

func TestFactoryFromMap(t *testing.T) {
	b, err := aibackend.New(TypeName, map[string]any{
		"Name":          "gemini-main",
		"BaseURL":       "https://generativelanguage.googleapis.com",
		"APIKey":        "AIxxx",
		"Model":         "gemini-1.5-pro",
		"Temperature":   0.5,
		"Timeout":       "30s",
		"StreamTimeout": "120s",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if b.Name() != "gemini-main" || b.Type() != "gemini" {
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
		hasKey   bool
		contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text,omitempty"`
			} `json:"parts"`
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path = r.URL.Path
		captured.hasKey = r.URL.Query().Get("key") != ""
		var body struct {
			Contents []struct {
				Role  string `json:"role"`
				Parts []struct {
					Text string `json:"text,omitempty"`
				} `json:"parts"`
			} `json:"contents"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		captured.contents = body.Contents
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"你好！"}]}}]}`)
	}))
	defer srv.Close()

	b := mustBackend(t, srv.URL)
	resp, err := b.Chat(context.Background(), aibackend.ChatReq{
		Messages: []aibackend.Message{
			{Role: "user", Content: "你好"},
			{Role: "assistant", Content: "已收到"}, // 应映射到 model
			{Role: "user", Content: "再聊聊"},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Answer != "你好！" {
		t.Errorf("Answer = %q, want 你好！", resp.Answer)
	}
	if !strings.HasPrefix(captured.path, "/v1beta/models/") || !strings.HasSuffix(captured.path, ":generateContent") {
		t.Errorf("path = %q, want /v1beta/models/.../:generateContent", captured.path)
	}
	if !captured.hasKey {
		t.Errorf("api key not present in query string")
	}
	// 角色检查：role=assistant 应被翻译成 model
	gotRoles := make([]string, 0, len(captured.contents))
	for _, c := range captured.contents {
		gotRoles = append(gotRoles, c.Role)
	}
	if !rolesContainModel(gotRoles) {
		t.Errorf("server got roles %v; expected one of them = 'model' (assistant→model)", gotRoles)
	}
}

func TestChatQueryFallback(t *testing.T) {
	var seenRoles []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Contents []struct {
				Role string `json:"role"`
			} `json:"contents"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		for _, c := range body.Contents {
			seenRoles = append(seenRoles, c.Role)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
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

func TestChatErrorReturned(t *testing.T) {
	// Gemini SDK 把 result.Code != 0 包成 errors.New(result.Message)。
	// 模拟 401-ish "invalid api key" 错误体。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"code":401,"message":"API key not valid","status":"UNAUTHENTICATED"}}`)
	}))
	defer srv.Close()

	b := mustBackend(t, srv.URL)
	_, err := b.Chat(context.Background(), aibackend.ChatReq{
		Messages: []aibackend.Message{{Role: "user", Content: "x"}},
	})
	// 上游 SDK 会拿 result.Message="API key not valid" 包成 error，
	// mapError 应映射到 ErrUnauthorized
	if !errors.Is(err, aibackend.ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}

func TestMapError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"nil", nil, nil},
		{"ctx-deadline", context.DeadlineExceeded, aibackend.ErrBackendUnavailable},
		{"api-key-invalid", fmt.Errorf("API key not valid. Please pass a valid API key."), aibackend.ErrUnauthorized},
		{"unauthenticated", fmt.Errorf("UNAUTHENTICATED"), aibackend.ErrUnauthorized},
		{"permission", fmt.Errorf("PERMISSION_DENIED: ..."), aibackend.ErrUnauthorized},
		{"quota", fmt.Errorf("Quota exceeded for ..."), aibackend.ErrRateLimited},
		{"resource-exhausted", fmt.Errorf("RESOURCE_EXHAUSTED"), aibackend.ErrRateLimited},
		{"invalid-arg", fmt.Errorf("INVALID_ARGUMENT: bad input"), aibackend.ErrBadRequest},
		{"context-too-long", fmt.Errorf("input too long for model"), aibackend.ErrBadRequest},
		{"internal", fmt.Errorf("INTERNAL: server error"), aibackend.ErrBackendUnavailable},
		{"unavailable", fmt.Errorf("UNAVAILABLE: backend down"), aibackend.ErrBackendUnavailable},
		{"timeout", fmt.Errorf("i/o timeout"), aibackend.ErrBackendUnavailable},
		{"refused", fmt.Errorf("connection refused"), aibackend.ErrBackendUnavailable},
		{"unknown", fmt.Errorf("weird"), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mapError(c.in)
			if c.want == nil {
				if c.name == "nil" {
					if got != nil {
						t.Errorf("got %v, want nil", got)
					}
					return
				}
				if got == nil {
					t.Errorf("got nil, want passthrough")
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
				t.Errorf("mapError(%v) = %v, want errors.Is %v", c.in, got, c.want)
			}
		})
	}
}

func rolesContainModel(roles []string) bool {
	for _, r := range roles {
		if r == "model" {
			return true
		}
	}
	return false
}

func mustBackend(t *testing.T, baseURL string) *Backend {
	t.Helper()
	b, err := New(Config{
		Name:    "gemini-test",
		BaseURL: baseURL,
		APIKey:  "AIxxx",
		Model:   "gemini-pro",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}
