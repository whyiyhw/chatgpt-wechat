// Package dify 是 AIBackend 接口的 Dify 实现，包装现有 chat/common/dify 客户端。
//
// 设计要点见 doc/PRD-refactor.md §6.3。
package dify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"chat/common/aibackend"
	difyclient "chat/common/dify"
)

const TypeName = "dify"

// healthProbeUser 是 Health() 探测时透传给 Dify 的 user 字段。
// Dify 强制要求 user 非空；这个值不应与真实业务用户冲突。
const healthProbeUser = "__aibackend_health_probe__"

// Backend 实现 aibackend.AIBackend。
type Backend struct {
	name       string
	apiKey     string
	baseURL    string
	timeout    time.Duration
	streamTO   time.Duration
	difyClient *difyclient.Client
}

// Config 是 Dify Backend 的配置，与 PRD §6.2 中的 yaml 形态对齐。
type Config struct {
	Name          string        `mapstructure:"Name"`
	BaseURL       string        `mapstructure:"BaseURL"`
	APIKey        string        `mapstructure:"APIKey"`
	Timeout       time.Duration `mapstructure:"Timeout"`
	StreamTimeout time.Duration `mapstructure:"StreamTimeout"`
}

// New 直接基于强类型 Config 构造实例（便于单元测试与显式注入）。
func New(cfg Config) (*Backend, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("%w: Dify BaseURL is required", aibackend.ErrBadRequest)
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%w: Dify APIKey is required", aibackend.ErrUnauthorized)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.StreamTimeout <= 0 {
		cfg.StreamTimeout = 120 * time.Second
	}

	client := difyclient.NewClientWithConfig(&difyclient.ClientConfig{
		Host:             cfg.BaseURL,
		DefaultAPISecret: cfg.APIKey,
		Timeout:          cfg.Timeout,
	})

	return &Backend{
		name:       cfg.Name,
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		timeout:    cfg.Timeout,
		streamTO:   cfg.StreamTimeout,
		difyClient: client,
	}, nil
}

// Factory 由 registry 调用，cfg 来自 yaml 反序列化后的 map。
func Factory(cfg map[string]any) (aibackend.AIBackend, error) {
	c := Config{}
	if v, ok := cfg["Name"].(string); ok {
		c.Name = v
	}
	if v, ok := cfg["BaseURL"].(string); ok {
		c.BaseURL = v
	}
	if v, ok := cfg["APIKey"].(string); ok {
		c.APIKey = v
	}
	if v, ok := cfg["Timeout"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			c.Timeout = d
		}
	}
	if v, ok := cfg["StreamTimeout"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			c.StreamTimeout = d
		}
	}
	return New(c)
}

func init() {
	aibackend.Register(TypeName, Factory)
}

// Name 返回实例名。
func (b *Backend) Name() string { return b.name }

// Type 返回后端类型标识。
func (b *Backend) Type() string { return TypeName }

// OpenConversation 对 Dify 是 no-op：Dify 在 ChatStream 首条 chunk 中返回 conversation_id。
func (b *Backend) OpenConversation(ctx context.Context, req aibackend.OpenConversationReq) (aibackend.OpenConversationResp, error) {
	return aibackend.OpenConversationResp{}, nil
}

// Chat 非流式对话。
func (b *Backend) Chat(ctx context.Context, req aibackend.ChatReq) (aibackend.ChatResp, error) {
	ctx, cancel := b.withTimeout(ctx, req.Timeout, b.timeout)
	defer cancel()

	resp, err := b.difyClient.API().ChatMessages(ctx, &difyclient.ChatMessageRequest{
		Inputs:         coalesceInputs(req.Inputs),
		Query:          req.Query,
		ConversationID: req.ConversationID,
		User:           req.User,
	})
	if err != nil {
		return aibackend.ChatResp{}, mapError(err)
	}
	return aibackend.ChatResp{
		MessageID:      resp.ID,
		ConversationID: resp.ConversationID,
		Answer:         resp.Answer,
	}, nil
}

// ChatStream 流式对话；走 Dify 的 chat-messages SSE 端点，支持普通 Chat App
// 与 Chatflow（workflow_started / node_finished / workflow_finished 事件）。
//
// 返回的 channel 由本方法负责关闭。ctx 取消会通过 streamCtx 中断上游 SSE 读取。
func (b *Backend) ChatStream(ctx context.Context, req aibackend.ChatReq) (<-chan aibackend.StreamChunk, error) {
	streamCtx, cancel := b.withTimeout(ctx, req.Timeout, b.streamTO)

	out := make(chan aibackend.StreamChunk, 64)

	// convID / msgID 通过 closure 在事件间累积；首条 workflow_started 携带 conversation_id 后，
	// 后续所有 chunk 都把它带上，方便消费方在任意时刻拿到。
	var convID, msgID string
	handler := &difyclient.DefaultEventHandler{
		StreamHandler: func(resp difyclient.StreamingResponse) {
			if resp.ConversationID != "" {
				convID = resp.ConversationID
			}
			if resp.MessageID != "" {
				msgID = resp.MessageID
			}

			chunk := aibackend.StreamChunk{
				ConversationID: convID,
				MessageID:      msgID,
				TaskID:         resp.TaskID,
			}
			switch resp.Event {
			case difyclient.EventWorkflowStarted:
				chunk.Type = aibackend.ChunkTypeWorkflowStarted
				chunk.Workflow = &aibackend.WorkflowEvent{RunID: resp.WorkflowRunID}
			case difyclient.EventNodeStarted:
				chunk.Type = aibackend.ChunkTypeNodeStarted
				chunk.Workflow = &aibackend.WorkflowEvent{
					RunID:    resp.WorkflowRunID,
					NodeID:   resp.Data.NodeID,
					NodeType: resp.Data.NodeType,
					Title:    resp.Data.Title,
					Inputs:   resp.Data.Inputs,
				}
			case difyclient.EventNodeFinished:
				chunk.Type = aibackend.ChunkTypeNodeFinished
				chunk.Workflow = &aibackend.WorkflowEvent{
					RunID:       resp.WorkflowRunID,
					NodeID:      resp.Data.NodeID,
					NodeType:    resp.Data.NodeType,
					Title:       resp.Data.Title,
					Status:      resp.Data.Status,
					Outputs:     resp.Data.Outputs,
					Error:       resp.Data.Error,
					ElapsedTime: resp.Data.ElapsedTime,
					TotalTokens: resp.Data.ExecutionMetadata.TotalTokens,
				}
			case difyclient.EventWorkflowFinished:
				chunk.Type = aibackend.ChunkTypeWorkflowFinished
				chunk.Workflow = &aibackend.WorkflowEvent{
					RunID:       resp.WorkflowRunID,
					Status:      resp.Data.Status,
					Outputs:     resp.Data.Outputs,
					Error:       resp.Data.Error,
					ElapsedTime: resp.Data.ElapsedTime,
					TotalTokens: resp.Data.ExecutionMetadata.TotalTokens,
				}
				if tt := resp.Data.ExecutionMetadata.TotalTokens; tt > 0 {
					chunk.Usage = &aibackend.Usage{TotalTokens: tt}
				}
			case difyclient.EventMessage:
				chunk.Type = aibackend.ChunkTypeMessage
			case difyclient.EventMessageEnd:
				chunk.Type = aibackend.ChunkTypeMessageEnd
			default:
				// 未识别事件（如 ping、agent_thought 等）暂不透传，避免无谓的 chunk 噪声。
				return
			}

			select {
			case <-streamCtx.Done():
			case out <- chunk:
			}
		},
	}

	go func() {
		defer cancel()
		defer close(out)

		wfReq := difyclient.WorkflowRequest{
			Query:          req.Query,
			User:           req.User,
			ConversationId: req.ConversationID,
			Inputs:         coalesceInputs(req.Inputs),
			ResponseMode:   "streaming",
			Files:          mapFileRefsToDify(req.Files),
		}

		err := b.difyClient.API().RunStreamWorkflowWithHandler(streamCtx, wfReq, handler)
		if err != nil {
			select {
			case <-streamCtx.Done():
			case out <- aibackend.StreamChunk{
				Type:           aibackend.ChunkTypeError,
				Err:            mapError(err),
				ConversationID: convID,
				MessageID:      msgID,
			}:
			}
			return
		}

		select {
		case <-streamCtx.Done():
		case out <- aibackend.StreamChunk{
			Type:           aibackend.ChunkTypeDone,
			ConversationID: convID,
			MessageID:      msgID,
		}:
		}
	}()
	return out, nil
}

// UploadFile 上传文件/图片到 Dify，返回 file_id（Dify 不直接返回可访问 URL）。
//
// 调用方拿到 file_id 后，在后续 ChatStream 调用里把 FileRef.UploadFileID 填上
// 即可让 Dify 关联到该消息。
func (b *Backend) UploadFile(ctx context.Context, req aibackend.UploadFileReq) (aibackend.UploadFileResp, error) {
	if req.User == "" {
		return aibackend.UploadFileResp{}, fmt.Errorf("%w: User is required for Dify file upload", aibackend.ErrBadRequest)
	}
	if len(req.Content) == 0 {
		return aibackend.UploadFileResp{}, fmt.Errorf("%w: empty file content", aibackend.ErrBadRequest)
	}

	ctx, cancel := b.withTimeout(ctx, 0, b.timeout)
	defer cancel()

	out, err := b.difyClient.API().FilesUpload(ctx, req.User, req.Name, req.MIME, req.Content)
	if err != nil {
		return aibackend.UploadFileResp{}, mapError(err)
	}
	return aibackend.UploadFileResp{FileID: out.ID}, nil
}

// Health 调用 Dify /v1/parameters 作为活性探测；该端点鉴权 + 轻量。
func (b *Backend) Health(ctx context.Context) error {
	ctx, cancel := b.withTimeout(ctx, 0, 5*time.Second)
	defer cancel()
	_, err := b.difyClient.API().Parameters(ctx, &difyclient.ParametersRequest{User: healthProbeUser})
	if err != nil {
		return mapError(err)
	}
	return nil
}

func (b *Backend) withTimeout(parent context.Context, override, fallback time.Duration) (context.Context, context.CancelFunc) {
	d := override
	if d <= 0 {
		d = fallback
	}
	if d <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, d)
}

func coalesceInputs(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return map[string]interface{}{}
	}
	return in
}

// mapFileRefsToDify 把 aibackend.FileRef 列表转成 dify.FileInput。
//
// 缺省策略：
//   - Type 留空 → "image"（Dify FileInput.Type 注释明示当前仅支持 "image"）
//   - TransferMethod 留空 → 有 UploadFileID 用 "local_file"，有 URL 用 "remote_url"
//
// 调用方可显式覆盖任一字段；本函数只做"合理默认"。
func mapFileRefsToDify(refs []aibackend.FileRef) []difyclient.FileInput {
	if len(refs) == 0 {
		return nil
	}
	out := make([]difyclient.FileInput, 0, len(refs))
	for _, r := range refs {
		fi := difyclient.FileInput{
			Type:           r.Type,
			TransferMethod: r.TransferMethod,
			URL:            r.URL,
			UploadFileID:   r.UploadFileID,
		}
		if fi.Type == "" {
			fi.Type = "image"
		}
		if fi.TransferMethod == "" {
			switch {
			case fi.UploadFileID != "":
				fi.TransferMethod = "local_file"
			case fi.URL != "":
				fi.TransferMethod = "remote_url"
			}
		}
		out = append(out, fi)
	}
	return out
}

// mapError 把 Dify client / HTTP / 网络层错误归一到 aibackend 标准错误。
//
// 优先用 typed *dify.APIError 走 errors.As 精确分类（HTTP status + Dify code），
// 网络层错误（DNS / refused / timeout / EOF）退化为字符串匹配。
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return fmt.Errorf("%w: %v", aibackend.ErrBackendUnavailable, err)
	}

	// 1) typed APIError：HTTP status + Dify business code 都可靠
	var apiErr *difyclient.APIError
	if errors.As(err, &apiErr) {
		if mapped := mapAPIError(apiErr); mapped != nil {
			return fmt.Errorf("%w: %v", mapped, err)
		}
		return err
	}

	// 2) 网络/IO 层错误：只能字符串匹配
	low := strings.ToLower(err.Error())
	switch {
	case strings.Contains(low, "timeout"),
		strings.Contains(low, "connection refused"),
		strings.Contains(low, "no such host"),
		strings.Contains(low, "eof"),
		strings.Contains(low, "broken pipe"),
		strings.Contains(low, "reset by peer"):
		return fmt.Errorf("%w: %v", aibackend.ErrBackendUnavailable, err)
	}
	return err
}

// mapAPIError 把 *dify.APIError 的 HTTP status + Dify code 映射到标准错误。
// 返回 nil 表示无明确归类（调用方按原样返回即可）。
func mapAPIError(e *difyclient.APIError) error {
	// 业务码优先（更精确）
	switch strings.ToLower(e.Code) {
	case "invalid_api_key", "unauthorized":
		return aibackend.ErrUnauthorized
	case "rate_limit", "rate_limit_exceeded", "too_many_requests":
		return aibackend.ErrRateLimited
	case "conversation_not_found", "conversation_not_exists", "not_chat_app":
		return aibackend.ErrConversationNotFound
	case "invalid_param", "bad_request":
		return aibackend.ErrBadRequest
	}
	// 业务码留空时按 HTTP status
	switch {
	case e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden:
		return aibackend.ErrUnauthorized
	case e.Status == http.StatusTooManyRequests:
		return aibackend.ErrRateLimited
	case e.Status == http.StatusBadRequest:
		return aibackend.ErrBadRequest
	case e.Status >= 500:
		return aibackend.ErrBackendUnavailable
	}
	return nil
}
