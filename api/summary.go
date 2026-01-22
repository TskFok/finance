package api

import (
	"net/http"
	"strconv"
	"time"

	"finance/database"
	"finance/middleware"
	"finance/models"

	"github.com/gin-gonic/gin"
)

// IncomeExpenseSummaryResponse 支出/收入汇总返回
type IncomeExpenseSummaryResponse struct {
	TotalExpense float64 `json:"total_expense" example:"123.45"` // 支出总和
	TotalIncome  float64 `json:"total_income" example:"5000.00"` // 收入总和
}

// GetIncomeExpenseSummary 获取支出和收入汇总（App端，JWT）
// @Summary 获取支出/收入汇总
// @Description 按时间范围统计当前用户的支出总和与收入总和。不传 start_time/end_time 则统计全部时间。
// @Tags 统计
// @Produce json
// @Security BearerAuth
// @Param start_time query string false "开始时间 (YYYY-MM-DD)，例如 2024-01-01"
// @Param end_time query string false "结束时间 (YYYY-MM-DD)，例如 2024-12-31"
// @Success 200 {object} Response{data=IncomeExpenseSummaryResponse} "获取成功"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/statistics/summary [get]
func (h *ExpenseHandler) GetIncomeExpenseSummary(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	expenseQ := database.DB.Model(&models.Expense{}).Where("user_id = ?", userID)
	incomeQ := database.DB.Model(&models.Income{}).Where("user_id = ?", userID)

	if startTimeStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", startTimeStr, time.Local); err == nil {
			expenseQ = expenseQ.Where("expense_time >= ?", t)
			incomeQ = incomeQ.Where("income_time >= ?", t)
		}
	}
	if endTimeStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", endTimeStr, time.Local); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			expenseQ = expenseQ.Where("expense_time <= ?", t)
			incomeQ = incomeQ.Where("income_time <= ?", t)
		}
	}

	var totalExpense float64
	var totalIncome float64
	expenseQ.Select("COALESCE(SUM(amount), 0)").Scan(&totalExpense)
	incomeQ.Select("COALESCE(SUM(amount), 0)").Scan(&totalIncome)

	Success(c, IncomeExpenseSummaryResponse{
		TotalExpense: totalExpense,
		TotalIncome:  totalIncome,
	})
}

// AdminIncomeExpenseSummary 获取支出和收入汇总（后台，Cookie）
// @Summary 获取支出/收入汇总（后台）
// @Description 按时间范围统计支出总和与收入总和。管理员可传user_id统计指定用户，非管理员只能统计自己的数据（忽略user_id）。不传start_time/end_time则统计全部时间。
// @Tags 后台管理-统计
// @Produce json
// @Param start_time query string false "开始时间 (YYYY-MM-DD)，例如 2024-01-01"
// @Param end_time query string false "结束时间 (YYYY-MM-DD)，例如 2024-12-31"
// @Param user_id query int false "用户ID（仅管理员可用）"
// @Success 200 {object} map[string]interface{} "获取成功，返回支出总和和收入总和"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /admin/statistics/summary [get]
func (h *AdminHandler) AdminIncomeExpenseSummary(c *gin.Context) {
	currentUser, err := getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")
	userIDFilter := c.Query("user_id")

	targetUserID := currentUser.ID
	if currentUser.IsAdmin && userIDFilter != "" {
		if uid, err := strconv.ParseUint(userIDFilter, 10, 32); err == nil {
			targetUserID = uint(uid)
		}
	}

	expenseQ := database.DB.Model(&models.Expense{}).Where("user_id = ?", targetUserID)
	incomeQ := database.DB.Model(&models.Income{}).Where("user_id = ?", targetUserID)

	if startTimeStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", startTimeStr, time.Local); err == nil {
			expenseQ = expenseQ.Where("expense_time >= ?", t)
			incomeQ = incomeQ.Where("income_time >= ?", t)
		}
	}
	if endTimeStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", endTimeStr, time.Local); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			expenseQ = expenseQ.Where("expense_time <= ?", t)
			incomeQ = incomeQ.Where("income_time <= ?", t)
		}
	}

	var totalExpense float64
	var totalIncome float64
	expenseQ.Select("COALESCE(SUM(amount), 0)").Scan(&totalExpense)
	incomeQ.Select("COALESCE(SUM(amount), 0)").Scan(&totalIncome)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_expense": totalExpense,
			"total_income":  totalIncome,
		},
	})
}
