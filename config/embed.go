package config

import (
	_ "embed"
)

// 嵌入默认配置文件
//
//go:embed default.yaml
var DefaultConfigYAML []byte

