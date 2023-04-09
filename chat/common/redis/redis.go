package redis

import (
	"github.com/go-redis/redis/v8"
)

var Rdb *redis.Client

const WelcomeCacheKey = "chat:wecome:%d:%s"
const CursorCacheKey = "chat:cursor:%s"
const EmbeddingsCacheKey = "chat:embeddings:%s"
const UserSessionAgentDefaultKey = "session_agent:default:%s:%s"
const UserSessionListKey = "user:session:list:%s"
const SessionKey = "session:%s"

func Init(Host, Pass string) {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     Host,
		Password: Pass,
		DB:       1,
	})
}

func Close() {
	err := Rdb.Close()
	if err != nil {
		return
	}
}
