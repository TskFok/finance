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

type sseChatFrame struct {
	Type    string `json:"type"`              // delta | done | error
	Content string `json:"content,omitempty"` // delta内容或错误信息
}

func writeSSEJSON(c *gin.Context, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	_, _ = c.Writer.WriteString("data: " + string(b) + "\n\n")
	c.Writer.Flush()
}

// AIChatHandler AI聊天处理器
type AIChatHandler struct{}

func NewAIChatHandler() *AIChatHandler {
	return &AIChatHandler{}
}

// AIChatRequest AI聊天请求
type AIChatRequest struct {
	ModelID uint   `json:"model_id" binding:"required"`
	Message string `json:"message" binding:"required,min=1"`
}

// ChatStream AI聊天（SSE流式返回），结束后写入聊天记录
// @Summary AI聊天（流式）
// @Description 选择AI模型，与AI进行对话，SSE流式返回JSON帧（delta/done/error）。结束后保存聊天记录。
// @Tags 后台管理-AI聊天
// @Accept json
// @Produce text/event-stream
// @Param request body AIChatRequest true "聊天请求"
// @Success 200 {string} string "SSE流：data: {\"type\":\"delta\",\"content\":\"...\"}"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 404 {object} map[string]interface{} "AI模型不存在"
// @Router /admin/ai-chat [post]
func (h *AIChatHandler) ChatStream(c *gin.Context) {
	var req AIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 读取模型配置（包含密钥）
	var aiModel models.AIModel
	if err := database.DB.First(&aiModel, req.ModelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "AI模型不存在"})
		return
	}

	// SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 构建请求（OpenAI兼容 chat/completions）
	requestBody := map[string]interface{}{
		"model": aiModel.Name,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个专业、友好、简洁的个人财务助手。请用中文回答。"},
			{"role": "user", "content": req.Message},
		},
		"stream":      true,
		"temperature": 0.3,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		writeSSEJSON(c, sseChatFrame{Type: "error", Content: "构建请求失败"})
		writeSSEJSON(c, sseChatFrame{Type: "done"})
		return
	}

	httpReq, err := http.NewRequest("POST", strings.TrimRight(aiModel.BaseURL, "/")+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		writeSSEJSON(c, sseChatFrame{Type: "error", Content: "创建请求失败"})
		writeSSEJSON(c, sseChatFrame{Type: "done"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+aiModel.APIKey)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		writeSSEJSON(c, sseChatFrame{Type: "error", Content: "请求AI服务失败: " + err.Error()})
		writeSSEJSON(c, sseChatFrame{Type: "done"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writeSSEJSON(c, sseChatFrame{Type: "error", Content: fmt.Sprintf("AI服务返回错误: %d %s", resp.StatusCode, string(body))})
		writeSSEJSON(c, sseChatFrame{Type: "done"})
		return
	}

	ctx := c.Request.Context()
	reader := bufio.NewReader(resp.Body)
	var aiText strings.Builder

	finishedNormally := false
	for {
		select {
		case <-ctx.Done():
			// 客户端断开：不落库（避免保存半截内容）
			return
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				// 有些兼容接口不会发送 [DONE]，EOF 视为结束
				finishedNormally = true
				break
			}
			// 读取异常：不落库
			return
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// OpenAI SSE: data: {...}
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := bytes.TrimPrefix(line, []byte("data: "))
		if string(data) == "[DONE]" {
			finishedNormally = true
			// 结束：写入数据库
			msg := models.AIChatMessage{
				AIModelID: req.ModelID,
				UserID: func() uint {
					if u, e := getCurrentUser(c); e == nil {
						return u.ID
					}
					return 0
				}(),
				UserText: req.Message,
				AIText:   aiText.String(),
			}
			_ = database.DB.Create(&msg).Error
			writeSSEJSON(c, sseChatFrame{Type: "done"})
			break
		}

		var streamData map[string]interface{}
		if err := json.Unmarshal(data, &streamData); err != nil {
			continue
		}

		// choices[0].delta.content
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
			continue
		}

		aiText.WriteString(content)
		writeSSEJSON(c, sseChatFrame{Type: "delta", Content: content})
	}

	// 如果是 EOF 正常结束但没收到 [DONE]，这里补一次 done + 落库
	if finishedNormally {
		msg := models.AIChatMessage{
			AIModelID: req.ModelID,
			UserID: func() uint {
				if u, e := getCurrentUser(c); e == nil {
					return u.ID
				}
				return 0
			}(),
			UserText: req.Message,
			AIText:   aiText.String(),
		}
		_ = database.DB.Create(&msg).Error
		writeSSEJSON(c, sseChatFrame{Type: "done"})
	}
}

// chatStreamScoped App端：仅写入当前 user_id（聊天内容本身不依赖账单数据）
func (h *AIChatHandler) chatStreamScoped(c *gin.Context, userID uint) {
	var req AIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 读取模型配置（包含密钥）
	var aiModel models.AIModel
	if err := database.DB.First(&aiModel, req.ModelID).Error; err != nil {
		NotFound(c, "AI模型不存在")
		return
	}

	// SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	requestBody := map[string]interface{}{
		"model": aiModel.Name,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个专业、友好、简洁的个人财务助手。请用中文回答。"},
			{"role": "user", "content": req.Message},
		},
		"stream":      true,
		"temperature": 0.3,
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		writeSSEJSON(c, sseChatFrame{Type: "error", Content: "构建请求失败"})
		writeSSEJSON(c, sseChatFrame{Type: "done"})
		return
	}

	httpReq, err := http.NewRequest("POST", strings.TrimRight(aiModel.BaseURL, "/")+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		writeSSEJSON(c, sseChatFrame{Type: "error", Content: "创建请求失败"})
		writeSSEJSON(c, sseChatFrame{Type: "done"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+aiModel.APIKey)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		writeSSEJSON(c, sseChatFrame{Type: "error", Content: "请求AI服务失败: " + err.Error()})
		writeSSEJSON(c, sseChatFrame{Type: "done"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writeSSEJSON(c, sseChatFrame{Type: "error", Content: fmt.Sprintf("AI服务返回错误: %d %s", resp.StatusCode, string(body))})
		writeSSEJSON(c, sseChatFrame{Type: "done"})
		return
	}

	ctx := c.Request.Context()
	reader := bufio.NewReader(resp.Body)
	var aiText strings.Builder
	finishedNormally := false

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				finishedNormally = true
				break
			}
			return
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		data := bytes.TrimPrefix(line, []byte("data: "))
		if string(data) == "[DONE]" {
			finishedNormally = true
			msg := models.AIChatMessage{
				AIModelID: req.ModelID,
				UserID:    userID,
				UserText:  req.Message,
				AIText:    aiText.String(),
			}
			_ = database.DB.Create(&msg).Error
			writeSSEJSON(c, sseChatFrame{Type: "done"})
			break
		}

		var streamData map[string]interface{}
		if err := json.Unmarshal(data, &streamData); err != nil {
			continue
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
			continue
		}
		aiText.WriteString(content)
		writeSSEJSON(c, sseChatFrame{Type: "delta", Content: content})
	}

	if finishedNormally {
		msg := models.AIChatMessage{
			AIModelID: req.ModelID,
			UserID:    userID,
			UserText:  req.Message,
			AIText:    aiText.String(),
		}
		_ = database.DB.Create(&msg).Error
		writeSSEJSON(c, sseChatFrame{Type: "done"})
	}
}

// chatHistoryScoped App端：按用户+模型分页返回（Response 结构）
func (h *AIChatHandler) chatHistoryScoped(c *gin.Context, userID uint, requireUser bool) {
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

	query := database.DB.Model(&models.AIChatMessage{}).Where("ai_model_id = ?", modelID)
	if requireUser {
		query = query.Where("user_id = ?", userID)
	}
	var total int64
	query.Count(&total)

	var list []models.AIChatMessage
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

// ChatHistory 获取聊天历史（按模型分页）
// @Summary 获取AI聊天历史
// @Description 获取AI聊天历史记录，按model_id分页返回（软删除不返回）
// @Tags 后台管理-AI聊天
// @Produce json
// @Param model_id query int true "AI模型ID"
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页条数，默认20，最大100"
// @Success 200 {object} map[string]interface{} "获取成功，返回分页数据"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Router /admin/ai-chat/history [get]
func (h *AIChatHandler) ChatHistory(c *gin.Context) {
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
		_ = func() error {
			v, e := strconv.Atoi(p)
			if e != nil {
				return e
			}
			if v > 0 {
				page = v
			}
			return nil
		}()
	}
	if ps := c.Query("page_size"); ps != "" {
		_ = func() error {
			v, e := strconv.Atoi(ps)
			if e != nil {
				return e
			}
			if v > 0 {
				pageSize = v
			}
			return nil
		}()
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := database.DB.Model(&models.AIChatMessage{}).Where("ai_model_id = ?", modelID)
	var total int64
	query.Count(&total)

	var list []models.AIChatMessage
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

// DeleteChatHistory 软删除聊天记录
// @Summary 删除AI聊天记录
// @Description 软删除指定的AI聊天记录
// @Tags 后台管理-AI聊天
// @Produce json
// @Param id path int true "聊天记录ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 400 {object} map[string]interface{} "无效的ID"
// @Failure 404 {object} map[string]interface{} "记录不存在"
// @Router /admin/ai-chat/history/{id} [delete]
func (h *AIChatHandler) DeleteChatHistory(c *gin.Context) {
	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var msg models.AIChatMessage
	if err := database.DB.First(&msg, uint(id64)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "记录不存在"})
		return
	}

	if err := database.DB.Delete(&msg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}
