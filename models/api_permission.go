package models

import (
	"time"

	"gorm.io/gorm"
)

// APIPermission 接口权限定义，用于权限校验
// Path 支持 :id 等占位符，如 /admin/users/:id
type APIPermission struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Method    string         `json:"method" gorm:"size:10;not null;index"` // GET, POST, PUT, DELETE
	Path      string         `json:"path" gorm:"size:255;not null;index"` // 如 /admin/expenses，支持 /admin/users/:id
	Desc      string         `json:"desc" gorm:"size:100"`                 // 接口描述
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 设置表名
func (APIPermission) TableName() string {
	return "api_permissions"
}
