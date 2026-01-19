package models

import (
	"time"

	"gorm.io/gorm"
)

// Expense 消费记录模型
type Expense struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	UserID      uint           `json:"user_id" gorm:"index;not null"`
	Amount      float64        `json:"amount" gorm:"type:decimal(10,2);not null"`
	Category    string         `json:"category" gorm:"size:50;not null"`
	Description string         `json:"description" gorm:"size:255"`
	ExpenseTime time.Time      `json:"expense_time" gorm:"not null"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
	User        User           `json:"-" gorm:"foreignKey:UserID"`
}

// TableName 设置表名
func (Expense) TableName() string {
	return "expenses"
}

// Category 消费类别常量
const (
	CategoryFood          = "餐饮"
	CategoryTransport     = "交通"
	CategoryShopping      = "购物"
	CategoryEntertainment = "娱乐"
	CategoryMedical       = "医疗"
	CategoryEducation     = "教育"
	CategoryHousing       = "住房"
	CategoryOther         = "其他"
)

// GetCategories 获取所有消费类别
func GetCategories() []string {
	return []string{
		CategoryFood,
		CategoryTransport,
		CategoryShopping,
		CategoryEntertainment,
		CategoryMedical,
		CategoryEducation,
		CategoryHousing,
		CategoryOther,
	}
}

