package models

import (
	"time"

	"gorm.io/gorm"
)

// Role 角色模型
type Role struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"size:50;not null"`
	Code        string         `json:"code" gorm:"size:50;not null;uniqueIndex"`
	Description string         `json:"description" gorm:"size:255"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 设置表名
func (Role) TableName() string {
	return "roles"
}
