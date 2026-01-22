package api

import (
	"net/http"
	"strconv"
	"strings"

	"finance/database"
	"finance/models"

	"github.com/gin-gonic/gin"
)

// CategoryHandler 消费类别管理
type CategoryHandler struct{}

func NewCategoryHandler() *CategoryHandler {
	return &CategoryHandler{}
}

type CategoryCreateRequest struct {
	Name  string `json:"name" binding:"required,min=1,max=50"`
	Sort  int    `json:"sort"`
	Color string `json:"color" binding:"omitempty,max=20"` // 颜色代码，如 #ef4444
}

type CategoryUpdateRequest struct {
	Name  string  `json:"name" binding:"omitempty,min=1,max=50"`
	Sort  *int    `json:"sort"`
	Color *string `json:"color" binding:"omitempty,max=20"`
}

// List 列出所有类别（不包含软删除）
// @Summary 获取消费类别列表
// @Description 获取所有消费类别列表，支持按名称模糊搜索
// @Tags 后台管理-消费类别
// @Produce json
// @Param name query string false "类别名称（模糊匹配）"
// @Success 200 {object} map[string]interface{} "获取成功，返回类别列表"
// @Router /admin/categories [get]
func (h *CategoryHandler) List(c *gin.Context) {
	var list []models.ExpenseCategory
	if err := database.DB.Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

// Create 创建类别
// @Summary 创建消费类别
// @Description 创建新的消费类别，支持设置名称、排序和颜色
// @Tags 后台管理-消费类别
// @Accept json
// @Produce json
// @Param request body CategoryCreateRequest true "类别信息"
// @Success 200 {object} map[string]interface{} "创建成功"
// @Failure 400 {object} map[string]interface{} "参数错误或类别名称已存在"
// @Router /admin/categories [post]
func (h *CategoryHandler) Create(c *gin.Context) {
	var req CategoryCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "名称不能为空"})
		return
	}

	// 唯一性
	var existing models.ExpenseCategory
	if err := database.DB.Where("name = ?", req.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "类别名称已存在"})
		return
	}

	color := req.Color
	if color == "" {
		color = "#64748b" // 默认灰色
	}
	cat := models.ExpenseCategory{Name: req.Name, Sort: req.Sort, Color: color}
	if err := database.DB.Create(&cat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "创建成功", "data": cat})
}

// Update 更新类别
// @Summary 更新消费类别
// @Description 更新指定的消费类别信息
// @Tags 后台管理-消费类别
// @Accept json
// @Produce json
// @Param id path int true "类别ID"
// @Param request body CategoryUpdateRequest true "更新的类别信息"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]interface{} "参数错误或类别名称已存在"
// @Failure 404 {object} map[string]interface{} "类别不存在"
// @Router /admin/categories/{id} [put]
func (h *CategoryHandler) Update(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var cat models.ExpenseCategory
	if err := database.DB.First(&cat, uint(id64)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "类别不存在"})
		return
	}

	var req CategoryUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "名称不能为空"})
			return
		}
		var existing models.ExpenseCategory
		if err := database.DB.Where("name = ? AND id != ?", req.Name, cat.ID).First(&existing).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "类别名称已存在"})
			return
		}
		updates["name"] = req.Name
	}
	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}
	if req.Color != nil {
		color := *req.Color
		if color == "" {
			color = "#64748b" // 默认灰色
		}
		updates["color"] = color
	}
	if len(updates) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "无需更新"})
		return
	}

	if err := database.DB.Model(&cat).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新失败: " + err.Error()})
		return
	}
	database.DB.First(&cat, cat.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "更新成功", "data": cat})
}

// Delete 软删除类别
// @Summary 删除消费类别
// @Description 软删除指定的消费类别
// @Tags 后台管理-消费类别
// @Produce json
// @Param id path int true "类别ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 400 {object} map[string]interface{} "无效的ID"
// @Failure 404 {object} map[string]interface{} "类别不存在"
// @Router /admin/categories/{id} [delete]
func (h *CategoryHandler) Delete(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var cat models.ExpenseCategory
	if err := database.DB.First(&cat, uint(id64)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "类别不存在"})
		return
	}
	if err := database.DB.Delete(&cat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}
