package models

import (
	"time"

	"gorm.io/gorm"
)

// IncomeCategory 收入类别（后台维护）
type IncomeCategory struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name" gorm:"size:50;not null;uniqueIndex"`
	Sort      int            `json:"sort" gorm:"default:0;index"`
	Color     string         `json:"color" gorm:"size:20;default:#64748b"` // 颜色代码，如 #10b981
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (IncomeCategory) TableName() string {
	return "income_categories"
}
