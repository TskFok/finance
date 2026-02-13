package api

import (
	"net/http"
	"strconv"

	"finance/database"
	"finance/models"

	"github.com/gin-gonic/gin"
)

// RoleHandler 角色管理
type RoleHandler struct{}

func NewRoleHandler() *RoleHandler {
	return &RoleHandler{}
}

// List 角色列表
func (h *RoleHandler) List(c *gin.Context) {
	var list []models.Role
	if err := database.DB.Order("id ASC").Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "查询失败")})
		return
	}
	type RoleWithMenus struct {
		models.Role
		MenuIDs []uint `json:"menu_ids"`
	}
	result := make([]RoleWithMenus, 0, len(list))
	for _, r := range list {
		var menuIDs []uint
		database.DB.Model(&models.RoleMenu{}).Where("role_id = ?", r.ID).Pluck("menu_id", &menuIDs)
		result = append(result, RoleWithMenus{Role: r, MenuIDs: menuIDs})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// Get 角色详情（含菜单ID列表）
func (h *RoleHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var role models.Role
	if err := database.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "角色不存在"})
		return
	}
	var menuIDs []uint
	database.DB.Model(&models.RoleMenu{}).Where("role_id = ?", role.ID).Pluck("menu_id", &menuIDs)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"id": role.ID, "name": role.Name, "code": role.Code, "description": role.Description, "menu_ids": menuIDs}})
}

type RoleCreateRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=50"`
	Code        string `json:"code" binding:"required,min=1,max=50"`
	Description string `json:"description" binding:"omitempty,max=255"`
}

type RoleUpdateRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=50"`
	Code        *string `json:"code" binding:"omitempty,min=1,max=50"`
	Description *string `json:"description" binding:"omitempty,max=255"`
}

// Create 创建角色
func (h *RoleHandler) Create(c *gin.Context) {
	var req RoleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}
	var exist models.Role
	if err := database.DB.Where("code = ?", req.Code).First(&exist).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "编码已存在"})
		return
	}
	role := models.Role{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
	}
	if err := database.DB.Create(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "创建失败")})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "创建成功", "data": role})
}

// Update 更新角色
func (h *RoleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var req RoleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}
	var role models.Role
	if err := database.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "角色不存在"})
		return
	}
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Code != nil {
		var exist models.Role
		if err := database.DB.Where("code = ? AND id != ?", *req.Code, id).First(&exist).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "编码已存在"})
			return
		}
		updates["code"] = *req.Code
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if len(updates) > 0 {
		if err := database.DB.Model(&role).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "更新失败")})
			return
		}
	}
	database.DB.First(&role, role.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "更新成功", "data": role})
}

// Delete 删除角色
func (h *RoleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var role models.Role
	if err := database.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "角色不存在"})
		return
	}
	if err := database.DB.Delete(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "删除失败")})
		return
	}
	_ = database.DB.Where("role_id = ?", id).Delete(&models.RoleMenu{})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}

type RoleMenusRequest struct {
	MenuIDs []uint `json:"menu_ids"`
}

// AssignMenus 为角色分配菜单
func (h *RoleHandler) AssignMenus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var req RoleMenusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}
	var role models.Role
	if err := database.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "角色不存在"})
		return
	}
	if err := database.DB.Where("role_id = ?", id).Delete(&models.RoleMenu{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "更新失败")})
		return
	}
	for _, menuID := range req.MenuIDs {
		_ = database.DB.Create(&models.RoleMenu{RoleID: uint(id), MenuID: menuID}).Error
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "分配成功"})
}
