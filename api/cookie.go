package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"finance/config"

	"github.com/gin-gonic/gin"
)

// signCookieValue 对敏感 Cookie 值进行 HMAC 签名，防止篡改
// 格式: value.signature，签名基于 value 和 JWT secret
func signCookieValue(value string) string {
	cfg := config.GetConfig()
	secret := cfg.JWT.Secret
	if secret == "" {
		secret = "finance-default-cookie-secret"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(value))
	sig := hex.EncodeToString(mac.Sum(nil))
	return value + "." + sig
}

// verifyCookieValue 验证签名并返回原始值，若签名无效返回错误
func verifyCookieValue(signed string) (string, error) {
	if signed == "" {
		return "", fmt.Errorf("empty cookie")
	}
	idx := strings.LastIndex(signed, ".")
	if idx <= 0 {
		return "", fmt.Errorf("invalid cookie format")
	}
	value := signed[:idx]
	signature := signed[idx+1:]
	cfg := config.GetConfig()
	secret := cfg.JWT.Secret
	if secret == "" {
		secret = "finance-default-cookie-secret"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(value))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return "", fmt.Errorf("cookie signature invalid")
	}
	return value, nil
}

// GetVerifiedAdminUserID 验证 admin_user_id cookie 签名并返回用户 ID
func GetVerifiedAdminUserID(c *gin.Context) (uint, error) {
	raw, err := c.Cookie("admin_user_id")
	if err != nil {
		return 0, err
	}
	value, err := verifyCookieValue(raw)
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid user id")
	}
	return uint(id), nil
}

// GetVerifiedOriginalAdminID 验证 original_admin_id cookie 签名并返回用户 ID
func GetVerifiedOriginalAdminID(c *gin.Context) (uint, error) {
	raw, err := c.Cookie("original_admin_id")
	if err != nil {
		return 0, err
	}
	value, err := verifyCookieValue(raw)
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid admin id")
	}
	return uint(id), nil
}

// escapeLikeValue 转义 LIKE 查询中的通配符 % 和 _，防止用户注入改变匹配语义
func escapeLikeValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)
	return s
}

// getCookieOptions 根据运行模式返回 Cookie 的安全选项
// release 模式下启用 Secure（仅 HTTPS 传输），并设置 SameSite 以防止 CSRF
func getCookieOptions() (secure bool, sameSite http.SameSite) {
	cfg := config.GetConfig()
	if cfg != nil && cfg.Server.Mode == "release" {
		secure = true
	}
	// SameSite=Lax: 防止跨站 POST 请求携带 Cookie，同时允许同站导航
	sameSite = http.SameSiteLaxMode
	return
}
