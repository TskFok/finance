package middleware

import (
	"net/http"
	"strings"

	"finance/adminauth"
	"finance/database"
	"finance/models"

	"github.com/gin-gonic/gin"
)

// noPermissionCheckPaths 无需权限校验的路径（登录后获取身份/配置等）
var noPermissionCheckPaths = map[string]bool{
	"/admin/current-user":     true,
	"/admin/feishu/bind-token": true,
}

// AdminPermissionMiddleware 后台管理接口权限校验中间件
// 需在 AdminAuthMiddleware 之后使用。is_admin=true 超管绕过；否则根据角色菜单绑定的接口进行校验。
func AdminPermissionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if noPermissionCheckPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		userID, err := adminauth.GetVerifiedAdminUserID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "请先登录"})
			c.Abort()
			return
		}

		var user models.User
		if err := database.DB.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "用户不存在"})
			c.Abort()
			return
		}

		// 超管绕过
		if user.IsAdmin {
			c.Next()
			return
		}

		// 获取用户可访问的接口集合
		allowed := getUserAllowedAPIs(user.RoleID)
		if allowed == nil {
			allowed = getUserAllowedAPIs(getViewerRoleID())
		}

		method := c.Request.Method
		path := c.Request.URL.Path
		if matchAPIPermission(method, path, allowed) {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "权限不足",
		})
		c.Abort()
	}
}

// getUserAllowedAPIs 根据角色ID获取可访问的 (method, pathPattern) 集合
func getUserAllowedAPIs(roleID *uint) map[string]bool {
	if roleID == nil {
		return nil
	}
	var menuIDs []uint
	database.DB.Model(&models.RoleMenu{}).Where("role_id = ?", *roleID).Pluck("menu_id", &menuIDs)
	if len(menuIDs) == 0 {
		return nil
	}
	var apiIDs []uint
	database.DB.Model(&models.MenuAPI{}).Where("menu_id IN ?", menuIDs).Distinct("api_id").Pluck("api_id", &apiIDs)
	if len(apiIDs) == 0 {
		return nil
	}
	var apis []models.APIPermission
	database.DB.Where("id IN ?", apiIDs).Find(&apis)
	allowed := make(map[string]bool)
	for _, a := range apis {
		allowed[a.Method+" "+a.Path] = true
	}
	return allowed
}

func getViewerRoleID() *uint {
	var role models.Role
	if err := database.DB.Where("code = ?", "viewer").First(&role).Error; err != nil {
		return nil
	}
	return &role.ID
}

// matchAPIPermission 检查 method+path 是否匹配任一允许的 pattern
// pattern 格式如 GET /admin/users/:id，支持 :param 占位符匹配单段
func matchAPIPermission(method, path string, allowed map[string]bool) bool {
	if allowed == nil {
		return false
	}
	path = normalizePath(path)
	for key := range allowed {
		parts := strings.SplitN(key, " ", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] != method {
			continue
		}
		if matchPath(path, parts[1]) {
			return true
		}
	}
	return false
}

func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	return p
}

// matchPath 检查实际路径是否匹配 pattern（支持 :id 等占位符）
// /admin/users/123 匹配 /admin/users/:id
func matchPath(actual, pattern string) bool {
	actual = normalizePath(actual)
	pattern = normalizePath(pattern)
	a := splitPath(actual)
	p := splitPath(pattern)
	if len(a) != len(p) {
		return false
	}
	for i := range a {
		if len(p[i]) > 0 && p[i][0] == ':' {
			if a[i] == "" {
				return false
			}
			continue
		}
		if a[i] != p[i] {
			return false
		}
	}
	return true
}

func splitPath(s string) []string {
	s = strings.Trim(s, "/")
	if s == "" {
		return nil
	}
	return strings.Split(s, "/")
}
