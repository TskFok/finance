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
func (h *CategoryHandler) List(c *gin.Context) {
	var list []models.ExpenseCategory
	if err := database.DB.Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

// Create 创建类别
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
