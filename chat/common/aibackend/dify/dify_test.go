package dify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

// fakeDifySSE 返回一个 httptest.Server，行为：
//   - POST /v1/chat-messages 时按 events 顺序回放 SSE 行；
//   - 其它路径返回 404，便于在测试里区分调用是否走到了预期端点。
func fakeDifySSE(t *testing.T, events []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat-messages" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for _, e := range events {
			io.WriteString(w, "data: "+e+"\n")
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
}

func TestChatStreamWorkflowEvents(t *testing.T) {
	events := []string{
		`{"event":"workflow_started","task_id":"t-1","conversation_id":"c-1","workflow_run_id":"wfr-1"}`,
		`{"event":"node_started","task_id":"t-1","conversation_id":"c-1","workflow_run_id":"wfr-1","data":{"id":"n-1","node_id":"n-1","node_type":"start","title":"开始"}}`,
		`{"event":"node_finished","task_id":"t-1","conversation_id":"c-1","workflow_run_id":"wfr-1","data":{"id":"n-2","node_id":"n-2","node_type":"answer","title":"回答","status":"succeeded","outputs":{"answer":"你好！"},"elapsed_time":0.42}}`,
		`{"event":"workflow_finished","task_id":"t-1","conversation_id":"c-1","workflow_run_id":"wfr-1","message_id":"m-1","data":{"status":"succeeded","outputs":{"answer":"你好！"},"elapsed_time":1.1,"execution_metadata":{"total_tokens":42}}}`,
	}
	srv := fakeDifySSE(t, events)
	defer srv.Close()

	b, err := New(Config{Name: "test", BaseURL: srv.URL, APIKey: "k", Timeout: 5 * time.Second, StreamTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := b.ChatStream(ctx, aibackend.ChatReq{Query: "hi", User: "u"})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var got []aibackend.ChunkType
	var convID string
	var lastWorkflow *aibackend.WorkflowEvent
	for c := range ch {
		got = append(got, c.Type)
		if c.ConversationID != "" {
			convID = c.ConversationID
		}
		if c.Type == aibackend.ChunkTypeWorkflowFinished {
			lastWorkflow = c.Workflow
			if c.Usage == nil || c.Usage.TotalTokens != 42 {
				t.Errorf("workflow_finished Usage = %+v, want TotalTokens=42", c.Usage)
			}
		}
		if c.Type == aibackend.ChunkTypeNodeFinished {
			if c.Workflow == nil {
				t.Errorf("node_finished should carry Workflow")
			} else if got, ok := c.Workflow.Outputs["answer"]; !ok || got != "你好！" {
				t.Errorf("node_finished Outputs[answer] = %v, want 你好！", got)
			}
		}
	}

	wantTypes := []aibackend.ChunkType{
		aibackend.ChunkTypeWorkflowStarted,
		aibackend.ChunkTypeNodeStarted,
		aibackend.ChunkTypeNodeFinished,
		aibackend.ChunkTypeWorkflowFinished,
		aibackend.ChunkTypeDone,
	}
	if !sameTypes(got, wantTypes) {
		t.Errorf("event types = %v, want %v", got, wantTypes)
	}
	if convID != "c-1" {
		t.Errorf("ConversationID = %q, want c-1", convID)
	}
	if lastWorkflow == nil || lastWorkflow.Status != "succeeded" {
		t.Errorf("workflow_finished workflow = %+v, want Status=succeeded", lastWorkflow)
	}
}

func TestChatStreamUnknownEventsIgnored(t *testing.T) {
	events := []string{
		`{"event":"ping"}`,
		`{"event":"workflow_started","conversation_id":"c-1"}`,
		`{"event":"agent_thought"}`,
		`{"event":"workflow_finished","data":{"status":"succeeded"}}`,
	}
	srv := fakeDifySSE(t, events)
	defer srv.Close()

	b, _ := New(Config{Name: "t", BaseURL: srv.URL, APIKey: "k", Timeout: time.Second, StreamTimeout: 2 * time.Second})
	ch, err := b.ChatStream(context.Background(), aibackend.ChatReq{Query: "hi", User: "u"})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	var seen []aibackend.ChunkType
	for c := range ch {
		seen = append(seen, c.Type)
	}
	want := []aibackend.ChunkType{
		aibackend.ChunkTypeWorkflowStarted,
		aibackend.ChunkTypeWorkflowFinished,
		aibackend.ChunkTypeDone,
	}
	if !sameTypes(seen, want) {
		t.Errorf("seen = %v, want %v", seen, want)
	}
}

func TestChatStreamUpstreamErrorMapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"code":"unauthorized","message":"bad key"}`)
	}))
	defer srv.Close()

	b, _ := New(Config{Name: "t", BaseURL: srv.URL, APIKey: "k", Timeout: time.Second, StreamTimeout: 2 * time.Second})
	ch, err := b.ChatStream(context.Background(), aibackend.ChatReq{Query: "hi", User: "u"})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var sawErr bool
	for c := range ch {
		if c.Type == aibackend.ChunkTypeError {
			sawErr = true
			if !errors.Is(c.Err, aibackend.ErrUnauthorized) {
				t.Errorf("err chunk = %v, want errors.Is ErrUnauthorized", c.Err)
			}
		}
	}
	if !sawErr {
		t.Errorf("expected an Error chunk; saw none")
	}
}

func sameTypes(a, b []aibackend.ChunkType) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
