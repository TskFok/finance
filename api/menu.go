package api

import (
	"net/http"
	"strconv"

	"finance/database"
	"finance/models"

	"github.com/gin-gonic/gin"
)

// MenuHandler 菜单管理
type MenuHandler struct{}

func NewMenuHandler() *MenuHandler {
	return &MenuHandler{}
}

// MenuTreeItem 菜单树节点
type MenuTreeItem struct {
	ID        uint          `json:"id"`
	ParentID  uint          `json:"parent_id"`
	Name      string        `json:"name"`
	Path      string        `json:"path"`
	Icon      string        `json:"icon"`
	SortOrder int           `json:"sort_order"`
	Children  []MenuTreeItem `json:"children,omitempty"`
	APIs      []APISimple   `json:"apis,omitempty"`
}

// APISimple 接口简要信息
type APISimple struct {
	ID     uint   `json:"id"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Desc   string `json:"desc"`
}

// List 菜单树
func (h *MenuHandler) List(c *gin.Context) {
	var menus []models.Menu
	if err := database.DB.Order("sort_order ASC, id ASC").Find(&menus).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "查询失败")})
		return
	}
	tree := buildMenuTree(menus, 0)
	// 加载每个菜单绑定的接口
	for i := range tree {
		loadMenuAPIs(&tree[i])
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tree})
}

func buildMenuTree(menus []models.Menu, parentID uint) []MenuTreeItem {
	var result []MenuTreeItem
	for _, m := range menus {
		if m.ParentID != parentID {
			continue
		}
		item := MenuTreeItem{
			ID:        m.ID,
			ParentID:  m.ParentID,
			Name:      m.Name,
			Path:      m.Path,
			Icon:      m.Icon,
			SortOrder: m.SortOrder,
		}
		item.Children = buildMenuTree(menus, m.ID)
		result = append(result, item)
	}
	return result
}

// collectMenuDescendantIDs 收集 rootID 的所有子孙节点 ID
func collectMenuDescendantIDs(menus []models.Menu, rootID uint) map[uint]bool {
	byParent := make(map[uint][]models.Menu)
	for _, m := range menus {
		byParent[m.ParentID] = append(byParent[m.ParentID], m)
	}
	set := make(map[uint]bool)
	var dfs func(id uint)
	dfs = func(id uint) {
		for _, c := range byParent[id] {
			set[c.ID] = true
			dfs(c.ID)
		}
	}
	dfs(rootID)
	return set
}

func loadMenuAPIs(item *MenuTreeItem) {
	var apiIDs []uint
	database.DB.Model(&models.MenuAPI{}).Where("menu_id = ?", item.ID).Pluck("api_id", &apiIDs)
	if len(apiIDs) > 0 {
		var apis []models.APIPermission
		database.DB.Where("id IN ?", apiIDs).Find(&apis)
		item.APIs = make([]APISimple, 0, len(apis))
		for _, a := range apis {
			item.APIs = append(item.APIs, APISimple{ID: a.ID, Method: a.Method, Path: a.Path, Desc: a.Desc})
		}
	}
	for i := range item.Children {
		loadMenuAPIs(&item.Children[i])
	}
}

type MenuCreateRequest struct {
	ParentID  uint   `json:"parent_id"`
	Name      string `json:"name" binding:"required,min=1,max=50"`
	Path      string `json:"path" binding:"required,min=1,max=100"`
	Icon      string `json:"icon" binding:"omitempty,max=50"`
	SortOrder int    `json:"sort_order"`
}

type MenuUpdateRequest struct {
	ParentID  *uint   `json:"parent_id"`
	Name      *string `json:"name" binding:"omitempty,min=1,max=50"`
	Path      *string `json:"path" binding:"omitempty,min=1,max=100"`
	Icon      *string `json:"icon" binding:"omitempty,max=50"`
	SortOrder *int    `json:"sort_order"`
}

// Create 创建菜单
func (h *MenuHandler) Create(c *gin.Context) {
	var req MenuCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}
	if req.ParentID > 0 {
		var parent models.Menu
		if err := database.DB.First(&parent, req.ParentID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "父级菜单不存在"})
			return
		}
	}
	menu := models.Menu{
		ParentID:  req.ParentID,
		Name:      req.Name,
		Path:      req.Path,
		Icon:      req.Icon,
		SortOrder: req.SortOrder,
	}
	if err := database.DB.Create(&menu).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "创建失败")})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "创建成功", "data": menu})
}

// Update 更新菜单
func (h *MenuHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var req MenuUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}
	var menu models.Menu
	if err := database.DB.First(&menu, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "菜单不存在"})
		return
	}
	if req.ParentID != nil {
		pid := *req.ParentID
		if pid > 0 {
			if pid == uint(id) {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "不能将父级设为自己"})
				return
			}
			var parent models.Menu
			if err := database.DB.First(&parent, pid).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "父级菜单不存在"})
				return
			}
			// 防止循环：parent_id 不能是当前菜单的任意子孙
			var allMenus []models.Menu
			database.DB.Find(&allMenus)
			descendants := collectMenuDescendantIDs(allMenus, uint(id))
			if descendants[pid] {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "不能将父级设为自身的子菜单"})
				return
			}
		}
	}

	updates := make(map[string]interface{})
	if req.ParentID != nil {
		updates["parent_id"] = *req.ParentID
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Path != nil {
		updates["path"] = *req.Path
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if len(updates) > 0 {
		if err := database.DB.Model(&menu).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "更新失败")})
			return
		}
	}
	database.DB.First(&menu, menu.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "更新成功", "data": menu})
}

// Delete 删除菜单
func (h *MenuHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var menu models.Menu
	if err := database.DB.First(&menu, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "菜单不存在"})
		return
	}
	if err := database.DB.Delete(&menu).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "删除失败")})
		return
	}
	_ = database.DB.Where("menu_id = ?", id).Delete(&models.MenuAPI{})
	_ = database.DB.Where("menu_id = ?", id).Delete(&models.RoleMenu{})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "删除成功"})
}

type MenuAPIsRequest struct {
	APIIDs []uint `json:"api_ids"`
}

// AssignAPIs 为菜单绑定接口
func (h *MenuHandler) AssignAPIs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的ID"})
		return
	}
	var req MenuAPIsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": SafeErrorMessage(err, "参数错误")})
		return
	}
	var menu models.Menu
	if err := database.DB.First(&menu, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "菜单不存在"})
		return
	}
	if err := database.DB.Where("menu_id = ?", id).Delete(&models.MenuAPI{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": SafeErrorMessage(err, "更新失败")})
		return
	}
	for _, apiID := range req.APIIDs {
		_ = database.DB.Create(&models.MenuAPI{MenuID: uint(id), APIID: apiID}).Error
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "绑定成功"})
}
