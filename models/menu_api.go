package models

// MenuAPI 菜单-接口多对多关联（拥有某菜单即可访问其绑定的接口）
type MenuAPI struct {
	MenuID uint `gorm:"primaryKey;autoIncrement:false"`
	APIID  uint `gorm:"primaryKey;autoIncrement:false"`
}

// TableName 设置表名
func (MenuAPI) TableName() string {
	return "menu_apis"
}
