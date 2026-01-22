package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"finance/database"
	"finance/middleware"
	"finance/models"

	"github.com/gin-gonic/gin"
)

// IncomeHandler 收入处理器（App端）
type IncomeHandler struct{}

func NewIncomeHandler() *IncomeHandler {
	return &IncomeHandler{}
}

type CreateIncomeRequest struct {
	Amount     float64 `json:"amount" binding:"required,gt=0" example:"5000.00"`
	Type       string  `json:"type" binding:"required" example:"工资"`
	IncomeTime string  `json:"income_time" binding:"required" example:"2024-01-15 09:00:00"`
}

type UpdateIncomeRequest struct {
	Amount     float64 `json:"amount" binding:"omitempty,gt=0"`
	Type       string  `json:"type"`
	IncomeTime string  `json:"income_time"`
}

type IncomeListRequest struct {
	Page      int    `form:"page" example:"1"`
	PageSize  int    `form:"page_size" example:"10"`
	Type      string `form:"type" example:"工资"`
	StartTime string `form:"start_time" example:"2024-01-01"`
	EndTime   string `form:"end_time" example:"2024-12-31"`
}

// Create 创建收入
// @Summary 创建收入
// @Description 创建一条新的收入记录
// @Tags 收入
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateIncomeRequest true "收入信息"
// @Success 200 {object} Response{data=models.Income} "创建成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/incomes [post]
func (h *IncomeHandler) Create(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var req CreateIncomeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", req.IncomeTime, time.Local)
	if err != nil {
		BadRequest(c, "时间格式错误，应为: 2006-01-02 15:04:05")
		return
	}
	in := models.Income{UserID: userID, Amount: req.Amount, Type: req.Type, IncomeTime: t}
	if err := database.DB.Create(&in).Error; err != nil {
		InternalError(c, "创建收入失败: "+err.Error())
		return
	}
	SuccessWithMessage(c, "创建成功", in)
}

// List 获取收入列表
// @Summary 获取收入列表
// @Description 获取当前用户的收入列表，支持分页与筛选
// @Tags 收入
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param type query string false "收入类型筛选"
// @Param start_time query string false "开始时间 (2024-01-01)"
// @Param end_time query string false "结束时间 (2024-12-31)"
// @Success 200 {object} Response{data=PageResponse{list=[]models.Income}} "获取成功"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/incomes [get]
func (h *IncomeHandler) List(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var req IncomeListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	query := database.DB.Model(&models.Income{}).Where("user_id = ?", userID)
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	if req.StartTime != "" {
		if t, err := time.ParseInLocation("2006-01-02", req.StartTime, time.Local); err == nil {
			query = query.Where("income_time >= ?", t)
		}
	}
	if req.EndTime != "" {
		if t, err := time.ParseInLocation("2006-01-02", req.EndTime, time.Local); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			query = query.Where("income_time <= ?", t)
		}
	}

	var total int64
	query.Count(&total)
	var list []models.Income
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("income_time DESC").Offset(offset).Limit(req.PageSize).Find(&list).Error; err != nil {
		InternalError(c, "查询失败: "+err.Error())
		return
	}
	Success(c, PageResponse{Total: total, Page: req.Page, PageSize: req.PageSize, List: list})
}

// Get 获取单条收入
// @Summary 获取单条收入
// @Description 根据ID获取收入详情
// @Tags 收入
// @Produce json
// @Security BearerAuth
// @Param id path int true "收入ID"
// @Success 200 {object} Response{data=models.Income} "获取成功"
// @Failure 401 {object} Response "未授权"
// @Failure 404 {object} Response "记录不存在"
// @Router /api/v1/incomes/{id} [get]
func (h *IncomeHandler) Get(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "无效的ID")
		return
	}
	var in models.Income
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&in).Error; err != nil {
		NotFound(c, "记录不存在")
		return
	}
	Success(c, in)
}

// Update 更新收入
// @Summary 更新收入
// @Description 更新指定的收入记录
// @Tags 收入
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "收入ID"
// @Param request body UpdateIncomeRequest true "收入信息"
// @Success 200 {object} Response{data=models.Income} "更新成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 401 {object} Response "未授权"
// @Failure 404 {object} Response "记录不存在"
// @Router /api/v1/incomes/{id} [put]
func (h *IncomeHandler) Update(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "无效的ID")
		return
	}
	var in models.Income
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&in).Error; err != nil {
		NotFound(c, "记录不存在")
		return
	}
	var req UpdateIncomeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}
	updates := map[string]interface{}{}
	if req.Amount > 0 {
		updates["amount"] = req.Amount
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.IncomeTime != "" {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", req.IncomeTime, time.Local)
		if err != nil {
			BadRequest(c, "时间格式错误，应为: 2006-01-02 15:04:05")
			return
		}
		updates["income_time"] = t
	}
	if err := database.DB.Model(&in).Updates(updates).Error; err != nil {
		InternalError(c, "更新失败: "+err.Error())
		return
	}
	database.DB.First(&in, in.ID)
	SuccessWithMessage(c, "更新成功", in)
}

// Delete 删除收入
// @Summary 删除收入
// @Description 删除指定的收入记录
// @Tags 收入
// @Produce json
// @Security BearerAuth
// @Param id path int true "收入ID"
// @Success 200 {object} Response "删除成功"
// @Failure 401 {object} Response "未授权"
// @Failure 404 {object} Response "记录不存在"
// @Router /api/v1/incomes/{id} [delete]
func (h *IncomeHandler) Delete(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "无效的ID")
		return
	}
	var in models.Income
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&in).Error; err != nil {
		NotFound(c, "记录不存在")
		return
	}
	if err := database.DB.Delete(&in).Error; err != nil {
		InternalError(c, "删除失败: "+err.Error())
		return
	}
	SuccessWithMessage(c, "删除成功", nil)
}

// ===== 后台管理（Admin） =====

type AdminCreateIncomeRequest struct {
	UserID     uint    `json:"user_id" binding:"required"`
	Amount     float64 `json:"amount" binding:"required,gt=0"`
	Type       string  `json:"type" binding:"required"`
	IncomeTime string  `json:"income_time" binding:"required"` // 2006-01-02 15:04:05
}

type AdminUpdateIncomeRequest struct {
	Amount     float64 `json:"amount" binding:"omitempty,gt=0"`
	Type       string  `json:"type"`
	IncomeTime string  `json:"income_time"`
}

// GetAllIncomes 获取收入记录列表（后台管理）
// @Summary 获取收入记录列表
// @Description 获取收入记录列表，支持分页、时间范围、类型、用户名筛选。管理员可查看所有记录并可按用户ID筛选，非管理员只能查看自己的记录。
// @Tags 后台管理-收入管理
// @Produce json
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页数量，默认20"
// @Param start_time query string false "开始时间 (YYYY-MM-DD)"
// @Param end_time query string false "结束时间 (YYYY-MM-DD)"
// @Param type query string false "收入类型筛选"
// @Param username query string false "用户名筛选（模糊匹配）"
// @Param user_id query int false "用户ID筛选（仅管理员可用）"
// @Success 200 {object} map[string]interface{} "获取成功，返回分页数据"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /admin/incomes [get]
func (h *AdminHandler) GetAllIncomes(c *gin.Context) {
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
	typ := c.Query("type")
	username := c.Query("username")
	userIDFilter := c.Query("user_id") // 管理员可以按用户ID筛选

	query := database.DB.Model(&models.Income{}).
		Select("incomes.*, users.username").
		Joins("LEFT JOIN users ON incomes.user_id = users.id")

	// 权限过滤：非管理员只能看自己的数据
	if !currentUser.IsAdmin {
		query = query.Where("incomes.user_id = ?", currentUser.ID)
	} else {
		// 管理员可以按用户ID筛选
		if userIDFilter != "" {
			if uid, err := strconv.ParseUint(userIDFilter, 10, 32); err == nil {
				query = query.Where("incomes.user_id = ?", uint(uid))
			}
		}
	}

	if startTime != "" {
		if t, err := time.ParseInLocation("2006-01-02", startTime, time.Local); err == nil {
			query = query.Where("incomes.income_time >= ?", t)
		}
	}
	if endTime != "" {
		if t, err := time.ParseInLocation("2006-01-02", endTime, time.Local); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			query = query.Where("incomes.income_time <= ?", t)
		}
	}
	if typ != "" {
		query = query.Where("incomes.type = ?", typ)
	}
	// 用户名查询只对管理员开放
	if username != "" && currentUser.IsAdmin {
		query = query.Where("users.username LIKE ?", "%"+username+"%")
	}

	var total int64
	query.Count(&total)

	type IncomeWithUser struct {
		models.Income
		Username string `json:"username"`
	}
	var list []IncomeWithUser
	offset := (page - 1) * pageSize
	query.Order("incomes.income_time DESC").Offset(offset).Limit(pageSize).Scan(&list)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"list":      list,
		},
	})
}

// CreateIncome 创建收入记录（后台管理）
// @Summary 创建收入记录
// @Description 创建一条新的收入记录。管理员可以为任何用户创建，非管理员只能为自己创建。
// @Tags 后台管理-收入管理
// @Accept json
// @Produce json
// @Param request body AdminCreateIncomeRequest true "收入信息"
// @Success 200 {object} map[string]interface{} "创建成功"
// @Failure 400 {object} map[string]interface{} "参数错误或时间格式错误"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "用户不存在"
// @Router /admin/incomes [post]
func (h *AdminHandler) CreateIncome(c *gin.Context) {
	// 获取当前用户
	userIDStr, err := c.Cookie("admin_user_id")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}
	var currentUser models.User
	if err := database.DB.First(&currentUser, uint(userID)).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	var req AdminCreateIncomeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 权限检查：非管理员只能为自己创建记录
	if !currentUser.IsAdmin && req.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足，只能为自己创建记录"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", req.IncomeTime, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "时间格式错误，应为: 2006-01-02 15:04:05"})
		return
	}
	in := models.Income{UserID: req.UserID, Amount: req.Amount, Type: req.Type, IncomeTime: t}
	if err := database.DB.Create(&in).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "创建成功", "data": in})
}

// UpdateIncome 更新收入记录（后台管理）
// @Summary 更新收入记录
// @Description 更新指定的收入记录。管理员可以更新任何记录，非管理员只能更新自己的记录。
// @Tags 后台管理-收入管理
// @Accept json
// @Produce json
// @Param id path int true "收入记录ID"
// @Param request body AdminUpdateIncomeRequest true "更新的收入信息"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]interface{} "参数错误或时间格式错误"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 404 {object} map[string]interface{} "记录不存在"
// @Router /admin/incomes/{id} [put]
func (h *AdminHandler) UpdateIncome(c *gin.Context) {
	// 获取当前用户
	userIDStr, err := c.Cookie("admin_user_id")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}
	var currentUser models.User
	if err := database.DB.First(&currentUser, uint(userID)).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	idStr := c.Param("id")
	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var in models.Income
	if err := database.DB.First(&in, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "记录不存在"})
		return
	}

	// 权限检查：非管理员只能修改自己的记录
	if !currentUser.IsAdmin && in.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "权限不足，只能修改自己的记录"})
		return
	}
	var req AdminUpdateIncomeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}
	updates := map[string]interface{}{}
	if req.Amount > 0 {
		updates["amount"] = req.Amount
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.IncomeTime != "" {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", req.IncomeTime, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "时间格式错误，应为: 2006-01-02 15:04:05"})
			return
		}
		updates["income_time"] = t
	}
	if err := database.DB.Model(&in).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新失败: " + err.Error()})
		return
	}
	database.DB.First(&in, in.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "更新成功", "data": in})
}

// DeleteIncome 删除收入记录（后台管理）
// @Summary 删除收入记录
// @Description 删除指定的收入记录（软删除）。管理员可以删除任何记录，非管理员只能删除自己的记录。
// @Tags 后台管理-收入管理
// @Produce json
// @Param id path int true "收入记录ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 400 {object} map[string]interface{} "无效的ID"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 404 {object} map[string]interface{} "记录不存在"
// @Router /admin/incomes/{id} [delete]
func (h *AdminHandler) DeleteIncome(c *gin.Context) {
	idStr := c.Param("id")
	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var in models.Income
	if err := database.DB.First(&in, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "记录不存在"})
		return
	}
	if err := database.DB.Delete(&in).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}
