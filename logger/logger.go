package logger

import (
	"log/slog"
	"os"

	"finance/config"

	"gorm.io/gorm/logger"
)

// Init 根据配置初始化日志，设置 slog 的默认 logger 级别
func Init(cfg *config.Config) {
	level := parseLevel(cfg.Log.Level)
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

// parseLevel 将字符串解析为 slog.Level
func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// GORMLogLevel 根据日志级别返回 GORM 的日志级别
// debug -> Info 显示 SQL；info -> Warn 慢查询+错误；warn/error -> Error 仅输出数据库错误（不输出 SQL 和慢查询）
func GORMLogLevel(cfg *config.Config) logger.LogLevel {
	if cfg == nil {
		return logger.Info
	}
	switch cfg.Log.Level {
	case "debug":
		return logger.Info
	case "info":
		return logger.Warn
	case "warn", "error":
		return logger.Error
	default:
		return logger.Warn
	}
}
