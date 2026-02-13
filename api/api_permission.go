package api

import (
	"net/http"
	"strconv"

	"finance/database"
	"finance/models"

	"github.com/gin-gonic/gin"
)

// APIPermissionHandler 接口权限管理
type APIPermissionHandler struct{}

func NewAPIPermissionHandler() *APIPermissionHandler {
	return &APIPermissionHandler{}
}

// List 接口列表
func (h *APIPermissionHandler) List(c *gin.Context) {
	var list []models.APIPermission
	if err := database.DB.Order("method ASC, path ASC").Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "查询失败")})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

type APIPermissionCreateRequest struct {
	Method string `json:"method" binding:"required,oneof=GET POST PUT DELETE PATCH"`
	Path   string `json:"path" binding:"required,min=1,max=255"`
	Desc   string `json:"desc" binding:"omitempty,max=100"`
}

type APIPermissionUpdateRequest struct {
	Method *string `json:"method" binding:"omitempty,oneof=GET POST PUT DELETE PATCH"`
	Path   *string `json:"path" binding:"omitempty,min=1,max=255"`
	Desc   *string `json:"desc" binding:"omitempty,max=100"`
}

// Create 创建接口
func (h *APIPermissionHandler) Create(c *gin.Context) {
	var req APIPermissionCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}
	var exist models.APIPermission
	if err := database.DB.Where("method = ? AND path = ?", req.Method, req.Path).First(&exist).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "方法+路径已存在"})
		return
	}
	api := models.APIPermission{
		Method: req.Method,
		Path:   req.Path,
		Desc:   req.Desc,
	}
	if err := database.DB.Create(&api).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "创建失败")})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "创建成功", "data": api})
}

// Update 更新接口
func (h *APIPermissionHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var req APIPermissionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}
	var api models.APIPermission
	if err := database.DB.First(&api, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "接口不存在"})
		return
	}
	method := api.Method
	path := api.Path
	if req.Method != nil {
		method = *req.Method
	}
	if req.Path != nil {
		path = *req.Path
	}
	if method != api.Method || path != api.Path {
		var exist models.APIPermission
		if err := database.DB.Where("method = ? AND path = ? AND id != ?", method, path, id).First(&exist).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "方法+路径已存在"})
			return
		}
	}
	updates := make(map[string]interface{})
	if req.Method != nil {
		updates["method"] = *req.Method
	}
	if req.Path != nil {
		updates["path"] = *req.Path
	}
	if req.Desc != nil {
		updates["desc"] = *req.Desc
	}
	if len(updates) > 0 {
		if err := database.DB.Model(&api).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "更新失败")})
			return
		}
	}
	database.DB.First(&api, api.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "更新成功", "data": api})
}

// Delete 删除接口
func (h *APIPermissionHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var api models.APIPermission
	if err := database.DB.First(&api, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "接口不存在"})
		return
	}
	if err := database.DB.Delete(&api).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "删除失败")})
		return
	}
	_ = database.DB.Where("api_id = ?", id).Delete(&models.MenuAPI{})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}
