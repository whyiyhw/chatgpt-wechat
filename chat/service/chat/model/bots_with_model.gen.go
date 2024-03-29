// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.

package model

import (
	"time"
)

const TableNameBotsWithModel = "bots_with_model"

// BotsWithModel mapped from table <bots_with_model>
type BotsWithModel struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement:true;comment:机器人初始设置ID" json:"id"`        // 机器人初始设置ID
	BotID       int64     `gorm:"column:bot_id;not null;comment:机器人ID 关联 bots.id" json:"bot_id"`              // 机器人ID 关联 bots.id
	ModelType   string    `gorm:"column:model_type;not null;comment:模型类型 openai/gemini" json:"model_type"`    // 模型类型 openai/gemini
	ModelName   string    `gorm:"column:model_name;not null;comment:模型名称" json:"model_name"`                  // 模型名称
	Temperature float64   `gorm:"column:temperature;not null;default:0.00;comment:温度" json:"temperature"`     // 温度
	CreatedAt   time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"` // 创建时间
	UpdatedAt   time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"` // 更新时间
}

// TableName BotsWithModel's table name
func (*BotsWithModel) TableName() string {
	return TableNameBotsWithModel
}
