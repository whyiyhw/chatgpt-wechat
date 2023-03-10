// Code generated by goctl. DO NOT EDIT.

package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/builder"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/core/stringx"
)

var (
	chatConfigFieldNames          = builder.RawFieldNames(&ChatConfig{})
	chatConfigRows                = strings.Join(chatConfigFieldNames, ",")
	chatConfigRowsExpectAutoSet   = strings.Join(stringx.Remove(chatConfigFieldNames, "`id`", "`create_at`", "`create_time`", "`created_at`", "`update_at`", "`update_time`", "`updated_at`"), ",")
	chatConfigRowsWithPlaceHolder = strings.Join(stringx.Remove(chatConfigFieldNames, "`id`", "`create_at`", "`create_time`", "`created_at`", "`update_at`", "`update_time`", "`updated_at`"), "=?,") + "=?"

	cacheChatConfigIdPrefix = "cache:chatConfig:id:"
)

type (
	chatConfigModel interface {
		Insert(ctx context.Context, data *ChatConfig) (sql.Result, error)
		FindOne(ctx context.Context, id int64) (*ChatConfig, error)
		Update(ctx context.Context, data *ChatConfig) error
		Delete(ctx context.Context, id int64) error
	}

	defaultChatConfigModel struct {
		sqlc.CachedConn
		table string
	}

	ChatConfig struct {
		Id        int64     `db:"id"`
		User      string    `db:"user"`       // 用户标识
		AgentId   int64     `db:"agent_id"`   // 应用ID
		Model     string    `db:"model"`      // 模型
		Prompt    string    `db:"prompt"`     // 系统初始设置
		CreatedAt time.Time `db:"created_at"` // 创建时间
		UpdatedAt time.Time `db:"updated_at"` // 更新时间
	}
)

func newChatConfigModel(conn sqlx.SqlConn, c cache.CacheConf) *defaultChatConfigModel {
	return &defaultChatConfigModel{
		CachedConn: sqlc.NewConn(conn, c),
		table:      "`chat_config`",
	}
}

func (m *defaultChatConfigModel) Delete(ctx context.Context, id int64) error {
	chatConfigIdKey := fmt.Sprintf("%s%v", cacheChatConfigIdPrefix, id)
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		query := fmt.Sprintf("delete from %s where `id` = ?", m.table)
		return conn.ExecCtx(ctx, query, id)
	}, chatConfigIdKey)
	return err
}

func (m *defaultChatConfigModel) FindOne(ctx context.Context, id int64) (*ChatConfig, error) {
	chatConfigIdKey := fmt.Sprintf("%s%v", cacheChatConfigIdPrefix, id)
	var resp ChatConfig
	err := m.QueryRowCtx(ctx, &resp, chatConfigIdKey, func(ctx context.Context, conn sqlx.SqlConn, v interface{}) error {
		query := fmt.Sprintf("select %s from %s where `id` = ? limit 1", chatConfigRows, m.table)
		return conn.QueryRowCtx(ctx, v, query, id)
	})
	switch err {
	case nil:
		return &resp, nil
	case sqlc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultChatConfigModel) Insert(ctx context.Context, data *ChatConfig) (sql.Result, error) {
	chatConfigIdKey := fmt.Sprintf("%s%v", cacheChatConfigIdPrefix, data.Id)
	ret, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?)", m.table, chatConfigRowsExpectAutoSet)
		return conn.ExecCtx(ctx, query, data.User, data.AgentId, data.Model, data.Prompt)
	}, chatConfigIdKey)
	return ret, err
}

func (m *defaultChatConfigModel) Update(ctx context.Context, data *ChatConfig) error {
	chatConfigIdKey := fmt.Sprintf("%s%v", cacheChatConfigIdPrefix, data.Id)
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		query := fmt.Sprintf("update %s set %s where `id` = ?", m.table, chatConfigRowsWithPlaceHolder)
		return conn.ExecCtx(ctx, query, data.User, data.AgentId, data.Model, data.Prompt, data.Id)
	}, chatConfigIdKey)
	return err
}

func (m *defaultChatConfigModel) formatPrimary(primary interface{}) string {
	return fmt.Sprintf("%s%v", cacheChatConfigIdPrefix, primary)
}

func (m *defaultChatConfigModel) queryPrimary(ctx context.Context, conn sqlx.SqlConn, v, primary interface{}) error {
	query := fmt.Sprintf("select %s from %s where `id` = ? limit 1", chatConfigRows, m.table)
	return conn.QueryRowCtx(ctx, v, query, primary)
}

func (m *defaultChatConfigModel) tableName() string {
	return m.table
}
