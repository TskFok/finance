package main

import (
	"flag"
	"log"
	"log/slog"
	"strings"

	"finance/config"
	"finance/database"
	"finance/logger"
	"finance/middleware"
	"finance/router"
)

// @title 记账系统 API
// @version 1.0
// @description 一个简单的记账系统 API，支持用户注册、登录、消费记录管理和数据导出功能
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

var (
	configFile  string
	port        string
	showVersion bool
)

func init() {
	flag.StringVar(&configFile, "config", "", "外部配置文件路径（可选）")
	flag.StringVar(&configFile, "c", "", "外部配置文件路径（简写）")
	flag.StringVar(&port, "port", "", "监听端口，如: 8080 或 :8080")
	flag.StringVar(&port, "p", "", "监听端口（简写）")
	flag.BoolVar(&showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&showVersion, "v", false, "显示版本信息（简写）")
}

func main() {
	flag.Parse()

	if showVersion {
		log.Println("记账系统 v1.0.0")
		return
	}

	// 加载配置（内置配置 + 可选的外部配置覆盖）
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 根据配置初始化日志级别
	logger.Init(cfg)

	// 命令行参数覆盖端口配置
	if port != "" {
		// 自动添加冒号前缀
		if !strings.HasPrefix(port, ":") {
			port = ":" + port
		}
		cfg.Server.Port = port
		slog.Info("命令行指定端口", "port", port)
	}

	// 打印配置信息
	config.PrintConfig()

	// 初始化数据库
	if err := database.Init(cfg); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 初始化 JWT
	middleware.InitJWT(cfg)

	// 设置路由
	r := router.SetupRouter(cfg)

	// 启动服务器
	slog.Info("==========================================")
	slog.Info("  💰 记账系统已启动")
	slog.Info("==========================================")
	slog.Info("  后台管理", "url", "http://localhost"+cfg.Server.Port+"/")
	slog.Info("  Swagger", "url", "http://localhost"+cfg.Server.Port+"/swagger/index.html")
	slog.Info("  API接口", "url", "http://localhost"+cfg.Server.Port+"/api/v1/")
	slog.Info("==========================================")

	if err := r.Run(cfg.Server.Port); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
