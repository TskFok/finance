package api

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"finance/database"
	"finance/middleware"
	"finance/models"

	"github.com/gin-gonic/gin"
)

// ExportHandler 导出处理器
type ExportHandler struct{}

// NewExportHandler 创建导出处理器
func NewExportHandler() *ExportHandler {
	return &ExportHandler{}
}

// ExportCSV 导出消费记录为 CSV
// @Summary 导出消费记录
// @Description 根据时间范围导出消费记录为 CSV 文件
// @Tags 导出
// @Accept json
// @Produce text/csv
// @Security BearerAuth
// @Param start_time query string true "开始时间 (2024-01-01)"
// @Param end_time query string true "结束时间 (2024-12-31)"
// @Success 200 {file} file "CSV 文件"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/export/csv [get]
func (h *ExportHandler) ExportCSV(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	if startTimeStr == "" || endTimeStr == "" {
		BadRequest(c, "请提供开始时间和结束时间")
		return
	}

	startTime, err := time.ParseInLocation("2006-01-02", startTimeStr, time.Local)
	if err != nil {
		BadRequest(c, "开始时间格式错误，应为: 2006-01-02")
		return
	}

	endTime, err := time.ParseInLocation("2006-01-02", endTimeStr, time.Local)
	if err != nil {
		BadRequest(c, "结束时间格式错误，应为: 2006-01-02")
		return
	}
	endTime = endTime.Add(24*time.Hour - time.Second)

	// 查询数据
	var expenses []models.Expense
	if err := database.DB.Where("user_id = ? AND expense_time >= ? AND expense_time <= ?", userID, startTime, endTime).
		Order("expense_time DESC").
		Find(&expenses).Error; err != nil {
		InternalError(c, "查询数据失败: "+err.Error())
		return
	}

	// 生成 CSV
	buf := new(bytes.Buffer)
	// 添加 BOM 以支持 Excel 中文显示
	buf.WriteString("\xEF\xBB\xBF")
	
	writer := csv.NewWriter(buf)

	// 写入表头
	headers := []string{"ID", "金额", "类别", "描述", "消费时间", "创建时间"}
	if err := writer.Write(headers); err != nil {
		InternalError(c, "生成 CSV 失败")
		return
	}

	// 写入数据
	for _, expense := range expenses {
		row := []string{
			fmt.Sprintf("%d", expense.ID),
			fmt.Sprintf("%.2f", expense.Amount),
			expense.Category,
			expense.Description,
			expense.ExpenseTime.Format("2006-01-02 15:04:05"),
			expense.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(row); err != nil {
			InternalError(c, "生成 CSV 失败")
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		InternalError(c, "生成 CSV 失败")
		return
	}

	// 设置响应头
	filename := fmt.Sprintf("expenses_%s_%s.csv", startTimeStr, endTimeStr)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Length", fmt.Sprintf("%d", buf.Len()))

	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

// ExportJSON 导出消费记录为 JSON
// @Summary 导出消费记录为 JSON
// @Description 根据时间范围导出消费记录为 JSON 格式
// @Tags 导出
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param start_time query string true "开始时间 (2024-01-01)"
// @Param end_time query string true "结束时间 (2024-12-31)"
// @Success 200 {object} Response{data=[]models.Expense} "导出成功"
// @Failure 400 {object} Response "请求参数错误"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/export/json [get]
func (h *ExportHandler) ExportJSON(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	if startTimeStr == "" || endTimeStr == "" {
		BadRequest(c, "请提供开始时间和结束时间")
		return
	}

	startTime, err := time.ParseInLocation("2006-01-02", startTimeStr, time.Local)
	if err != nil {
		BadRequest(c, "开始时间格式错误，应为: 2006-01-02")
		return
	}

	endTime, err := time.ParseInLocation("2006-01-02", endTimeStr, time.Local)
	if err != nil {
		BadRequest(c, "结束时间格式错误，应为: 2006-01-02")
		return
	}
	endTime = endTime.Add(24*time.Hour - time.Second)

	// 查询数据
	var expenses []models.Expense
	if err := database.DB.Where("user_id = ? AND expense_time >= ? AND expense_time <= ?", userID, startTime, endTime).
		Order("expense_time DESC").
		Find(&expenses).Error; err != nil {
		InternalError(c, "查询数据失败: "+err.Error())
		return
	}

	// 计算汇总信息
	var totalAmount float64
	for _, expense := range expenses {
		totalAmount += expense.Amount
	}

	Success(c, gin.H{
		"start_time":   startTimeStr,
		"end_time":     endTimeStr,
		"total_count":  len(expenses),
		"total_amount": totalAmount,
		"expenses":     expenses,
	})
}

