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

// ===== App/前端 API（JWT）专用：按用户隔离 AI分析/聊天/历史 =====

// AnalyzeExpensesApp AI分析（App端，流式）
// @Summary AI分析（流式）
// @Description 选择时间范围与AI模型，对当前用户在该时间范围内的消费记录进行AI分析，SSE流式返回 JSON 帧（delta/done/error）。分析结束后会保存到历史记录。
// @Tags AI
// @Accept json
// @Produce text/event-stream
// @Security BearerAuth
// @Param request body AnalysisRequest true "分析请求"
// @Success 200 {string} string "SSE流：data: {\"type\":\"delta\",\"content\":\"...\"}"
// @Failure 400 {object} Response "参数错误"
// @Failure 401 {object} Response "未授权"
// @Failure 403 {object} Response "账号锁定或无权限"
// @Router /api/v1/ai-analysis [post]
func (h *AIAnalysisHandler) AnalyzeExpensesApp(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	h.analyzeExpensesScoped(c, userID)
}

// ListAnalysisHistoryApp 获取AI分析历史（App端，按模型分页）
// @Summary 获取AI分析历史
// @Description 获取当前用户的AI分析历史记录，按 model_id 分页返回（软删除不返回）。
// @Tags AI
// @Produce json
// @Security BearerAuth
// @Param model_id query int true "AI模型ID"
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页条数，默认20，最大100"
// @Success 200 {object} Response "获取成功"
// @Failure 400 {object} Response "参数错误"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/ai-analysis/history [get]
func (h *AIAnalysisHandler) ListAnalysisHistoryApp(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	h.listAnalysisHistoryScoped(c, userID, true)
}

// DeleteAnalysisHistoryApp 删除AI分析历史（App端，仅可删自己的）
// @Summary 删除AI分析历史
// @Tags AI
// @Produce json
// @Security BearerAuth
// @Param id path int true "历史记录ID"
// @Success 200 {object} Response "删除成功"
// @Failure 401 {object} Response "未授权"
// @Failure 403 {object} Response "无权限"
// @Failure 404 {object} Response "记录不存在"
// @Router /api/v1/ai-analysis/history/{id} [delete]
func (h *AIAnalysisHandler) DeleteAnalysisHistoryApp(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "无效的ID")
		return
	}
	var his models.AIAnalysisHistory
	if err := database.DB.First(&his, uint(id64)).Error; err != nil {
		NotFound(c, "记录不存在")
		return
	}
	if his.UserID != 0 && his.UserID != userID {
		Error(c, http.StatusForbidden, "无权限")
		return
	}
	if err := database.DB.Delete(&his).Error; err != nil {
		InternalError(c, "删除失败: "+err.Error())
		return
	}
	SuccessWithMessage(c, "删除成功", nil)
}

// ChatStreamApp AI聊天（App端，流式）
// @Summary AI聊天（流式）
// @Description 选择AI模型，与AI进行对话，SSE流式返回 JSON 帧（delta/done/error）。结束后保存聊天记录。
// @Tags AI
// @Accept json
// @Produce text/event-stream
// @Security BearerAuth
// @Param request body AIChatRequest true "聊天请求"
// @Success 200 {string} string "SSE流：data: {\"type\":\"delta\",\"content\":\"...\"}"
// @Failure 400 {object} Response "参数错误"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/ai-chat [post]
func (h *AIChatHandler) ChatStreamApp(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	h.chatStreamScoped(c, userID)
}

// ChatHistoryApp 获取聊天历史（App端，按模型分页）
// @Summary 获取AI聊天历史
// @Description 获取当前用户的AI聊天历史记录，按 model_id 分页返回（软删除不返回）。
// @Tags AI
// @Produce json
// @Security BearerAuth
// @Param model_id query int true "AI模型ID"
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页条数，默认20，最大100"
// @Success 200 {object} Response "获取成功"
// @Failure 400 {object} Response "参数错误"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/ai-chat/history [get]
func (h *AIChatHandler) ChatHistoryApp(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	h.chatHistoryScoped(c, userID, true)
}

// DeleteChatHistoryApp 删除聊天记录（App端，仅可删自己的）
// @Summary 删除AI聊天记录
// @Tags AI
// @Produce json
// @Security BearerAuth
// @Param id path int true "聊天记录ID"
// @Success 200 {object} Response "删除成功"
// @Failure 401 {object} Response "未授权"
// @Failure 403 {object} Response "无权限"
// @Failure 404 {object} Response "记录不存在"
// @Router /api/v1/ai-chat/history/{id} [delete]
func (h *AIChatHandler) DeleteChatHistoryApp(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "无效的ID")
		return
	}
	var msg models.AIChatMessage
	if err := database.DB.First(&msg, uint(id64)).Error; err != nil {
		NotFound(c, "记录不存在")
		return
	}
	if msg.UserID != 0 && msg.UserID != userID {
		Error(c, http.StatusForbidden, "无权限")
		return
	}
	if err := database.DB.Delete(&msg).Error; err != nil {
		InternalError(c, "删除失败: "+err.Error())
		return
	}
	SuccessWithMessage(c, "删除成功", nil)
}

// ListAIModelsApp 获取可用AI模型列表（App端）
// @Summary 获取AI模型列表
// @Description 获取系统可用的AI模型配置列表（不包含APIKey），用于前端选择模型。
// @Tags AI
// @Produce json
// @Security BearerAuth
// @Success 200 {object} Response{data=[]models.AIModel} "获取成功"
// @Failure 401 {object} Response "未授权"
// @Router /api/v1/ai-models [get]
func (h *AIModelHandler) ListAIModelsApp(c *gin.Context) {
	var list []models.AIModel
	if err := database.DB.Order("sort_order ASC, id ASC").Find(&list).Error; err != nil {
		InternalError(c, "查询失败: "+err.Error())
		return
	}
	Success(c, list)
}

// ===== 供 handler 复用的 scoped 实现（在原文件里实现） =====

// analyzeExpensesScoped 在 ai_analysis.go 里实现
// listAnalysisHistoryScoped 在 ai_analysis.go 里实现
// chatStreamScoped/chatHistoryScoped 在 ai_chat.go 里实现

// parseDateRange helper（App端同用）
func parseDateRange(startStr, endStr string) (time.Time, time.Time, error) {
	startTime, err := time.ParseInLocation("2006-01-02", startStr, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endTime, err := time.ParseInLocation("2006-01-02", endStr, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endTime = endTime.Add(24*time.Hour - time.Second)
	return startTime, endTime, nil
}
