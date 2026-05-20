package aibackend

import "time"

// ChunkType 流式分段类型
type ChunkType string

const (
	// 基础事件（所有 backend 都可能产生）
	ChunkTypeText     ChunkType = "text"
	ChunkTypeImage    ChunkType = "image"
	ChunkTypeAudio    ChunkType = "audio"
	ChunkTypeToolCall ChunkType = "tool_call"
	ChunkTypeDone     ChunkType = "done"
	ChunkTypeError    ChunkType = "error"

	// Workflow / Chatflow 事件（目前由 Dify 一类支持 chatflow 的 backend 产生；
	// 其它 backend 实现可视为"按需透传"——不产生即不发）
	ChunkTypeWorkflowStarted  ChunkType = "workflow_started"
	ChunkTypeNodeStarted      ChunkType = "node_started"
	ChunkTypeNodeFinished     ChunkType = "node_finished"
	ChunkTypeWorkflowFinished ChunkType = "workflow_finished"
	ChunkTypeMessage          ChunkType = "message"
	ChunkTypeMessageEnd       ChunkType = "message_end"
)

// WorkflowEvent 承载 ChunkTypeWorkflow* / ChunkTypeNode* 类型的事件详情。
// 其它 ChunkType 此字段为 nil。
type WorkflowEvent struct {
	RunID       string         // workflow_run_id
	NodeID      string         // workflow_started 不带；node_* 事件必带
	NodeType    string         // start / llm / answer / end / ...
	Title       string         // 节点标题
	Status      string         // succeeded / failed / running
	Outputs     map[string]any // 节点/工作流输出；通常 Outputs["answer"] 是文本
	Inputs      map[string]any // node_started 携带
	Error       string         // 失败时的错误信息
	ElapsedTime float64        // 节点/工作流耗时（秒）
	TotalTokens int            // 仅 workflow_finished 时有意义
}

// FileRef 文件引用，可以是已上传的后端文件 ID，也可以是远端 URL
type FileRef struct {
	Type           string // image / document / audio
	TransferMethod string // remote_url | local_file
	URL            string
	UploadFileID   string
	MIME           string
}

// Usage Token 用量
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// OpenConversationReq 创建/恢复会话请求
type OpenConversationReq struct {
	AppID    string
	User     string
	Metadata map[string]string
}

// OpenConversationResp 创建/恢复会话响应
type OpenConversationResp struct {
	ConversationID string
}

// Message 用于"客户端管 context"的 backend（OpenAI/Gemini/DeepSeek 等）。
//
// Dify 类"服务端管 context"的 backend 自己用 conversation_id + Query 拼接历史，
// 这种 backend 应忽略 ChatReq.Messages 字段。
type Message struct {
	Role    string // "system" / "user" / "assistant"
	Content string
}

// ChatReq 统一对话请求（流式与非流式共用）
type ChatReq struct {
	AppID          string                 // 后端 App / Workflow 标识
	ConversationID string                 // 后端会话 ID；空表示首次创建
	User           string                 // 终端用户标识（透传给后端用于配额/标识）
	Query          string                 // 文本内容
	Inputs         map[string]interface{} // 工作流变量
	Files          []FileRef              // 文件 / 图片引用
	Metadata       map[string]string      // trace_id / channel / kf_id 等

	// Messages 完整对话上下文（含历史 + 当前问题）。
	//
	// - OpenAI / Gemini / DeepSeek 类 adapter 使用本字段，无视 ConversationID/Query
	//   （或仅把 Query 当成 fallback：Messages 为空时包成单条 user message）。
	// - Dify 类 adapter 忽略本字段，仍用 ConversationID + Query。
	//
	// 调用方建议：Messages 非空时，最后一条 role=user 的 Content 应与 Query 一致，
	// 让两类 backend 都能正确解读"当前问题"。
	Messages []Message

	// Timeout 控制单次调用超时，<=0 表示走 Backend 默认
	Timeout time.Duration
}

// ChatResp 非流式对话响应
type ChatResp struct {
	MessageID      string
	ConversationID string
	Answer         string
	Usage          *Usage
}

// StreamChunk 流式分段
type StreamChunk struct {
	Type           ChunkType
	Text           string
	ImageURL       string
	AudioURL       string
	ToolCallName   string
	ToolCallArgs   string
	MessageID      string
	ConversationID string
	TaskID         string // 后端侧的任务标识；用于取消、限流诊断
	Err            error
	Usage          *Usage // 仅 ChunkTypeDone / ChunkTypeWorkflowFinished 时携带

	// Workflow 仅 ChunkTypeWorkflow* / ChunkTypeNode* 类型时非空
	Workflow *WorkflowEvent
}

// UploadFileReq 上传文件请求
type UploadFileReq struct {
	User    string
	Name    string
	MIME    string
	Content []byte
}

// UploadFileResp 上传文件响应
type UploadFileResp struct {
	FileID string
	URL    string
}
