package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
