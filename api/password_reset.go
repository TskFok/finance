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

// ResetPasswordRequest 重置密码请求（验证码流程）
type ResetPasswordRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Code        string `json:"code" binding:"required,len=6"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// AdminResetPasswordRequest 管理员直接重置密码请求
type AdminResetPasswordRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// RequestPasswordReset 请求密码重置（发送验证码）
// @Summary 请求密码重置
// @Description 通过邮箱请求密码重置，系统会发送验证码到邮箱。为了安全，即使用户不存在也返回成功。
// @Tags 后台管理-密码重置
// @Accept json
// @Produce json
// @Param request body RequestResetRequest true "邮箱地址"
// @Success 200 {object} map[string]interface{} "请求成功（无论用户是否存在）"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 429 {object} map[string]interface{} "请求过于频繁"
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
			"message": "如果该邮箱已注册，您将收到密码重置验证码",
		})
		return
	}

	// 检查是否有未使用的有效验证码（防止频繁发送）
	var existingReset models.PasswordReset
	if err := database.DB.Where("user_id = ? AND used = ? AND expires_at > ?", user.ID, false, time.Now()).First(&existingReset).Error; err == nil {
		if time.Since(existingReset.CreatedAt) < time.Minute {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}
		database.DB.Model(&existingReset).Update("used", true)
	}

	// 生成6位数字验证码
	code, err := models.GenerateVerificationCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "生成验证码失败"})
		return
	}

	// 保存验证码
	passwordReset := models.PasswordReset{
		UserID:    user.ID,
		Token:     code,
		Email:     req.Email,
		ExpiresAt: time.Now().Add(10 * time.Minute), // 10分钟有效期
	}

	if err := database.DB.Create(&passwordReset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建重置验证码失败"})
		return
	}

	// 发送验证码邮件
	if err := h.emailService.SendAppPasswordResetEmail(req.Email, user.Username, code); err != nil {
		database.DB.Delete(&passwordReset)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "邮件发送失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "验证码已发送，请查收邮件",
	})
}

// ResetPassword 重置密码
// @Summary 重置密码
// @Description 使用邮箱收到的验证码设置新密码
// @Tags 后台管理-密码重置
// @Accept json
// @Produce json
// @Param request body ResetPasswordRequest true "重置密码信息"
// @Success 200 {object} map[string]interface{} "密码重置成功"
// @Failure 400 {object} map[string]interface{} "参数错误或验证码无效"
// @Router /admin/password/reset [post]
func (h *PasswordResetHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 查找验证码
	var passwordReset models.PasswordReset
	if err := database.DB.Where("email = ? AND token = ?", req.Email, req.Code).First(&passwordReset).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "验证码错误"})
		return
	}

	// 验证验证码
	if !passwordReset.IsValid() {
		if passwordReset.Used {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "验证码已被使用"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "验证码已过期，请重新获取"})
		}
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

	// 使旧验证码失效
	database.DB.Model(&models.PasswordReset{}).
		Where("user_id = ? AND used = ?", user.ID, false).
		Update("used", true)

	// 生成6位数字验证码
	code, err := models.GenerateVerificationCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "生成验证码失败"})
		return
	}

	// 保存验证码
	passwordReset := models.PasswordReset{
		UserID:    user.ID,
		Token:     code,
		Email:     user.Email,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	if err := database.DB.Create(&passwordReset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建重置验证码失败"})
		return
	}

	// 发送验证码邮件
	if err := h.emailService.SendAppPasswordResetEmail(user.Email, user.Username, code); err != nil {
		database.DB.Delete(&passwordReset)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "邮件发送失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "密码重置验证码已发送至 " + user.Email + "，请提示用户到忘记密码页面输入验证码完成重置",
	})
}

// AdminSendBindEmailCodeRequest 管理员发送绑定邮箱验证码请求
type AdminSendBindEmailCodeRequest struct {
	UserID uint   `json:"user_id" binding:"required"`
	Email  string `json:"email" binding:"required,email"`
}

// AdminSendBindEmailCode 管理员为指定用户发送绑定邮箱验证码
// @Summary 发送绑定邮箱验证码
// @Description 管理员为用户绑定邮箱时，需先向目标邮箱发送验证码以验证邮箱可用性
// @Tags 后台管理-用户管理
// @Accept json
// @Produce json
// @Param request body AdminSendBindEmailCodeRequest true "用户ID和邮箱"
// @Success 200 {object} map[string]interface{} "发送成功"
// @Failure 400 {object} map[string]interface{} "参数错误或邮箱已被占用"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Failure 500 {object} map[string]interface{} "邮件发送失败"
// @Router /admin/users/email/send-code [post]
func (h *PasswordResetHandler) AdminSendBindEmailCode(c *gin.Context) {
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足"})
		return
	}

	var req AdminSendBindEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请输入有效的邮箱地址"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	// 检查邮箱是否已被其他用户使用
	var other models.User
	if err := database.DB.Where("email = ? AND id != ?", req.Email, req.UserID).First(&other).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "该邮箱已被其他用户绑定"})
		return
	}

	// 检查是否有未使用的有效验证码（防止频繁发送）
	const vtype = "admin_bind"
	var existingCode models.EmailVerification
	if err := database.DB.Where("email = ? AND type = ? AND used = ? AND expires_at > ?",
		req.Email, vtype, false, time.Now()).First(&existingCode).Error; err == nil {
		if time.Since(existingCode.CreatedAt) < time.Minute {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}
		database.DB.Model(&existingCode).Update("used", true)
	}

	code, err := models.GenerateVerificationCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "生成验证码失败"})
		return
	}

	verification := models.EmailVerification{
		Email:     req.Email,
		Code:      code,
		Type:      vtype,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := database.DB.Create(&verification).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存验证码失败"})
		return
	}

	if err := h.emailService.SendVerificationEmail(req.Email, code, "admin_bind"); err != nil {
		database.DB.Delete(&verification)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "邮件发送失败，请检查邮件配置"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "验证码已发送，请查收邮件"})
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
