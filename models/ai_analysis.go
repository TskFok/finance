package models

import (
	"time"

	"gorm.io/gorm"
)

// AIAnalysisHistory AI分析历史记录（单次分析）
type AIAnalysisHistory struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	AIModelID uint           `json:"ai_model_id" gorm:"index;not null"`
	StartDate string         `json:"start_date" gorm:"size:10;not null"` // YYYY-MM-DD
	EndDate   string         `json:"end_date" gorm:"size:10;not null"`   // YYYY-MM-DD
	Result    string         `json:"result" gorm:"type:longtext;not null"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	AIModel AIModel `json:"-" gorm:"foreignKey:AIModelID"`
}

func (AIAnalysisHistory) TableName() string {
	return "ai_analysis_histories"
}


