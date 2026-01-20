package database

import (
	"fmt"
	"log"

	"finance/config"
	"finance/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
		&models.PasswordReset{},
		&models.EmailVerification{},
		&models.AIModel{},
		&models.AIChatMessage{},
		&models.AIAnalysisHistory{},
	); err != nil {
		return err
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

	log.Println("数据库初始化成功")
	return nil
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	return DB
}
