package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"chat/common/redis"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
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
	Client       *ChatClient        `json:"chat_client"`    // openai 客户端
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
	fmt.Println("sessionKey", sessionKey)
	if sessionKey == "" {
		// 创建新的会话
		sessionKey = uuid.New().String()

		// 存入 redis
		redis.Rdb.Set(context.Background(), userUniqueID, sessionKey, 0)
		redis.Rdb.SAdd(context.Background(), UserSessionListKey(userUniqueID), sessionKey)
	}

	// 再通过 会话ID 从 redis 中 获取 会话上下文
	data, _ := redis.Rdb.Get(context.Background(), getSessionKey(sessionKey)).Result()
	fmt.Println("data", data)
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

func (c *UserContext) Set(q, a string, save bool) *UserContext {

	if q != "" {
		c.Messages = append(c.Messages, ChatModelMessage{
			Role:    "user",
			Content: q,
		})
	}

	if a != "" {
		c.Messages = append(c.Messages, ChatModelMessage{
			Role:    "assistant",
			Content: a,
		})
	}

	if q != "" {
		c.Summary = append(c.Summary, ChatModelMessage{
			Role:    "user",
			Content: q,
		})
	}

	if a != "" {
		c.Summary = append(c.Summary, ChatModelMessage{
			Role:    "assistant",
			Content: a,
		})
	}

	if save {
		// 去保存数据
		byteData, _ := json.Marshal(c)
		redis.Rdb.Set(context.Background(), getSessionKey(c.SessionKey), string(byteData), 0)
		// 一个汉字大概 2.7 ~ 1.9 个token，最大长度为 4000 / ( 2.7 ~ 1.9) = 1480 ~ 2100 左右
		// 因为响应已经用了 2000 左右的 token，所以请求必须在 2000 以下 也就是600字左右
		// 窗口给 150 字，其他的都需要摘要到 summary 中
		maxToken := 150
		currTokenStr := ""
		//当录入最新的对话信息时，从新到旧，一轮轮累加评估，是否大于设置的 max_token，
		//如果大于，就会对那一轮之前的对话进行 summery + 窗口内的会话，得到实际的上下文环境
		for i := 0; i < len(c.Summary); i++ {
			s := c.Summary[len(c.Summary)-i-1]
			currTokenStr += s.Content
			if i%2 == 0 {
				continue
			}
			if len([]rune(currTokenStr)) > maxToken {
				// 去总结 这个数据之前的数据
				go func() {
					newSummary, err := c.doSummary(c.Summary[:len(c.Summary)-i-1])
					if err != nil {
						fmt.Println("summary error", err)
						return
					}
					// 将新的 summary 赋值给 c.Summary
					c.Summary = append(newSummary, c.Summary[len(c.Summary)-i-1:]...)
					// 重新保存数据
					byteData, _ := json.Marshal(c)
					redis.Rdb.Set(context.Background(), getSessionKey(c.SessionKey), string(byteData), 0)
				}()
				break
			}
		}
	}
	return c
}

func (c *UserContext) doSummary(summary []ChatModelMessage) ([]ChatModelMessage, error) {

	prompt := "请总结以下信息至100字内,并以json形式进行响应，如：{\"summary\":[{\"q\":\"问题\",\"a\":\"回答\"}]}\n"

	// 因为 summary 存在多个会话，且会话长度 可能超过 1000字，所以需要进行 分段的 summary
	var summarys []string
	var currSummary string
	for i := 0; i < len(summary); i++ {
		if summary[i].Role == "assistant" {
			currSummary += "A: " + summary[i].Content + "\n"
		} else {
			currSummary += "Q: " + summary[i].Content + "\n"
		}
		currSummary += summary[i].Content
		if i%2 == 0 {
			continue
		}
		if len([]rune(currSummary)) > 500 {
			summarys = append(summarys, currSummary)
			currSummary = ""
		}
	}

	if currSummary != "" {
		summarys = append(summarys, currSummary)
	}

	var newSummary []ChatModelMessage

	for k, val := range summarys {
		newPrompt := prompt + "\n" + val

		logx.Info("summary_req", k, ": "+newPrompt)
		logx.Info("summary_req_length", k, ": ", len([]rune(newPrompt)))

		// 调用 openai api 进行 summary 简化到 100 字以内
		summaryStr, err := c.Client.WithModel(TextModel).WithMaxToken(1500).WithTemperature(0.3).
			Completion(newPrompt)

		logx.Info("summary_reps", k, ": "+summaryStr)
		logx.Info("summary_reps_length", k, ": ", len([]rune(summaryStr)))

		type AutoGenerated struct {
			Summary []struct {
				Q string `json:"q"`
				A string `json:"a"`
			} `json:"summary"`
		}

		if err == nil {
			var summary AutoGenerated
			err = json.Unmarshal([]byte(summaryStr), &summary)
			if err != nil {
				return c.Summary, err
			}

			for _, val := range summary.Summary {
				newSummary = append(newSummary, ChatModelMessage{
					Role:    "user",
					Content: val.Q,
				})
				newSummary = append(newSummary, ChatModelMessage{
					Role:    "assistant",
					Content: val.A,
				})
			}
		} else {
			// log 不处理
			logx.Info("summary_err", k, ": "+err.Error())
		}
	}

	return newSummary, nil
}

// GetCompletionSummary 获取补全的摘要
func (c *UserContext) GetCompletionSummary() string {
	basePrompt := c.Prompt + "\n"
	l := len(c.Summary)
	for k, val := range c.Summary {
		switch val.Role {
		case "user":
			basePrompt += "Q: " + val.Content + "\n"
			if l == k+1 {
				basePrompt += "A: "
			}
		case "assistant":
			basePrompt += "A: " + val.Content + "\n"
		}
	}
	return basePrompt
}

// GetChatSummary 获取对话摘要
func (c *UserContext) GetChatSummary() []ChatModelMessage {
	var summary []ChatModelMessage
	summary = append(summary, ChatModelMessage{
		Role:    "system",
		Content: c.Prompt,
	})
	summary = append(summary, c.Summary...)
	return summary
}

func (c *UserContext) getCompletionSummary() string {
	basePrompt := ""
	l := len(c.Summary)
	for k, val := range c.Summary {
		switch val.Role {
		case "user":
			basePrompt += "Q: " + val.Content + "\n"
			if l == k+1 {
				basePrompt += "A: "
			}
		case "assistant":
			basePrompt += "A: " + val.Content + "\n"
		}
	}
	return basePrompt
}

func NewSession(userUniqueID string) {
	// 去 redis 中 获取 userUniqueID 对应的会话ID
	sessionKey := uuid.New().String()
	// 存入 redis
	redis.Rdb.Set(context.Background(), userUniqueID, sessionKey, 0)
	redis.Rdb.SAdd(context.Background(), UserSessionListKey(userUniqueID), sessionKey)
}

// SetSession 设置用户的会话
func SetSession(userUniqueID string, sessionKey string) error {
	//判断集合中是否存在此会话
	if redis.Rdb.SIsMember(context.Background(), UserSessionListKey(userUniqueID), sessionKey).Val() {
		redis.Rdb.Set(context.Background(), userUniqueID, sessionKey, 0)
		return nil
	}
	return fmt.Errorf("此 seession 不存在或已被删除～")
}

// GetSessions 获取用户的所有会话
func GetSessions(userUniqueID string) []string {
	// 去 redis 中 获取 userUniqueID 对应的会话ID
	sessionKeys, _ := redis.Rdb.SMembers(context.Background(), UserSessionListKey(userUniqueID)).Result()

	return sessionKeys
}

// ClearSessions 清除用户的所有会话
func ClearSessions(userUniqueID string) {
	// 去 redis 中 获取 userUniqueID 对应的会话ID
	sessionKeys, _ := redis.Rdb.SMembers(context.Background(), UserSessionListKey(userUniqueID)).Result()
	for _, sessionKey := range sessionKeys {
		redis.Rdb.Del(context.Background(), getSessionKey(sessionKey))
	}
	redis.Rdb.Del(context.Background(), UserSessionListKey(userUniqueID))
}

// Clear 清除会话上下文
func (c *UserContext) Clear() {
	_, _ = redis.Rdb.Del(context.Background(), c.UserUniqueID).Result()
	_, _ = redis.Rdb.Del(context.Background(), getSessionKey(c.SessionKey)).Result()
	_, _ = redis.Rdb.SRem(context.Background(), UserSessionListKey(c.UserUniqueID), c.SessionKey).Result()
}
