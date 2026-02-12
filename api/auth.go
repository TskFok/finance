package api

import (
	"net/http"
	"time"

	"finance/config"
	"finance/database"
	"finance/middleware"
	"finance/models"
	"finance/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	cfg          *config.Config
	emailService *service.EmailService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		cfg:          cfg,
		emailService: service.NewEmailService(&cfg.Email),
	}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50" example:"testuser"`
	Password string `json:"password" binding:"required,min=6,max=50" example:"password123"`
	Email    string `json:"email" binding:"omitempty,email" example:"test@example.com"`
}

// LoginRequest 登录请求（支持用户名或邮箱）
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"testuser"` // 可为用户名或邮箱
	Password string `json:"password" binding:"required" example:"password123"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token    string      `json:"token"`
	UserInfo models.User `json:"user_info"`
}

// Register 用户注册
// @Summary 用户注册
// @Description 创建新用户账号。注意：新注册用户默认处于“锁定(locked)”状态，需要管理员在后台将状态改为“正常(active)”后才能登录。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "注册信息"
// @Success 200 {object} Response{data=models.User} "注册成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 检查用户名是否已存在
	var existingUser models.User
	if err := database.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		BadRequest(c, "用户名已存在")
		return
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		InternalError(c, "密码加密失败")
		return
	}

	// 创建用户
	user := models.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Status:   models.UserStatusLocked,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		InternalError(c, "创建用户失败: "+err.Error())
		return
	}

	SuccessWithMessage(c, "注册成功", user)
}

// Login 用户登录
// @Summary 用户登录
// @Description 用户登录获取 JWT token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body LoginRequest true "登录信息"
// @Success 200 {object} Response{data=LoginResponse} "登录成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 401 {object} Response "用户名或密码错误"
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 查找用户（支持用户名或邮箱）
	var user models.User
	if err := database.DB.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
		Unauthorized(c, "用户名或密码错误")
		return
	}

	// 仅正常用户可登录
	if user.Status != models.UserStatusActive {
		Error(c, http.StatusForbidden, "账号已锁定，请联系管理员解锁")
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		Unauthorized(c, "用户名或密码错误")
		return
	}

	// 生成 token
	token, err := middleware.GenerateToken(user.ID, user.Username, h.cfg.JWT.ExpireTime)
	if err != nil {
		InternalError(c, "生成 token 失败")
		return
	}

	Success(c, LoginResponse{
		Token:    token,
		UserInfo: user,
	})
}

// GetProfile 获取用户信息
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的详细信息
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} Response{data=models.User} "获取成功"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/auth/profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		NotFound(c, "用户不存在")
		return
	}

	Success(c, user)
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required" example:"oldpassword123"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=50" example:"newpassword123"`
}

// ChangePassword 修改密码
// @Summary 修改密码
// @Description 修改当前用户密码
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ChangePasswordRequest true "密码信息"
// @Success 200 {object} Response "修改成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 401 {object} Response "原密码错误"
// @Router /api/v1/auth/password [put]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 获取用户
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		NotFound(c, "用户不存在")
		return
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		Unauthorized(c, "原密码错误")
		return
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		InternalError(c, "密码加密失败")
		return
	}

	// 更新密码
	if err := database.DB.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		InternalError(c, "更新密码失败")
		return
	}

	SuccessWithMessage(c, "密码修改成功", nil)
}

// ============== 邮箱验证相关接口 ==============

// SendVerificationCodeRequest 发送验证码请求
type SendVerificationCodeRequest struct {
	Email string `json:"email" binding:"required,email" example:"test@example.com"`
	Type  string `json:"type" binding:"required,oneof=register bind" example:"register"` // register: 注册, bind: 绑定邮箱
}

// SendVerificationCode 发送邮箱验证码
// @Summary 发送邮箱验证码
// @Description 发送邮箱验证码用于注册或绑定邮箱
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body SendVerificationCodeRequest true "验证码请求信息"
// @Success 200 {object} Response "发送成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/auth/send-code [post]
func (h *AuthHandler) SendVerificationCode(c *gin.Context) {
	var req SendVerificationCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请输入有效的邮箱地址")
		return
	}

	// 如果是注册验证，检查邮箱是否已被使用
	if req.Type == "register" {
		var existingUser models.User
		if err := database.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
			BadRequest(c, "该邮箱已被注册")
			return
		}
	}

	// 检查是否有未使用的有效验证码（防止频繁发送）
	var existingCode models.EmailVerification
	if err := database.DB.Where("email = ? AND type = ? AND used = ? AND expires_at > ?",
		req.Email, req.Type, false, time.Now()).First(&existingCode).Error; err == nil {
		// 如果距离上次发送不到1分钟，拒绝发送
		if time.Since(existingCode.CreatedAt) < time.Minute {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}
		// 使旧验证码失效
		database.DB.Model(&existingCode).Update("used", true)
	}

	// 生成验证码
	code, err := models.GenerateVerificationCode()
	if err != nil {
		InternalError(c, "生成验证码失败")
		return
	}

	// 保存验证码
	verification := models.EmailVerification{
		Email:     req.Email,
		Code:      code,
		Type:      req.Type,
		ExpiresAt: time.Now().Add(10 * time.Minute), // 10分钟有效期
	}

	if err := database.DB.Create(&verification).Error; err != nil {
		InternalError(c, "保存验证码失败")
		return
	}

	// 发送邮件
	var purpose string
	if req.Type == "register" {
		purpose = "register"
	} else {
		purpose = "bind"
	}

	if err := h.emailService.SendVerificationEmail(req.Email, code, purpose); err != nil {
		database.DB.Delete(&verification)
		InternalError(c, "邮件发送失败: "+err.Error())
		return
	}

	SuccessWithMessage(c, "验证码已发送，请查收邮件", nil)
}

// VerifyEmailCodeRequest 验证邮箱验证码请求
type VerifyEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email" example:"test@example.com"`
	Code  string `json:"code" binding:"required,len=6" example:"123456"`
	Type  string `json:"type" binding:"required,oneof=register bind" example:"register"`
}

// VerifyEmailCode 验证邮箱验证码
// @Summary 验证邮箱验证码
// @Description 验证邮箱验证码是否正确
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body VerifyEmailCodeRequest true "验证请求信息"
// @Success 200 {object} Response "验证成功"
// @Failure 400 {object} Response "验证码错误或已过期"
// @Router /api/v1/auth/verify-code [post]
func (h *AuthHandler) VerifyEmailCode(c *gin.Context) {
	var req VerifyEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误")
		return
	}

	var verification models.EmailVerification
	if err := database.DB.Where("email = ? AND code = ? AND type = ?",
		req.Email, req.Code, req.Type).First(&verification).Error; err != nil {
		BadRequest(c, "验证码错误")
		return
	}

	if !verification.IsValid() {
		if verification.Used {
			BadRequest(c, "验证码已被使用")
		} else {
			BadRequest(c, "验证码已过期，请重新获取")
		}
		return
	}

	SuccessWithMessage(c, "验证成功", nil)
}

// RegisterWithVerificationRequest 带邮箱验证的注册请求
type RegisterWithVerificationRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50" example:"testuser"`
	Password string `json:"password" binding:"required,min=6,max=50" example:"password123"`
	Email    string `json:"email" binding:"required,email" example:"test@example.com"`
	Code     string `json:"code" binding:"required,len=6" example:"123456"`
}

// RegisterWithVerification 带邮箱验证的用户注册
// @Summary 带邮箱验证的用户注册
// @Description 需要先发送验证码，验证通过后创建用户账号。注意：新注册用户默认处于“锁定(locked)”状态，需要管理员在后台将状态改为“正常(active)”后才能登录。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body RegisterWithVerificationRequest true "注册信息"
// @Success 200 {object} Response{data=models.User} "注册成功"
// @Failure 400 {object} Response "请求参数错误或验证码错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/auth/register-verified [post]
func (h *AuthHandler) RegisterWithVerification(c *gin.Context) {
	var req RegisterWithVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 验证验证码
	var verification models.EmailVerification
	if err := database.DB.Where("email = ? AND code = ? AND type = ?",
		req.Email, req.Code, "register").First(&verification).Error; err != nil {
		BadRequest(c, "验证码错误")
		return
	}

	if !verification.IsValid() {
		if verification.Used {
			BadRequest(c, "验证码已被使用")
		} else {
			BadRequest(c, "验证码已过期，请重新获取")
		}
		return
	}

	// 检查用户名是否已存在
	var existingUser models.User
	if err := database.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		BadRequest(c, "用户名已存在")
		return
	}

	// 检查邮箱是否已被使用
	if err := database.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		BadRequest(c, "该邮箱已被注册")
		return
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		InternalError(c, "密码加密失败")
		return
	}

	// 创建用户
	user := models.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Status:   models.UserStatusLocked,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		InternalError(c, "创建用户失败: "+err.Error())
		return
	}

	// 标记验证码为已使用
	database.DB.Model(&verification).Update("used", true)

	SuccessWithMessage(c, "注册成功", user)
}

// ============== App 端密码重置相关接口 ==============

// AppRequestPasswordResetRequest App端请求密码重置
type AppRequestPasswordResetRequest struct {
	Email string `json:"email" binding:"required,email" example:"test@example.com"`
}

// AppRequestPasswordReset App端请求密码重置（发送验证码）
// @Summary App端请求密码重置
// @Description 通过邮箱发送密码重置验证码
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body AppRequestPasswordResetRequest true "密码重置请求"
// @Success 200 {object} Response "验证码已发送"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 429 {object} Response "请求过于频繁"
// @Router /api/v1/auth/password/request-reset [post]
func (h *AuthHandler) AppRequestPasswordReset(c *gin.Context) {
	var req AppRequestPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请输入有效的邮箱地址")
		return
	}

	// 查找用户
	var user models.User
	if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// 为了安全，即使用户不存在也返回成功
		SuccessWithMessage(c, "如果该邮箱已注册，您将收到密码重置验证码", nil)
		return
	}

	// 检查是否有未使用的有效验证码（防止频繁发送）
	var existingReset models.PasswordReset
	if err := database.DB.Where("user_id = ? AND used = ? AND expires_at > ?",
		user.ID, false, time.Now()).First(&existingReset).Error; err == nil {
		// 如果距离上次发送不到1分钟，拒绝发送
		if time.Since(existingReset.CreatedAt) < time.Minute {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}
		// 使旧验证码失效
		database.DB.Model(&existingReset).Update("used", true)
	}

	// 生成6位数字验证码
	code, err := models.GenerateVerificationCode()
	if err != nil {
		InternalError(c, "生成验证码失败")
		return
	}

	// 保存重置令牌（使用验证码作为 token）
	passwordReset := models.PasswordReset{
		UserID:    user.ID,
		Token:     code, // App 端使用6位验证码
		Email:     req.Email,
		ExpiresAt: time.Now().Add(10 * time.Minute), // 10分钟有效期
	}

	if err := database.DB.Create(&passwordReset).Error; err != nil {
		InternalError(c, "创建重置令牌失败")
		return
	}

	// 发送邮件
	if err := h.emailService.SendAppPasswordResetEmail(req.Email, user.Username, code); err != nil {
		database.DB.Delete(&passwordReset)
		InternalError(c, "邮件发送失败: "+err.Error())
		return
	}

	SuccessWithMessage(c, "密码重置验证码已发送，请查收邮件", nil)
}

// AppVerifyResetCodeRequest App端验证重置验证码
type AppVerifyResetCodeRequest struct {
	Email string `json:"email" binding:"required,email" example:"test@example.com"`
	Code  string `json:"code" binding:"required,len=6" example:"123456"`
}

// AppVerifyResetCode App端验证重置验证码
// @Summary App端验证重置验证码
// @Description 验证密码重置验证码是否正确
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body AppVerifyResetCodeRequest true "验证请求"
// @Success 200 {object} Response "验证成功"
// @Failure 400 {object} Response "验证码错误或已过期"
// @Router /api/v1/auth/password/verify-code [post]
func (h *AuthHandler) AppVerifyResetCode(c *gin.Context) {
	var req AppVerifyResetCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误")
		return
	}

	var passwordReset models.PasswordReset
	if err := database.DB.Where("email = ? AND token = ?", req.Email, req.Code).First(&passwordReset).Error; err != nil {
		BadRequest(c, "验证码错误")
		return
	}

	if !passwordReset.IsValid() {
		if passwordReset.Used {
			BadRequest(c, "验证码已被使用")
		} else {
			BadRequest(c, "验证码已过期，请重新获取")
		}
		return
	}

	SuccessWithMessage(c, "验证成功", nil)
}

// AppResetPasswordRequest App端重置密码请求
type AppResetPasswordRequest struct {
	Email       string `json:"email" binding:"required,email" example:"test@example.com"`
	Code        string `json:"code" binding:"required,len=6" example:"123456"`
	NewPassword string `json:"new_password" binding:"required,min=6" example:"newpassword123"`
}

// AppResetPassword App端重置密码
// @Summary App端重置密码
// @Description 使用验证码重置密码
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body AppResetPasswordRequest true "重置密码请求"
// @Success 200 {object} Response "密码重置成功"
// @Failure 400 {object} Response "验证码错误或已过期"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/auth/password/reset [post]
func (h *AuthHandler) AppResetPassword(c *gin.Context) {
	var req AppResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误")
		return
	}

	// 查找验证码
	var passwordReset models.PasswordReset
	if err := database.DB.Where("email = ? AND token = ?", req.Email, req.Code).First(&passwordReset).Error; err != nil {
		BadRequest(c, "验证码错误")
		return
	}

	// 验证验证码
	if !passwordReset.IsValid() {
		if passwordReset.Used {
			BadRequest(c, "验证码已被使用")
		} else {
			BadRequest(c, "验证码已过期，请重新获取")
		}
		return
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		InternalError(c, "密码加密失败")
		return
	}

	// 更新密码
	if err := database.DB.Model(&models.User{}).Where("id = ?", passwordReset.UserID).Update("password", string(hashedPassword)).Error; err != nil {
		InternalError(c, "更新密码失败")
		return
	}

	// 标记验证码为已使用
	database.DB.Model(&passwordReset).Update("used", true)

	// 使该用户所有未使用的重置令牌失效
	database.DB.Model(&models.PasswordReset{}).
		Where("user_id = ? AND used = ?", passwordReset.UserID, false).
		Update("used", true)

	SuccessWithMessage(c, "密码重置成功，请使用新密码登录", nil)
}
