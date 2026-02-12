package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"finance/config"
	"finance/database"
	"finance/models"
	"finance/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// 飞书绑定令牌存储（解决跨站重定向时 Cookie 不发送的问题）
var (
	feishuBindTokens     = make(map[string]feishuBindTokenEntry)
	feishuBindTokensMu   sync.RWMutex
	feishuBindTokenTTL   = 5 * time.Minute
)

type feishuBindTokenEntry struct {
	UserID    uint
	ExpiresAt time.Time
}

// FeishuAuthHandler 飞书扫码登录处理器
type FeishuAuthHandler struct {
	cfg *config.Config
}

// NewFeishuAuthHandler 创建飞书认证处理器
func NewFeishuAuthHandler(cfg *config.Config) *FeishuAuthHandler {
	return &FeishuAuthHandler{cfg: cfg}
}

// GetFeishuBindToken 获取飞书绑定用一次性令牌（需已登录）
// 绑定流程中飞书会重定向回本站，跨站请求可能导致 Cookie 不发送，故通过 state 携带令牌识别用户
// @Summary 获取飞书绑定令牌
// @Tags 后台管理
// @Produce json
// @Success 200 {object} map[string]interface{} "含 bind_token"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /admin/feishu/bind-token [get]
func (h *FeishuAuthHandler) GetFeishuBindToken(c *gin.Context) {
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "请先登录"})
		return
	}
	b := make([]byte, 24)
	rand.Read(b)
	token := hex.EncodeToString(b)
	feishuBindTokensMu.Lock()
	feishuBindTokens[token] = feishuBindTokenEntry{UserID: currentUser.ID, ExpiresAt: time.Now().Add(feishuBindTokenTTL)}
	// 清理过期条目
	for k, v := range feishuBindTokens {
		if time.Now().After(v.ExpiresAt) {
			delete(feishuBindTokens, k)
		}
	}
	feishuBindTokensMu.Unlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"bind_token": token},
	})
}

// GetFeishuConfig 获取飞书前端配置（app_id、redirect_uri、auth_url）
// @Summary 获取飞书扫码登录配置
// @Description 返回前端初始化二维码所需参数，仅当飞书登录已启用时有效
// @Tags 后台管理
// @Produce json
// @Success 200 {object} map[string]interface{} "配置信息"
// @Failure 400 {object} map[string]interface{} "飞书登录未启用"
// @Router /admin/feishu/config [get]
func (h *FeishuAuthHandler) GetFeishuConfig(c *gin.Context) {
	feishu := &h.cfg.Feishu
	if !feishu.Enabled || feishu.AppID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "飞书扫码登录未启用",
		})
		return
	}

	baseURL := h.cfg.Server.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost" + h.cfg.Server.Port
	}
	redirectURI := baseURL + "/admin/feishu/callback"
	state := c.Query("state") // 可选：bind 表示绑定流程
	authURL := service.BuildAuthURL(feishu.AppID, redirectURI, state)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"app_id":       feishu.AppID,
			"redirect_uri": redirectURI,
			"auth_url":     authURL,
		},
	})
}

// FeishuCallback 飞书 OAuth 回调
// @Summary 飞书授权回调
// @Description 飞书授权后重定向到此地址，使用 code 换取 token 并完成登录
// @Tags 后台管理
// @Param code query string true "授权码"
// @Success 302 "重定向到首页或登录页"
// @Failure 302 "重定向到登录页并携带 error 参数"
// @Router /admin/feishu/callback [get]
func (h *FeishuAuthHandler) FeishuCallback(c *gin.Context) {
	feishu := &h.cfg.Feishu
	if !feishu.Enabled || feishu.AppID == "" || feishu.AppSecret == "" {
		redirectToLogin(c, "飞书登录未配置")
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	if code == "" {
		redirectToLogin(c, "未获取到授权码")
		return
	}

	baseURL := h.cfg.Server.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost" + h.cfg.Server.Port
	}
	redirectURI := baseURL + "/admin/feishu/callback"

	// state=bind 或 state=bind:TOKEN 表示绑定流程
	if strings.HasPrefix(state, "bind") {
		var currentUser *models.User
		if strings.HasPrefix(state, "bind:") {
			token := strings.TrimPrefix(state, "bind:")
			feishuBindTokensMu.Lock()
			entry, ok := feishuBindTokens[token]
			if ok {
				delete(feishuBindTokens, token)
			}
			feishuBindTokensMu.Unlock()
			if ok && time.Now().Before(entry.ExpiresAt) {
				var u models.User
				if database.DB.First(&u, entry.UserID).Error == nil {
					currentUser = &u
				}
			}
		}
		if currentUser == nil {
			currentUser, _ = getCurrentUser(c)
		}
		h.handleFeishuBind(c, code, redirectURI, currentUser)
		return
	}

	h.handleFeishuLogin(c, code, redirectURI)
}

func (h *FeishuAuthHandler) handleFeishuLogin(c *gin.Context, code, redirectURI string) {
	feishu := &h.cfg.Feishu

	// 兑换 token
	tokenData, err := service.ExchangeToken(feishu.AppID, feishu.AppSecret, code, redirectURI)
	if err != nil {
		redirectToLogin(c, "飞书授权失败: "+err.Error())
		return
	}

	// 获取用户信息
	userInfo, err := service.GetUserInfo(tokenData.AccessToken)
	if err != nil {
		redirectToLogin(c, "获取飞书用户信息失败")
		return
	}

	if userInfo.OpenID == "" {
		redirectToLogin(c, "飞书返回的用户信息不完整")
		return
	}

	// 查找已绑定用户
	var user models.User
	if err := database.DB.Where("feishu_open_id = ?", userInfo.OpenID).First(&user).Error; err == nil {
		// 已绑定，检查状态后登录
		if user.Status != models.UserStatusActive {
			redirectToLogin(c, "账号已锁定，请联系管理员")
			return
		}
		setAdminCookies(c, &user)
		c.Redirect(http.StatusFound, "/")
		return
	}

	// 未绑定：根据配置决定是否自动创建
	if !feishu.AutoCreateUser {
		redirectToLogin(c, "该飞书账号未绑定系统用户，请联系管理员先绑定")
		return
	}

	// 自动创建用户
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(generateRandomPassword()), bcrypt.DefaultCost)
	if err != nil {
		redirectToLogin(c, "创建用户失败")
		return
	}

	username := "feishu_" + userInfo.OpenID
	if len(username) > 50 {
		username = username[:50]
	}
	if userInfo.Name != "" {
		// 尝试使用飞书名称，若冲突则加后缀
		candidate := userInfo.Name
		if len(candidate) > 47 {
			candidate = candidate[:47]
		}
		var exist models.User
		if database.DB.Where("username = ?", candidate).First(&exist).Error != nil {
			username = candidate
		}
	}

	openID := userInfo.OpenID
	user = models.User{
		Username:      username,
		Password:      string(hashedPassword),
		Email:         userInfo.Email,
		Status:        models.UserStatusLocked, // 飞书自动创建的账号默认锁定，需管理员解锁后才能登录
		FeishuOpenID:  &openID,
		FeishuUnionID: userInfo.UnionID,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		redirectToLogin(c, "创建用户失败，用户名可能已存在")
		return
	}

	// 飞书自动创建的账号默认锁定，不直接登录，需管理员解锁后再用飞书扫码登录
	redirectToLogin(c, "账号已创建，请联系管理员解锁后再登录")
}

// handleFeishuBind 处理飞书绑定（当前已登录用户绑定飞书 open_id）
// currentUser 可为 nil（跨站重定向时 Cookie 未发送），此时会提示请先登录
func (h *FeishuAuthHandler) handleFeishuBind(c *gin.Context, code, redirectURI string, currentUser *models.User) {
	feishu := &h.cfg.Feishu

	if currentUser == nil {
		redirectToLogin(c, "请先登录后再绑定飞书（若已登录，请从右上角「绑定飞书」重新扫码）")
		return
	}

	tokenData, err := service.ExchangeToken(feishu.AppID, feishu.AppSecret, code, redirectURI)
	if err != nil {
		redirectToLogin(c, "飞书授权失败: "+err.Error())
		return
	}

	userInfo, err := service.GetUserInfo(tokenData.AccessToken)
	if err != nil || userInfo.OpenID == "" {
		redirectToLogin(c, "获取飞书用户信息失败")
		return
	}

	// 检查该 open_id 是否已被其他用户绑定
	var other models.User
	if err := database.DB.Where("feishu_open_id = ? AND id != ?", userInfo.OpenID, currentUser.ID).First(&other).Error; err == nil {
		redirectToLogin(c, "该飞书账号已被其他用户绑定")
		return
	}

	openID := userInfo.OpenID
	if err := database.DB.Model(&currentUser).Updates(map[string]interface{}{
		"feishu_open_id":  &openID,
		"feishu_union_id": userInfo.UnionID,
	}).Error; err != nil {
		redirectToLogin(c, "绑定失败")
		return
	}

	// 绑定成功，重定向回首页
	c.Redirect(http.StatusFound, "/?feishu_bind=success")
}

func redirectToLogin(c *gin.Context, errMsg string) {
	u := "/?error=" + url.QueryEscape(errMsg)
	c.Redirect(http.StatusFound, u)
}

func setAdminCookies(c *gin.Context, user *models.User) {
	c.SetCookie("admin_user_id", fmt.Sprintf("%d", user.ID), 86400, "/", "", false, true)
	c.SetCookie("admin_username", user.Username, 86400, "/", "", false, false)
	c.SetCookie("admin_is_admin", fmt.Sprintf("%t", user.IsAdmin), 86400, "/", "", false, false)
}

func generateRandomPassword() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
