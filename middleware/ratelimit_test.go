package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLoginRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 短窗口 200ms，最多 2 次
	router := gin.New()
	router.Use(LoginRateLimit(2, 200*time.Millisecond))
	router.POST("/login", func(c *gin.Context) {
		c.String(200, "ok")
	})

	// 同一 IP 连续 3 次，第 3 次应返回 429
	doReq := func(ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/login", nil)
		req.Header.Set("X-Real-IP", ip)
		req.RemoteAddr = ip + ":12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	w1 := doReq("192.168.1.1")
	w2 := doReq("192.168.1.1")
	w3 := doReq("192.168.1.1")

	assert.Equal(t, 200, w1.Code)
	assert.Equal(t, 200, w2.Code)
	assert.Equal(t, http.StatusTooManyRequests, w3.Code)
	assert.Contains(t, w3.Body.String(), "频繁")

	// 不同 IP 互不影响
	w4 := doReq("192.168.1.2")
	w5 := doReq("192.168.1.2")
	assert.Equal(t, 200, w4.Code)
	assert.Equal(t, 200, w5.Code)
}
