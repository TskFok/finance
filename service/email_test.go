package service

import (
	"testing"

	"finance/config"

	"github.com/stretchr/testify/assert"
)

func newTestEmailService() *EmailService {
	return NewEmailService(&config.EmailConfig{})
}

func TestGenerateResetEmailBody(t *testing.T) {
	s := newTestEmailService()
	body := s.generateResetEmailBody("张三", "https://example.com/reset?token=abc")
	assert.Contains(t, body, "张三")
	assert.Contains(t, body, "https://example.com/reset?token=abc")
	assert.Contains(t, body, "重置密码")
	assert.Contains(t, body, "30 分钟")
}

func TestGenerateVerificationEmailBody(t *testing.T) {
	s := newTestEmailService()

	// register
	body := s.generateVerificationEmailBody("123456", "register")
	assert.Contains(t, body, "123456")
	assert.Contains(t, body, "完成账号注册")

	// bind
	body2 := s.generateVerificationEmailBody("654321", "bind")
	assert.Contains(t, body2, "654321")
	assert.Contains(t, body2, "绑定您的邮箱")

	// admin_bind
	body3 := s.generateVerificationEmailBody("111222", "admin_bind")
	assert.Contains(t, body3, "111222")
	assert.Contains(t, body3, "绑定您的邮箱")

	// 默认 purpose
	body4 := s.generateVerificationEmailBody("000000", "other")
	assert.Contains(t, body4, "验证您的邮箱")
}

func TestGenerateAppResetEmailBody(t *testing.T) {
	s := newTestEmailService()
	body := s.generateAppResetEmailBody("李四", "888999")
	assert.Contains(t, body, "李四")
	assert.Contains(t, body, "888999")
	assert.Contains(t, body, "密码重置")
}
