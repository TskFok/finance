package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	// UserStatusLocked 锁定：不可登录
	UserStatusLocked = "locked"
	// UserStatusActive 正常：可登录
	UserStatusActive = "active"
)

// User 用户模型
type User struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Username     string         `json:"username" gorm:"uniqueIndex;size:50;not null"`
	Password     string         `json:"-" gorm:"size:255;not null"`
	Email        string         `json:"email" gorm:"size:100"`
	IsAdmin      bool           `json:"is_admin" gorm:"default:false;index"`        // 是否为管理员
	Status       string         `json:"status" gorm:"size:20;default:locked;index"` // 用户状态：locked/active
	FeishuOpenID  *string `json:"feishu_open_id,omitempty" gorm:"size:64;uniqueIndex"` // 飞书 open_id，NULL 表示未绑定
	FeishuUnionID string  `json:"-" gorm:"size:64;index;default:''"`                   // 飞书 union_id
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 设置表名
func (User) TableName() string {
	return "users"
}
