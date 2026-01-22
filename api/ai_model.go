package api

import (
	"net/http"
	"strconv"

	"finance/database"
	"finance/models"

	"github.com/gin-gonic/gin"
)

// AIModelHandler AI模型管理处理器
type AIModelHandler struct{}

// NewAIModelHandler 创建AI模型管理处理器
func NewAIModelHandler() *AIModelHandler {
	return &AIModelHandler{}
}

// CreateAIModelRequest 创建AI模型请求
type CreateAIModelRequest struct {
	Name    string `json:"name" binding:"required,min=1,max=100" example:"OpenAI GPT-4"`
	BaseURL string `json:"base_url" binding:"required,url" example:"https://api.openai.com/v1"`
	APIKey  string `json:"api_key" binding:"required,min=1" example:"sk-..."`
}

// UpdateAIModelRequest 更新AI模型请求
type UpdateAIModelRequest struct {
	Name    string `json:"name" binding:"omitempty,min=1,max=100"`
	BaseURL string `json:"base_url" binding:"omitempty,url"`
	APIKey  string `json:"api_key" binding:"omitempty,min=1"`
}

// CreateAIModel 创建AI模型配置
// @Summary 创建AI模型
// @Description 创建新的AI模型配置，包括名称、API地址和密钥
// @Tags 后台管理-AI模型
// @Accept json
// @Produce json
// @Param request body CreateAIModelRequest true "AI模型信息"
// @Success 200 {object} map[string]interface{} "创建成功"
// @Failure 400 {object} map[string]interface{} "参数错误或模型名称已存在"
// @Router /admin/ai-models [post]
func (h *AIModelHandler) CreateAIModel(c *gin.Context) {
	var req CreateAIModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 检查名称是否已存在
	var existing models.AIModel
	if err := database.DB.Where("name = ?", req.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "模型名称已存在"})
		return
	}

	aiModel := models.AIModel{
		Name:    req.Name,
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
	}

	if err := database.DB.Create(&aiModel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "创建成功",
		"data":    aiModel,
	})
}

// GetAllAIModels 获取所有AI模型列表
// @Summary 获取AI模型列表
// @Description 获取系统中所有AI模型配置列表（不包含APIKey）
// @Tags 后台管理-AI模型
// @Produce json
// @Success 200 {object} map[string]interface{} "获取成功，返回模型列表"
// @Router /admin/ai-models [get]
func (h *AIModelHandler) GetAllAIModels(c *gin.Context) {
	var models []models.AIModel
	if err := database.DB.Find(&models).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

// GetAIModel 获取单个AI模型
// @Summary 获取单个AI模型
// @Description 根据ID获取AI模型配置详情（不包含APIKey）
// @Tags 后台管理-AI模型
// @Produce json
// @Param id path int true "AI模型ID"
// @Success 200 {object} map[string]interface{} "获取成功"
// @Failure 400 {object} map[string]interface{} "无效的ID"
// @Failure 404 {object} map[string]interface{} "模型不存在"
// @Router /admin/ai-models/{id} [get]
func (h *AIModelHandler) GetAIModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var aiModel models.AIModel
	if err := database.DB.First(&aiModel, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "模型不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    aiModel,
	})
}

// UpdateAIModel 更新AI模型配置
// @Summary 更新AI模型
// @Description 更新指定的AI模型配置信息
// @Tags 后台管理-AI模型
// @Accept json
// @Produce json
// @Param id path int true "AI模型ID"
// @Param request body UpdateAIModelRequest true "更新的模型信息"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]interface{} "参数错误或模型名称已存在"
// @Failure 404 {object} map[string]interface{} "模型不存在"
// @Router /admin/ai-models/{id} [put]
func (h *AIModelHandler) UpdateAIModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var aiModel models.AIModel
	if err := database.DB.First(&aiModel, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "模型不存在"})
		return
	}

	var req UpdateAIModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 如果更新名称，检查是否与其他模型冲突
	if req.Name != "" && req.Name != aiModel.Name {
		var existing models.AIModel
		if err := database.DB.Where("name = ? AND id != ?", req.Name, id).First(&existing).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "模型名称已存在"})
			return
		}
	}

	// 更新字段
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.BaseURL != "" {
		updates["base_url"] = req.BaseURL
	}
	if req.APIKey != "" {
		updates["api_key"] = req.APIKey
	}

	if err := database.DB.Model(&aiModel).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新失败: " + err.Error()})
		return
	}

	// 重新获取更新后的记录
	database.DB.First(&aiModel, aiModel.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "更新成功",
		"data":    aiModel,
	})
}

// DeleteAIModel 删除AI模型配置
// @Summary 删除AI模型
// @Description 删除指定的AI模型配置（软删除）
// @Tags 后台管理-AI模型
// @Produce json
// @Param id path int true "AI模型ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 400 {object} map[string]interface{} "无效的ID"
// @Failure 404 {object} map[string]interface{} "模型不存在"
// @Router /admin/ai-models/{id} [delete]
func (h *AIModelHandler) DeleteAIModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var aiModel models.AIModel
	if err := database.DB.First(&aiModel, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "模型不存在"})
		return
	}

	if err := database.DB.Delete(&aiModel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}
