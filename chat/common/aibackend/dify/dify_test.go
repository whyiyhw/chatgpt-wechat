package dify

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"chat/common/aibackend"
)

// compile-time 接口契约检查
var _ aibackend.AIBackend = (*Backend)(nil)

func TestFactoryRegistered(t *testing.T) {
	// init() 应已把 "dify" 注册进 registry
	types := aibackend.RegisteredTypes()
	var found bool
	for _, n := range types {
		if n == TypeName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("dify backend not registered; got %v", types)
	}
}

func TestFactoryFromMap(t *testing.T) {
	b, err := aibackend.New(TypeName, map[string]any{
		"Name":          "dify-main",
		"BaseURL":       "https://dify.example.com",
		"APIKey":        "sk-xxx",
		"Timeout":       "30s",
		"StreamTimeout": "90s",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if b.Name() != "dify-main" {
		t.Errorf("Name = %q, want %q", b.Name(), "dify-main")
	}
	if b.Type() != "dify" {
		t.Errorf("Type = %q, want %q", b.Type(), "dify")
	}
}

func TestNewRequiresBaseURLAndAPIKey(t *testing.T) {
	if _, err := New(Config{APIKey: "k"}); !errors.Is(err, aibackend.ErrBadRequest) {
		t.Errorf("missing BaseURL: err = %v, want ErrBadRequest", err)
	}
	if _, err := New(Config{BaseURL: "https://x"}); !errors.Is(err, aibackend.ErrUnauthorized) {
		t.Errorf("missing APIKey: err = %v, want ErrUnauthorized", err)
	}
}

func TestOpenConversationIsNoop(t *testing.T) {
	b := mustBackend(t)
	resp, err := b.OpenConversation(context.Background(), aibackend.OpenConversationReq{User: "u"})
	if err != nil {
		t.Fatalf("OpenConversation: %v", err)
	}
	if resp.ConversationID != "" {
		t.Errorf("ConversationID = %q, want empty (Dify creates on first chat)", resp.ConversationID)
	}
}

func TestUploadFileNotImplemented(t *testing.T) {
	b := mustBackend(t)
	_, err := b.UploadFile(context.Background(), aibackend.UploadFileReq{})
	if !errors.Is(err, aibackend.ErrNotImplemented) {
		t.Errorf("UploadFile err = %v, want ErrNotImplemented", err)
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
		{"ctx-canceled", context.Canceled, aibackend.ErrBackendUnavailable},
		{"401", fmt.Errorf("HTTP response error: [401]unauthorized"), aibackend.ErrUnauthorized},
		{"invalid_api_key", fmt.Errorf("HTTP response error: [invalid_api_key]bad key"), aibackend.ErrUnauthorized},
		{"429", fmt.Errorf("HTTP response error: [429]too many requests"), aibackend.ErrRateLimited},
		{"rate-limit-text", fmt.Errorf("dify: rate limit exceeded"), aibackend.ErrRateLimited},
		{"conv-not-found", fmt.Errorf("HTTP response error: [conversation_not_found]conversation not exist"), aibackend.ErrConversationNotFound},
		{"400", fmt.Errorf("HTTP response error: [400]bad request"), aibackend.ErrBadRequest},
		{"500", fmt.Errorf("HTTP response error: [500]internal"), aibackend.ErrBackendUnavailable},
		{"timeout", fmt.Errorf("dial tcp: i/o timeout"), aibackend.ErrBackendUnavailable},
		{"no-host", fmt.Errorf("dial tcp: lookup x: no such host"), aibackend.ErrBackendUnavailable},
		{"unrecognized-passthrough", fmt.Errorf("weird"), nil}, // expect passthrough; we check explicitly below
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mapError(c.in)
			if c.want == nil && c.name == "nil" {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			if c.want == nil {
				// 未识别错误应原样返回，而不是被错分类
				if got == nil {
					t.Fatalf("got nil, want passthrough non-nil")
				}
				for _, std := range []error{
					aibackend.ErrBackendUnavailable,
					aibackend.ErrBadRequest,
					aibackend.ErrUnauthorized,
					aibackend.ErrRateLimited,
					aibackend.ErrConversationNotFound,
					aibackend.ErrNotImplemented,
				} {
					if errors.Is(got, std) {
						t.Errorf("unrecognized error mapped to %v unexpectedly", std)
					}
				}
				return
			}
			if !errors.Is(got, c.want) {
				t.Errorf("mapError(%v) = %v, want errors.Is == %v", c.in, got, c.want)
			}
		})
	}
}

func mustBackend(t *testing.T) *Backend {
	t.Helper()
	b, err := New(Config{
		Name:    "dify-test",
		BaseURL: "https://dify.example.com",
		APIKey:  "sk-test",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}
