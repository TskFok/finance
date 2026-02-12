package api

import (
	"net/http"
	"strconv"
	"strings"
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
		BadRequest(c, SafeErrorMessage(err, "参数错误"))
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
		InternalError(c, SafeErrorMessage(err, "创建消费记录失败"))
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
		BadRequest(c, SafeErrorMessage(err, "参数错误"))
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
		InternalError(c, SafeErrorMessage(err, "查询失败"))
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
		BadRequest(c, SafeErrorMessage(err, "参数错误"))
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
		InternalError(c, SafeErrorMessage(err, "更新失败"))
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
		InternalError(c, SafeErrorMessage(err, "删除失败"))
		return
	}

	SuccessWithMessage(c, "删除成功", nil)
}

// GetCategories 获取消费类别列表
// @Summary 获取消费类别列表
// @Description 获取所有可用的消费类别列表，返回完整的类别对象数组。类别按排序字段（sort）升序排列，排序相同时按ID升序排列。
// @Description
// @Description 返回的每个类别对象包含以下字段：
// @Description - id (uint): 类别唯一标识符，主键
// @Description - name (string): 类别名称，最大长度50字符，唯一索引，必填
// @Description - sort (int): 排序值，用于控制类别显示顺序，值越小越靠前，默认值为0
// @Description - created_at (time.Time): 创建时间，ISO 8601格式的时间字符串
// @Description - updated_at (time.Time): 更新时间，ISO 8601格式的时间字符串
// @Description
// @Description 示例响应：
// @Description {
// @Description   "code": 200,
// @Description   "message": "success",
// @Description   "data": [
// @Description     {
// @Description       "id": 1,
// @Description       "name": "餐饮",
// @Description       "sort": 0,
// @Description       "created_at": "2024-01-01T00:00:00Z",
// @Description       "updated_at": "2024-01-01T00:00:00Z"
// @Description     },
// @Description     {
// @Description       "id": 2,
// @Description       "name": "交通",
// @Description       "sort": 1,
// @Description       "created_at": "2024-01-01T00:00:00Z",
// @Description       "updated_at": "2024-01-01T00:00:00Z"
// @Description     }
// @Description   ]
// @Description }
// @Tags 消费记录
// @Accept json
// @Produce json
// @Success 200 {object} Response{data=[]models.ExpenseCategory} "获取成功，返回类别列表数组"
// @Failure 500 {object} Response "服务器内部错误，查询失败时返回错误信息"
// @Router /api/v1/categories [get]
func (h *ExpenseHandler) GetCategories(c *gin.Context) {
	var list []models.ExpenseCategory
	if err := database.DB.Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		InternalError(c, SafeErrorMessage(err, "查询失败"))
		return
	}
	// 返回完整的类别对象数组，包含ID、名称、排序等信息
	Success(c, list)
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
		"total_amount":   totalAmount,
		"category_stats": categoryStats,
	})
}

// GetDetailedStatistics 获取详细消费统计（支持月/年/自定义时间范围和多个类别筛选）
// @Summary 获取详细消费统计
// @Description 获取消费统计信息，支持多种时间范围筛选（月、年、自定义）和多个类别筛选。返回按类别统计的数据，适合绘制饼图和柱状图。
// @Description
// @Description 时间范围类型说明：
// @Description - month: 按月统计，需要传入 year_month 参数（格式：2024-01）
// @Description - year: 按年统计，需要传入 year 参数（格式：2024）
// @Description - custom: 自定义时间范围，需要传入 start_time 和 end_time 参数（格式：2024-01-01）
// @Description
// @Description 类别筛选说明：
// @Description - categories: 可选的类别筛选，多个类别用逗号分隔（如：餐饮,交通），不传则统计所有类别
// @Description
// @Description 返回数据说明：
// @Description - total_amount: 总金额
// @Description - total_count: 总记录数
// @Description - category_stats: 按类别统计的数组，每个元素包含 category（类别名称）、total（总金额）、count（记录数）、percentage（占比百分比）
// @Tags 消费记录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param range_type query string true "时间范围类型：month（月）/year（年）/custom（自定义）" Enums(month,year,custom)
// @Param year_month query string false "年月（当range_type=month时必填，格式：2024-01）"
// @Param year query string false "年份（当range_type=year时必填，格式：2024）"
// @Param start_time query string false "开始时间（当range_type=custom时必填，格式：2024-01-01）"
// @Param end_time query string false "结束时间（当range_type=custom时必填，格式：2024-12-31）"
// @Param categories query string false "类别筛选，多个类别用逗号分隔（如：餐饮,交通）"
// @Success 200 {object} Response "获取成功，返回统计数据和分类统计"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/expenses/detailed-statistics [get]
func (h *ExpenseHandler) GetDetailedStatistics(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	rangeType := c.Query("range_type")
	if rangeType == "" {
		BadRequest(c, "range_type参数必填，可选值：month、year、custom")
		return
	}

	query := database.DB.Model(&models.Expense{}).Where("user_id = ?", userID)

	var startTime, endTime time.Time
	var err error

	// 根据时间范围类型设置时间范围
	switch rangeType {
	case "month":
		yearMonth := c.Query("year_month")
		if yearMonth == "" {
			BadRequest(c, "range_type=month时，year_month参数必填（格式：2024-01）")
			return
		}
		startTime, err = time.ParseInLocation("2006-01", yearMonth, time.Local)
		if err != nil {
			BadRequest(c, "year_month格式错误，应为：2024-01")
			return
		}
		// 该月的第一天 00:00:00
		startTime = time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, time.Local)
		// 该月的最后一天 23:59:59
		endTime = startTime.AddDate(0, 1, 0).Add(-time.Second)

	case "year":
		yearStr := c.Query("year")
		if yearStr == "" {
			BadRequest(c, "range_type=year时，year参数必填（格式：2024）")
			return
		}
		year, err := strconv.Atoi(yearStr)
		if err != nil || year < 2000 || year > 2100 {
			BadRequest(c, "year格式错误，应为4位数字（如：2024）")
			return
		}
		// 该年的第一天
		startTime = time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
		// 该年的最后一天
		endTime = time.Date(year, 12, 31, 23, 59, 59, 0, time.Local)

	case "custom":
		startTimeStr := c.Query("start_time")
		endTimeStr := c.Query("end_time")
		if startTimeStr == "" || endTimeStr == "" {
			BadRequest(c, "range_type=custom时，start_time和end_time参数必填（格式：2024-01-01）")
			return
		}
		startTime, err = time.ParseInLocation("2006-01-02", startTimeStr, time.Local)
		if err != nil {
			BadRequest(c, "start_time格式错误，应为：2024-01-01")
			return
		}
		endTime, err = time.ParseInLocation("2006-01-02", endTimeStr, time.Local)
		if err != nil {
			BadRequest(c, "end_time格式错误，应为：2024-12-31")
			return
		}
		// 包含结束日期当天
		endTime = endTime.Add(24*time.Hour - time.Second)

	default:
		BadRequest(c, "range_type参数值错误，可选值：month、year、custom")
		return
	}

	// 应用时间范围筛选
	query = query.Where("expense_time >= ? AND expense_time <= ?", startTime, endTime)

	// 类别筛选（支持多个类别）
	categoriesStr := c.Query("categories")
	if categoriesStr != "" {
		categories := strings.Split(categoriesStr, ",")
		// 去除空格
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
	query.Select("COALESCE(SUM(amount), 0)").Scan(&totalAmount)
	query.Count(&totalCount)

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
		Where("user_id = ? AND expense_time >= ? AND expense_time <= ?", userID, startTime, endTime)

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

	Success(c, gin.H{
		"range_type":     rangeType,
		"start_time":     startTime.Format("2006-01-02 15:04:05"),
		"end_time":       endTime.Format("2006-01-02 15:04:05"),
		"total_amount":   totalAmount,
		"total_count":    totalCount,
		"category_stats": categoryStats,
	})
}
