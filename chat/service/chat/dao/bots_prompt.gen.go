// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.

package dao

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"gorm.io/gen"
	"gorm.io/gen/field"

	"gorm.io/plugin/dbresolver"

	"chat/service/chat/model"
)

func newBotsPrompt(db *gorm.DB, opts ...gen.DOOption) botsPrompt {
	_botsPrompt := botsPrompt{}

	_botsPrompt.botsPromptDo.UseDB(db, opts...)
	_botsPrompt.botsPromptDo.UseModel(&model.BotsPrompt{})

	tableName := _botsPrompt.botsPromptDo.TableName()
	_botsPrompt.ALL = field.NewAsterisk(tableName)
	_botsPrompt.ID = field.NewInt64(tableName, "id")
	_botsPrompt.BotID = field.NewInt64(tableName, "bot_id")
	_botsPrompt.Prompt = field.NewString(tableName, "prompt")
	_botsPrompt.CreatedAt = field.NewTime(tableName, "created_at")
	_botsPrompt.UpdatedAt = field.NewTime(tableName, "updated_at")

	_botsPrompt.fillFieldMap()

	return _botsPrompt
}

type botsPrompt struct {
	botsPromptDo botsPromptDo

	ALL       field.Asterisk
	ID        field.Int64  // 机器人初始设置ID
	BotID     field.Int64  // 机器人ID 关联 bots.id
	Prompt    field.String // 机器人初始设置
	CreatedAt field.Time   // 创建时间
	UpdatedAt field.Time   // 更新时间

	fieldMap map[string]field.Expr
}

func (b botsPrompt) Table(newTableName string) *botsPrompt {
	b.botsPromptDo.UseTable(newTableName)
	return b.updateTableName(newTableName)
}

func (b botsPrompt) As(alias string) *botsPrompt {
	b.botsPromptDo.DO = *(b.botsPromptDo.As(alias).(*gen.DO))
	return b.updateTableName(alias)
}

func (b *botsPrompt) updateTableName(table string) *botsPrompt {
	b.ALL = field.NewAsterisk(table)
	b.ID = field.NewInt64(table, "id")
	b.BotID = field.NewInt64(table, "bot_id")
	b.Prompt = field.NewString(table, "prompt")
	b.CreatedAt = field.NewTime(table, "created_at")
	b.UpdatedAt = field.NewTime(table, "updated_at")

	b.fillFieldMap()

	return b
}

func (b *botsPrompt) WithContext(ctx context.Context) *botsPromptDo {
	return b.botsPromptDo.WithContext(ctx)
}

func (b botsPrompt) TableName() string { return b.botsPromptDo.TableName() }

func (b botsPrompt) Alias() string { return b.botsPromptDo.Alias() }

func (b botsPrompt) Columns(cols ...field.Expr) gen.Columns { return b.botsPromptDo.Columns(cols...) }

func (b *botsPrompt) GetFieldByName(fieldName string) (field.OrderExpr, bool) {
	_f, ok := b.fieldMap[fieldName]
	if !ok || _f == nil {
		return nil, false
	}
	_oe, ok := _f.(field.OrderExpr)
	return _oe, ok
}

func (b *botsPrompt) fillFieldMap() {
	b.fieldMap = make(map[string]field.Expr, 5)
	b.fieldMap["id"] = b.ID
	b.fieldMap["bot_id"] = b.BotID
	b.fieldMap["prompt"] = b.Prompt
	b.fieldMap["created_at"] = b.CreatedAt
	b.fieldMap["updated_at"] = b.UpdatedAt
}

func (b botsPrompt) clone(db *gorm.DB) botsPrompt {
	b.botsPromptDo.ReplaceConnPool(db.Statement.ConnPool)
	return b
}

func (b botsPrompt) replaceDB(db *gorm.DB) botsPrompt {
	b.botsPromptDo.ReplaceDB(db)
	return b
}

type botsPromptDo struct{ gen.DO }

func (b botsPromptDo) Debug() *botsPromptDo {
	return b.withDO(b.DO.Debug())
}

func (b botsPromptDo) WithContext(ctx context.Context) *botsPromptDo {
	return b.withDO(b.DO.WithContext(ctx))
}

func (b botsPromptDo) ReadDB() *botsPromptDo {
	return b.Clauses(dbresolver.Read)
}

func (b botsPromptDo) WriteDB() *botsPromptDo {
	return b.Clauses(dbresolver.Write)
}

func (b botsPromptDo) Session(config *gorm.Session) *botsPromptDo {
	return b.withDO(b.DO.Session(config))
}

func (b botsPromptDo) Clauses(conds ...clause.Expression) *botsPromptDo {
	return b.withDO(b.DO.Clauses(conds...))
}

func (b botsPromptDo) Returning(value interface{}, columns ...string) *botsPromptDo {
	return b.withDO(b.DO.Returning(value, columns...))
}

func (b botsPromptDo) Not(conds ...gen.Condition) *botsPromptDo {
	return b.withDO(b.DO.Not(conds...))
}

func (b botsPromptDo) Or(conds ...gen.Condition) *botsPromptDo {
	return b.withDO(b.DO.Or(conds...))
}

func (b botsPromptDo) Select(conds ...field.Expr) *botsPromptDo {
	return b.withDO(b.DO.Select(conds...))
}

func (b botsPromptDo) Where(conds ...gen.Condition) *botsPromptDo {
	return b.withDO(b.DO.Where(conds...))
}

func (b botsPromptDo) Order(conds ...field.Expr) *botsPromptDo {
	return b.withDO(b.DO.Order(conds...))
}

func (b botsPromptDo) Distinct(cols ...field.Expr) *botsPromptDo {
	return b.withDO(b.DO.Distinct(cols...))
}

func (b botsPromptDo) Omit(cols ...field.Expr) *botsPromptDo {
	return b.withDO(b.DO.Omit(cols...))
}

func (b botsPromptDo) Join(table schema.Tabler, on ...field.Expr) *botsPromptDo {
	return b.withDO(b.DO.Join(table, on...))
}

func (b botsPromptDo) LeftJoin(table schema.Tabler, on ...field.Expr) *botsPromptDo {
	return b.withDO(b.DO.LeftJoin(table, on...))
}

func (b botsPromptDo) RightJoin(table schema.Tabler, on ...field.Expr) *botsPromptDo {
	return b.withDO(b.DO.RightJoin(table, on...))
}

func (b botsPromptDo) Group(cols ...field.Expr) *botsPromptDo {
	return b.withDO(b.DO.Group(cols...))
}

func (b botsPromptDo) Having(conds ...gen.Condition) *botsPromptDo {
	return b.withDO(b.DO.Having(conds...))
}

func (b botsPromptDo) Limit(limit int) *botsPromptDo {
	return b.withDO(b.DO.Limit(limit))
}

func (b botsPromptDo) Offset(offset int) *botsPromptDo {
	return b.withDO(b.DO.Offset(offset))
}

func (b botsPromptDo) Scopes(funcs ...func(gen.Dao) gen.Dao) *botsPromptDo {
	return b.withDO(b.DO.Scopes(funcs...))
}

func (b botsPromptDo) Unscoped() *botsPromptDo {
	return b.withDO(b.DO.Unscoped())
}

func (b botsPromptDo) Create(values ...*model.BotsPrompt) error {
	if len(values) == 0 {
		return nil
	}
	return b.DO.Create(values)
}

func (b botsPromptDo) CreateInBatches(values []*model.BotsPrompt, batchSize int) error {
	return b.DO.CreateInBatches(values, batchSize)
}

// Save : !!! underlying implementation is different with GORM
// The method is equivalent to executing the statement: db.Clauses(clause.OnConflict{UpdateAll: true}).Create(values)
func (b botsPromptDo) Save(values ...*model.BotsPrompt) error {
	if len(values) == 0 {
		return nil
	}
	return b.DO.Save(values)
}

func (b botsPromptDo) First() (*model.BotsPrompt, error) {
	if result, err := b.DO.First(); err != nil {
		return nil, err
	} else {
		return result.(*model.BotsPrompt), nil
	}
}

func (b botsPromptDo) Take() (*model.BotsPrompt, error) {
	if result, err := b.DO.Take(); err != nil {
		return nil, err
	} else {
		return result.(*model.BotsPrompt), nil
	}
}

func (b botsPromptDo) Last() (*model.BotsPrompt, error) {
	if result, err := b.DO.Last(); err != nil {
		return nil, err
	} else {
		return result.(*model.BotsPrompt), nil
	}
}

func (b botsPromptDo) Find() ([]*model.BotsPrompt, error) {
	result, err := b.DO.Find()
	return result.([]*model.BotsPrompt), err
}

func (b botsPromptDo) FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*model.BotsPrompt, err error) {
	buf := make([]*model.BotsPrompt, 0, batchSize)
	err = b.DO.FindInBatches(&buf, batchSize, func(tx gen.Dao, batch int) error {
		defer func() { results = append(results, buf...) }()
		return fc(tx, batch)
	})
	return results, err
}

func (b botsPromptDo) FindInBatches(result *[]*model.BotsPrompt, batchSize int, fc func(tx gen.Dao, batch int) error) error {
	return b.DO.FindInBatches(result, batchSize, fc)
}

func (b botsPromptDo) Attrs(attrs ...field.AssignExpr) *botsPromptDo {
	return b.withDO(b.DO.Attrs(attrs...))
}

func (b botsPromptDo) Assign(attrs ...field.AssignExpr) *botsPromptDo {
	return b.withDO(b.DO.Assign(attrs...))
}

func (b botsPromptDo) Joins(fields ...field.RelationField) *botsPromptDo {
	for _, _f := range fields {
		b = *b.withDO(b.DO.Joins(_f))
	}
	return &b
}

func (b botsPromptDo) Preload(fields ...field.RelationField) *botsPromptDo {
	for _, _f := range fields {
		b = *b.withDO(b.DO.Preload(_f))
	}
	return &b
}

func (b botsPromptDo) FirstOrInit() (*model.BotsPrompt, error) {
	if result, err := b.DO.FirstOrInit(); err != nil {
		return nil, err
	} else {
		return result.(*model.BotsPrompt), nil
	}
}

func (b botsPromptDo) FirstOrCreate() (*model.BotsPrompt, error) {
	if result, err := b.DO.FirstOrCreate(); err != nil {
		return nil, err
	} else {
		return result.(*model.BotsPrompt), nil
	}
}

func (b botsPromptDo) FindByPage(offset int, limit int) (result []*model.BotsPrompt, count int64, err error) {
	result, err = b.Offset(offset).Limit(limit).Find()
	if err != nil {
		return
	}

	if size := len(result); 0 < limit && 0 < size && size < limit {
		count = int64(size + offset)
		return
	}

	count, err = b.Offset(-1).Limit(-1).Count()
	return
}

func (b botsPromptDo) ScanByPage(result interface{}, offset int, limit int) (count int64, err error) {
	count, err = b.Count()
	if err != nil {
		return
	}

	err = b.Offset(offset).Limit(limit).Scan(result)
	return
}

func (b botsPromptDo) Scan(result interface{}) (err error) {
	return b.DO.Scan(result)
}

func (b botsPromptDo) Delete(models ...*model.BotsPrompt) (result gen.ResultInfo, err error) {
	return b.DO.Delete(models)
}

func (b *botsPromptDo) withDO(do gen.Dao) *botsPromptDo {
	b.DO = *do.(*gen.DO)
	return b
}
