package crontab

import (
	"time"

	"chat/service/websocket/cmd/bucket"
)

// TimingClearConn 定时器，每分钟都会去清理 未认证/认证-无心跳 的连接
func TimingClearConn(b, m *bucket.MapBucket) {
	for {
		// 休眠 1 分钟
		time.Sleep(time.Minute)
		// 实际清理数据
		go clearFunc(b, m)
	}
}

func clearFunc(b *bucket.MapBucket, m *bucket.MapBucket) {
	d := time.Minute * -1
	// 每分钟都会去清理 未认证的连接
	b.EachDelete(d)
	// 每分钟都会去清理 认证后未进行心跳回复的连接
	m.EachDelete(d)
}
