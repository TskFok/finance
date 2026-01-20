package router

import (
	"io/fs"
	"net/http"

	"finance/api"
	"finance/config"
	_ "finance/docs"
	"finance/middleware"
	"finance/web"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupRouter 设置路由
func SetupRouter(cfg *config.Config) *gin.Engine {
	// 设置运行模式
	gin.SetMode(cfg.Server.Mode)

	r := gin.Default()

	// CORS 中间件
	r.Use(CORSMiddleware())

	// 嵌入的静态文件 - 后台管理页面
	staticFS, _ := fs.Sub(web.StaticFS, ".")
	r.GET("/", func(c *gin.Context) {
		content, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "加载页面失败")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})

	// 后台管理 API
	adminHandler := api.NewAdminHandler()
	passwordResetHandler := api.NewPasswordResetHandler(cfg)
	admin := r.Group("/admin")
	{
		admin.POST("/login", adminHandler.AdminLogin)
		admin.POST("/logout", adminHandler.AdminLogout)

		// 密码重置（无需登录）
		admin.POST("/password/request-reset", passwordResetHandler.RequestPasswordReset)
		admin.GET("/password/verify-token", passwordResetHandler.VerifyResetToken)
		admin.POST("/password/reset", passwordResetHandler.ResetPassword)

		// 需要 Cookie 认证的后台接口
		adminAuth := admin.Group("")
		adminAuth.Use(AdminAuthMiddleware())
		{
			adminAuth.GET("/expenses", adminHandler.GetAllExpenses)
			adminAuth.POST("/expenses", adminHandler.CreateExpense)
			adminAuth.PUT("/expenses/:id", adminHandler.UpdateExpense)
			adminAuth.DELETE("/expenses/:id", adminHandler.DeleteExpense)
			adminAuth.GET("/expenses/detailed-statistics", adminHandler.GetDetailedStatistics)
			categoryHandler := api.NewCategoryHandler()
			adminAuth.GET("/categories", categoryHandler.List)
			adminAuth.POST("/categories", categoryHandler.Create)
			adminAuth.PUT("/categories/:id", categoryHandler.Update)
			adminAuth.DELETE("/categories/:id", categoryHandler.Delete)
			adminAuth.GET("/users", adminHandler.GetAllUsers)
			adminAuth.GET("/statistics", adminHandler.GetStatistics)
			// 收入管理
			adminAuth.GET("/incomes", adminHandler.GetAllIncomes)
			adminAuth.POST("/incomes", adminHandler.CreateIncome)
			adminAuth.PUT("/incomes/:id", adminHandler.UpdateIncome)
			adminAuth.DELETE("/incomes/:id", adminHandler.DeleteIncome)
			adminAuth.GET("/export/excel", adminHandler.ExportExcel)

			// 管理员密码重置功能
			adminAuth.POST("/password/admin-reset", passwordResetHandler.AdminResetPassword)
			adminAuth.POST("/password/send-reset-email", passwordResetHandler.SendPasswordResetEmail)
			adminAuth.GET("/email-config", passwordResetHandler.GetEmailConfig)

			// AI模型管理
			aiModelHandler := api.NewAIModelHandler()
			adminAuth.GET("/ai-models", aiModelHandler.GetAllAIModels)
			adminAuth.GET("/ai-models/:id", aiModelHandler.GetAIModel)
			adminAuth.POST("/ai-models", aiModelHandler.CreateAIModel)
			adminAuth.PUT("/ai-models/:id", aiModelHandler.UpdateAIModel)
			adminAuth.DELETE("/ai-models/:id", aiModelHandler.DeleteAIModel)

			// AI分析
			aiAnalysisHandler := api.NewAIAnalysisHandler()
			adminAuth.POST("/ai-analysis", aiAnalysisHandler.AnalyzeExpenses)
			adminAuth.GET("/ai-analysis/history", aiAnalysisHandler.ListAnalysisHistory)
			adminAuth.DELETE("/ai-analysis/history/:id", aiAnalysisHandler.DeleteAnalysisHistory)

			// AI聊天（流式 + 历史）
			aiChatHandler := api.NewAIChatHandler()
			adminAuth.POST("/ai-chat", aiChatHandler.ChatStream)
			adminAuth.GET("/ai-chat/history", aiChatHandler.ChatHistory)
			adminAuth.DELETE("/ai-chat/history/:id", aiChatHandler.DeleteChatHistory)
		}
	}

	// Swagger 文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API v1 路由组（供安卓 App 使用）
	v1 := r.Group("/api/v1")
	{
		// 认证相关路由（无需登录）
		authHandler := api.NewAuthHandler(cfg)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)

			// 邮箱验证相关
			auth.POST("/send-code", authHandler.SendVerificationCode)
			auth.POST("/verify-code", authHandler.VerifyEmailCode)
			auth.POST("/register-verified", authHandler.RegisterWithVerification)

			// App 端密码重置
			auth.POST("/password/request-reset", authHandler.AppRequestPasswordReset)
			auth.POST("/password/verify-code", authHandler.AppVerifyResetCode)
			auth.POST("/password/reset", authHandler.AppResetPassword)
		}

		// 消费类别（无需登录）
		expenseHandler := api.NewExpenseHandler()
		v1.GET("/categories", expenseHandler.GetCategories)

		// 需要 JWT 认证的路由
		authorized := v1.Group("")
		authorized.Use(middleware.JWTAuth())
		{
			// 用户相关
			authorized.GET("/auth/profile", authHandler.GetProfile)
			authorized.PUT("/auth/password", authHandler.ChangePassword)

			// 消费记录相关
			expenses := authorized.Group("/expenses")
			{
				expenses.POST("", expenseHandler.Create)
				expenses.GET("", expenseHandler.List)
				expenses.GET("/statistics", expenseHandler.GetStatistics)
				expenses.GET("/detailed-statistics", expenseHandler.GetDetailedStatistics)
				expenses.GET("/:id", expenseHandler.Get)
				expenses.PUT("/:id", expenseHandler.Update)
				expenses.DELETE("/:id", expenseHandler.Delete)
			}

			// 收入相关
			incomeHandler := api.NewIncomeHandler()
			incomes := authorized.Group("/incomes")
			{
				incomes.POST("", incomeHandler.Create)
				incomes.GET("", incomeHandler.List)
				incomes.GET("/:id", incomeHandler.Get)
				incomes.PUT("/:id", incomeHandler.Update)
				incomes.DELETE("/:id", incomeHandler.Delete)
			}

			// 导出相关
			exportHandler := api.NewExportHandler()
			export := authorized.Group("/export")
			{
				export.GET("/csv", exportHandler.ExportCSV)
				export.GET("/json", exportHandler.ExportJSON)
			}
		}
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	return r
}

// CORSMiddleware CORS 跨域中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// AdminAuthMiddleware 后台管理 Cookie 认证中间件
func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := c.Cookie("admin_user_id")
		if err != nil || userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "请先登录",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
