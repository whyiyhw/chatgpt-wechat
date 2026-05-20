package dify

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
	difyclient "chat/common/dify"
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

func TestUploadFileValidation(t *testing.T) {
	b := mustBackend(t)
	_, err := b.UploadFile(context.Background(), aibackend.UploadFileReq{Name: "x.png", Content: []byte{1}})
	if !errors.Is(err, aibackend.ErrBadRequest) {
		t.Errorf("missing user: err = %v, want ErrBadRequest", err)
	}
	_, err = b.UploadFile(context.Background(), aibackend.UploadFileReq{User: "u", Name: "x.png"})
	if !errors.Is(err, aibackend.ErrBadRequest) {
		t.Errorf("empty content: err = %v, want ErrBadRequest", err)
	}
}

func TestUploadFileHappyPath(t *testing.T) {
	var captured struct {
		hasFilePart bool
		fileBytes   []byte
		fileMIME    string
		userField   string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/files/upload" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		captured.userField = r.FormValue("user")
		if files := r.MultipartForm.File["file"]; len(files) > 0 {
			captured.hasFilePart = true
			captured.fileMIME = files[0].Header.Get("Content-Type")
			if fh, err := files[0].Open(); err == nil {
				captured.fileBytes, _ = io.ReadAll(fh)
				fh.Close()
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"file-abc","name":"avatar.png","size":11,"extension":"png","mime_type":"image/png","created_by":"u","created_at":1700000000}`)
	}))
	defer srv.Close()

	b, _ := New(Config{Name: "t", BaseURL: srv.URL, APIKey: "k", Timeout: 5 * time.Second})
	resp, err := b.UploadFile(context.Background(), aibackend.UploadFileReq{
		User:    "user-1",
		Name:    "avatar.png",
		MIME:    "image/png",
		Content: []byte("hello-world"),
	})
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if resp.FileID != "file-abc" {
		t.Errorf("FileID = %q, want file-abc", resp.FileID)
	}
	if !captured.hasFilePart {
		t.Errorf("server did not see file part")
	}
	if got := string(captured.fileBytes); got != "hello-world" {
		t.Errorf("file content = %q, want hello-world", got)
	}
	if captured.fileMIME != "image/png" {
		t.Errorf("file Content-Type = %q, want image/png", captured.fileMIME)
	}
	if captured.userField != "user-1" {
		t.Errorf("user field = %q, want user-1", captured.userField)
	}
}

func TestUploadFileUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"code":"invalid_api_key","message":"bad key","status":401}`)
	}))
	defer srv.Close()

	b, _ := New(Config{Name: "t", BaseURL: srv.URL, APIKey: "k", Timeout: 2 * time.Second})
	_, err := b.UploadFile(context.Background(), aibackend.UploadFileReq{
		User: "u", Name: "x.png", Content: []byte{1, 2, 3},
	})
	if !errors.Is(err, aibackend.ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}

func TestChatStreamPassesFiles(t *testing.T) {
	var seenFiles []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if files, ok := body["files"].([]any); ok {
			for _, f := range files {
				if fm, ok := f.(map[string]any); ok {
					seenFiles = append(seenFiles, fm)
				}
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `data: {"event":"workflow_started","conversation_id":"c-1"}`+"\n")
		io.WriteString(w, `data: {"event":"workflow_finished","data":{"status":"succeeded"}}`+"\n")
	}))
	defer srv.Close()

	b, _ := New(Config{Name: "t", BaseURL: srv.URL, APIKey: "k", Timeout: 2 * time.Second, StreamTimeout: 2 * time.Second})
	ch, err := b.ChatStream(context.Background(), aibackend.ChatReq{
		Query: "describe", User: "u",
		Files: []aibackend.FileRef{
			{UploadFileID: "file-xyz"},                                            // 默认 type=image / method=local_file
			{Type: "image", TransferMethod: "remote_url", URL: "https://x/y.png"}, // 显式
		},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	for range ch {
	}

	if len(seenFiles) != 2 {
		t.Fatalf("server saw %d files, want 2 (raw=%v)", len(seenFiles), seenFiles)
	}
	if seenFiles[0]["type"] != "image" || seenFiles[0]["transfer_method"] != "local_file" || seenFiles[0]["upload_file_id"] != "file-xyz" {
		t.Errorf("file[0] = %v, want default image/local_file", seenFiles[0])
	}
	if seenFiles[1]["type"] != "image" || seenFiles[1]["transfer_method"] != "remote_url" || seenFiles[1]["url"] != "https://x/y.png" {
		t.Errorf("file[1] = %v, want image/remote_url", seenFiles[1])
	}
}


func TestMapError(t *testing.T) {
	apiErr := func(status int, code, msg string) error {
		return &difyclient.APIError{Status: status, Code: code, Message: msg}
	}
	cases := []struct {
		name string
		in   error
		want error
	}{
		// 边界 / ctx
		{"nil", nil, nil},
		{"ctx-deadline", context.DeadlineExceeded, aibackend.ErrBackendUnavailable},
		{"ctx-canceled", context.Canceled, aibackend.ErrBackendUnavailable},

		// typed APIError：业务码优先
		{"code-invalid-api-key", apiErr(401, "invalid_api_key", "bad key"), aibackend.ErrUnauthorized},
		{"code-rate-limit", apiErr(429, "rate_limit_exceeded", "slow down"), aibackend.ErrRateLimited},
		{"code-conv-not-found", apiErr(404, "conversation_not_found", "no convo"), aibackend.ErrConversationNotFound},
		{"code-invalid-param", apiErr(400, "invalid_param", "missing"), aibackend.ErrBadRequest},

		// typed APIError：仅有 HTTP status（业务码留空）
		{"status-401", apiErr(401, "", "Unauthorized"), aibackend.ErrUnauthorized},
		{"status-403", apiErr(403, "", "Forbidden"), aibackend.ErrUnauthorized},
		{"status-429", apiErr(429, "", "Too Many Requests"), aibackend.ErrRateLimited},
		{"status-400", apiErr(400, "", "Bad Request"), aibackend.ErrBadRequest},
		{"status-500", apiErr(500, "", "Internal Server Error"), aibackend.ErrBackendUnavailable},
		{"status-502", apiErr(502, "", "Bad Gateway"), aibackend.ErrBackendUnavailable},

		// typed APIError：包了一层（确保 errors.As 仍能拆出来）
		{"wrapped-apierr", fmt.Errorf("upstream: %w", apiErr(401, "invalid_api_key", "x")), aibackend.ErrUnauthorized},

		// 网络/IO 层错误：字符串匹配兜底
		{"timeout", fmt.Errorf("dial tcp: i/o timeout"), aibackend.ErrBackendUnavailable},
		{"no-host", fmt.Errorf("dial tcp: lookup x: no such host"), aibackend.ErrBackendUnavailable},
		{"refused", fmt.Errorf("dial tcp: connection refused"), aibackend.ErrBackendUnavailable},
		{"eof", fmt.Errorf("read: EOF"), aibackend.ErrBackendUnavailable},

		// 未识别错误：原样返回
		{"unrecognized-passthrough", fmt.Errorf("weird"), nil},
		// typed APIError 但无业务码 + 不在 HTTP status 映射范围（如 418）：原样返回
		{"status-418-passthrough", apiErr(418, "", "I'm a teapot"), nil},
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

func TestChatHappyPath(t *testing.T) {
	var captured struct {
		path         string
		responseMode string
		query        string
		conv         string
		user         string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path = r.URL.Path
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		captured.responseMode, _ = body["response_mode"].(string)
		captured.query, _ = body["query"].(string)
		captured.conv, _ = body["conversation_id"].(string)
		captured.user, _ = body["user"].(string)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"m-7","answer":"你好！","conversation_id":"c-9","created_at":1700000000}`)
	}))
	defer srv.Close()

	b, _ := New(Config{Name: "t", BaseURL: srv.URL, APIKey: "k", Timeout: 5 * time.Second})
	resp, err := b.Chat(context.Background(), aibackend.ChatReq{
		Query: "你好", User: "u-1", ConversationID: "c-prev",
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Answer != "你好！" {
		t.Errorf("Answer = %q, want 你好！", resp.Answer)
	}
	if resp.MessageID != "m-7" || resp.ConversationID != "c-9" {
		t.Errorf("got MessageID=%q ConversationID=%q, want m-7/c-9", resp.MessageID, resp.ConversationID)
	}
	if captured.path != "/v1/chat-messages" {
		t.Errorf("path = %q, want /v1/chat-messages", captured.path)
	}
	if captured.responseMode != "blocking" {
		t.Errorf("response_mode = %q, want blocking", captured.responseMode)
	}
	if captured.query != "你好" || captured.user != "u-1" || captured.conv != "c-prev" {
		t.Errorf("request body fields wrong: %+v", captured)
	}
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
