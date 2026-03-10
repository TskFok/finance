package adminauth

import (
	"testing"

	"finance/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestConfig(secret string) {
	config.GlobalConfig = &config.Config{JWT: config.JWTConfig{Secret: secret}}
}

func TestSignCookieValue(t *testing.T) {
	initTestConfig("test-secret")
	defer func() { config.GlobalConfig = nil }()

	signed := SignCookieValue("user123")
	assert.Contains(t, signed, ".")
	assert.Equal(t, "user123", signed[:7])
}

func TestVerifyCookieValue(t *testing.T) {
	initTestConfig("test-secret")
	defer func() { config.GlobalConfig = nil }()

	signed := SignCookieValue("ok")
	val, err := VerifyCookieValue(signed)
	require.NoError(t, err)
	assert.Equal(t, "ok", val)

	_, err = VerifyCookieValue("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")

	_, err = VerifyCookieValue("x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")

	_, err = VerifyCookieValue(".onlysig")
	assert.Error(t, err)
}
