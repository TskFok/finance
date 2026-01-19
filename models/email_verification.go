package models

import (
	cryptoRand "crypto/rand"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// EmailVerification 邮箱验证令牌模型
type EmailVerification struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Email     string         `json:"email" gorm:"index;size:100;not null"`
	Code      string         `json:"code" gorm:"size:6;not null"`        // 6位验证码
	Type      string         `json:"type" gorm:"size:20;not null;index"` // register: 注册验证, bind: 绑定邮箱
	ExpiresAt time.Time      `json:"expires_at" gorm:"not null"`
	Used      bool           `json:"used" gorm:"default:false"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 设置表名
func (EmailVerification) TableName() string {
	return "email_verifications"
}

// IsExpired 检查验证码是否过期
func (e *EmailVerification) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// IsValid 检查验证码是否有效
func (e *EmailVerification) IsValid() bool {
	return !e.Used && !e.IsExpired()
}

// GenerateVerificationCode 生成6位数字验证码
func GenerateVerificationCode() (string, error) {
	bytes := make([]byte, 3)
	if _, err := randRead(bytes); err != nil {
		return "", err
	}
	// 生成6位数字验证码
	code := int(bytes[0])<<16 | int(bytes[1])<<8 | int(bytes[2])
	code = code % 900000 + 100000 // 确保是6位数
	return fmt.Sprintf("%06d", code), nil
}

// 为了使用 crypto/rand
var randRead = func(b []byte) (int, error) {
	return cryptoRand.Read(b)
}

