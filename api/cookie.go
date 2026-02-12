package api

import (
	"net/http"
	"strings"

	"finance/config"
)

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
