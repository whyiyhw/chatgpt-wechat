// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.

package model

import (
	"time"
)

const TableNameBotsWithCustom = "bots_with_custom"

// BotsWithCustom mapped from table <bots_with_custom>
type BotsWithCustom struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement:true;comment:机器人初始设置ID" json:"id"`        // 机器人初始设置ID
	BotID     int64     `gorm:"column:bot_id;not null;comment:机器人ID 关联 bots.id" json:"bot_id"`              // 机器人ID 关联 bots.id
	OpenKfID  string    `gorm:"column:open_kf_id;not null;comment:客服id 关联企业微信 客服ID" json:"open_kf_id"`      // 客服id 关联企业微信 客服ID
	CreatedAt time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"` // 创建时间
	UpdatedAt time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"` // 更新时间
}

// TableName BotsWithCustom's table name
func (*BotsWithCustom) TableName() string {
	return TableNameBotsWithCustom
}
