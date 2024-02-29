package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chat/common/redis"
	"chat/common/tiktoken"

	"github.com/google/uuid"
)

// UserContext is the context of a user once started a chat session
type UserContext struct {
	SessionKey   string             `json:"session_key"`    // 会话ID
	Model        string             `json:"model"`          // 模型
	Prompt       string             `json:"prompt"`         // 对话基础设定
	UserUniqueID string             `json:"user_unique_id"` // 用户唯一标识
	Messages     []ChatModelMessage `json:"messages"`       // 存储此会话的完整上下文
	Summary      []ChatModelMessage `json:"summary"`        // 存储此会话的实际上下文
	MaxTokens    int                `json:"max_tokens"`     // 需要控制的最大token数
	Client       *ChatClient        `json:"chat_client"`    // gemini 客户端
	TimeOut      int64              `json:"time_out"`       // 超时时间 默认为 -1 永不超时
}

func GetUserUniqueID(userId string, agentID string) string {
	return fmt.Sprintf(redis.UserSessionAgentDefaultKey, userId, agentID)
}

func UserSessionListKey(UserUniqueID string) string {
	return fmt.Sprintf(redis.UserSessionListKey, UserUniqueID)
}

func getSessionKey(sessionKey string) string {
	return fmt.Sprintf(redis.SessionKey, sessionKey)
}

func (c *UserContext) Set(q ChatContent, a, conversationId string, save bool) *UserContext {

	if q.Data != "" {
		c.Messages = append(c.Messages, ChatModelMessage{
			MessageId: conversationId,
			Role:      UserRole,
			Content: ChatContent{
				MIMEType: q.MIMEType,
				Data:     q.Data,
			},
		})
		c.Summary = append(c.Summary, ChatModelMessage{
			MessageId: conversationId,
			Role:      UserRole,
			Content: ChatContent{
				MIMEType: q.MIMEType,
				Data:     q.Data,
			},
		})
	}

	if a != "" {
		c.Messages = append(c.Messages, ChatModelMessage{
			MessageId: conversationId,
			Role:      ModelRole,
			Content: ChatContent{
				MIMEType: MimetypeTextPlain,
				Data:     a,
			},
		})
		c.Summary = append(c.Summary, ChatModelMessage{
			MessageId: conversationId,
			Role:      ModelRole,
			Content: ChatContent{
				MIMEType: MimetypeTextPlain,
				Data:     a,
			},
		})
	}

	if save {
		// 去保存数据
		byteData, _ := json.Marshal(c)
		redis.Rdb.Set(context.Background(), getSessionKey(c.SessionKey), string(byteData), time.Duration(c.TimeOut)*time.Minute)
	}
	return c
}

// GetChatSummary 获取对话摘要
func (c *UserContext) GetChatSummary() []ChatModelMessage {
	var summary []ChatModelMessage
	summary = append(summary, c.Summary...)
	if c.Prompt != "" {
		summary = append([]ChatModelMessage{
			{
				Role: UserRole,
				Content: ChatContent{
					MIMEType: MimetypeTextPlain,
					Data:     c.Prompt,
				},
			},
			{
				Role: ModelRole,
				Content: ChatContent{
					MIMEType: MimetypeTextPlain,
					Data:     "好的，收到！",
				},
			}}, summary...)
	}
	return summary
}

// GetChatHistory 获取对话历史
func (c *UserContext) GetChatHistory() []ChatModelMessage {
	var summary []ChatModelMessage
	return append(summary, c.Messages...)
}

// ChatClear 清理会话
func (c *UserContext) ChatClear(userUniqueID string) {
	// 去 redis 中 获取 userUniqueID 对应的会话ID
	sessionKey, _ := redis.Rdb.Get(context.Background(), userUniqueID).Result()
	if sessionKey != "" {
		// 清理会话
		redis.Rdb.Del(context.Background(), userUniqueID)
		redis.Rdb.SRem(context.Background(), UserSessionListKey(userUniqueID), sessionKey)
		// 清理会话上下文
		redis.Rdb.Del(context.Background(), getSessionKey(sessionKey))
	}
}

// NewUserContext 通过用户唯一标识获取会话上下文
func NewUserContext(userUniqueID string) *UserContext {
	// 去 redis 中 获取 userUniqueID 对应的会话ID
	sessionKey, _ := redis.Rdb.Get(context.Background(), userUniqueID).Result()
	if sessionKey == "" {
		// 创建新的会话
		sessionKey = uuid.New().String()

		// 存入 redis
		redis.Rdb.Set(context.Background(), userUniqueID, sessionKey, 0)
		redis.Rdb.SAdd(context.Background(), UserSessionListKey(userUniqueID), sessionKey)
	}

	// 再通过 会话ID 从 redis 中 获取 会话上下文
	data, _ := redis.Rdb.Get(context.Background(), getSessionKey(sessionKey)).Result()
	if data == "" {
		res := UserContext{
			SessionKey:   sessionKey,
			UserUniqueID: userUniqueID,
			MaxTokens:    4096,
		}
		byteData, _ := json.Marshal(res)
		redis.Rdb.Set(context.Background(), getSessionKey(sessionKey), string(byteData), 0)
		return &res
	}

	// 反序列化
	res := new(UserContext)
	_ = json.Unmarshal([]byte(data), res)
	return res
}

func (c *UserContext) WithModel(model string) *UserContext {
	c.Model = model
	return c
}

func (c *UserContext) WithPrompt(prompt string) *UserContext {
	c.Prompt = prompt
	return c
}

func (c *UserContext) GetSummary() []ChatModelMessage {
	return c.Summary
}

// WithClient 通过 openai 客户端初始化会话上下文
func (c *UserContext) WithClient(client *ChatClient) *UserContext {
	c.Client = client
	return c
}

func (c *UserContext) WithImage(agentID int64, userID string) *UserContext {
	// 将 URL 存入memory 中，需要时候，再取出来 进行 base64
	cacheKey := fmt.Sprintf(redis.ImageTemporaryKey, agentID, userID)
	// 可存入多张图片
	ok, _ := redis.Rdb.Exists(context.Background(), cacheKey).Result()
	if ok > 0 {
		// 从 redis 中取出图片信息，加入请求
		images := redis.Rdb.HGetAll(context.Background(), cacheKey).Val()
		for _, image := range images {
			content, mime, err := GetImageContent(image)
			if err != nil {
				// log info and continue
				//sendToUser(req.AgentID, req.UserID, "读取图片文件失败:"+err.Error(), l.svcCtx.Config)
				return c
			}
			c.Set(NewChatContent(content, mime), "", "", false)
		}
		// 清理图片信息
		redis.Rdb.Del(context.Background(), cacheKey)
	}

	return c
}

// NumTokensFromMessages returns the number of tokens that will be used for a given model
func NumTokensFromMessages(messages []ChatModelMessage, model string) (numTokens int) {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		err = fmt.Errorf("EncodingForModel: %v", err)
		fmt.Println(err)
		return
	}

	tokensPerMessage := 3

	for _, message := range messages {
		numTokens += tokensPerMessage
		numTokens += len(tkm.Encode(message.Content.Data, nil, nil))
		numTokens += len(tkm.Encode(message.Role, nil, nil))
	}
	numTokens += 3
	return numTokens
}

// GetNewChatMessage 获取全新的对话
func (c *UserContext) GetNewChatMessage(prompt string) []ChatModelMessage {
	var summary []ChatModelMessage
	if prompt != "" {
		summary = append([]ChatModelMessage{
			{
				Role: UserRole,
				Content: ChatContent{
					MIMEType: MimetypeTextPlain,
					Data:     prompt,
				},
			},
			{
				Role: ModelRole,
				Content: ChatContent{
					MIMEType: MimetypeTextPlain,
					Data:     "好的，收到！",
				},
			}}, summary...)
	}
	return summary
}
