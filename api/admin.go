package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"finance/adminauth"
	"finance/database"
	"finance/models"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
)

func setAdminCookie(c *gin.Context, name, value string, maxAge int, httpOnly bool) {
	secure, sameSite := getCookieOptions()
	c.SetCookieData(&http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: sameSite,
	})
}

// setSignedAdminCookie 设置签名后的敏感 Cookie，防止客户端篡改
func setSignedAdminCookie(c *gin.Context, name, value string, maxAge int, httpOnly bool) {
	setAdminCookie(c, name, adminauth.SignCookieValue(value), maxAge, httpOnly)
}

// AdminHandler 后台管理处理器
type AdminHandler struct{}

// NewAdminHandler 创建后台管理处理器
func NewAdminHandler() *AdminHandler {
	return &AdminHandler{}
}

// getCurrentUser 获取当前登录用户信息（校验 Cookie 签名，防止篡改越权）
func getCurrentUser(c *gin.Context) (*models.User, error) {
	userID, err := adminauth.GetVerifiedAdminUserID(c)
	if err != nil {
		return nil, err
	}
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// AdminLoginRequest 管理员登录请求（支持用户名或邮箱）
type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"` // 可为用户名或邮箱
	Password string `json:"password" binding:"required"`
}

// AdminLogin 管理员登录（使用 session/cookie 方式）
// @Summary 管理员登录
// @Description 管理员使用用户名和密码登录，登录成功后设置 Cookie。只有状态为 active 的用户可以登录。
// @Tags 后台管理
// @Accept json
// @Produce json
// @Param request body AdminLoginRequest true "登录信息"
// @Success 200 {object} map[string]interface{} "登录成功，返回用户信息"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 401 {object} map[string]interface{} "用户名或密码错误"
// @Failure 403 {object} map[string]interface{} "账号已锁定"
// @Router /admin/login [post]
func (h *AdminHandler) AdminLogin(c *gin.Context) {
	var req AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 查找用户（支持用户名或邮箱）
	var user models.User
	if err := database.DB.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "用户名或密码错误"})
		return
	}

	// 仅正常用户可登录
	if user.Status != models.UserStatusActive {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "账号已锁定，请联系管理员解锁"})
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "用户名或密码错误"})
		return
	}

	// 设置 Cookie（admin_user_id、admin_is_admin 使用签名防篡改）
	setSignedAdminCookie(c, "admin_user_id", fmt.Sprintf("%d", user.ID), 86400, true)
	setAdminCookie(c, "admin_username", user.Username, 86400, false)
	setSignedAdminCookie(c, "admin_is_admin", fmt.Sprintf("%t", user.IsAdmin), 86400, false)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "登录成功",
		"data": gin.H{
			"user_id":  user.ID,
			"username": user.Username,
			"is_admin": user.IsAdmin,
		},
	})
}

// UserMenuItem 用户可见菜单项（简化结构，供前端侧栏渲染）
type UserMenuItem struct {
	ID       uint          `json:"id"`
	Name     string        `json:"name"`
	Path     string        `json:"path"`
	Icon     string        `json:"icon"`
	Children []UserMenuItem `json:"children,omitempty"`
}

// GetCurrentUserInfo 获取当前登录用户信息（含角色、菜单树）
// @Summary 获取当前登录用户信息
// @Description 获取当前登录用户的详细信息，包括角色和可见菜单树
// @Tags 后台管理
// @Produce json
// @Success 200 {object} map[string]interface{} "获取成功"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /admin/current-user [get]
func (h *AdminHandler) GetCurrentUserInfo(c *gin.Context) {
	user, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	menus := getUserMenus(user)
	var role *models.Role
	if user.RoleID != nil {
		var r models.Role
		if database.DB.First(&r, *user.RoleID).Error == nil {
			role = &r
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"is_admin": user.IsAdmin,
			"status":   user.Status,
			"role_id":  user.RoleID,
			"role":     role,
			"menus":    menus,
		},
	})
}

// getUserMenus 获取用户可见的菜单树（超管全部，否则按角色）
func getUserMenus(user *models.User) []UserMenuItem {
	var menuIDs []uint
	if user.IsAdmin {
		database.DB.Model(&models.Menu{}).Pluck("id", &menuIDs)
	} else if user.RoleID != nil {
		database.DB.Model(&models.RoleMenu{}).Where("role_id = ?", *user.RoleID).Pluck("menu_id", &menuIDs)
	} else {
		// 无角色时使用 viewer 的菜单
		var viewer models.Role
		if database.DB.Where("code = ?", "viewer").First(&viewer).Error == nil {
			database.DB.Model(&models.RoleMenu{}).Where("role_id = ?", viewer.ID).Pluck("menu_id", &menuIDs)
		}
	}
	if len(menuIDs) == 0 {
		return nil
	}
	var menus []models.Menu
	database.DB.Where("id IN ?", menuIDs).Order("sort_order ASC, id ASC").Find(&menus)
	return buildUserMenuTree(menus, menuIDs, 0)
}

func buildUserMenuTree(menus []models.Menu, allowedIDs []uint, parentID uint) []UserMenuItem {
	allowedSet := make(map[uint]bool)
	for _, id := range allowedIDs {
		allowedSet[id] = true
	}
	var result []UserMenuItem
	for _, m := range menus {
		if m.ParentID != parentID || !allowedSet[m.ID] {
			continue
		}
		item := UserMenuItem{
			ID:   m.ID,
			Name: m.Name,
			Path: m.Path,
			Icon: m.Icon,
		}
		item.Children = buildUserMenuTree(menus, allowedIDs, m.ID)
		result = append(result, item)
	}
	return result
}

// AdminLogout 管理员退出登录
// @Summary 管理员退出登录
// @Description 清除登录 Cookie，退出登录
// @Tags 后台管理
// @Produce json
// @Success 200 {object} map[string]interface{} "退出成功"
// @Router /admin/logout [post]
func (h *AdminHandler) AdminLogout(c *gin.Context) {
	setAdminCookie(c, "admin_user_id", "", -1, true)
	setAdminCookie(c, "admin_username", "", -1, false)
	setAdminCookie(c, "admin_is_admin", "", -1, false)
	setAdminCookie(c, "original_admin_id", "", -1, true)
	setAdminCookie(c, "original_admin_username", "", -1, false)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "已退出登录"})
}

// ImpersonateUserRequest 模拟登录请求
type ImpersonateUserRequest struct {
	UserID uint `json:"user_id" binding:"required"`
}

// ImpersonateUser 模拟登录（仅管理员可用）
// @Summary 模拟登录用户
// @Description 管理员可以模拟登录非管理员用户，用于查看用户视角。不能模拟其他管理员。模拟登录后，原始管理员信息会保存在 Cookie 中，可以通过退出模拟恢复。
// @Tags 后台管理-用户管理
// @Accept json
// @Produce json
// @Param request body ImpersonateUserRequest true "用户ID"
// @Success 200 {object} map[string]interface{} "模拟登录成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足或不能模拟管理员"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Router /admin/users/impersonate [post]
func (h *AdminHandler) ImpersonateUser(c *gin.Context) {
	// 获取当前用户（必须是管理员）
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "只有管理员可以模拟登录"})
		return
	}

	var req ImpersonateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}

	// 查找要模拟的用户
	var targetUser models.User
	if err := database.DB.First(&targetUser, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	// 不能模拟其他管理员（防止权限提升）
	if targetUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "不能模拟其他管理员账户"})
		return
	}

	// 不能模拟自己
	if targetUser.ID == currentUser.ID {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "不能模拟自己的账户"})
		return
	}

	// 保存原始管理员信息到 Cookie（用于退出模拟时恢复，使用签名防篡改）
	setSignedAdminCookie(c, "original_admin_id", fmt.Sprintf("%d", currentUser.ID), 86400, true)
	setAdminCookie(c, "original_admin_username", currentUser.Username, 86400, false)

	// 设置被模拟用户的 Cookie（admin_user_id、admin_is_admin 使用签名防篡改）
	setSignedAdminCookie(c, "admin_user_id", fmt.Sprintf("%d", targetUser.ID), 86400, true)
	setAdminCookie(c, "admin_username", targetUser.Username, 86400, false)
	setSignedAdminCookie(c, "admin_is_admin", fmt.Sprintf("%t", targetUser.IsAdmin), 86400, false)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("已模拟登录用户：%s", targetUser.Username),
		"data": gin.H{
			"user_id":  targetUser.ID,
			"username": targetUser.Username,
			"is_admin": targetUser.IsAdmin,
		},
	})
}

// ExitImpersonation 退出模拟登录
// @Summary 退出模拟登录
// @Description 退出模拟登录，恢复原始管理员身份
// @Tags 后台管理-用户管理
// @Produce json
// @Success 200 {object} map[string]interface{} "退出模拟成功"
// @Failure 401 {object} map[string]interface{} "未登录或未在模拟状态"
// @Router /admin/users/exit-impersonation [post]
func (h *AdminHandler) ExitImpersonation(c *gin.Context) {
	// 获取并验证原始管理员信息（校验签名防止篡改）
	originalAdminID, err := adminauth.GetVerifiedOriginalAdminID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "未在模拟登录状态或会话无效"})
		return
	}

	// 查找原始管理员
	var originalAdmin models.User
	if err := database.DB.First(&originalAdmin, uint(originalAdminID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "原始管理员不存在"})
		return
	}

	// 恢复原始管理员 Cookie（admin_user_id、admin_is_admin 使用签名防篡改）
	setSignedAdminCookie(c, "admin_user_id", fmt.Sprintf("%d", originalAdmin.ID), 86400, true)
	setAdminCookie(c, "admin_username", originalAdmin.Username, 86400, false)
	setSignedAdminCookie(c, "admin_is_admin", fmt.Sprintf("%t", originalAdmin.IsAdmin), 86400, false)

	// 清除原始管理员信息 Cookie
	setAdminCookie(c, "original_admin_id", "", -1, true)
	setAdminCookie(c, "original_admin_username", "", -1, false)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("已退出模拟，恢复为管理员：%s", originalAdmin.Username),
		"data": gin.H{
			"user_id":  originalAdmin.ID,
			"username": originalAdmin.Username,
			"is_admin": originalAdmin.IsAdmin,
		},
	})
}

// GetAllExpenses 获取消费记录（管理员看全部，非管理员只看自己的）
// @Summary 获取消费记录列表
// @Description 获取消费记录列表，支持分页、时间范围、类别、用户名筛选。管理员可查看所有记录并可按用户ID筛选，非管理员只能查看自己的记录。
// @Tags 后台管理-消费记录
// @Produce json
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页数量，默认20"
// @Param start_time query string false "开始时间 (YYYY-MM-DD)"
// @Param end_time query string false "结束时间 (YYYY-MM-DD)"
// @Param category query string false "类别筛选"
// @Param username query string false "用户名筛选（模糊匹配）"
// @Param user_id query int false "用户ID筛选（仅管理员可用）"
// @Success 200 {object} map[string]interface{} "获取成功，返回分页数据"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /admin/expenses [get]
func (h *AdminHandler) GetAllExpenses(c *gin.Context) {
	// 获取当前用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if ps := c.Query("page_size"); ps != "" {
		fmt.Sscanf(ps, "%d", &pageSize)
	}

	startTime := c.Query("start_time")
	endTime := c.Query("end_time")
	category := c.Query("category")
	username := c.Query("username")
	userIDFilter := c.Query("user_id") // 管理员可以按用户ID筛选

	query := database.DB.Model(&models.Expense{}).
		Select("expenses.*, users.username").
		Joins("LEFT JOIN users ON expenses.user_id = users.id")

	// 权限过滤：非管理员只能看自己的数据
	if !currentUser.IsAdmin {
		query = query.Where("expenses.user_id = ?", currentUser.ID)
	} else {
		// 管理员可以按用户ID筛选
		if userIDFilter != "" {
			if uid, err := strconv.ParseUint(userIDFilter, 10, 32); err == nil {
				query = query.Where("expenses.user_id = ?", uint(uid))
			}
		}
	}

	// 筛选条件
	if startTime != "" {
		if t, err := time.ParseInLocation("2006-01-02", startTime, time.Local); err == nil {
			query = query.Where("expenses.expense_time >= ?", t)
		}
	}
	if endTime != "" {
		if t, err := time.ParseInLocation("2006-01-02", endTime, time.Local); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			query = query.Where("expenses.expense_time <= ?", t)
		}
	}
	if category != "" {
		query = query.Where("expenses.category = ?", category)
	}
	if username != "" {
		escaped := escapeLikeValue(username)
		query = query.Where("users.username LIKE ?", "%"+escaped+"%")
	}

	// 计算总数
	var total int64
	query.Count(&total)

	// 查询数据
	type ExpenseWithUser struct {
		models.Expense
		Username string `json:"username"`
	}

	var expenses []ExpenseWithUser
	offset := (page - 1) * pageSize
	query.Order("expenses.expense_time DESC").Offset(offset).Limit(pageSize).Scan(&expenses)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"list":      expenses,
		},
	})
}

// GetAllUsers 获取所有用户列表
// @Summary 获取用户列表
// @Description 获取系统中所有用户列表（包含软删除的用户）
// @Tags 后台管理-用户管理
// @Produce json
// @Success 200 {object} map[string]interface{} "获取成功，返回用户列表"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /admin/users [get]
func (h *AdminHandler) GetAllUsers(c *gin.Context) {
	// 获取当前用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	// 只有管理员可以查看所有用户
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足"})
		return
	}

	var users []models.User
	database.DB.Find(&users)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    users,
	})
}

// UpdateUserPasswordRequest 更新用户密码请求
type UpdateUserPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// UpdateUserPassword 更新用户密码（仅管理员）
// @Summary 更新用户密码
// @Description 管理员可以修改指定用户的密码
// @Tags 后台管理-用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param request body UpdateUserPasswordRequest true "新密码"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Router /admin/users/{id}/password [put]
func (h *AdminHandler) UpdateUserPassword(c *gin.Context) {
	// 获取当前用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	// 只有管理员可以修改其他用户密码
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足"})
		return
	}

	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的用户ID"})
		return
	}

	var req UpdateUserPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}

	var user models.User
	if err := database.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "密码加密失败"})
		return
	}

	user.Password = string(hashedPassword)
	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "更新失败")})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "密码更新成功",
	})
}

// DeleteUser 删除用户（仅管理员，软删除）
// @Summary 删除用户
// @Description 管理员可以删除用户（软删除），不能删除自己
// @Tags 后台管理-用户管理
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 400 {object} map[string]interface{} "不能删除自己"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Router /admin/users/{id} [delete]
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	// 获取当前用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	// 只有管理员可以删除用户
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足"})
		return
	}

	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的用户ID"})
		return
	}

	// 不能删除自己
	if uint(userID) == currentUser.ID {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "不能删除自己的账号"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	if err := database.DB.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "删除失败")})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "用户删除成功",
	})
}

// SetAdminRequest 设置管理员权限请求
type SetAdminRequest struct {
	IsAdmin bool `json:"is_admin"`
}

// UpdateUserStatusRequest 更新用户状态请求
type UpdateUserStatusRequest struct {
	// Status 用户状态：active（正常）/ locked（锁定）
	Status string `json:"status" binding:"required,oneof=active locked"`
}

// SetAdmin 设置用户管理员权限（仅管理员）
// @Summary 设置管理员权限
// @Description 管理员可以设置或取消其他用户的管理员权限，不能取消自己的管理员权限
// @Tags 后台管理-用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param request body SetAdminRequest true "管理员权限设置"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]interface{} "不能取消自己的管理员权限"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Router /admin/users/{id}/admin [put]
func (h *AdminHandler) SetAdmin(c *gin.Context) {
	// 获取当前用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	// 只有管理员可以设置其他用户的管理员权限
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足"})
		return
	}

	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的用户ID"})
		return
	}

	var req SetAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}

	var user models.User
	if err := database.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	// 不能取消自己的管理员权限
	if uint(userID) == currentUser.ID && !req.IsAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "不能取消自己的管理员权限"})
		return
	}

	user.IsAdmin = req.IsAdmin
	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "更新失败")})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "权限更新成功",
		"data":    user,
	})
}

// UpdateUserStatus 更新用户状态（仅管理员）
// @Summary 更新用户状态
// @Description 管理员可将用户状态设置为 normal(active) 或 locked。只有 active 状态的用户可以登录。
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param request body UpdateUserStatusRequest true "状态信息"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Router /admin/users/{id}/status [put]
func (h *AdminHandler) UpdateUserStatus(c *gin.Context) {
	// 获取当前用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	// 只有管理员可以更新用户状态
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足"})
		return
	}

	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的用户ID"})
		return
	}

	// 不能锁定自己，避免自锁导致无法登录后台
	if uint(userID) == currentUser.ID {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "不能修改自己的状态"})
		return
	}

	var req UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}

	status := strings.TrimSpace(req.Status)
	if status != models.UserStatusActive && status != models.UserStatusLocked {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的状态，支持：active/locked"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	user.Status = status
	if err := database.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "更新失败")})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "状态更新成功",
		"data":    user,
	})
}

// UpdateUserFeishuRequest 更新用户飞书绑定请求
type UpdateUserFeishuRequest struct {
	FeishuOpenID string `json:"feishu_open_id"`
}

// UpdateUserFeishu 设置用户飞书绑定（仅管理员）
// @Summary 设置用户飞书绑定
// @Description 管理员可为用户设置 feishu_open_id，设置后该用户可通过飞书扫码登录
// @Tags 后台管理-用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param request body UpdateUserFeishuRequest true "飞书 open_id"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Router /admin/users/{id}/feishu [put]
func (h *AdminHandler) UpdateUserFeishu(c *gin.Context) {
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足"})
		return
	}

	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的用户ID"})
		return
	}

	var req UpdateUserFeishuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
		return
	}

	feishuOpenID := strings.TrimSpace(req.FeishuOpenID)
	if feishuOpenID != "" {
		var other models.User
		if err := database.DB.Where("feishu_open_id = ? AND id != ?", feishuOpenID, userID).First(&other).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "该飞书账号已被其他用户绑定"})
			return
		}
	}

	var user models.User
	if err := database.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	var openIDPtr *string
	if feishuOpenID != "" {
		openIDPtr = &feishuOpenID
	}
	updates := map[string]interface{}{"feishu_open_id": openIDPtr}
	if feishuOpenID == "" {
		updates["feishu_union_id"] = ""
	}
	if err := database.DB.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "飞书绑定更新成功",
		"data":    user,
	})
}

// UpdateUserRoleRequest 更新用户角色请求
type UpdateUserRoleRequest struct {
	RoleID *uint `json:"role_id"` // nil 表示清除角色
}

// UpdateUserRole 设置用户角色（仅超管）
func (h *AdminHandler) UpdateUserRole(c *gin.Context) {
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足"})
		return
	}

	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的用户ID"})
		return
	}

	var req UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}

	if req.RoleID != nil {
		var role models.Role
		if err := database.DB.First(&role, *req.RoleID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "角色不存在"})
			return
		}
	}

	var user models.User
	if err := database.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	if err := database.DB.Model(&user).Update("role_id", req.RoleID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "更新失败")})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "角色更新成功",
		"data":    user,
	})
}

// UpdateUserEmailRequest 更新用户邮箱请求
type UpdateUserEmailRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"` // 绑定邮箱时必填，用于验证邮箱可用性
}

// UpdateUserEmail 绑定/修改用户邮箱（仅管理员）
// @Summary 绑定或修改用户邮箱
// @Description 管理员可为用户设置邮箱。绑定新邮箱必须先发送验证码，验证通过后才能绑定。清除邮箱无需验证。
// @Tags 后台管理-用户管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param request body UpdateUserEmailRequest true "邮箱地址和验证码"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]interface{} "参数错误或验证码错误"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Router /admin/users/{id}/email [put]
func (h *AdminHandler) UpdateUserEmail(c *gin.Context) {
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足"})
		return
	}

	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的用户ID"})
		return
	}

	var req UpdateUserEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
		return
	}

	email := strings.TrimSpace(req.Email)
	code := strings.TrimSpace(req.Code)

	if email != "" {
		// 绑定邮箱：必须提供验证码
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请先发送验证码并输入收到的验证码"})
			return
		}
		// 验证码必须是6位数字
		if len(code) != 6 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "验证码格式错误"})
			return
		}
		// 验证验证码
		var verification models.EmailVerification
		if err := database.DB.Where("email = ? AND code = ? AND type = ?",
			email, code, "admin_bind").First(&verification).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "验证码错误"})
			return
		}
		if verification.Used {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "验证码已被使用，请重新获取"})
			return
		}
		if verification.IsExpired() {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "验证码已过期，请重新获取"})
			return
		}
		// 检查邮箱是否已被其他用户使用
		var other models.User
		if err := database.DB.Where("email = ? AND id != ?", email, userID).First(&other).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "该邮箱已被其他用户绑定"})
			return
		}
	}

	var user models.User
	if err := database.DB.First(&user, uint(userID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	if err := database.DB.Model(&user).Update("email", email).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新失败"})
		return
	}

	// 绑定成功后使验证码失效
	if email != "" {
		var verification models.EmailVerification
		if err := database.DB.Where("email = ? AND code = ? AND type = ?", email, code, "admin_bind").First(&verification).Error; err == nil {
			database.DB.Model(&verification).Update("used", true)
		}
	}

	msg := "邮箱已绑定"
	if email == "" {
		msg = "邮箱已清除"
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": msg,
	})
}

// GetStatistics 获取统计数据
// @Summary 获取统计数据
// @Description 获取支出和收入的统计数据，包括总金额、总记录数、类别统计等。管理员可查看所有数据，非管理员只能查看自己的数据。
// @Tags 后台管理-统计
// @Produce json
// @Param start_time query string false "开始时间 (YYYY-MM-DD)"
// @Param end_time query string false "结束时间 (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{} "获取成功"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /admin/statistics [get]
func (h *AdminHandler) GetStatistics(c *gin.Context) {
	// 获取当前用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	startTime := c.Query("start_time")
	endTime := c.Query("end_time")

	query := database.DB.Model(&models.Expense{})
	incomeQuery := database.DB.Model(&models.Income{})

	// 权限过滤：非管理员只能看自己的数据
	if !currentUser.IsAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
		incomeQuery = incomeQuery.Where("user_id = ?", currentUser.ID)
	}

	if startTime != "" {
		if t, err := time.ParseInLocation("2006-01-02", startTime, time.Local); err == nil {
			query = query.Where("expense_time >= ?", t)
			incomeQuery = incomeQuery.Where("income_time >= ?", t)
		}
	}
	if endTime != "" {
		if t, err := time.ParseInLocation("2006-01-02", endTime, time.Local); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			query = query.Where("expense_time <= ?", t)
			incomeQuery = incomeQuery.Where("income_time <= ?", t)
		}
	}

	// 总金额和总记录数
	var totalAmount float64
	var totalCount int64
	query.Select("COALESCE(SUM(amount), 0)").Scan(&totalAmount)
	query.Count(&totalCount)

	// 收入总金额和总记录数
	var totalIncome float64
	var incomeCount int64
	incomeQuery.Select("COALESCE(SUM(amount), 0)").Scan(&totalIncome)
	incomeQuery.Count(&incomeCount)

	// 按类别统计（使用已过滤的query）
	type CategoryStat struct {
		Category string  `json:"category"`
		Total    float64 `json:"total"`
		Count    int64   `json:"count"`
	}
	var categoryStats []CategoryStat
	// 重新构建查询以应用相同的过滤条件
	categoryQuery := database.DB.Model(&models.Expense{})
	if !currentUser.IsAdmin {
		categoryQuery = categoryQuery.Where("user_id = ?", currentUser.ID)
	}
	if startTime != "" {
		if t, err := time.ParseInLocation("2006-01-02", startTime, time.Local); err == nil {
			categoryQuery = categoryQuery.Where("expense_time >= ?", t)
		}
	}
	if endTime != "" {
		if t, err := time.ParseInLocation("2006-01-02", endTime, time.Local); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			categoryQuery = categoryQuery.Where("expense_time <= ?", t)
		}
	}
	categoryQuery.
		Select("category, SUM(amount) as total, COUNT(*) as count").
		Group("category").
		Order("total DESC").
		Scan(&categoryStats)

	// 用户数量（仅管理员可见）
	var userCount int64
	if currentUser.IsAdmin {
		database.DB.Model(&models.User{}).Count(&userCount)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_amount":   totalAmount,
			"total_count":    totalCount,
			"total_income":   totalIncome,
			"income_count":   incomeCount,
			"user_count":     userCount,
			"category_stats": categoryStats,
		},
	})
}

// GetDetailedStatistics 获取详细消费统计（支持月/年/自定义时间范围和多个类别筛选）
// @Summary 获取详细消费统计
// @Description 获取详细的消费统计数据，支持按月、按年或自定义时间范围统计，支持多个类别筛选。管理员可按用户ID筛选，非管理员只能查看自己的数据。
// @Tags 后台管理-统计
// @Produce json
// @Param range_type query string true "时间范围类型：month(按月)、year(按年)、custom(自定义)"
// @Param year_month query string false "当range_type=month时必填，格式：2024-01"
// @Param year query string false "当range_type=year时必填，格式：2024"
// @Param start_time query string false "当range_type=custom时必填，格式：2024-01-01"
// @Param end_time query string false "当range_type=custom时必填，格式：2024-12-31"
// @Param categories query string false "类别筛选，多个类别用逗号分隔，如：餐饮,交通"
// @Param user_id query int false "用户ID筛选（仅管理员可用）"
// @Success 200 {object} map[string]interface{} "获取成功，包含总金额、总记录数、类别统计等"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /admin/expenses/detailed-statistics [get]
func (h *AdminHandler) GetDetailedStatistics(c *gin.Context) {
	// 获取当前用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	rangeType := c.Query("range_type")
	if rangeType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "range_type参数必填，可选值：month、year、custom"})
		return
	}

	query := database.DB.Model(&models.Expense{})

	// 权限过滤：非管理员只能看自己的数据
	if !currentUser.IsAdmin {
		query = query.Where("user_id = ?", currentUser.ID)
	} else {
		// 管理员可以按用户ID筛选
		if userIDFilter := c.Query("user_id"); userIDFilter != "" {
			if uid, err := strconv.ParseUint(userIDFilter, 10, 32); err == nil {
				query = query.Where("user_id = ?", uint(uid))
			}
		}
	}

	var startTime, endTime time.Time

	// 根据时间范围类型设置时间范围
	switch rangeType {
	case "month":
		yearMonth := c.Query("year_month")
		if yearMonth == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "range_type=month时，year_month参数必填（格式：2024-01）"})
			return
		}
		startTime, err = time.ParseInLocation("2006-01", yearMonth, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "year_month格式错误，应为：2024-01"})
			return
		}
		startTime = time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, time.Local)
		endTime = startTime.AddDate(0, 1, 0).Add(-time.Second)

	case "year":
		yearStr := c.Query("year")
		if yearStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "range_type=year时，year参数必填（格式：2024）"})
			return
		}
		year, err := strconv.Atoi(yearStr)
		if err != nil || year < 2000 || year > 2100 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "year格式错误，应为4位数字（如：2024）"})
			return
		}
		startTime = time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
		endTime = time.Date(year, 12, 31, 23, 59, 59, 0, time.Local)

	case "custom":
		startTimeStr := c.Query("start_time")
		endTimeStr := c.Query("end_time")
		if startTimeStr == "" || endTimeStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "range_type=custom时，start_time和end_time参数必填（格式：2024-01-01）"})
			return
		}
		startTime, err = time.ParseInLocation("2006-01-02", startTimeStr, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "start_time格式错误，应为：2024-01-01"})
			return
		}
		endTime, err = time.ParseInLocation("2006-01-02", endTimeStr, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "end_time格式错误，应为：2024-12-31"})
			return
		}
		endTime = endTime.Add(24*time.Hour - time.Second)

	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "range_type参数值错误，可选值：month、year、custom"})
		return
	}

	// 应用时间范围筛选
	query = query.Where("expense_time >= ? AND expense_time <= ?", startTime, endTime)

	// 类别筛选（支持多个类别）
	categoriesStr := c.Query("categories")
	if categoriesStr != "" {
		categories := strings.Split(categoriesStr, ",")
		for i := range categories {
			categories[i] = strings.TrimSpace(categories[i])
		}
		if len(categories) > 0 {
			query = query.Where("category IN ?", categories)
		}
	}

	// 总金额和总记录数
	var totalAmount float64
	var totalCount int64
	// 先获取总数（需要在Select之前）
	query.Count(&totalCount)
	// 再获取总金额
	query.Select("COALESCE(SUM(amount), 0)").Scan(&totalAmount)

	// 按类别统计
	type CategoryStat struct {
		Category   string  `json:"category"`
		Total      float64 `json:"total"`
		Count      int64   `json:"count"`
		Percentage float64 `json:"percentage"`
	}
	var categoryStats []CategoryStat

	// 构建类别统计查询
	categoryQuery := database.DB.Model(&models.Expense{}).
		Select("category, SUM(amount) as total, COUNT(*) as count").
		Where("expense_time >= ? AND expense_time <= ?", startTime, endTime)

	// 权限过滤：非管理员只能看自己的数据
	if !currentUser.IsAdmin {
		categoryQuery = categoryQuery.Where("user_id = ?", currentUser.ID)
	} else {
		// 管理员可以按用户ID筛选
		if userIDFilter := c.Query("user_id"); userIDFilter != "" {
			if uid, err := strconv.ParseUint(userIDFilter, 10, 32); err == nil {
				categoryQuery = categoryQuery.Where("user_id = ?", uint(uid))
			}
		}
	}

	// 应用类别筛选
	if categoriesStr != "" {
		categories := strings.Split(categoriesStr, ",")
		for i := range categories {
			categories[i] = strings.TrimSpace(categories[i])
		}
		if len(categories) > 0 {
			categoryQuery = categoryQuery.Where("category IN ?", categories)
		}
	}

	categoryQuery.Group("category").Order("total DESC").Scan(&categoryStats)

	// 计算每个类别的占比
	for i := range categoryStats {
		if totalAmount > 0 {
			categoryStats[i].Percentage = (categoryStats[i].Total / totalAmount) * 100
		} else {
			categoryStats[i].Percentage = 0
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"range_type":     rangeType,
			"start_time":     startTime.Format("2006-01-02 15:04:05"),
			"end_time":       endTime.Format("2006-01-02 15:04:05"),
			"total_amount":   totalAmount,
			"total_count":    totalCount,
			"category_stats": categoryStats,
		},
	})
}

// AdminCreateExpenseRequest 管理员创建消费记录请求
type AdminCreateExpenseRequest struct {
	UserID      uint    `json:"user_id" binding:"required"`
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	Category    string  `json:"category" binding:"required"`
	Description string  `json:"description"`
	ExpenseTime string  `json:"expense_time" binding:"required"` // 格式: 2006-01-02 15:04:05
}

// CreateExpense 创建消费记录
// @Summary 创建消费记录
// @Description 创建一条新的消费记录。管理员可以为任何用户创建，非管理员只能为自己创建。
// @Tags 后台管理-消费记录
// @Accept json
// @Produce json
// @Param request body AdminCreateExpenseRequest true "消费记录信息"
// @Success 200 {object} map[string]interface{} "创建成功"
// @Failure 400 {object} map[string]interface{} "参数错误或类别不存在"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Router /admin/expenses [post]
func (h *AdminHandler) CreateExpense(c *gin.Context) {
	// 获取当前用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	var req AdminCreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}

	// 权限检查：非管理员只能为自己创建记录
	if !currentUser.IsAdmin && req.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足，只能为自己创建记录"})
		return
	}

	// 验证用户是否存在
	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	// 解析时间
	expenseTime, err2 := time.ParseInLocation("2006-01-02 15:04:05", req.ExpenseTime, time.Local)
	if err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "时间格式错误，应为: 2006-01-02 15:04:05"})
		return
	}

	// 校验类别是否存在（来源于数据库）
	req.Category = strings.TrimSpace(req.Category)
	if req.Category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "类别不能为空"})
		return
	}
	var cat models.ExpenseCategory
	if err := database.DB.Where("name = ?", req.Category).First(&cat).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的消费类别，请先在“消费类别”中维护"})
		return
	}

	// 创建消费记录
	expense := models.Expense{
		UserID:      req.UserID,
		Amount:      req.Amount,
		Category:    req.Category,
		Description: req.Description,
		ExpenseTime: expenseTime,
	}

	if err := database.DB.Create(&expense).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "创建失败")})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "创建成功",
		"data":    expense,
	})
}

// AdminUpdateExpenseRequest 管理员更新消费记录请求
type AdminUpdateExpenseRequest struct {
	Amount      float64 `json:"amount" binding:"omitempty,gt=0"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	ExpenseTime string  `json:"expense_time"` // 格式: 2006-01-02 15:04:05
}

// UpdateExpense 更新消费记录
// @Summary 更新消费记录
// @Description 更新指定的消费记录。管理员可以更新任何记录，非管理员只能更新自己的记录。
// @Tags 后台管理-消费记录
// @Accept json
// @Produce json
// @Param id path int true "消费记录ID"
// @Param request body AdminUpdateExpenseRequest true "更新的消费记录信息"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]interface{} "参数错误或类别不存在"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "记录不存在"
// @Router /admin/expenses/{id} [put]
func (h *AdminHandler) UpdateExpense(c *gin.Context) {
	// 获取当前用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	idStr := c.Param("id")
	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var expense models.Expense
	if err := database.DB.First(&expense, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "记录不存在"})
		return
	}

	// 权限检查：非管理员只能修改自己的记录
	if !currentUser.IsAdmin && expense.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足，只能修改自己的记录"})
		return
	}

	var req AdminUpdateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}

	// 更新字段
	updates := make(map[string]interface{})
	if req.Amount > 0 {
		updates["amount"] = req.Amount
	}
	if req.Category != "" {
		req.Category = strings.TrimSpace(req.Category)
		if req.Category == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "类别不能为空"})
			return
		}
		var cat models.ExpenseCategory
		if err := database.DB.Where("name = ?", req.Category).First(&cat).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的消费类别，请先在“消费类别”中维护"})
			return
		}
		updates["category"] = req.Category
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.ExpenseTime != "" {
		expenseTime, err := time.ParseInLocation("2006-01-02 15:04:05", req.ExpenseTime, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "时间格式错误，应为: 2006-01-02 15:04:05"})
			return
		}
		updates["expense_time"] = expenseTime
	}

	if err := database.DB.Model(&expense).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "更新失败")})
		return
	}

	// 重新获取更新后的记录
	database.DB.First(&expense, expense.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "更新成功",
		"data":    expense,
	})
}

// DeleteExpense 删除消费记录
// @Summary 删除消费记录
// @Description 删除指定的消费记录（软删除）。管理员可以删除任何记录，非管理员只能删除自己的记录。
// @Tags 后台管理-消费记录
// @Produce json
// @Param id path int true "消费记录ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "记录不存在"
// @Router /admin/expenses/{id} [delete]
func (h *AdminHandler) DeleteExpense(c *gin.Context) {
	// 获取当前用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	idStr := c.Param("id")
	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var expense models.Expense
	if err := database.DB.First(&expense, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "记录不存在"})
		return
	}

	// 权限检查：非管理员只能删除自己的记录
	if !currentUser.IsAdmin && expense.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足，只能删除自己的记录"})
		return
	}

	if err := database.DB.Delete(&expense).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "删除失败")})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// GetCategories 已废弃：路由已切到 CategoryHandler.List

// ExportExcel 导出 Excel
// @Summary 导出消费记录为Excel
// @Description 根据时间范围导出消费记录为Excel文件。管理员可导出所有用户数据，普通用户只能导出自己的数据。
// @Tags 后台管理-导出
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param start_time query string true "开始时间 (YYYY-MM-DD)"
// @Param end_time query string true "结束时间 (YYYY-MM-DD)"
// @Success 200 {file} file "Excel文件"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /admin/export/excel [get]
func (h *AdminHandler) ExportExcel(c *gin.Context) {
	// 获取当前登录用户
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	startTime := c.Query("start_time")
	endTime := c.Query("end_time")

	if startTime == "" || endTime == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请提供开始时间和结束时间"})
		return
	}

	start, err := time.ParseInLocation("2006-01-02", startTime, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "开始时间格式错误"})
		return
	}

	end, err := time.ParseInLocation("2006-01-02", endTime, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "结束时间格式错误"})
		return
	}
	end = end.Add(24*time.Hour - time.Second)

	// 查询数据
	type ExpenseWithUser struct {
		models.Expense
		Username string
	}

	var expenses []ExpenseWithUser
	query := database.DB.Model(&models.Expense{}).
		Select("expenses.*, users.username").
		Joins("LEFT JOIN users ON expenses.user_id = users.id").
		Where("expenses.expense_time >= ? AND expenses.expense_time <= ?", start, end)

	// 如果不是管理员，只导出当前用户的数据
	if !currentUser.IsAdmin {
		query = query.Where("expenses.user_id = ?", currentUser.ID)
	}

	query.Order("expenses.expense_time DESC").Scan(&expenses)

	// 创建 Excel 文件
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "消费记录"
	f.SetSheetName("Sheet1", sheetName)

	// 设置表头样式
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4F81BD"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// 数据样式
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// 设置列宽
	f.SetColWidth(sheetName, "A", "A", 10)
	f.SetColWidth(sheetName, "B", "B", 15)
	f.SetColWidth(sheetName, "C", "C", 12)
	f.SetColWidth(sheetName, "D", "D", 12)
	f.SetColWidth(sheetName, "E", "E", 30)
	f.SetColWidth(sheetName, "F", "F", 20)
	f.SetColWidth(sheetName, "G", "G", 20)

	// 写入表头
	headers := []string{"ID", "用户名", "金额", "类别", "描述", "消费时间", "创建时间"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// 写入数据
	var totalAmount float64
	for i, expense := range expenses {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), expense.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), expense.Username)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), expense.Amount)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), expense.Category)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), expense.Description)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), expense.ExpenseTime.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), expense.CreatedAt.Format("2006-01-02 15:04:05"))

		// 设置数据样式
		f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("G%d", row), dataStyle)
		totalAmount += expense.Amount
	}

	// 添加汇总行
	summaryRow := len(expenses) + 2
	summaryStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFC000"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", summaryRow), "合计")
	f.MergeCell(sheetName, fmt.Sprintf("A%d", summaryRow), fmt.Sprintf("B%d", summaryRow))
	f.SetCellValue(sheetName, fmt.Sprintf("C%d", summaryRow), totalAmount)
	f.SetCellValue(sheetName, fmt.Sprintf("D%d", summaryRow), fmt.Sprintf("共 %d 条记录", len(expenses)))
	f.MergeCell(sheetName, fmt.Sprintf("D%d", summaryRow), fmt.Sprintf("G%d", summaryRow))
	f.SetCellStyle(sheetName, fmt.Sprintf("A%d", summaryRow), fmt.Sprintf("G%d", summaryRow), summaryStyle)

	// 设置响应头
	filename := fmt.Sprintf("消费记录_%s_%s.xlsx", startTime, endTime)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", filename))

	// 写入响应
	if err := f.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "生成 Excel 失败"})
		return
	}
}
