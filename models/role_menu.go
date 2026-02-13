package models

// RoleMenu 角色-菜单多对多关联
type RoleMenu struct {
	RoleID uint `gorm:"primaryKey;autoIncrement:false"`
	MenuID uint `gorm:"primaryKey;autoIncrement:false"`
}

// TableName 设置表名
func (RoleMenu) TableName() string {
	return "role_menus"
}
