package database

import (
	"fmt"
	"log"
	"strings"

	"finance/config"
	"finance/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func splitMethodPath(s string) (method, path string) {
	idx := strings.Index(s, ":")
	if idx <= 0 || idx >= len(s)-1 {
		return "", ""
	}
	return s[:idx], s[idx+1:]
}

var DB *gorm.DB

// Init 初始化数据库连接
func Init(cfg *config.Config) error {
	// 构建 MySQL DSN 连接字符串
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DBName,
		cfg.Database.Charset,
	)

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true, // 禁止迁移时创建外键
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 获取底层 *sql.DB 连接池配置
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(10)  // 最大空闲连接数
	sqlDB.SetMaxOpenConns(100) // 最大打开连接数

	// 自动迁移数据库表
	if err := DB.AutoMigrate(
		&models.User{},
		&models.Expense{},
		&models.Income{},
		&models.ExpenseCategory{},
		&models.IncomeCategory{},
		&models.PasswordReset{},
		&models.EmailVerification{},
		&models.AIModel{},
		&models.AIChatMessage{},
		&models.AIAnalysisHistory{},
		&models.Role{},
		&models.Menu{},
		&models.APIPermission{},
		&models.RoleMenu{},
		&models.MenuAPI{},
	); err != nil {
		return err
	}

	// 兼容历史数据：老版本没有 status 字段，默认设置为 active，避免升级后无法登录
	_ = DB.Model(&models.User{}).
		Where("status IS NULL OR status = ''").
		Update("status", models.UserStatusActive).Error

	// 兼容历史数据：当所有 AIModel 的 sort_order 均为 0 且有多条时，按 id 赋 0,1,2,...
	var total, zeroCnt int64
	DB.Model(&models.AIModel{}).Count(&total)
	DB.Model(&models.AIModel{}).Where("sort_order = 0").Count(&zeroCnt)
	if total > 1 && zeroCnt == total {
		var aiModels []models.AIModel
		if err := DB.Order("id").Find(&aiModels).Error; err == nil {
			for i, m := range aiModels {
				_ = DB.Model(&m).Update("sort_order", i).Error
			}
		}
	}

	// 初始化默认消费类别（仅当表为空时）
	var catCount int64
	DB.Model(&models.ExpenseCategory{}).Count(&catCount)
	if catCount == 0 {
		defaultCats := models.GetCategories()
		// 默认类别对应的颜色（与前端 CSS 保持一致）
		colorMap := map[string]string{
			"餐饮": "#ef4444", // 红色
			"交通": "#3b82f6", // 蓝色
			"购物": "#a855f7", // 紫色
			"娱乐": "#ec4899", // 粉色
			"医疗": "#10b981", // 绿色
			"教育": "#f59e0b", // 橙色
			"住房": "#14b8a6", // 青色
			"其他": "#64748b", // 灰色
		}
		var cats []models.ExpenseCategory
		for i, name := range defaultCats {
			color := colorMap[name]
			if color == "" {
				color = "#64748b" // 默认灰色
			}
			cats = append(cats, models.ExpenseCategory{
				Name:  name,
				Sort:  (i + 1) * 10,
				Color: color,
			})
		}
		if len(cats) > 0 {
			_ = DB.Create(&cats).Error
		}
	}

	// 初始化默认收入类别（仅当表为空时）
	var incomeCatCount int64
	DB.Model(&models.IncomeCategory{}).Count(&incomeCatCount)
	if incomeCatCount == 0 {
		defaultIncomeCats := []struct {
			Name  string
			Sort  int
			Color string
		}{
			{"工资", 10, "#10b981"},
			{"奖金", 20, "#3b82f6"},
			{"理财", 30, "#a855f7"},
			{"兼职", 40, "#f59e0b"},
			{"其他", 50, "#64748b"},
		}
		var incomeCats []models.IncomeCategory
		for _, item := range defaultIncomeCats {
			incomeCats = append(incomeCats, models.IncomeCategory{
				Name:  item.Name,
				Sort:  item.Sort,
				Color: item.Color,
			})
		}
		if len(incomeCats) > 0 {
			_ = DB.Create(&incomeCats).Error
		}
	}

	// 初始化角色、菜单、接口权限及关联（仅当表为空时）
	initRoleMenuAPI()

	log.Println("数据库初始化成功")
	return nil
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	return DB
}

// initRoleMenuAPI 初始化默认角色、菜单、接口权限及关联
func initRoleMenuAPI() {
	var roleCount int64
	DB.Model(&models.Role{}).Count(&roleCount)
	if roleCount > 0 {
		return
	}

	// 默认角色
	roles := []models.Role{
		{Name: "超级管理员", Code: "admin", Description: "拥有所有权限"},
		{Name: "运营员", Code: "operator", Description: "可管理数据，不含用户和系统配置"},
		{Name: "查看者", Code: "viewer", Description: "仅可查看数据"},
	}
	if err := DB.Create(&roles).Error; err != nil {
		log.Printf("初始化角色失败: %v", err)
		return
	}

	// 默认菜单（与现有侧栏对应）
	menus := []models.Menu{
		{ParentID: 0, Name: "数据概览", Path: "dashboard", Icon: "fa-chart-pie", SortOrder: 10},
		{ParentID: 0, Name: "账单管理", Path: "expenses", Icon: "fa-receipt", SortOrder: 20},
		{ParentID: 0, Name: "账单统计", Path: "statistics", Icon: "fa-chart-column", SortOrder: 30},
		{ParentID: 0, Name: "用户管理", Path: "users", Icon: "fa-users", SortOrder: 40},
		{ParentID: 0, Name: "消费类别", Path: "categories", Icon: "fa-layer-group", SortOrder: 50},
		{ParentID: 0, Name: "收入类别", Path: "income-categories", Icon: "fa-tags", SortOrder: 60},
		{ParentID: 0, Name: "数据导出", Path: "export", Icon: "fa-file-export", SortOrder: 70},
		{ParentID: 0, Name: "收入", Path: "incomes", Icon: "fa-arrow-trend-up", SortOrder: 80},
		{ParentID: 0, Name: "AI模型", Path: "ai-models", Icon: "fa-cubes", SortOrder: 90},
		{ParentID: 0, Name: "AI分析", Path: "ai-analysis", Icon: "fa-brain", SortOrder: 100},
		{ParentID: 0, Name: "AI聊天", Path: "ai-chat", Icon: "fa-comments", SortOrder: 110},
		{ParentID: 0, Name: "角色管理", Path: "roles", Icon: "fa-user-shield", SortOrder: 115},
		{ParentID: 0, Name: "菜单管理", Path: "menus", Icon: "fa-list", SortOrder: 120},
		{ParentID: 0, Name: "接口管理", Path: "apis", Icon: "fa-plug", SortOrder: 130},
	}
	if err := DB.Create(&menus).Error; err != nil {
		log.Printf("初始化菜单失败: %v", err)
		return
	}

	// 默认接口权限（从 router 提取的 admin 路由）
	apis := []models.APIPermission{
		{Method: "GET", Path: "/admin/current-user", Desc: "当前用户信息"},
		{Method: "GET", Path: "/admin/feishu/bind-token", Desc: "飞书绑定Token"},
		{Method: "GET", Path: "/admin/expenses", Desc: "消费记录列表"},
		{Method: "POST", Path: "/admin/expenses", Desc: "创建消费记录"},
		{Method: "PUT", Path: "/admin/expenses/:id", Desc: "更新消费记录"},
		{Method: "DELETE", Path: "/admin/expenses/:id", Desc: "删除消费记录"},
		{Method: "GET", Path: "/admin/expenses/detailed-statistics", Desc: "消费详细统计"},
		{Method: "GET", Path: "/admin/statistics/summary", Desc: "收支汇总"},
		{Method: "GET", Path: "/admin/categories", Desc: "消费类别列表"},
		{Method: "POST", Path: "/admin/categories", Desc: "创建消费类别"},
		{Method: "PUT", Path: "/admin/categories/:id", Desc: "更新消费类别"},
		{Method: "DELETE", Path: "/admin/categories/:id", Desc: "删除消费类别"},
		{Method: "GET", Path: "/admin/income-categories", Desc: "收入类别列表"},
		{Method: "POST", Path: "/admin/income-categories", Desc: "创建收入类别"},
		{Method: "PUT", Path: "/admin/income-categories/:id", Desc: "更新收入类别"},
		{Method: "DELETE", Path: "/admin/income-categories/:id", Desc: "删除收入类别"},
		{Method: "GET", Path: "/admin/users", Desc: "用户列表"},
		{Method: "POST", Path: "/admin/users/email/send-code", Desc: "发送绑定邮箱验证码"},
		{Method: "PUT", Path: "/admin/users/:id/password", Desc: "更新用户密码"},
		{Method: "PUT", Path: "/admin/users/:id/email", Desc: "更新用户邮箱"},
		{Method: "DELETE", Path: "/admin/users/:id", Desc: "删除用户"},
		{Method: "PUT", Path: "/admin/users/:id/admin", Desc: "设置管理员"},
		{Method: "PUT", Path: "/admin/users/:id/status", Desc: "更新用户状态"},
		{Method: "PUT", Path: "/admin/users/:id/feishu", Desc: "更新飞书绑定"},
		{Method: "POST", Path: "/admin/users/impersonate", Desc: "模拟登录"},
		{Method: "POST", Path: "/admin/users/exit-impersonation", Desc: "退出模拟"},
		{Method: "GET", Path: "/admin/statistics", Desc: "统计数据"},
		{Method: "GET", Path: "/admin/incomes", Desc: "收入列表"},
		{Method: "POST", Path: "/admin/incomes", Desc: "创建收入"},
		{Method: "PUT", Path: "/admin/incomes/:id", Desc: "更新收入"},
		{Method: "DELETE", Path: "/admin/incomes/:id", Desc: "删除收入"},
		{Method: "GET", Path: "/admin/export/excel", Desc: "导出Excel"},
		{Method: "POST", Path: "/admin/password/admin-reset", Desc: "管理员重置密码"},
		{Method: "POST", Path: "/admin/password/send-reset-email", Desc: "发送重置邮件"},
		{Method: "GET", Path: "/admin/email-config", Desc: "邮件配置"},
		{Method: "GET", Path: "/admin/ai-models", Desc: "AI模型列表"},
		{Method: "PUT", Path: "/admin/ai-models/reorder", Desc: "AI模型排序"},
		{Method: "GET", Path: "/admin/ai-models/:id", Desc: "AI模型详情"},
		{Method: "POST", Path: "/admin/ai-models", Desc: "创建AI模型"},
		{Method: "POST", Path: "/admin/ai-models/:id/test", Desc: "测试AI模型"},
		{Method: "PUT", Path: "/admin/ai-models/:id", Desc: "更新AI模型"},
		{Method: "DELETE", Path: "/admin/ai-models/:id", Desc: "删除AI模型"},
		{Method: "POST", Path: "/admin/ai-analysis", Desc: "AI分析"},
		{Method: "GET", Path: "/admin/ai-analysis/history", Desc: "AI分析历史"},
		{Method: "DELETE", Path: "/admin/ai-analysis/history/:id", Desc: "删除AI分析历史"},
		{Method: "POST", Path: "/admin/ai-chat", Desc: "AI聊天"},
		{Method: "GET", Path: "/admin/ai-chat/history", Desc: "AI聊天历史"},
		{Method: "DELETE", Path: "/admin/ai-chat/history/:id", Desc: "删除AI聊天历史"},
		{Method: "GET", Path: "/admin/roles", Desc: "角色列表"},
		{Method: "GET", Path: "/admin/roles/:id", Desc: "角色详情"},
		{Method: "POST", Path: "/admin/roles", Desc: "创建角色"},
		{Method: "PUT", Path: "/admin/roles/:id", Desc: "更新角色"},
		{Method: "DELETE", Path: "/admin/roles/:id", Desc: "删除角色"},
		{Method: "PUT", Path: "/admin/roles/:id/menus", Desc: "分配角色菜单"},
		{Method: "GET", Path: "/admin/menus", Desc: "菜单列表"},
		{Method: "POST", Path: "/admin/menus", Desc: "创建菜单"},
		{Method: "PUT", Path: "/admin/menus/:id", Desc: "更新菜单"},
		{Method: "DELETE", Path: "/admin/menus/:id", Desc: "删除菜单"},
		{Method: "PUT", Path: "/admin/menus/:id/apis", Desc: "绑定菜单接口"},
		{Method: "GET", Path: "/admin/apis", Desc: "接口列表"},
		{Method: "POST", Path: "/admin/apis", Desc: "创建接口"},
		{Method: "PUT", Path: "/admin/apis/:id", Desc: "更新接口"},
		{Method: "DELETE", Path: "/admin/apis/:id", Desc: "删除接口"},
		{Method: "PUT", Path: "/admin/users/:id/role", Desc: "设置用户角色"},
	}
	if err := DB.Create(&apis).Error; err != nil {
		log.Printf("初始化接口权限失败: %v", err)
		return
	}

	// 菜单与接口绑定（按功能模块，通过 method+path 查询 api_id）
	menuPathToPaths := map[string][]string{
		"dashboard":  {"GET:/admin/current-user", "GET:/admin/statistics/summary", "GET:/admin/statistics"},
		"expenses":   {"GET:/admin/expenses", "POST:/admin/expenses", "PUT:/admin/expenses/:id", "DELETE:/admin/expenses/:id", "GET:/admin/expenses/detailed-statistics"},
		"statistics": {"GET:/admin/statistics/summary", "GET:/admin/statistics"},
		"users":      {"GET:/admin/users", "POST:/admin/users/email/send-code", "PUT:/admin/users/:id/password", "PUT:/admin/users/:id/email", "DELETE:/admin/users/:id", "PUT:/admin/users/:id/admin", "PUT:/admin/users/:id/status", "PUT:/admin/users/:id/feishu", "POST:/admin/users/impersonate", "POST:/admin/users/exit-impersonation", "PUT:/admin/users/:id/role"},
		"categories": {"GET:/admin/categories", "POST:/admin/categories", "PUT:/admin/categories/:id", "DELETE:/admin/categories/:id"},
		"income-categories": {"GET:/admin/income-categories", "POST:/admin/income-categories", "PUT:/admin/income-categories/:id", "DELETE:/admin/income-categories/:id"},
		"export":    {"GET:/admin/export/excel"},
		"incomes":   {"GET:/admin/incomes", "POST:/admin/incomes", "PUT:/admin/incomes/:id", "DELETE:/admin/incomes/:id"},
		"ai-models": {"GET:/admin/ai-models", "PUT:/admin/ai-models/reorder", "GET:/admin/ai-models/:id", "POST:/admin/ai-models", "POST:/admin/ai-models/:id/test", "PUT:/admin/ai-models/:id", "DELETE:/admin/ai-models/:id"},
		"ai-analysis": {"POST:/admin/ai-analysis", "GET:/admin/ai-analysis/history", "DELETE:/admin/ai-analysis/history/:id"},
		"ai-chat":    {"POST:/admin/ai-chat", "GET:/admin/ai-chat/history", "DELETE:/admin/ai-chat/history/:id"},
		"roles":      {"GET:/admin/roles", "GET:/admin/roles/:id", "POST:/admin/roles", "PUT:/admin/roles/:id", "DELETE:/admin/roles/:id", "PUT:/admin/roles/:id/menus"},
		"menus":      {"GET:/admin/menus", "POST:/admin/menus", "PUT:/admin/menus/:id", "DELETE:/admin/menus/:id", "PUT:/admin/menus/:id/apis"},
		"apis":       {"GET:/admin/apis", "POST:/admin/apis", "PUT:/admin/apis/:id", "DELETE:/admin/apis/:id"},
	}
	for i, m := range menus {
		menuID := uint(i + 1)
		paths, ok := menuPathToPaths[m.Path]
		if !ok {
			continue
		}
		for _, s := range paths {
			method, path := splitMethodPath(s)
			if method == "" || path == "" {
				continue
			}
			var api models.APIPermission
			if err := DB.Where("method = ? AND path = ?", method, path).First(&api).Error; err == nil {
				_ = DB.Create(&models.MenuAPI{MenuID: menuID, APIID: api.ID}).Error
			}
		}
	}

	// 超级管理员角色关联所有菜单
	adminRoleID := uint(1)
	for i := 1; i <= len(menus); i++ {
		_ = DB.Create(&models.RoleMenu{RoleID: adminRoleID, MenuID: uint(i)}).Error
	}

	// 运营员：除角色/菜单/接口管理外的所有菜单
	operatorPaths := map[string]bool{
		"dashboard": true, "expenses": true, "statistics": true, "users": true,
		"categories": true, "income-categories": true, "export": true, "incomes": true,
		"ai-models": true, "ai-analysis": true, "ai-chat": true,
	}
	for _, m := range menus {
		if operatorPaths[m.Path] {
			_ = DB.Create(&models.RoleMenu{RoleID: 2, MenuID: m.ID}).Error
		}
	}

	// 查看者：仅数据查看相关
	viewerPaths := map[string]bool{
		"dashboard": true, "expenses": true, "statistics": true, "incomes": true,
		"export": true, "ai-analysis": true, "ai-chat": true,
	}
	for _, m := range menus {
		if viewerPaths[m.Path] {
			_ = DB.Create(&models.RoleMenu{RoleID: 3, MenuID: m.ID}).Error
		}
	}
}
