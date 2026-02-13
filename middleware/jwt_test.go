package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"finance/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initJWTTestConfig() {
	config.GlobalConfig = &config.Config{
		Server: config.ServerConfig{Mode: "debug"},
		JWT:    config.JWTConfig{Secret: "test-jwt-secret-key"},
	}
}

func TestGenerateToken(t *testing.T) {
	initJWTTestConfig()
	defer func() { config.GlobalConfig = nil }()

	InitJWT(config.GlobalConfig)

	token, err := GenerateToken(1, "testuser", 24*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Greater(t, len(token), 20)

	// 可解析
	claims, err := ParseToken(token)
	require.NoError(t, err)
	assert.Equal(t, uint(1), claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
}

func TestParseToken(t *testing.T) {
	initJWTTestConfig()
	defer func() { config.GlobalConfig = nil }()

	InitJWT(config.GlobalConfig)

	// 合法 token
	token, _ := GenerateToken(100, "admin", time.Hour)
	claims, err := ParseToken(token)
	require.NoError(t, err)
	assert.Equal(t, uint(100), claims.UserID)
	assert.Equal(t, "admin", claims.Username)

	// 空字符串
	_, err = ParseToken("")
	assert.Error(t, err)

	// 无效格式
	_, err = ParseToken("not.a.valid.jwt")
	assert.Error(t, err)
	_, err = ParseToken("eyJhbGciOiJmb29iIn0.xxxx.yyyy")
	assert.Error(t, err)
}

func TestJWTAuth(t *testing.T) {
	initJWTTestConfig()
	defer func() { config.GlobalConfig = nil }()

	InitJWT(config.GlobalConfig)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		id := GetCurrentUserID(c)
		c.String(200, "id:%d", id)
	})

	// 无 token
	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "401")

	// 格式错误（非 Bearer）
	req2 := httptest.NewRequest("GET", "/protected", nil)
	req2.Header.Set("Authorization", "Basic xyz")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)

	// 格式错误（仅 Bearer 无 token）
	req3 := httptest.NewRequest("GET", "/protected", nil)
	req3.Header.Set("Authorization", "Bearer ")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusUnauthorized, w3.Code)

	// 有效 token
	token, _ := GenerateToken(42, "user42", time.Hour)
	req4 := httptest.NewRequest("GET", "/protected", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, req4)
	assert.Equal(t, 200, w4.Code)
	assert.Equal(t, "id:42", w4.Body.String())
}

func TestGetCurrentUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, uint(0), GetCurrentUserID(c))

	c.Set("userID", uint(99))
	assert.Equal(t, uint(99), GetCurrentUserID(c))
}
