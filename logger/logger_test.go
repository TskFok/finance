package logger

import (
	"testing"

	"finance/config"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/logger"
)

func TestGORMLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected logger.LogLevel
	}{
		{"nil config returns Info", nil, logger.Info},
		{"debug level returns Info (show SQL)", &config.Config{Log: config.LogConfig{Level: "debug"}}, logger.Info},
		{"info level returns Warn", &config.Config{Log: config.LogConfig{Level: "info"}}, logger.Warn},
		{"warn level returns Error", &config.Config{Log: config.LogConfig{Level: "warn"}}, logger.Error},
		{"error level returns Error (no SQL)", &config.Config{Log: config.LogConfig{Level: "error"}}, logger.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GORMLogLevel(tt.cfg)
			assert.Equal(t, tt.expected, got)
		})
	}
}
