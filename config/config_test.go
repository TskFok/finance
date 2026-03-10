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
