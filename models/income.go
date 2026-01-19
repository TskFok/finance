package models

import (
	"time"

	"gorm.io/gorm"
)

// Income 收入记录模型
type Income struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	UserID     uint           `json:"user_id" gorm:"index;not null"`
	Amount     float64        `json:"amount" gorm:"type:decimal(10,2);not null"`
	Type       string         `json:"type" gorm:"size:50;not null"` // 收入类型
	IncomeTime time.Time      `json:"income_time" gorm:"not null"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
	User       User           `json:"-" gorm:"foreignKey:UserID"`
}

func (Income) TableName() string {
	return "incomes"
}


