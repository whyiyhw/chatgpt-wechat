package model

import (
	"context"

	"chat/common/page"

	"github.com/Masterminds/squirrel"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PromptConfigModel = (*customPromptConfigModel)(nil)

type (
	// PromptConfigModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPromptConfigModel.
	PromptConfigModel interface {
		promptConfigModel
		Trans(ctx context.Context, fn func(context context.Context, session sqlx.Session) error) error
		RowBuilder() squirrel.SelectBuilder
		CountBuilder(field string) squirrel.SelectBuilder
		SumBuilder(field string) squirrel.SelectBuilder
		FindOneByQuery(ctx context.Context, rowBuilder squirrel.SelectBuilder) (*PromptConfig, error)
		FindSum(ctx context.Context, sumBuilder squirrel.SelectBuilder) (float64, error)
		FindCount(ctx context.Context, countBuilder squirrel.SelectBuilder) (int64, error)
		FindAll(ctx context.Context, rowBuilder squirrel.SelectBuilder) ([]*PromptConfig, error)
		FindPageListByPage(ctx context.Context, rowBuilder squirrel.SelectBuilder, p, s int) ([]*PromptConfig, error)
	}

	customPromptConfigModel struct {
		*defaultPromptConfigModel
	}
)

// NewPromptConfigModel returns a model for the database table.
func NewPromptConfigModel(conn sqlx.SqlConn, c cache.CacheConf) PromptConfigModel {
	return &customPromptConfigModel{
		defaultPromptConfigModel: newPromptConfigModel(conn, c),
	}
}

func (m *defaultPromptConfigModel) FindOneByQuery(ctx context.Context, rowBuilder squirrel.SelectBuilder) (*PromptConfig, error) {

	query, values, err := rowBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	var resp PromptConfig
	err = m.QueryRowNoCacheCtx(ctx, &resp, query, values...)
	switch err {
	case nil:
		return &resp, nil
	default:
		return nil, err
	}
}

func (m *defaultPromptConfigModel) FindSum(ctx context.Context, sumBuilder squirrel.SelectBuilder) (float64, error) {

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

func (m *defaultPromptConfigModel) FindCount(ctx context.Context, countBuilder squirrel.SelectBuilder) (int64, error) {

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

func (m *defaultPromptConfigModel) FindAll(ctx context.Context, rowBuilder squirrel.SelectBuilder) ([]*PromptConfig, error) {

	query, values, err := rowBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	var resp []*PromptConfig
	err = m.QueryRowsNoCacheCtx(ctx, &resp, query, values...)
	switch err {
	case nil:
		return resp, nil
	default:
		return nil, err
	}
}

func (m *defaultPromptConfigModel) FindPageListByPage(ctx context.Context, rowBuilder squirrel.SelectBuilder, p, s int) ([]*PromptConfig, error) {

	pg := page.NewPage(p, s)

	query, values, err := rowBuilder.Limit(pg.Limit()).Offset(pg.Offset()).ToSql()
	if err != nil {
		return nil, err
	}

	var resp []*PromptConfig
	err = m.QueryRowsNoCacheCtx(ctx, &resp, query, values...)
	switch err {
	case nil:
		return resp, nil
	default:
		return nil, err
	}
}

// export logic
func (m *defaultPromptConfigModel) Trans(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {

	return m.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return fn(ctx, session)
	})

}

// export logic
func (m *defaultPromptConfigModel) RowBuilder() squirrel.SelectBuilder {
	return squirrel.Select(promptConfigRows).From(m.table)
}

// export logic
func (m *defaultPromptConfigModel) CountBuilder(field string) squirrel.SelectBuilder {
	return squirrel.Select("COUNT(" + field + ")").From(m.table)
}

// export logic
func (m *defaultPromptConfigModel) SumBuilder(field string) squirrel.SelectBuilder {
	return squirrel.Select("IFNULL(SUM(" + field + "),0)").From(m.table)
}
