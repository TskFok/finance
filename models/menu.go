package models

import (
	"time"

	"gorm.io/gorm"
)

// Menu 菜单模型
type Menu struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	ParentID  uint           `json:"parent_id" gorm:"default:0;index"` // 0 表示顶级
	Name      string         `json:"name" gorm:"size:50;not null"`
	Path      string         `json:"path" gorm:"size:100;not null;index"` // 前端路由/code，如 dashboard、expenses
	Icon      string         `json:"icon" gorm:"size:50"`                 // FontAwesome class，如 fa-chart-pie
	SortOrder int            `json:"sort_order" gorm:"default:0;index"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 设置表名
func (Menu) TableName() string {
	return "menus"
}
