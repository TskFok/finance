package models

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateVerificationCode(t *testing.T) {
	code, err := GenerateVerificationCode()
	require.NoError(t, err)
	assert.Len(t, code, 6)

	// 全为数字
	digitRegex := regexp.MustCompile(`^\d{6}$`)
	assert.True(t, digitRegex.MatchString(code), "code should be 6 digits")

	// 范围 100000-999999（GenerateVerificationCode 保证）
	assert.True(t, code >= "100000" && code <= "999999")
}

func TestEmailVerification_IsExpired(t *testing.T) {
	now := time.Now()

	// 已过期
	e := &EmailVerification{ExpiresAt: now.Add(-time.Hour)}
	assert.True(t, e.IsExpired())

	// 未过期
	e2 := &EmailVerification{ExpiresAt: now.Add(time.Hour)}
	assert.False(t, e2.IsExpired())
}

func TestEmailVerification_IsValid(t *testing.T) {
	now := time.Now()

	// 有效：未使用且未过期
	e := &EmailVerification{Used: false, ExpiresAt: now.Add(time.Hour)}
	assert.True(t, e.IsValid())

	// 无效：已使用
	e2 := &EmailVerification{Used: true, ExpiresAt: now.Add(time.Hour)}
	assert.False(t, e2.IsValid())

	// 无效：已过期
	e3 := &EmailVerification{Used: false, ExpiresAt: now.Add(-time.Hour)}
	assert.False(t, e3.IsValid())

	// 无效：已使用且过期
	e4 := &EmailVerification{Used: true, ExpiresAt: now.Add(-time.Hour)}
	assert.False(t, e4.IsValid())
}
