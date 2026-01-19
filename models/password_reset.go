package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"gorm.io/gorm"
)

// PasswordReset 密码重置令牌模型
type PasswordReset struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	UserID    uint           `json:"user_id" gorm:"index;not null"`
	Token     string         `json:"token" gorm:"uniqueIndex;size:64;not null"`
	Email     string         `json:"email" gorm:"size:100;not null"`
	ExpiresAt time.Time      `json:"expires_at" gorm:"not null"`
	Used      bool           `json:"used" gorm:"default:false"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
	User      User           `json:"-" gorm:"foreignKey:UserID"`
}

// TableName 设置表名
func (PasswordReset) TableName() string {
	return "password_resets"
}

// GenerateToken 生成随机令牌
func GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// IsExpired 检查令牌是否过期
func (p *PasswordReset) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}

// IsValid 检查令牌是否有效
func (p *PasswordReset) IsValid() bool {
	return !p.Used && !p.IsExpired()
}

