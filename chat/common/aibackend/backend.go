package aibackend

import "context"

// AIBackend 统一的 AI 后端适配器接口
//
// 所有具体后端（Dify / OpenClaw / OpenAI fallback / 自定义 HTTP）都需要实现该接口。
// 网关层只依赖该接口，不依赖任何具体 Provider 的 SDK 细节。
type AIBackend interface {
	// Name 返回后端实例名称（用于路由与日志）
	Name() string

	// Type 返回后端类型标识（如 "dify" / "openclaw" / "openai"）
	Type() string

	// OpenConversation 创建/恢复一次会话；首次调用返回后端侧的 conversation_id。
	// 部分后端（如 Dify）不强制预先创建会话，可在 ChatStream 首条响应中拿到 ConversationID，
	// 该方法允许返回空 ConversationID，调用方据此判断是否需要在首条 chunk 中回填。
	OpenConversation(ctx context.Context, req OpenConversationReq) (OpenConversationResp, error)

	// Chat 非流式对话
	Chat(ctx context.Context, req ChatReq) (ChatResp, error)

	// ChatStream 流式对话；返回的 channel 在结束或出错时由实现方负责关闭。
	// 调用方应通过 ctx 取消来中断流式接收。
	ChatStream(ctx context.Context, req ChatReq) (<-chan StreamChunk, error)

	// UploadFile 上传文件/图片到后端；返回后端文件 ID 或可访问 URL。
	UploadFile(ctx context.Context, req UploadFileReq) (UploadFileResp, error)

	// Health 健康检查；用于主备切换与可用性探测。
	Health(ctx context.Context) error
}
