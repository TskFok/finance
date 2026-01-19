package api

import (
	"net/http"
	"strings"
	"strconv"
	"time"

	"finance/database"
	"finance/middleware"
	"finance/models"

	"github.com/gin-gonic/gin"
)

// ExpenseHandler 消费记录处理器
type ExpenseHandler struct{}

// NewExpenseHandler 创建消费记录处理器
func NewExpenseHandler() *ExpenseHandler {
	return &ExpenseHandler{}
}

// CreateExpenseRequest 创建消费记录请求
type CreateExpenseRequest struct {
	Amount      float64 `json:"amount" binding:"required,gt=0" example:"99.99"`
	Category    string  `json:"category" binding:"required" example:"餐饮"`
	Description string  `json:"description" example:"午餐"`
	ExpenseTime string  `json:"expense_time" binding:"required" example:"2024-01-15 12:30:00"`
}

// UpdateExpenseRequest 更新消费记录请求
type UpdateExpenseRequest struct {
	Amount      float64 `json:"amount" binding:"omitempty,gt=0" example:"99.99"`
	Category    string  `json:"category" example:"餐饮"`
	Description string  `json:"description" example:"午餐"`
	ExpenseTime string  `json:"expense_time" example:"2024-01-15 12:30:00"`
}

// ExpenseListRequest 消费记录列表请求
type ExpenseListRequest struct {
	Page      int    `form:"page" example:"1"`
	PageSize  int    `form:"page_size" example:"10"`
	Category  string `form:"category" example:"餐饮"`
	StartTime string `form:"start_time" example:"2024-01-01"`
	EndTime   string `form:"end_time" example:"2024-12-31"`
}

// Create 创建消费记录
// @Summary 创建消费记录
// @Description 创建一条新的消费记录
// @Tags 消费记录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateExpenseRequest true "消费记录信息"
// @Success 200 {object} Response{data=models.Expense} "创建成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/expenses [post]
func (h *ExpenseHandler) Create(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	var req CreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 校验类别是否存在（来源于数据库）
	req.Category = strings.TrimSpace(req.Category)
	if req.Category == "" {
		BadRequest(c, "类别不能为空")
		return
	}
	var cat models.ExpenseCategory
	if err := database.DB.Where("name = ?", req.Category).First(&cat).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的消费类别，请先在后台维护类别"})
		return
	}

	// 解析时间
	expenseTime, err := time.ParseInLocation("2006-01-02 15:04:05", req.ExpenseTime, time.Local)
	if err != nil {
		BadRequest(c, "时间格式错误，应为: 2006-01-02 15:04:05")
		return
	}

	expense := models.Expense{
		UserID:      userID,
		Amount:      req.Amount,
		Category:    req.Category,
		Description: req.Description,
		ExpenseTime: expenseTime,
	}

	if err := database.DB.Create(&expense).Error; err != nil {
		InternalError(c, "创建消费记录失败: "+err.Error())
		return
	}

	SuccessWithMessage(c, "创建成功", expense)
}

// List 获取消费记录列表
// @Summary 获取消费记录列表
// @Description 获取当前用户的消费记录列表，支持分页和筛选
// @Tags 消费记录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param category query string false "类别筛选"
// @Param start_time query string false "开始时间 (2024-01-01)"
// @Param end_time query string false "结束时间 (2024-12-31)"
// @Success 200 {object} Response{data=PageResponse{list=[]models.Expense}} "获取成功"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/expenses [get]
func (h *ExpenseHandler) List(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	var req ExpenseListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 默认分页参数
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	query := database.DB.Model(&models.Expense{}).Where("user_id = ?", userID)

	// 类别筛选
	if req.Category != "" {
		query = query.Where("category = ?", req.Category)
	}

	// 时间范围筛选
	if req.StartTime != "" {
		startTime, err := time.ParseInLocation("2006-01-02", req.StartTime, time.Local)
		if err == nil {
			query = query.Where("expense_time >= ?", startTime)
		}
	}
	if req.EndTime != "" {
		endTime, err := time.ParseInLocation("2006-01-02", req.EndTime, time.Local)
		if err == nil {
			// 包含结束日期当天
			endTime = endTime.Add(24*time.Hour - time.Second)
			query = query.Where("expense_time <= ?", endTime)
		}
	}

	// 获取总数
	var total int64
	query.Count(&total)

	// 获取列表
	var expenses []models.Expense
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("expense_time DESC").Offset(offset).Limit(req.PageSize).Find(&expenses).Error; err != nil {
		InternalError(c, "查询失败: "+err.Error())
		return
	}

	Success(c, PageResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		List:     expenses,
	})
}

// Get 获取单条消费记录
// @Summary 获取单条消费记录
// @Description 根据ID获取消费记录详情
// @Tags 消费记录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "消费记录ID"
// @Success 200 {object} Response{data=models.Expense} "获取成功"
// @Failure 401 {object} Response "未授权"
// @Failure 404 {object} Response "记录不存在"
// @Router /api/v1/expenses/{id} [get]
func (h *ExpenseHandler) Get(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "无效的ID")
		return
	}

	var expense models.Expense
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&expense).Error; err != nil {
		NotFound(c, "记录不存在")
		return
	}

	Success(c, expense)
}

// Update 更新消费记录
// @Summary 更新消费记录
// @Description 更新指定的消费记录
// @Tags 消费记录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "消费记录ID"
// @Param request body UpdateExpenseRequest true "消费记录信息"
// @Success 200 {object} Response{data=models.Expense} "更新成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 401 {object} Response "未授权"
// @Failure 404 {object} Response "记录不存在"
// @Router /api/v1/expenses/{id} [put]
func (h *ExpenseHandler) Update(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "无效的ID")
		return
	}

	var expense models.Expense
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&expense).Error; err != nil {
		NotFound(c, "记录不存在")
		return
	}

	var req UpdateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
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
			BadRequest(c, "类别不能为空")
			return
		}
		var cat models.ExpenseCategory
		if err := database.DB.Where("name = ?", req.Category).First(&cat).Error; err != nil {
			BadRequest(c, "无效的消费类别，请先在后台维护类别")
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
			BadRequest(c, "时间格式错误，应为: 2006-01-02 15:04:05")
			return
		}
		updates["expense_time"] = expenseTime
	}

	if err := database.DB.Model(&expense).Updates(updates).Error; err != nil {
		InternalError(c, "更新失败: "+err.Error())
		return
	}

	// 重新获取更新后的记录
	database.DB.First(&expense, expense.ID)
	SuccessWithMessage(c, "更新成功", expense)
}

// Delete 删除消费记录
// @Summary 删除消费记录
// @Description 删除指定的消费记录
// @Tags 消费记录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "消费记录ID"
// @Success 200 {object} Response "删除成功"
// @Failure 401 {object} Response "未授权"
// @Failure 404 {object} Response "记录不存在"
// @Router /api/v1/expenses/{id} [delete]
func (h *ExpenseHandler) Delete(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "无效的ID")
		return
	}

	var expense models.Expense
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&expense).Error; err != nil {
		NotFound(c, "记录不存在")
		return
	}

	if err := database.DB.Delete(&expense).Error; err != nil {
		InternalError(c, "删除失败: "+err.Error())
		return
	}

	SuccessWithMessage(c, "删除成功", nil)
}

// GetCategories 获取消费类别列表
// @Summary 获取消费类别列表
// @Description 获取所有可用的消费类别
// @Tags 消费记录
// @Accept json
// @Produce json
// @Success 200 {object} Response{data=[]string} "获取成功"
// @Router /api/v1/categories [get]
func (h *ExpenseHandler) GetCategories(c *gin.Context) {
	var list []models.ExpenseCategory
	if err := database.DB.Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		InternalError(c, "查询失败: "+err.Error())
		return
	}
	// App 端返回 string[]（保持兼容）
	var names []string
	for _, it := range list {
		names = append(names, it.Name)
	}
	Success(c, names)
}

// GetStatistics 获取消费统计
// @Summary 获取消费统计
// @Description 获取指定时间范围内的消费统计
// @Tags 消费记录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param start_time query string false "开始时间 (2024-01-01)"
// @Param end_time query string false "结束时间 (2024-12-31)"
// @Success 200 {object} Response "获取成功"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/expenses/statistics [get]
func (h *ExpenseHandler) GetStatistics(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	query := database.DB.Model(&models.Expense{}).Where("user_id = ?", userID)

	// 时间范围筛选
	if startTimeStr != "" {
		startTime, err := time.ParseInLocation("2006-01-02", startTimeStr, time.Local)
		if err == nil {
			query = query.Where("expense_time >= ?", startTime)
		}
	}
	if endTimeStr != "" {
		endTime, err := time.ParseInLocation("2006-01-02", endTimeStr, time.Local)
		if err == nil {
			endTime = endTime.Add(24*time.Hour - time.Second)
			query = query.Where("expense_time <= ?", endTime)
		}
	}

	// 总金额
	var totalAmount float64
	query.Select("COALESCE(SUM(amount), 0)").Scan(&totalAmount)

	// 按类别统计
	type CategoryStat struct {
		Category string  `json:"category"`
		Total    float64 `json:"total"`
		Count    int64   `json:"count"`
	}
	var categoryStats []CategoryStat

	database.DB.Model(&models.Expense{}).
		Select("category, SUM(amount) as total, COUNT(*) as count").
		Where("user_id = ?", userID).
		Group("category").
		Order("total DESC").
		Scan(&categoryStats)

	Success(c, gin.H{
		"total_amount":    totalAmount,
		"category_stats":  categoryStats,
	})
}

