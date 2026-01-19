package models

import (
	"time"

	"gorm.io/gorm"
)

// AIModel AI模型配置
type AIModel struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name" gorm:"size:100;not null;uniqueIndex"` // 模型名称
	BaseURL   string         `json:"base_url" gorm:"size:255;not null"`        // 调用地址
	APIKey    string         `json:"-" gorm:"size:255;not null"`                // API密钥（不返回给前端）
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 设置表名
func (AIModel) TableName() string {
	return "ai_models"
}

