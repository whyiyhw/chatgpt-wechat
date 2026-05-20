// Package gemini 是 AIBackend 接口的 Google Gemini 实现，包装 chat/common/gemini。
//
// 语义同 DeepSeek/OpenAI："客户端管 context"，上下文由 ChatReq.Messages 提供。
//
// 角色映射差异：Gemini 用 "user" / "model"（不是 "user" / "assistant"），
// buildMessages 在转换时把 "assistant" → "model"。
//
// 错误识别：上游 SDK 没有 typed error，错误信息是裸 errors.New(message) 包装的
// 字符串，mapError 只能靠 substring 匹配 Google API 的常见错误片段。
package gemini

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"chat/common/aibackend"
	gclient "chat/common/gemini"
)

const TypeName = "gemini"

type Backend struct {
	name        string
	apiKey      string
	baseURL     string
	model       string
	temperature float32
	timeout     time.Duration
	streamTO    time.Duration
}

type Config struct {
	Name          string
	BaseURL       string // 留空走 SDK 默认 generativelanguage.googleapis.com
	APIKey        string
	Model         string // gemini-pro / gemini-1.5-pro / gemini-1.5-flash 等
	Temperature   float32
	Timeout       time.Duration
	StreamTimeout time.Duration
}

func New(cfg Config) (*Backend, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%w: Gemini APIKey is required", aibackend.ErrUnauthorized)
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = gclient.DefaultHost
	}
	if cfg.Model == "" {
		cfg.Model = gclient.ChatModel
	}
	if cfg.Temperature <= 0 {
		cfg.Temperature = float32(gclient.Temperature)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.StreamTimeout <= 0 {
		cfg.StreamTimeout = 300 * time.Second
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
	if v, ok := cfg["Temperature"].(float64); ok {
		c.Temperature = float32(v)
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

func (b *Backend) Name() string { return b.name }
func (b *Backend) Type() string { return TypeName }

func (b *Backend) OpenConversation(_ context.Context, _ aibackend.OpenConversationReq) (aibackend.OpenConversationResp, error) {
	return aibackend.OpenConversationResp{}, nil
}

// Chat 非流式对话。
func (b *Backend) Chat(ctx context.Context, req aibackend.ChatReq) (aibackend.ChatResp, error) {
	messages := b.buildMessages(req)
	if len(messages) == 0 {
		return aibackend.ChatResp{}, fmt.Errorf("%w: Gemini requires Messages or Query", aibackend.ErrBadRequest)
	}
	_, cancel := withTimeout(ctx, req.Timeout, b.timeout)
	defer cancel()

	client := b.newClient()
	text, err := client.Chat(messages)
	if err != nil {
		return aibackend.ChatResp{}, mapError(err)
	}
	return aibackend.ChatResp{Answer: text}, nil
}

// ChatStream 流式对话。
//
// 上游 SDK 的 ChatStream(messages, channel) 返回 (text, error)；channel 由 SDK
// 内部关闭，跟 DeepSeek 模式一致。本方法把 chan string 翻译成 <-chan StreamChunk。
func (b *Backend) ChatStream(ctx context.Context, req aibackend.ChatReq) (<-chan aibackend.StreamChunk, error) {
	messages := b.buildMessages(req)
	if len(messages) == 0 {
		return nil, fmt.Errorf("%w: Gemini requires Messages or Query", aibackend.ErrBadRequest)
	}
	streamCtx, cancel := withTimeout(ctx, req.Timeout, b.streamTO)

	out := make(chan aibackend.StreamChunk, 64)
	upstream := make(chan string, 64)
	errCh := make(chan error, 1)

	client := b.newClient()

	// goroutine 1：跑上游 ChatStream，错误写 errCh，channel 由 SDK 关闭
	go func() {
		_, err := client.ChatStream(messages, upstream)
		errCh <- err
	}()

	// goroutine 2：把 string 翻译成 StreamChunk，结束后发 Done/Error
	go func() {
		defer cancel()
		defer close(out)

		for {
			select {
			case <-streamCtx.Done():
				return
			case text, ok := <-upstream:
				if !ok {
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

func (b *Backend) UploadFile(_ context.Context, _ aibackend.UploadFileReq) (aibackend.UploadFileResp, error) {
	return aibackend.UploadFileResp{}, aibackend.ErrNotImplemented
}

// Health 不主动探测，避免每次健康检查打 LLM 请求费用。
func (b *Backend) Health(_ context.Context) error {
	return nil
}

// buildMessages 把 ChatReq.Messages 转成 SDK 的 []ChatModelMessage。
//
// Gemini 角色 = "user" / "model"。本函数把 "assistant" 也映射到 "model"，
// "system" 暂保留——SDK commonChat 会把非 user 的统一改成 model。
func (b *Backend) buildMessages(req aibackend.ChatReq) []gclient.ChatModelMessage {
	if len(req.Messages) > 0 {
		out := make([]gclient.ChatModelMessage, 0, len(req.Messages))
		for _, m := range req.Messages {
			role := m.Role
			if role == "assistant" {
				role = gclient.ModelRole
			}
			out = append(out, gclient.ChatModelMessage{
				Role:    role,
				Content: gclient.NewChatContent(m.Content),
			})
		}
		return out
	}
	if req.Query != "" {
		return []gclient.ChatModelMessage{
			{Role: gclient.UserRole, Content: gclient.NewChatContent(req.Query)},
		}
	}
	return nil
}

func (b *Backend) newClient() *gclient.ChatClient {
	return gclient.NewChatClient(b.apiKey).
		WithHost(b.baseURL).
		WithModel(b.model).
		WithTemperature(b.temperature)
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

// mapError 把 Gemini SDK 错误归一到 aibackend 标准错误。
//
// 上游 SDK 错误是 errors.New(result.Message) 包出来的裸字符串（Google API
// 错误体一般是 {"error":{"code":N,"message":"...","status":"INVALID_ARGUMENT"}}
// 这种），没有 typed error 可 errors.As，只能 substring 匹配。给上游加 typed
// error 后再换 errors.As。
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
		"api key not valid", "api_key_invalid", "invalid api key",
		"permission_denied", "unauthenticated"):
		return fmt.Errorf("%w: %v", aibackend.ErrUnauthorized, err)
	case containsAny(low,
		"resource_exhausted", "quota", "rate limit", "too many requests"):
		return fmt.Errorf("%w: %v", aibackend.ErrRateLimited, err)
	case containsAny(low,
		"invalid_argument", "invalid argument",
		"context length", "too long",
		"failed_precondition"):
		return fmt.Errorf("%w: %v", aibackend.ErrBadRequest, err)
	case containsAny(low,
		"internal", "unavailable", "deadline_exceeded",
		"timeout", "connection refused", "no such host",
		"eof", "broken pipe", "reset by peer"):
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
