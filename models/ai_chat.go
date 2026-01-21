package models

import (
	"time"

	"gorm.io/gorm"
)

// AIChatMessage AI聊天记录（单轮：用户输入 + AI输出）
type AIChatMessage struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	AIModelID uint           `json:"ai_model_id" gorm:"index;not null"`
	UserID    uint           `json:"user_id" gorm:"index;default:0"` // 发起聊天的用户ID（App端按用户隔离）
	UserText  string         `json:"user_text" gorm:"type:text;not null"`
	AIText    string         `json:"ai_text" gorm:"type:longtext;not null"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	AIModel AIModel `json:"-" gorm:"foreignKey:AIModelID"`
}

func (AIChatMessage) TableName() string {
	return "ai_chat_messages"
}
