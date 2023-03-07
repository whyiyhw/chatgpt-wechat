package model

import (
	"context"

	"chat/common/page"

	"github.com/Masterminds/squirrel"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ChatConfigModel = (*customChatConfigModel)(nil)

type (
	// ChatConfigModel is an interface to be customized, add more methods here,
	// and implement the added methods in customChatConfigModel.
	ChatConfigModel interface {
		chatConfigModel
		Trans(ctx context.Context, fn func(context context.Context, session sqlx.Session) error) error
		RowBuilder() squirrel.SelectBuilder
		CountBuilder(field string) squirrel.SelectBuilder
		SumBuilder(field string) squirrel.SelectBuilder
		FindOneByQuery(ctx context.Context, rowBuilder squirrel.SelectBuilder) (*ChatConfig, error)
		FindSum(ctx context.Context, sumBuilder squirrel.SelectBuilder) (float64, error)
		FindCount(ctx context.Context, countBuilder squirrel.SelectBuilder) (int64, error)
		FindAll(ctx context.Context, rowBuilder squirrel.SelectBuilder) ([]*ChatConfig, error)
		FindPageListByPage(ctx context.Context, rowBuilder squirrel.SelectBuilder, p, s int) ([]*ChatConfig, error)
	}

	customChatConfigModel struct {
		*defaultChatConfigModel
	}
)

// NewChatConfigModel returns a model for the database table.
func NewChatConfigModel(conn sqlx.SqlConn, c cache.CacheConf) ChatConfigModel {
	return &customChatConfigModel{
		defaultChatConfigModel: newChatConfigModel(conn, c),
	}
}

func (m *defaultChatConfigModel) FindOneByQuery(ctx context.Context, rowBuilder squirrel.SelectBuilder) (*ChatConfig, error) {

	query, values, err := rowBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	var resp ChatConfig
	err = m.QueryRowNoCacheCtx(ctx, &resp, query, values...)
	switch err {
	case nil:
		return &resp, nil
	default:
		return nil, err
	}
}

func (m *defaultChatConfigModel) FindSum(ctx context.Context, sumBuilder squirrel.SelectBuilder) (float64, error) {

	query, values, err := sumBuilder.ToSql()
	if err != nil {
		return 0, err
	}

	var resp float64
	err = m.QueryRowNoCacheCtx(ctx, &resp, query, values...)
	switch err {
	case nil:
		return resp, nil
	default:
		return 0, err
	}
}

func (m *defaultChatConfigModel) FindCount(ctx context.Context, countBuilder squirrel.SelectBuilder) (int64, error) {

	query, values, err := countBuilder.ToSql()
	if err != nil {
		return 0, err
	}

	var resp int64
	err = m.QueryRowNoCacheCtx(ctx, &resp, query, values...)
	switch err {
	case nil:
		return resp, nil
	default:
		return 0, err
	}
}

func (m *defaultChatConfigModel) FindAll(ctx context.Context, rowBuilder squirrel.SelectBuilder) ([]*ChatConfig, error) {

	query, values, err := rowBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	var resp []*ChatConfig
	err = m.QueryRowsNoCacheCtx(ctx, &resp, query, values...)
	switch err {
	case nil:
		return resp, nil
	default:
		return nil, err
	}
}

func (m *defaultChatConfigModel) FindPageListByPage(ctx context.Context, rowBuilder squirrel.SelectBuilder, p, s int) ([]*ChatConfig, error) {

	pg := page.NewPage(p, s)

	query, values, err := rowBuilder.Limit(pg.Limit()).Offset(pg.Offset()).ToSql()
	if err != nil {
		return nil, err
	}

	var resp []*ChatConfig
	err = m.QueryRowsNoCacheCtx(ctx, &resp, query, values...)
	switch err {
	case nil:
		return resp, nil
	default:
		return nil, err
	}
}

// export logic
func (m *defaultChatConfigModel) Trans(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {

	return m.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return fn(ctx, session)
	})

}

// export logic
func (m *defaultChatConfigModel) RowBuilder() squirrel.SelectBuilder {
	return squirrel.Select(chatConfigRows).From(m.table)
}

// export logic
func (m *defaultChatConfigModel) CountBuilder(field string) squirrel.SelectBuilder {
	return squirrel.Select("COUNT(" + field + ")").From(m.table)
}

// export logic
func (m *defaultChatConfigModel) SumBuilder(field string) squirrel.SelectBuilder {
	return squirrel.Select("IFNULL(SUM(" + field + "),0)").From(m.table)
}
