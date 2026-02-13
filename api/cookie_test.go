package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"finance/adminauth"
	"finance/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initCookieTestConfig(mode string, jwtSecret string) {
	config.GlobalConfig = &config.Config{
		Server: config.ServerConfig{Mode: mode},
		JWT:    config.JWTConfig{Secret: jwtSecret},
	}
}

func TestSignCookieValue(t *testing.T) {
	initCookieTestConfig("debug", "test-secret")
	defer func() { config.GlobalConfig = nil }()

	// 相同输入得到相同签名
	signed1 := adminauth.SignCookieValue("123")
	signed2 := adminauth.SignCookieValue("123")
	assert.Equal(t, signed1, signed2)
	assert.Contains(t, signed1, ".")
	assert.Equal(t, "123", signed1[:3])

	// 空 secret 使用默认值
	initCookieTestConfig("debug", "")
	signed := adminauth.SignCookieValue("abc")
	assert.NotEmpty(t, signed)
	assert.Contains(t, signed, ".")
	assert.True(t, len(signed) > len("abc")+1)
}

func TestVerifyCookieValue(t *testing.T) {
	initCookieTestConfig("debug", "test-secret")
	defer func() { config.GlobalConfig = nil }()

	// 合法签名返回 value
	signed := adminauth.SignCookieValue("user123")
	value, err := adminauth.VerifyCookieValue(signed)
	require.NoError(t, err)
	assert.Equal(t, "user123", value)

	// 空值返回错误
	_, err = adminauth.VerifyCookieValue("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")

	// 格式错误（无点号）返回错误
	_, err = adminauth.VerifyCookieValue("novalue")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")

	// 格式错误（点号在开头）返回错误
	_, err = adminauth.VerifyCookieValue(".sigonly")
	assert.Error(t, err)

	// 篡改 value 后签名无效
	tampered := "hacker.0000000000000000000000000000000000000000000000000000000000000000"
	_, err = adminauth.VerifyCookieValue(tampered)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature")
}

func TestEscapeLikeValue(t *testing.T) {
	// % 和 _ 正确转义
	assert.Equal(t, `\%`, escapeLikeValue("%"))
	assert.Equal(t, `\_`, escapeLikeValue("_"))
	assert.Equal(t, `\\`, escapeLikeValue(`\`))

	// 组合转义
	assert.Equal(t, `\%admin\%`, escapeLikeValue("%admin%"))
	assert.Equal(t, `\_test\_`, escapeLikeValue("_test_"))
	assert.Equal(t, `\\\%\_`, escapeLikeValue(`\%_`))

	// 空字符串
	assert.Equal(t, "", escapeLikeValue(""))

	// 普通字符串不变
	assert.Equal(t, "hello", escapeLikeValue("hello"))
}

func TestGetCookieOptions(t *testing.T) {
	// debug 模式 secure=false
	initCookieTestConfig("debug", "")
	defer func() { config.GlobalConfig = nil }()
	secure, sameSite := getCookieOptions()
	assert.False(t, secure)
	assert.Equal(t, http.SameSiteLaxMode, sameSite)

	// release 模式 secure=true
	initCookieTestConfig("release", "")
	secure, sameSite = getCookieOptions()
	assert.True(t, secure)
	assert.Equal(t, http.SameSiteLaxMode, sameSite)
}

func TestGetVerifiedAdminUserID(t *testing.T) {
	initCookieTestConfig("debug", "test-secret")
	defer func() { config.GlobalConfig = nil }()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 正常 Cookie
	signed := adminauth.SignCookieValue("42")
	router.GET("/ok", func(c *gin.Context) {
		c.SetCookie("admin_user_id", signed, 86400, "/", "", false, true)
		c.String(200, "ok")
	})
	router.GET("/verify", func(c *gin.Context) {
		id, err := GetVerifiedAdminUserID(c)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		c.String(200, "id:%d", id)
	})

	req := httptest.NewRequest("GET", "/ok", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	req2 := httptest.NewRequest("GET", "/verify", nil)
	for _, c := range w.Result().Cookies() {
		req2.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value, Expires: time.Now().Add(24 * time.Hour)})
	}
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code)
	assert.Equal(t, "id:42", w2.Body.String())

	// 无 Cookie
	req3 := httptest.NewRequest("GET", "/verify", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	assert.Equal(t, 500, w3.Code)

	// 非法 Cookie（篡改）
	router2 := gin.New()
	router2.GET("/bad", func(c *gin.Context) {
		c.SetCookie("admin_user_id", "1.invalidsig", 86400, "/", "", false, true)
		c.String(200, "ok")
	})
	router2.GET("/v", func(c *gin.Context) {
		_, err := GetVerifiedAdminUserID(c)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		c.String(200, "ok")
	})
	req4 := httptest.NewRequest("GET", "/bad", nil)
	wr := httptest.NewRecorder()
	router2.ServeHTTP(wr, req4)
	req5 := httptest.NewRequest("GET", "/v", nil)
	for _, c := range wr.Result().Cookies() {
		req5.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}
	w5 := httptest.NewRecorder()
	router2.ServeHTTP(w5, req5)
	assert.Equal(t, 500, w5.Code)
}

func TestGetVerifiedOriginalAdminID(t *testing.T) {
	initCookieTestConfig("debug", "test-secret")
	defer func() { config.GlobalConfig = nil }()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	signed := adminauth.SignCookieValue("100")
	router.GET("/set", func(c *gin.Context) {
		c.SetCookie("original_admin_id", signed, 86400, "/", "", false, true)
		c.String(200, "ok")
	})
	router.GET("/get", func(c *gin.Context) {
		id, err := GetVerifiedOriginalAdminID(c)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		c.String(200, "%d", id)
	})

	req := httptest.NewRequest("GET", "/set", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	req2 := httptest.NewRequest("GET", "/get", nil)
	for _, c := range w.Result().Cookies() {
		req2.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value, Expires: time.Now().Add(24 * time.Hour)})
	}
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code)
	assert.Equal(t, "100", w2.Body.String())
}
