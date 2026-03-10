package adminauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"finance/config"

	"github.com/gin-gonic/gin"
)

const (
	cookieAdminUserID     = "admin_user_id"
	cookieOriginalAdminID = "original_admin_id"
	defaultSecret         = "default-cookie-secret"
)

func getSecret() []byte {
	cfg := config.GetConfig()
	if cfg != nil && cfg.JWT.Secret != "" {
		return []byte(cfg.JWT.Secret)
	}
	return []byte(defaultSecret)
}

// SignCookieValue 对 value 进行 HMAC-SHA256 签名，返回 "value.hex(signature)" 格式
func SignCookieValue(value string) string {
	secret := getSecret()
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(value))
	sig := hex.EncodeToString(mac.Sum(nil))
	return value + "." + sig
}

// VerifyCookieValue 验证签名并返回原始 value
func VerifyCookieValue(signed string) (string, error) {
	if signed == "" {
		return "", errors.New("empty signed value")
	}
	idx := strings.LastIndex(signed, ".")
	if idx <= 0 {
		return "", errors.New("invalid format: no signature")
	}
	value := signed[:idx]
	sigHex := signed[idx+1:]

	secret := getSecret()
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(value))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sigHex), []byte(expectedSig)) {
		return "", fmt.Errorf("signature mismatch")
	}
	return value, nil
}

// GetVerifiedAdminUserID 从 admin_user_id cookie 验证签名并返回用户 ID
func GetVerifiedAdminUserID(c *gin.Context) (uint, error) {
	val, err := c.Cookie(cookieAdminUserID)
	if err != nil || val == "" {
		return 0, errors.New("no admin_user_id cookie")
	}
	raw, err := VerifyCookieValue(val)
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, errors.New("invalid user id")
	}
	return uint(id), nil
}

// GetVerifiedOriginalAdminID 从 original_admin_id cookie 验证签名并返回原始管理员 ID（模拟登录场景）
func GetVerifiedOriginalAdminID(c *gin.Context) (uint, error) {
	val, err := c.Cookie(cookieOriginalAdminID)
	if err != nil || val == "" {
		return 0, errors.New("no original_admin_id cookie")
	}
	raw, err := VerifyCookieValue(val)
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, errors.New("invalid admin id")
	}
	return uint(id), nil
}
