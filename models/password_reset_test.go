package models

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordReset_GenerateToken(t *testing.T) {
	token, err := GenerateToken()
	require.NoError(t, err)
	assert.Len(t, token, 64, "hex of 32 bytes = 64 chars")

	hexRegex := regexp.MustCompile(`^[0-9a-f]{64}$`)
	assert.True(t, hexRegex.MatchString(token), "token should be hex string")
}

func TestPasswordReset_IsExpired(t *testing.T) {
	now := time.Now()

	p := &PasswordReset{ExpiresAt: now.Add(-time.Hour)}
	assert.True(t, p.IsExpired())

	p2 := &PasswordReset{ExpiresAt: now.Add(time.Hour)}
	assert.False(t, p2.IsExpired())
}

func TestPasswordReset_IsValid(t *testing.T) {
	now := time.Now()

	// 有效
	p := &PasswordReset{Used: false, ExpiresAt: now.Add(time.Hour)}
	assert.True(t, p.IsValid())

	// 无效：已使用
	p2 := &PasswordReset{Used: true, ExpiresAt: now.Add(time.Hour)}
	assert.False(t, p2.IsValid())

	// 无效：已过期
	p3 := &PasswordReset{Used: false, ExpiresAt: now.Add(-time.Hour)}
	assert.False(t, p3.IsValid())
}
