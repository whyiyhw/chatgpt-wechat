// Package deepseek 是 AIBackend 接口的 DeepSeek 实现，包装 chat/common/deepseek。
//
// 语义差异（与 dify adapter 对比）：
//   - DeepSeek 是"客户端管 context"的 backend：上下文由调用方在 ChatReq.Messages
//     里完整提供。本 adapter 不维护任何对话状态，ConversationID 字段被忽略。
//   - 没有 file_id 概念，UploadFile 直接返回 ErrNotImplemented。
//   - 没有便宜的 Health 探测端点，Health 返回 nil 避免 startup 探测打费用。
package deepseek

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"chat/common/aibackend"
	dsclient "chat/common/deepseek"
)

const TypeName = "deepseek"

type Backend struct {
	name        string
	apiKey      string
	baseURL     string
	model       string
	temperature float32
	timeout     time.Duration
	streamTO    time.Duration
}

// Config 是 DeepSeek backend 的配置，与 PRD §6.2 yaml 形态对齐。
type Config struct {
	Name          string
	BaseURL       string
	APIKey        string
	Model         string // deepseek-chat | deepseek-reasoner；留空走 SDK 默认
	Temperature   float32
	Timeout       time.Duration
	StreamTimeout time.Duration
}

// New 基于强类型 Config 构造实例。
func New(cfg Config) (*Backend, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%w: DeepSeek APIKey is required", aibackend.ErrUnauthorized)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.StreamTimeout <= 0 {
		cfg.StreamTimeout = 300 * time.Second
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = dsclient.DefaultHost
	}
	if cfg.Temperature <= 0 {
		cfg.Temperature = float32(dsclient.Temperature)
	}
	return &Backend{
		name:        cfg.Name,
		apiKey:      cfg.APIKey,
		baseURL:     cfg.BaseURL,
		model:       cfg.Model,
		temperature: cfg.Temperature,
		timeout:     cfg.Timeout,
		streamTO:    cfg.StreamTimeout,
	}, nil
}

// Factory 由 registry 调用；cfg 是 yaml 反序列化后的 map。
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
	if v, ok := cfg["Model"].(string); ok {
		c.Model = v
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
	if v, ok := cfg["Temperature"].(float64); ok {
		c.Temperature = float32(v)
	}
	return New(c)
}

func init() {
	aibackend.Register(TypeName, Factory)
}

func (b *Backend) Name() string { return b.name }
func (b *Backend) Type() string { return TypeName }

// OpenConversation DeepSeek 是无状态的，不维护后端 conversation_id。
func (b *Backend) OpenConversation(_ context.Context, _ aibackend.OpenConversationReq) (aibackend.OpenConversationResp, error) {
	return aibackend.OpenConversationResp{}, nil
}

// Chat 非流式对话。
func (b *Backend) Chat(ctx context.Context, req aibackend.ChatReq) (aibackend.ChatResp, error) {
	messages := b.buildMessages(req)
	if len(messages) == 0 {
		return aibackend.ChatResp{}, fmt.Errorf("%w: DeepSeek requires Messages or Query", aibackend.ErrBadRequest)
	}
	_, cancel := withTimeout(ctx, req.Timeout, b.timeout)
	defer cancel()

	// SDK 当前不接 ctx；timeout 通过 buildConfig 内的 HTTPClient.Timeout 控制。
	// 这里 cancel 仅占位、便于未来 SDK 升级支持 ctx 取消时无侵入接入。
	client := b.newClient(b.timeout)
	text, err := client.Chat(messages)
	if err != nil {
		return aibackend.ChatResp{}, mapError(err)
	}
	return aibackend.ChatResp{Answer: text}, nil
}

// ChatStream 流式对话。
// 上游 SDK 的 ChatStream(messages, channel) 是同步阻塞、内部 close(channel)、
// 出错时 channel close 之前/之后 return error。本方法把它转成 <-chan StreamChunk。
func (b *Backend) ChatStream(ctx context.Context, req aibackend.ChatReq) (<-chan aibackend.StreamChunk, error) {
	messages := b.buildMessages(req)
	if len(messages) == 0 {
		return nil, fmt.Errorf("%w: DeepSeek requires Messages or Query", aibackend.ErrBadRequest)
	}
	streamCtx, cancel := withTimeout(ctx, req.Timeout, b.streamTO)

	out := make(chan aibackend.StreamChunk, 64)
	upstream := make(chan string, 64)
	errCh := make(chan error, 1)

	client := b.newClient(b.streamTO)

	// goroutine 1：跑上游 ChatStream，把错误写 errCh，channel 由 SDK 关闭
	go func() {
		errCh <- client.ChatStream(messages, upstream)
	}()

	// goroutine 2：把上游 string 翻译成 StreamChunk，结束后发 Done/Error
	go func() {
		defer cancel()
		defer close(out)

		for {
			select {
			case <-streamCtx.Done():
				// ctx 取消：不再读 upstream，让 SDK goroutine 自然结束
				return
			case text, ok := <-upstream:
				if !ok {
					// upstream 关闭：SDK 已结束，检查 errCh
					err := <-errCh
					if err != nil {
						select {
						case <-streamCtx.Done():
						case out <- aibackend.StreamChunk{Type: aibackend.ChunkTypeError, Err: mapError(err)}:
						}
						return
					}
					select {
					case <-streamCtx.Done():
					case out <- aibackend.StreamChunk{Type: aibackend.ChunkTypeDone}:
					}
					return
				}
				if text == "" {
					continue
				}
				select {
				case <-streamCtx.Done():
					return
				case out <- aibackend.StreamChunk{Type: aibackend.ChunkTypeText, Text: text}:
				}
			}
		}
	}()

	return out, nil
}

// UploadFile DeepSeek 当前没有文件上传 API。
func (b *Backend) UploadFile(_ context.Context, _ aibackend.UploadFileReq) (aibackend.UploadFileResp, error) {
	return aibackend.UploadFileResp{}, aibackend.ErrNotImplemented
}

// Health 不主动探测（DeepSeek 无便宜端点，每次探测都打费用）。
func (b *Backend) Health(_ context.Context) error {
	return nil
}

// buildMessages 把 ChatReq 转成 SDK 的 []ChatModelMessage。
//
// 优先用 req.Messages；若为空但有 Query，则把 Query 包成单条 user message
// 作为兜底（即"无上下文单轮对话"）。
func (b *Backend) buildMessages(req aibackend.ChatReq) []dsclient.ChatModelMessage {
	if len(req.Messages) > 0 {
		out := make([]dsclient.ChatModelMessage, 0, len(req.Messages))
		for _, m := range req.Messages {
			out = append(out, dsclient.ChatModelMessage{
				Role:    m.Role,
				Content: dsclient.NewChatContent(m.Content),
			})
		}
		return out
	}
	if req.Query != "" {
		return []dsclient.ChatModelMessage{
			{Role: dsclient.UserRole, Content: dsclient.NewChatContent(req.Query)},
		}
	}
	return nil
}

func (b *Backend) newClient(timeout time.Duration) *dsclient.ChatClient {
	c := dsclient.NewChatClient(b.apiKey).
		WithHost(b.baseURL).
		WithTemperature(b.temperature)
	if b.model != "" {
		c = c.WithModel(b.model)
	}
	// SDK 内部用固定 300s HTTPClient.Timeout（buildConfig）；本 adapter 暂不覆盖。
	// 真要细控 timeout 时建议给上游 SDK 加 WithHTTPTimeout 选项；留作后续。
	_ = timeout
	return c
}

func withTimeout(parent context.Context, override, fallback time.Duration) (context.Context, context.CancelFunc) {
	d := override
	if d <= 0 {
		d = fallback
	}
	if d <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, d)
}

// mapError 把 DeepSeek SDK / 网络层错误归一到 aibackend 标准错误。
//
// 上游 SDK 错误是 fmt.Errorf("API error: %s, status code: %d", body, status) +
// errors.Wrap(...) 包装，没 typed error，只能靠字符串/HTTP 状态码 substring 匹配。
// 等给上游加 typed error 再换 errors.As。
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return fmt.Errorf("%w: %v", aibackend.ErrBackendUnavailable, err)
	}
	low := strings.ToLower(err.Error())
	switch {
	case containsAny(low,
		"status code: 401", "status code: 403",
		"unauthorized", "invalid_api_key", "invalid api key"):
		return fmt.Errorf("%w: %v", aibackend.ErrUnauthorized, err)
	case containsAny(low,
		"status code: 429",
		"rate limit", "too many requests"):
		return fmt.Errorf("%w: %v", aibackend.ErrRateLimited, err)
	case containsAny(low, "status code: 400", "bad request", "invalid_param"):
		return fmt.Errorf("%w: %v", aibackend.ErrBadRequest, err)
	case containsAny(low,
		"status code: 5", // 500/502/503/504
		"timeout", "connection refused", "no such host", "eof", "broken pipe"):
		return fmt.Errorf("%w: %v", aibackend.ErrBackendUnavailable, err)
	}
	return err
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

