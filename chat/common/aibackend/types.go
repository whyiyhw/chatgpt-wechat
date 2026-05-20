package aibackend

import "time"

// ChunkType 流式分段类型
type ChunkType string

const (
	ChunkTypeText     ChunkType = "text"
	ChunkTypeImage    ChunkType = "image"
	ChunkTypeAudio    ChunkType = "audio"
	ChunkTypeToolCall ChunkType = "tool_call"
	ChunkTypeDone     ChunkType = "done"
	ChunkTypeError    ChunkType = "error"
)

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

// ChatReq 统一对话请求（流式与非流式共用）
type ChatReq struct {
	AppID          string                 // 后端 App / Workflow 标识
	ConversationID string                 // 后端会话 ID；空表示首次创建
	User           string                 // 终端用户标识（透传给后端用于配额/标识）
	Query          string                 // 文本内容
	Inputs         map[string]interface{} // 工作流变量
	Files          []FileRef              // 文件 / 图片引用
	Metadata       map[string]string      // trace_id / channel / kf_id 等

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
	Err            error
	Usage          *Usage // 仅 ChunkTypeDone 时携带
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
