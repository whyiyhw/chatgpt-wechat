package redis

import (
	"crypto/tls"
	"github.com/redis/go-redis/v9"
	"net/url"
	"strings"
)

var Rdb *redis.Client

const WelcomeCacheKey = "chat:wecome:%d:%s"
const CursorCacheKey = "chat:cursor:%s"
const EmbeddingsCacheKey = "chat:embeddings:%s"
const UserSessionAgentDefaultKey = "session_agent:default:%s:%s"
const UserSessionListKey = "user:session:list:%s"
const SessionKey = "session:%s"

// DifyConversationKey Dify会话ID存储
const DifyConversationKey = "dify:conversation:%d:%s"

// DifyCustomerConversationKey Dify客户会话ID存储
const DifyCustomerConversationKey = "dify:conversation:%s:%s"

// ImageTemporaryKey 图片临时存储
const ImageTemporaryKey = "chat:image:temporary:%d-%s"

func Init(connString string) {
	options := &redis.Options{
		DB: 0,
	}

	// 解析连接字符串
	if strings.HasPrefix(connString, "rediss://") || strings.HasPrefix(connString, "redis://") {
		// 使用 url.Parse 解析整个 URL
		parsedURL, err := url.Parse(connString)
		if err != nil {
			panic(err)
		}

		// 设置地址（主机名+端口）
		options.Addr = parsedURL.Host

		// 解析用户名和密码
		if parsedURL.User != nil {
			options.Username = parsedURL.User.Username()
			password, isSet := parsedURL.User.Password()
			if isSet {
				options.Password = password
			}
		}

		// 如果是 rediss 协议，启用 TLS
		if parsedURL.Scheme == "rediss" {
			options.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		}
	} else {
		// 如果没有协议前缀，假设是普通的 host:port
		options.Addr = connString
	}

	Rdb = redis.NewClient(options)
}

func Close() {
	err := Rdb.Close()
	if err != nil {
		return
	}
}
