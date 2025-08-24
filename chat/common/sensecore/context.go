package sensecore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"chat/common/redis"
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
	Client       *ChatClient        `json:"chat_client"`    // sensecore 客户端
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

// WithClient 通过 sensecore 客户端初始化会话上下文
func (c *UserContext) WithClient(client *ChatClient) *UserContext {
	c.Client = client
	return c
}

// WithTimeOut 设置会话超时时间
func (c *UserContext) WithTimeOut(timeOut int64) *UserContext {
	c.TimeOut = timeOut
	return c
}

// WithMaxTokens 设置最大token数
func (c *UserContext) WithMaxTokens(maxTokens int) *UserContext {
	c.MaxTokens = maxTokens
	return c
}

// WithMessages 设置消息
func (c *UserContext) WithMessages(messages []ChatModelMessage) *UserContext {
	c.Messages = messages
	return c
}

// WithSummary 设置摘要
func (c *UserContext) WithSummary(summary []ChatModelMessage) *UserContext {
	c.Summary = summary
	return c
}

// GetSessionKey 获取会话ID
func (c *UserContext) GetSessionKey() string {
	return c.SessionKey
}

// GetUserUniqueID 获取用户唯一标识
func (c *UserContext) GetUserUniqueID() string {
	return c.UserUniqueID
}

// GetModel 获取模型
func (c *UserContext) GetModel() string {
	return c.Model
}

// GetPrompt 获取提示词
func (c *UserContext) GetPrompt() string {
	return c.Prompt
}

// GetMessages 获取消息
func (c *UserContext) GetMessages() []ChatModelMessage {
	return c.Messages
}

// GetMaxTokens 获取最大token数
func (c *UserContext) GetMaxTokens() int {
	return c.MaxTokens
}

// GetClient 获取客户端
func (c *UserContext) GetClient() *ChatClient {
	return c.Client
}

// GetTimeOut 获取超时时间
func (c *UserContext) GetTimeOut() int64 {
	return c.TimeOut
}

// Save 保存会话上下文到redis
func (c *UserContext) Save() error {
	byteData, err := json.Marshal(c)
	if err != nil {
		return errors.Wrap(err, "marshal user context failed")
	}

	err = redis.Rdb.Set(context.Background(), getSessionKey(c.SessionKey), string(byteData), 0).Err()
	if err != nil {
		return errors.Wrap(err, "save user context to redis failed")
	}

	return nil
}

// Delete 删除会话上下文
func (c *UserContext) Delete() error {
	err := redis.Rdb.Del(context.Background(), getSessionKey(c.SessionKey)).Err()
	if err != nil {
		return errors.Wrap(err, "delete user context from redis failed")
	}

	err = redis.Rdb.SRem(context.Background(), UserSessionListKey(c.UserUniqueID), c.SessionKey).Err()
	if err != nil {
		return errors.Wrap(err, "remove session from user session list failed")
	}

	return nil
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

func (c *UserContext) GetChatSummary() []ChatModelMessage {
	var summary []ChatModelMessage
	summary = append(summary, c.Summary...)
	if c.Prompt != "" {
		summary = append([]ChatModelMessage{
			{
				Role: SystemRole,
				Content: ChatContent{
					MIMEType: MimetypeTextPlain,
					Data:     c.Prompt,
				},
			}}, summary...)
	}
	return summary
}
