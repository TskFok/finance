package api

import (
	"net/http"
	"time"

	"finance/config"
	"finance/database"
	"finance/models"
	"finance/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// PasswordResetHandler 密码重置处理器
type PasswordResetHandler struct {
	cfg          *config.Config
	emailService *service.EmailService
}

// NewPasswordResetHandler 创建密码重置处理器
func NewPasswordResetHandler(cfg *config.Config) *PasswordResetHandler {
	return &PasswordResetHandler{
		cfg:          cfg,
		emailService: service.NewEmailService(&cfg.Email),
	}
}

// RequestResetRequest 请求重置密码
type RequestResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// AdminResetPasswordRequest 管理员直接重置密码请求
type AdminResetPasswordRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// RequestPasswordReset 请求密码重置（发送邮件）
// @Summary 请求密码重置
// @Description 通过邮箱请求密码重置，系统会发送包含重置链接的邮件。为了安全，即使用户不存在也返回成功。
// @Tags 后台管理-密码重置
// @Accept json
// @Produce json
// @Param request body RequestResetRequest true "邮箱地址"
// @Success 200 {object} map[string]interface{} "请求成功（无论用户是否存在）"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 500 {object} map[string]interface{} "邮件发送失败"
// @Router /admin/password/request-reset [post]
func (h *PasswordResetHandler) RequestPasswordReset(c *gin.Context) {
	var req RequestResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请输入有效的邮箱地址"})
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// 为了安全，即使用户不存在也返回成功
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "如果该邮箱已注册，您将收到密码重置邮件",
		})
		return
	}

	// 检查是否有未使用的有效令牌
	var existingToken models.PasswordReset
	if err := database.DB.Where("user_id = ? AND used = ? AND expires_at > ?", user.ID, false, time.Now()).First(&existingToken).Error; err == nil {
		// 已有有效令牌，提示用户检查邮箱
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "已发送过重置邮件，请检查您的邮箱（包括垃圾邮件）",
		})
		return
	}

	// 生成新令牌
	token, err := models.GenerateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "生成令牌失败"})
		return
	}

	// 保存令牌
	passwordReset := models.PasswordReset{
		UserID:    user.ID,
		Token:     token,
		Email:     req.Email,
		ExpiresAt: time.Now().Add(30 * time.Minute), // 30分钟有效期
	}

	if err := database.DB.Create(&passwordReset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建重置令牌失败"})
		return
	}

	// 生成重置链接
	resetLink := h.cfg.Server.BaseURL + "/#/reset-password?token=" + token

	// 发送邮件
	if err := h.emailService.SendPasswordResetEmail(req.Email, user.Username, resetLink); err != nil {
		// 邮件发送失败，删除令牌
		database.DB.Delete(&passwordReset)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "邮件发送失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "密码重置邮件已发送，请检查您的邮箱",
	})
}

// VerifyResetToken 验证重置令牌
// @Summary 验证重置令牌
// @Description 验证密码重置令牌是否有效，返回关联的用户信息
// @Tags 后台管理-密码重置
// @Produce json
// @Param token query string true "重置令牌"
// @Success 200 {object} map[string]interface{} "令牌有效，返回用户信息"
// @Failure 400 {object} map[string]interface{} "令牌无效、已使用或已过期"
// @Router /admin/password/verify-token [get]
func (h *PasswordResetHandler) VerifyResetToken(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "缺少令牌"})
		return
	}

	var passwordReset models.PasswordReset
	if err := database.DB.Where("token = ?", token).First(&passwordReset).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的令牌"})
		return
	}

	if !passwordReset.IsValid() {
		message := "令牌已失效"
		if passwordReset.Used {
			message = "该令牌已被使用"
		} else if passwordReset.IsExpired() {
			message = "令牌已过期，请重新申请"
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	// 获取用户信息
	var user models.User
	database.DB.First(&user, passwordReset.UserID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"email":    passwordReset.Email,
			"username": user.Username,
		},
	})
}

// ResetPassword 重置密码
// @Summary 重置密码
// @Description 使用有效的重置令牌设置新密码
// @Tags 后台管理-密码重置
// @Accept json
// @Produce json
// @Param request body ResetPasswordRequest true "重置密码信息"
// @Success 200 {object} map[string]interface{} "密码重置成功"
// @Failure 400 {object} map[string]interface{} "参数错误或令牌无效"
// @Router /admin/password/reset [post]
func (h *PasswordResetHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 查找令牌
	var passwordReset models.PasswordReset
	if err := database.DB.Where("token = ?", req.Token).First(&passwordReset).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的令牌"})
		return
	}

	// 验证令牌
	if !passwordReset.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "令牌已过期或已使用"})
		return
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "密码加密失败"})
		return
	}

	// 更新密码
	if err := database.DB.Model(&models.User{}).Where("id = ?", passwordReset.UserID).Update("password", string(hashedPassword)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新密码失败"})
		return
	}

	// 标记令牌为已使用
	database.DB.Model(&passwordReset).Update("used", true)

	// 使该用户所有未使用的重置令牌失效
	database.DB.Model(&models.PasswordReset{}).
		Where("user_id = ? AND used = ?", passwordReset.UserID, false).
		Update("used", true)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "密码重置成功，请使用新密码登录",
	})
}

// AdminResetPassword 管理员直接重置用户密码
// @Summary 管理员重置用户密码
// @Description 管理员可以直接重置指定用户的密码，无需令牌
// @Tags 后台管理-密码重置
// @Accept json
// @Produce json
// @Param request body AdminResetPasswordRequest true "重置密码信息"
// @Success 200 {object} map[string]interface{} "密码重置成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Router /admin/password/admin-reset [post]
func (h *PasswordResetHandler) AdminResetPassword(c *gin.Context) {
	var req AdminResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "密码加密失败"})
		return
	}

	// 更新密码
	if err := database.DB.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "密码重置成功",
	})
}

// SendPasswordResetEmail 管理员发送密码重置邮件
// @Summary 管理员发送密码重置邮件
// @Description 管理员可以为指定用户发送密码重置邮件
// @Tags 后台管理-密码重置
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "用户ID" example({"user_id": 1})
// @Success 200 {object} map[string]interface{} "邮件发送成功"
// @Failure 400 {object} map[string]interface{} "参数错误或用户未设置邮箱"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Failure 500 {object} map[string]interface{} "邮件发送失败"
// @Router /admin/password/send-reset-email [post]
func (h *PasswordResetHandler) SendPasswordResetEmail(c *gin.Context) {
	type SendEmailRequest struct {
		UserID uint `json:"user_id" binding:"required"`
	}

	var req SendEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	if user.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "该用户未设置邮箱地址"})
		return
	}

	// 使旧令牌失效
	database.DB.Model(&models.PasswordReset{}).
		Where("user_id = ? AND used = ?", user.ID, false).
		Update("used", true)

	// 生成新令牌
	token, err := models.GenerateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "生成令牌失败"})
		return
	}

	// 保存令牌
	passwordReset := models.PasswordReset{
		UserID:    user.ID,
		Token:     token,
		Email:     user.Email,
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	if err := database.DB.Create(&passwordReset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建重置令牌失败"})
		return
	}

	// 生成重置链接
	resetLink := h.cfg.Server.BaseURL + "/#/reset-password?token=" + token

	// 发送邮件
	if err := h.emailService.SendPasswordResetEmail(user.Email, user.Username, resetLink); err != nil {
		database.DB.Delete(&passwordReset)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "邮件发送失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "密码重置邮件已发送至 " + user.Email,
	})
}

// GetEmailConfig 获取邮件配置状态
// @Summary 获取邮件配置状态
// @Description 获取邮件服务配置信息（邮箱地址会被部分隐藏）
// @Tags 后台管理-密码重置
// @Produce json
// @Success 200 {object} map[string]interface{} "获取成功，返回邮件配置信息"
// @Router /admin/email-config [get]
func (h *PasswordResetHandler) GetEmailConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":  h.cfg.Email.Enabled,
			"host":     h.cfg.Email.Host,
			"port":     h.cfg.Email.Port,
			"username": maskEmail(h.cfg.Email.Username),
		},
	})
}

// maskEmail 隐藏邮箱中间部分
func maskEmail(email string) string {
	if email == "" {
		return ""
	}
	if len(email) < 5 {
		return "****"
	}
	return email[:2] + "****" + email[len(email)-4:]
}
