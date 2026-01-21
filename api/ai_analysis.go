package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"finance/database"
	"finance/models"

	"github.com/gin-gonic/gin"
)

// AIAnalysisHandler AI分析处理器
type AIAnalysisHandler struct{}

// NewAIAnalysisHandler 创建AI分析处理器
func NewAIAnalysisHandler() *AIAnalysisHandler {
	return &AIAnalysisHandler{}
}

// ExpenseWithUser 带用户名的消费记录
type ExpenseWithUser struct {
	models.Expense
	Username string `json:"username"`
}

// AnalysisRequest AI分析请求
type AnalysisRequest struct {
	ModelID   uint   `json:"model_id" binding:"required"`
	StartTime string `json:"start_time" binding:"required" example:"2024-01-01"`
	EndTime   string `json:"end_time" binding:"required" example:"2024-12-31"`
}

type sseAnalysisFrame struct {
	Type    string `json:"type"`              // delta | done | error
	Content string `json:"content,omitempty"` // delta内容或错误信息
}

func writeAnalysisSSE(c *gin.Context, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	_, _ = c.Writer.WriteString("data: " + string(b) + "\n\n")
	c.Writer.Flush()
}

// AnalyzeExpenses 分析消费记录（流式输出）
func (h *AIAnalysisHandler) AnalyzeExpenses(c *gin.Context) {
	var req AnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 获取AI模型配置
	var aiModel models.AIModel
	if err := database.DB.First(&aiModel, req.ModelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "AI模型不存在"})
		return
	}

	// 解析时间范围
	startTime, endTime, err := parseDateRange(req.StartTime, req.EndTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "时间格式错误"})
		return
	}

	// 查询消费记录
	var expenses []ExpenseWithUser
	q := database.DB.Model(&models.Expense{}).
		Select("expenses.*, users.username").
		Joins("LEFT JOIN users ON expenses.user_id = users.id").
		Where("expenses.expense_time >= ? AND expenses.expense_time <= ?", startTime, endTime)
	// Admin 端：如果是非管理员（cookie 登录），只分析自己的消费；管理员默认分析全局
	if u, e := getCurrentUser(c); e == nil && !u.IsAdmin {
		q = q.Where("expenses.user_id = ?", u.ID)
	}
	if err := q.Order("expenses.expense_time DESC").Scan(&expenses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询消费记录失败"})
		return
	}

	if len(expenses) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "该时间范围内没有消费记录"})
		return
	}

	// 构建分析提示词
	prompt := h.buildAnalysisPrompt(expenses, req.StartTime, req.EndTime)

	// 调用AI模型API（流式）
	uid := uint(0)
	if u, e := getCurrentUser(c); e == nil {
		uid = u.ID
	}
	if err := h.callAIModelStreamAndStore(c, aiModel, uid, req.StartTime, req.EndTime, prompt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "AI分析失败: " + err.Error()})
		return
	}
}

// buildAnalysisPrompt 构建分析提示词
func (h *AIAnalysisHandler) buildAnalysisPrompt(expenses []ExpenseWithUser, startTime, endTime string) string {
	// 统计信息
	var totalAmount float64
	categoryStats := make(map[string]float64)
	categoryCount := make(map[string]int)

	for _, exp := range expenses {
		totalAmount += exp.Amount
		categoryStats[exp.Category] += exp.Amount
		categoryCount[exp.Category]++
	}

	// 构建提示词
	prompt := fmt.Sprintf(`请分析以下消费记录数据，并提供详细的总结和建议：

时间范围：%s 至 %s
总记录数：%d 条
总消费金额：%.2f 元

消费类别统计：
`, startTime, endTime, len(expenses), totalAmount)

	for category, amount := range categoryStats {
		prompt += fmt.Sprintf("- %s: %.2f 元 (%d 条记录)\n", category, amount, categoryCount[category])
	}

	prompt += "\n详细消费记录（最近20条）：\n"
	maxRecords := 20
	if len(expenses) < maxRecords {
		maxRecords = len(expenses)
	}
	for i := 0; i < maxRecords; i++ {
		exp := expenses[i]
		prompt += fmt.Sprintf("- %s: %s 在 %s 消费 %.2f 元，类别：%s",
			exp.ExpenseTime.Format("2006-01-02 15:04"),
			exp.Username,
			exp.ExpenseTime.Format("2006-01-02 15:04:05"),
			exp.Amount,
			exp.Category)
		if exp.Description != "" {
			prompt += fmt.Sprintf("，说明：%s", exp.Description)
		}
		prompt += "\n"
	}

	prompt += `\n请提供：
1. 消费趋势分析
2. 主要消费类别分析
3. 消费习惯总结
4. 优化建议和理财建议

请用中文回答，内容要详细、专业、实用。`

	return prompt
}

// callAIModelStreamAndStore 调用AI模型API（流式输出），并在结束后保存分析历史（软删除支持）
func (h *AIAnalysisHandler) callAIModelStreamAndStore(c *gin.Context, aiModel models.AIModel, userID uint, startDate, endDate, prompt string) error {
	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用nginx缓冲

	// 构建请求体（兼容OpenAI格式）
	requestBody := map[string]interface{}{
		"model": aiModel.Name, // 可以根据模型配置调整
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"stream": true,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("构建请求失败: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", strings.TrimRight(aiModel.BaseURL, "/")+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aiModel.APIKey)

	// 发送请求
	client := &http.Client{Timeout: 300 * time.Second} // 5分钟超时
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求AI服务失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AI服务返回错误: %d, %s", resp.StatusCode, string(body))
	}

	// 使用带缓冲的读取器，逐行读取
	reader := bufio.NewReader(resp.Body)

	// 创建上下文用于检查客户端连接
	ctx := c.Request.Context()

	var out strings.Builder
	finished := false

	for {
		// 检查客户端是否断开连接（非阻塞检查）
		select {
		case <-ctx.Done():
			return fmt.Errorf("客户端断开连接")
		default:
		}

		// 设置读取超时，避免无限等待
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				// 正常结束
				finished = true
				break
			}
			return fmt.Errorf("读取流数据失败: %w", err)
		}

		// 处理这一行
		if len(line) == 0 {
			continue
		}
		// 处理并转发（JSON帧），同时累计输出
		delta, done := h.processAnalysisLineToJSON(c, line)
		if delta != "" {
			out.WriteString(delta)
		}
		if done {
			finished = true
			break
		}
	}

	// 存储历史（只有正常结束且客户端未断开才保存）
	if finished {
		his := models.AIAnalysisHistory{
			AIModelID: aiModel.ID,
			UserID:    userID,
			StartDate: startDate,
			EndDate:   endDate,
			Result:    out.String(),
		}
		_ = database.DB.Create(&his).Error
		// 确保前端一定收到 done
		writeAnalysisSSE(c, sseAnalysisFrame{Type: "done"})
	}

	return nil
}

// analyzeExpensesScoped App端：仅分析当前用户的消费，并写入 user_id
func (h *AIAnalysisHandler) analyzeExpensesScoped(c *gin.Context, userID uint) {
	var req AnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	var aiModel models.AIModel
	if err := database.DB.First(&aiModel, req.ModelID).Error; err != nil {
		NotFound(c, "AI模型不存在")
		return
	}

	startTime, endTime, err := parseDateRange(req.StartTime, req.EndTime)
	if err != nil {
		BadRequest(c, "时间格式错误")
		return
	}

	var expenses []ExpenseWithUser
	if err := database.DB.Model(&models.Expense{}).
		Select("expenses.*, users.username").
		Joins("LEFT JOIN users ON expenses.user_id = users.id").
		Where("expenses.user_id = ?", userID).
		Where("expenses.expense_time >= ? AND expenses.expense_time <= ?", startTime, endTime).
		Order("expenses.expense_time DESC").
		Scan(&expenses).Error; err != nil {
		InternalError(c, "查询消费记录失败")
		return
	}
	if len(expenses) == 0 {
		BadRequest(c, "该时间范围内没有消费记录")
		return
	}

	prompt := h.buildAnalysisPrompt(expenses, req.StartTime, req.EndTime)
	if err := h.callAIModelStreamAndStore(c, aiModel, userID, req.StartTime, req.EndTime, prompt); err != nil {
		InternalError(c, "AI分析失败: "+err.Error())
		return
	}
}

// listAnalysisHistoryScoped App端：按用户+模型分页返回（Response 结构）
func (h *AIAnalysisHandler) listAnalysisHistoryScoped(c *gin.Context, userID uint, requireUser bool) {
	modelIDStr := c.Query("model_id")
	if modelIDStr == "" {
		BadRequest(c, "缺少 model_id")
		return
	}
	modelID64, err := strconv.ParseUint(modelIDStr, 10, 32)
	if err != nil {
		BadRequest(c, "无效的 model_id")
		return
	}
	modelID := uint(modelID64)

	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		if v, e := strconv.Atoi(p); e == nil && v > 0 {
			page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, e := strconv.Atoi(ps); e == nil && v > 0 {
			pageSize = v
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := database.DB.Model(&models.AIAnalysisHistory{}).Where("ai_model_id = ?", modelID)
	if requireUser {
		query = query.Where("user_id = ?", userID)
	}
	var total int64
	query.Count(&total)

	var list []models.AIAnalysisHistory
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		InternalError(c, "查询失败: "+err.Error())
		return
	}
	Success(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"list":      list,
	})
}

// processAnalysisLineToJSON 解析上游SSE行，向前端输出 JSON 帧；返回增量文本与是否结束
func (h *AIAnalysisHandler) processAnalysisLineToJSON(c *gin.Context, line []byte) (string, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return "", false
	}
	if !bytes.HasPrefix(line, []byte("data: ")) {
		return "", false
	}
	data := bytes.TrimPrefix(line, []byte("data: "))
	if string(data) == "[DONE]" {
		writeAnalysisSSE(c, sseAnalysisFrame{Type: "done"})
		return "", true
	}
	var streamData map[string]interface{}
	if err := json.Unmarshal(data, &streamData); err != nil {
		return "", false
	}
	content := ""
	if choices, ok := streamData["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if v, ok := delta["content"].(string); ok {
					content = v
				}
			}
		}
	}
	if content == "" {
		return "", false
	}
	writeAnalysisSSE(c, sseAnalysisFrame{Type: "delta", Content: content})
	return content, false
}

// ListAnalysisHistory 获取AI分析历史（按模型分页）
func (h *AIAnalysisHandler) ListAnalysisHistory(c *gin.Context) {
	modelIDStr := c.Query("model_id")
	if modelIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "缺少 model_id"})
		return
	}
	modelID64, err := strconv.ParseUint(modelIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 model_id"})
		return
	}
	modelID := uint(modelID64)

	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		if v, e := strconv.Atoi(p); e == nil && v > 0 {
			page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, e := strconv.Atoi(ps); e == nil && v > 0 {
			pageSize = v
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := database.DB.Model(&models.AIAnalysisHistory{}).Where("ai_model_id = ?", modelID)
	var total int64
	query.Count(&total)

	var list []models.AIAnalysisHistory
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询失败: " + err.Error()})
		return
	}

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

// DeleteAnalysisHistory 软删除AI分析历史
func (h *AIAnalysisHandler) DeleteAnalysisHistory(c *gin.Context) {
	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var his models.AIAnalysisHistory
	if err := database.DB.First(&his, uint(id64)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "记录不存在"})
		return
	}

	if err := database.DB.Delete(&his).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}
