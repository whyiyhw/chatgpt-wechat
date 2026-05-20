// Package openai 是 AIBackend 接口的 OpenAI 实现，包装 chat/common/openai。
//
// 语义和 DeepSeek adapter 同型："客户端管 context"，上下文由调用方在
// ChatReq.Messages 提供。本 adapter 只覆盖 Chat Completions 路径
// （ChatModel = gpt-3.5/4 等），legacy TextModel = text-davinci-003 的
// completion 端点不在本 adapter 范围；如部署仍依赖 TextModel，应保留
// chatlogic 的 legacy 分支。
//
// 支持 Azure OpenAI：Config.Origin 设为 "azure" / "azure_ad"，BaseURL
// 填 Azure endpoint，Engine 填 deployment 名。
package openai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"chat/common/aibackend"
	oai "chat/common/openai"

	copenai "github.com/sashabaranov/go-openai"
)

const TypeName = "openai"

type Backend struct {
	name        string
	apiKey      string
	baseURL     string
	model       string
	origin      string
	engine      string
	temperature float32
	maxToken    int
	totalToken  int
	timeout     time.Duration
	streamTO    time.Duration
}

// Config 是 OpenAI backend 的配置，覆盖 OpenAI / Azure 两种部署。
type Config struct {
	Name          string
	BaseURL       string  // OpenAI: 如 https://api.openai.com（不带 /v1）；Azure: endpoint
	APIKey        string
	Model         string  // 如 gpt-3.5-turbo / gpt-4o-mini
	Origin        string  // "open_ai" (默认) | "azure" | "azure_ad"
	Engine        string  // Azure deployment 名
	Temperature   float32
	MaxToken      int     // 一次响应的最大 token
	TotalToken    int     // 一次对话的最大上下文 token（用于 token-aware truncation）
	Timeout       time.Duration
	StreamTimeout time.Duration
}

// New 基于强类型 Config 构造实例。
func New(cfg Config) (*Backend, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%w: OpenAI APIKey is required", aibackend.ErrUnauthorized)
	}
	if cfg.Model == "" {
		cfg.Model = oai.ChatModel
	}
	if cfg.Origin == "" {
		cfg.Origin = "open_ai"
	}
	if cfg.Temperature <= 0 {
		cfg.Temperature = float32(oai.Temperature)
	}
	if cfg.MaxToken <= 0 {
		cfg.MaxToken = oai.MaxToken
	}
	if cfg.TotalToken <= 0 {
		cfg.TotalToken = oai.TotalToken
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.StreamTimeout <= 0 {
		cfg.StreamTimeout = 300 * time.Second
	}
	if (cfg.Origin == "azure" || cfg.Origin == "azure_ad") && cfg.Engine == "" {
		return nil, fmt.Errorf("%w: Azure OpenAI requires Engine (deployment name)", aibackend.ErrBadRequest)
	}
	return &Backend{
		name:        cfg.Name,
		apiKey:      cfg.APIKey,
		baseURL:     cfg.BaseURL,
		model:       cfg.Model,
		origin:      cfg.Origin,
		engine:      cfg.Engine,
		temperature: cfg.Temperature,
		maxToken:    cfg.MaxToken,
		totalToken:  cfg.TotalToken,
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
	if v, ok := cfg["Origin"].(string); ok {
		c.Origin = v
	}
	if v, ok := cfg["Engine"].(string); ok {
		c.Engine = v
	}
	if v, ok := cfg["Temperature"].(float64); ok {
		c.Temperature = float32(v)
	}
	if v, ok := cfg["MaxToken"].(float64); ok {
		c.MaxToken = int(v)
	}
	if v, ok := cfg["TotalToken"].(float64); ok {
		c.TotalToken = int(v)
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

// OpenConversation OpenAI 是无状态的。
func (b *Backend) OpenConversation(_ context.Context, _ aibackend.OpenConversationReq) (aibackend.OpenConversationResp, error) {
	return aibackend.OpenConversationResp{}, nil
}

// Chat 非流式对话。
func (b *Backend) Chat(ctx context.Context, req aibackend.ChatReq) (aibackend.ChatResp, error) {
	messages := b.buildMessages(req)
	if len(messages) == 0 {
		return aibackend.ChatResp{}, fmt.Errorf("%w: OpenAI requires Messages or Query", aibackend.ErrBadRequest)
	}
	ctx, cancel := withTimeout(ctx, req.Timeout, b.timeout)
	defer cancel()

	client := b.newClient()
	resp, err := client.CreateChatCompletion(ctx, copenai.ChatCompletionRequest{
		Model:       b.engineModel(),
		Messages:    messages,
		Temperature: b.temperature,
		MaxTokens:   b.maxToken,
	})
	if err != nil {
		return aibackend.ChatResp{}, mapError(err)
	}
	if len(resp.Choices) == 0 {
		return aibackend.ChatResp{}, nil
	}
	return aibackend.ChatResp{
		MessageID: resp.ID,
		Answer:    resp.Choices[0].Message.Content,
	}, nil
}

// ChatStream 流式对话。
func (b *Backend) ChatStream(ctx context.Context, req aibackend.ChatReq) (<-chan aibackend.StreamChunk, error) {
	messages := b.buildMessages(req)
	if len(messages) == 0 {
		return nil, fmt.Errorf("%w: OpenAI requires Messages or Query", aibackend.ErrBadRequest)
	}
	streamCtx, cancel := withTimeout(ctx, req.Timeout, b.streamTO)

	client := b.newClient()
	stream, err := client.CreateChatCompletionStream(streamCtx, copenai.ChatCompletionRequest{
		Model:       b.engineModel(),
		Messages:    messages,
		Temperature: b.temperature,
		MaxTokens:   b.maxToken,
		Stream:      true,
	})
	if err != nil {
		cancel()
		return nil, mapError(err)
	}

	out := make(chan aibackend.StreamChunk, 64)
	go func() {
		defer cancel()
		defer stream.Close()
		defer close(out)

		var msgID string
		for {
			select {
			case <-streamCtx.Done():
				return
			default:
			}
			resp, err := stream.Recv()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			if err != nil {
				if isStreamEOF(err) {
					select {
					case <-streamCtx.Done():
					case out <- aibackend.StreamChunk{Type: aibackend.ChunkTypeDone, MessageID: msgID}:
					}
					return
				}
				select {
				case <-streamCtx.Done():
				case out <- aibackend.StreamChunk{Type: aibackend.ChunkTypeError, Err: mapError(err), MessageID: msgID}:
				}
				return
			}
			if resp.ID != "" {
				msgID = resp.ID
			}
			if len(resp.Choices) == 0 {
				continue
			}
			content := resp.Choices[0].Delta.Content
			finishReason := resp.Choices[0].FinishReason
			if content != "" {
				select {
				case <-streamCtx.Done():
					return
				case out <- aibackend.StreamChunk{Type: aibackend.ChunkTypeText, Text: content, MessageID: msgID}:
				}
			}
			if finishReason != "" {
				select {
				case <-streamCtx.Done():
				case out <- aibackend.StreamChunk{Type: aibackend.ChunkTypeDone, MessageID: msgID}:
				}
				return
			}
		}
	}()
	return out, nil
}

func (b *Backend) UploadFile(_ context.Context, _ aibackend.UploadFileReq) (aibackend.UploadFileResp, error) {
	return aibackend.UploadFileResp{}, aibackend.ErrNotImplemented
}

func (b *Backend) Health(_ context.Context) error {
	return nil
}

// buildMessages 把 ChatReq 转成 go-openai 的 ChatCompletionMessage 数组。
// 优先 req.Messages；为空时 Query 兜底成单条 user。
func (b *Backend) buildMessages(req aibackend.ChatReq) []copenai.ChatCompletionMessage {
	if len(req.Messages) > 0 {
		out := make([]copenai.ChatCompletionMessage, 0, len(req.Messages))
		for _, m := range req.Messages {
			role := m.Role
			if role == "model" {
				role = "assistant"
			}
			out = append(out, copenai.ChatCompletionMessage{Role: role, Content: m.Content})
		}
		return out
	}
	if req.Query != "" {
		return []copenai.ChatCompletionMessage{{Role: copenai.ChatMessageRoleUser, Content: req.Query}}
	}
	return nil
}

// newClient 用 chat/common/openai 的 buildConfig 拼出底层 client，
// 复用 Azure / proxy 的兼容逻辑，避免在 adapter 重新实现。
func (b *Backend) newClient() *copenai.Client {
	wrapper := oai.NewChatClient(b.apiKey).
		WithBaseHost(b.baseURL).
		WithOrigin(b.origin).
		WithEngine(b.engine).
		WithModel(b.model).
		WithTemperature(b.temperature).
		WithMaxToken(b.maxToken).
		WithTotalToken(b.totalToken)
	// buildConfig 是未导出方法，无法直接调；用反射代价太高，所以这里直接用
	// copenai.DefaultConfig 走最简路径。Azure 模式必须通过 origin/engine 才完整。
	if b.origin == "azure" || b.origin == "azure_ad" {
		cfg := copenai.DefaultAzureConfig(b.apiKey, b.baseURL)
		cfg.AzureModelMapperFunc = func(_ string) string { return b.engine }
		return copenai.NewClientWithConfig(cfg)
	}
	cfg := copenai.DefaultConfig(b.apiKey)
	if b.baseURL != "" {
		cfg.BaseURL = strings.TrimRight(b.baseURL, "/") + "/v1"
	}
	_ = wrapper
	return copenai.NewClientWithConfig(cfg)
}

// engineModel 返回送给 SDK 的 model 字段。Azure 模式下 SDK 用 AzureModelMapperFunc
// 决定真实 deployment 名，所以 Model 字段仅做 token-count；普通 OpenAI 直传 Model。
func (b *Backend) engineModel() string {
	return b.model
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

// isStreamEOF 区分"流自然结束"和真错误。go-openai 1.x 用 io.EOF 表示流尾。
func isStreamEOF(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errStreamFinished) {
		return true
	}
	low := strings.ToLower(err.Error())
	return strings.Contains(low, "eof") && !strings.Contains(low, "unexpected")
}

// errStreamFinished 保留给 go-openai 升级到自定义 sentinel 时复用；
// 当前 SDK 直接返回 io.EOF，靠 isStreamEOF 字符串识别。
var errStreamFinished = errors.New("stream finished")

// mapError 把 go-openai 错误 / 网络错误归一到 aibackend 标准错误。
//
// 优先 errors.As(*copenai.APIError) 拿 HTTPStatusCode + Code 精确分类；
// 网络/IO 层错误退到字符串匹配。
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return fmt.Errorf("%w: %v", aibackend.ErrBackendUnavailable, err)
	}

	var apiErr *copenai.APIError
	if errors.As(err, &apiErr) {
		if mapped := mapAPIError(apiErr); mapped != nil {
			return fmt.Errorf("%w: %v", mapped, err)
		}
		return err
	}
	// go-openai 偶尔包成 RequestError；状态码在 .HTTPStatusCode
	var reqErr *copenai.RequestError
	if errors.As(err, &reqErr) {
		switch {
		case reqErr.HTTPStatusCode == http.StatusUnauthorized || reqErr.HTTPStatusCode == http.StatusForbidden:
			return fmt.Errorf("%w: %v", aibackend.ErrUnauthorized, err)
		case reqErr.HTTPStatusCode == http.StatusTooManyRequests:
			return fmt.Errorf("%w: %v", aibackend.ErrRateLimited, err)
		case reqErr.HTTPStatusCode == http.StatusBadRequest:
			return fmt.Errorf("%w: %v", aibackend.ErrBadRequest, err)
		case reqErr.HTTPStatusCode >= 500:
			return fmt.Errorf("%w: %v", aibackend.ErrBackendUnavailable, err)
		}
	}

	low := strings.ToLower(err.Error())
	switch {
	case strings.Contains(low, "timeout"),
		strings.Contains(low, "connection refused"),
		strings.Contains(low, "no such host"),
		strings.Contains(low, "broken pipe"),
		strings.Contains(low, "reset by peer"),
		strings.Contains(low, "eof"):
		return fmt.Errorf("%w: %v", aibackend.ErrBackendUnavailable, err)
	}
	return err
}

func mapAPIError(e *copenai.APIError) error {
	// 业务码优先（OpenAI 用 string code 如 "invalid_api_key" / "rate_limit_exceeded"）
	if codeStr, ok := e.Code.(string); ok {
		switch strings.ToLower(codeStr) {
		case "invalid_api_key", "unauthorized":
			return aibackend.ErrUnauthorized
		case "rate_limit_exceeded", "too_many_requests":
			return aibackend.ErrRateLimited
		case "context_length_exceeded", "invalid_request_error":
			return aibackend.ErrBadRequest
		}
	}
	// 业务码留空时按 HTTP status
	switch {
	case e.HTTPStatusCode == http.StatusUnauthorized || e.HTTPStatusCode == http.StatusForbidden:
		return aibackend.ErrUnauthorized
	case e.HTTPStatusCode == http.StatusTooManyRequests:
		return aibackend.ErrRateLimited
	case e.HTTPStatusCode == http.StatusBadRequest:
		return aibackend.ErrBadRequest
	case e.HTTPStatusCode >= 500:
		return aibackend.ErrBackendUnavailable
	}
	return nil
}
