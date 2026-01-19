package config

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Email    EmailConfig    `mapstructure:"email"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port    string `mapstructure:"port"`
	Mode    string `mapstructure:"mode"`
	BaseURL string `mapstructure:"base_url"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	Charset  string `mapstructure:"charset"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret      string        `mapstructure:"secret"`
	ExpireHours int           `mapstructure:"expire_hours"`
	ExpireTime  time.Duration `mapstructure:"-"`
}

// EmailConfig 邮件配置
type EmailConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

var (
	// GlobalConfig 全局配置实例
	GlobalConfig *Config
)

// LoadConfig 加载配置
// 优先级: 外部配置文件 > 嵌入的默认配置
// configPath: 可选的外部配置文件路径
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	// 1. 首先加载嵌入的默认配置
	if err := v.ReadConfig(bytes.NewReader(DefaultConfigYAML)); err != nil {
		return nil, fmt.Errorf("读取内置配置失败: %w", err)
	}
	log.Println("已加载内置默认配置")

	// 2. 尝试加载外部配置文件（可选，用于覆盖默认配置）
	if configPath != "" {
		// 指定了配置文件路径
		v.SetConfigFile(configPath)
		if err := v.MergeInConfig(); err != nil {
			log.Printf("警告: 无法读取指定配置文件 %s: %v", configPath, err)
		} else {
			log.Printf("已合并外部配置文件: %s", configPath)
		}
	} else {
		// 尝试查找外部配置文件
		externalViper := viper.New()
		externalViper.SetConfigName("config")
		externalViper.SetConfigType("yaml")
		externalViper.AddConfigPath(".")
		externalViper.AddConfigPath("./config")
		externalViper.AddConfigPath("/etc/finance")
		externalViper.AddConfigPath("$HOME/.finance")

		if err := externalViper.ReadInConfig(); err == nil {
			// 找到外部配置文件，合并配置
			if err := v.MergeConfigMap(externalViper.AllSettings()); err != nil {
				log.Printf("警告: 合并外部配置失败: %v", err)
			} else {
				log.Printf("已合并外部配置文件: %s", externalViper.ConfigFileUsed())
			}
		}
	}

	// 3. 支持环境变量覆盖（可选）
	v.SetEnvPrefix("FINANCE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 解析配置
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 设置 JWT 过期时间
	if cfg.JWT.ExpireHours <= 0 {
		cfg.JWT.ExpireHours = 24
	}
	cfg.JWT.ExpireTime = time.Duration(cfg.JWT.ExpireHours) * time.Hour

	// 保存到全局变量
	GlobalConfig = &cfg

	return &cfg, nil
}

// MustLoadConfig 加载配置，失败则 panic
func MustLoadConfig(configPath string) *Config {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		panic(fmt.Sprintf("加载配置失败: %v", err))
	}
	return cfg
}

// GetConfig 获取全局配置
func GetConfig() *Config {
	if GlobalConfig == nil {
		panic("配置未初始化，请先调用 LoadConfig")
	}
	return GlobalConfig
}

// PrintConfig 打印当前配置（隐藏敏感信息）
func PrintConfig() {
	if GlobalConfig == nil {
		return
	}
	log.Printf("当前配置:")
	log.Printf("  服务器: %s (模式: %s)", GlobalConfig.Server.Port, GlobalConfig.Server.Mode)
	log.Printf("  数据库: %s@%s:%s/%s",
		GlobalConfig.Database.Username,
		GlobalConfig.Database.Host,
		GlobalConfig.Database.Port,
		GlobalConfig.Database.DBName)
	log.Printf("  邮件服务: %v", GlobalConfig.Email.Enabled)
}
