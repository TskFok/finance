package config

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig_LogLevel(t *testing.T) {
	// 清除可能影响测试的环境变量
	orig := os.Getenv("FINANCE_LOG_LEVEL")
	defer os.Setenv("FINANCE_LOG_LEVEL", orig)
	os.Unsetenv("FINANCE_LOG_LEVEL")

	cfg, err := LoadConfig("")
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	// 默认配置中 log.level 为 info
	assert.Equal(t, "info", cfg.Log.Level)
}

func TestLoadConfig_LogLevel_InvalidDefaultsToInfo(t *testing.T) {
	orig := os.Getenv("FINANCE_LOG_LEVEL")
	defer os.Setenv("FINANCE_LOG_LEVEL", orig)
	os.Setenv("FINANCE_LOG_LEVEL", "invalid")

	cfg, err := LoadConfig("")
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	// 无效值应回退为 info
	assert.Equal(t, "info", cfg.Log.Level)
}

func TestLoadConfig_EnvOverridesDatabaseAndJWT(t *testing.T) {
	keys := []string{
		"FINANCE_DATABASE_HOST",
		"FINANCE_DATABASE_PORT",
		"FINANCE_DATABASE_USERNAME",
		"FINANCE_DATABASE_PASSWORD",
		"FINANCE_DATABASE_DBNAME",
		"FINANCE_JWT_SECRET",
		"FINANCE_SERVER_PORT",
		"FINANCE_SERVER_BASE_URL",
	}
	orig := make(map[string]string, len(keys))
	for _, key := range keys {
		orig[key] = os.Getenv(key)
		defer os.Setenv(key, orig[key])
	}

	os.Setenv("FINANCE_DATABASE_HOST", "mysql")
	os.Setenv("FINANCE_DATABASE_PORT", "3306")
	os.Setenv("FINANCE_DATABASE_USERNAME", "finance")
	os.Setenv("FINANCE_DATABASE_PASSWORD", "db-secret")
	os.Setenv("FINANCE_DATABASE_DBNAME", "finance")
	os.Setenv("FINANCE_JWT_SECRET", "jwt-secret")
	os.Setenv("FINANCE_SERVER_PORT", ":8811")
	os.Setenv("FINANCE_SERVER_BASE_URL", "http://localhost:8811")

	cfg, err := LoadConfig("")
	assert.NoError(t, err)
	assert.Equal(t, "mysql", cfg.Database.Host)
	assert.Equal(t, "3306", cfg.Database.Port)
	assert.Equal(t, "finance", cfg.Database.Username)
	assert.Equal(t, "db-secret", cfg.Database.Password)
	assert.Equal(t, "finance", cfg.Database.DBName)
	assert.Equal(t, "jwt-secret", cfg.JWT.Secret)
	assert.Equal(t, ":8811", cfg.Server.Port)
	assert.Equal(t, "http://localhost:8811", cfg.Server.BaseURL)
}

func TestLoadConfig_EnvOverridesConfigYAMLFields(t *testing.T) {
	keys := []string{
		"FINANCE_LOG_LEVEL",
		"FINANCE_SERVER_MODE",
		"FINANCE_DATABASE_CHARSET",
		"FINANCE_JWT_EXPIRE_HOURS",
		"FINANCE_EMAIL_ENABLED",
		"FINANCE_EMAIL_HOST",
		"FINANCE_EMAIL_PORT",
		"FINANCE_EMAIL_USERNAME",
		"FINANCE_EMAIL_PASSWORD",
		"FINANCE_EMAIL_FROM",
		"FINANCE_FEISHU_ENABLED",
		"FINANCE_FEISHU_APP_ID",
		"FINANCE_FEISHU_APP_SECRET",
		"FINANCE_FEISHU_AUTO_CREATE_USER",
	}
	for _, key := range keys {
		orig := os.Getenv(key)
		defer os.Setenv(key, orig)
	}

	os.Setenv("FINANCE_LOG_LEVEL", "warn")
	os.Setenv("FINANCE_SERVER_MODE", "debug")
	os.Setenv("FINANCE_DATABASE_CHARSET", "utf8mb4")
	os.Setenv("FINANCE_JWT_EXPIRE_HOURS", "12")
	os.Setenv("FINANCE_EMAIL_ENABLED", "true")
	os.Setenv("FINANCE_EMAIL_HOST", "smtp.example.com")
	os.Setenv("FINANCE_EMAIL_PORT", "587")
	os.Setenv("FINANCE_EMAIL_USERNAME", "mailer@example.com")
	os.Setenv("FINANCE_EMAIL_PASSWORD", "mail-secret")
	os.Setenv("FINANCE_EMAIL_FROM", "记账系统")
	os.Setenv("FINANCE_FEISHU_ENABLED", "true")
	os.Setenv("FINANCE_FEISHU_APP_ID", "cli_example")
	os.Setenv("FINANCE_FEISHU_APP_SECRET", "feishu-secret")
	os.Setenv("FINANCE_FEISHU_AUTO_CREATE_USER", "true")

	cfg, err := LoadConfig("")
	assert.NoError(t, err)
	assert.Equal(t, "warn", cfg.Log.Level)
	assert.Equal(t, "debug", cfg.Server.Mode)
	assert.Equal(t, "utf8mb4", cfg.Database.Charset)
	assert.Equal(t, 12, cfg.JWT.ExpireHours)
	assert.True(t, cfg.Email.Enabled)
	assert.Equal(t, "smtp.example.com", cfg.Email.Host)
	assert.Equal(t, 587, cfg.Email.Port)
	assert.Equal(t, "mailer@example.com", cfg.Email.Username)
	assert.Equal(t, "mail-secret", cfg.Email.Password)
	assert.Equal(t, "记账系统", cfg.Email.From)
	assert.True(t, cfg.Feishu.Enabled)
	assert.Equal(t, "cli_example", cfg.Feishu.AppID)
	assert.Equal(t, "feishu-secret", cfg.Feishu.AppSecret)
	assert.True(t, cfg.Feishu.AutoCreateUser)
}

func TestLoadConfig_LogLevel_ValidValues(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		orig := os.Getenv("FINANCE_LOG_LEVEL")
		os.Setenv("FINANCE_LOG_LEVEL", level)
		cfg, err := LoadConfig("")
		os.Setenv("FINANCE_LOG_LEVEL", orig)
		assert.NoError(t, err)
		assert.Equal(t, level, cfg.Log.Level, "level %s", level)
	}
}

func TestSafeErrorMessage(t *testing.T) {
	fallback := "操作失败"
	testErr := errors.New("internal database error")

	// nil err 返回 fallback
	assert.Equal(t, fallback, SafeErrorMessage(nil, fallback))

	// release 模式返回 fallback，不暴露错误详情
	GlobalConfig = &Config{Server: ServerConfig{Mode: "release"}}
	defer func() { GlobalConfig = nil }()
	assert.Equal(t, fallback, SafeErrorMessage(testErr, fallback))

	// debug 模式返回 err.Error()
	GlobalConfig = &Config{Server: ServerConfig{Mode: "debug"}}
	assert.Equal(t, "internal database error", SafeErrorMessage(testErr, fallback))

	// GlobalConfig 为 nil 时返回 err.Error()（视为开发环境）
	GlobalConfig = nil
	assert.Equal(t, "internal database error", SafeErrorMessage(testErr, fallback))
}
