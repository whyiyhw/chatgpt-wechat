package bucket

import (
	"fmt"
	"sync"
	"time"

	"chat/service/websocket/cmd/response"
	"github.com/whyiyhw/gws"
)

type Bucket struct {
	Conn *gws.Conn // demo-mvp-project 连接

	Uid int // demo-mvp-project 绑定的用户连接ID

	CreatedAt time.Time // 绑定的时间
	UpdatedAt time.Time // 心跳包更新的时间
}

// AuthAt time.Time // 认证的时间
// AuthToken string // 授权时使用的token

func New(c *gws.Conn, uid int) *Bucket {
	b := Bucket{
		Conn:      c,
		Uid:       uid,
		CreatedAt: time.Now().Local(),
		UpdatedAt: time.Now().Local(),
	}
	return &b
}

type MapBucket struct {
	sync.RWMutex
	Buckets map[int]*Bucket
}

// NewMapBucket 新建一个RWMap
func NewMapBucket(n int) *MapBucket {
	return &MapBucket{
		Buckets: make(map[int]*Bucket, n),
	}
}

func (m *MapBucket) Set(fd int, b *Bucket) {
	m.Lock()
	defer m.Unlock()

	m.Buckets[fd] = b
}

func (m *MapBucket) Get(k int) (*Bucket, bool) { //从map中读取一个值
	m.RLock()
	defer m.RUnlock()
	v, existed := m.Buckets[k] // 在锁的保护下从map中读取
	return v, existed
}

func (m *MapBucket) Delete(k int) { //删除一个键
	m.Lock() // 锁保护
	defer m.Unlock()

	// 删除 map
	delete(m.Buckets, k)
}

func (m *MapBucket) Len() int { // map的长度
	m.RLock() // 锁保护
	defer m.RUnlock()
	return len(m.Buckets)
}

func (m *MapBucket) Each(f func(k int, v *Bucket) bool) { // 遍历map
	m.RLock() //遍历期间一直持有读锁
	defer m.RUnlock()

	for k, v := range m.Buckets {
		if !f(k, v) {
			return
		}
	}
}

func (m *MapBucket) Exist(fd int) bool {
	m.RLock() //遍历期间一直持有读锁
	defer m.RUnlock()

	_, ok := m.Buckets[fd]
	return ok
}

func (m *MapBucket) Heartbeat(uid int) {
	m.Lock()
	defer m.Unlock()
	_, ok := m.Buckets[uid]
	if ok {
		m.Buckets[uid].UpdatedAt = time.Now().Local()
	}
}

func (m *MapBucket) EachStatus() string { // 遍历map
	m.RLock() //遍历期间一直持有读锁
	defer m.RUnlock()
	str := ""
	for _, v := range m.Buckets {
		str += fmt.Sprintf(
			"client %d ,created_at %s updated_at %s \r\n", v.Uid, v.CreatedAt.Local().Format("2006-01-02 15:04:05"),
			v.UpdatedAt.Local().Format("2006-01-02 15:04:05"),
		)
	}
	return str
}

// EachSendMsg 遍历发送消息
func (m *MapBucket) EachSendMsg(str string, ids []int) int {
	//遍历期间一直持有读锁
	m.RLock()
	defer m.RUnlock()

	i := 0
	if len(ids) > 0 {
		for _, v := range m.Buckets {
			for _, uid := range ids {
				if v.Uid == uid {
					_, _ = v.Conn.Write([]byte(str))
					i++
				}
			}
		}
		return i
	}

	for _, v := range m.Buckets {
		_, _ = v.Conn.Write([]byte(str))
		i++
	}
	return i
}

func (m *MapBucket) EachDelete(layer time.Duration) { // 遍历map
	m.RLock() //遍历期间一直持有读锁
	defer m.RUnlock()

	for k, v := range m.Buckets {
		if v.UpdatedAt.Before(time.Now().Local().Add(layer)) {
			_, _ = v.Conn.Write(response.Error(503, "连接超时"))
			delete(m.Buckets, k)
		} else {
			_, _ = v.Conn.Write(response.Success(203))
		}
	}
}
