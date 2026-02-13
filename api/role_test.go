package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"finance/database"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestRoleHandler_Create_DuplicateCode(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	oldDB := database.DB
	database.DB = gormDB
	defer func() { database.DB = oldDB }()

	// 检查 code 是否已存在：返回一条记录表示重复
	mock.ExpectQuery("SELECT .* FROM `roles`").
		WithArgs("admin").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "description", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, "超级管理员", "admin", "", time.Now(), time.Now(), nil))

	router := gin.New()
	router.POST("/admin/roles", NewRoleHandler().Create)
	body := `{"name":"测试角色","code":"admin","description":"重复编码"}`
	req := httptest.NewRequest("POST", "/admin/roles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
	assert.Equal(t, "编码已存在", resp["message"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleHandler_Create_Success(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	oldDB := database.DB
	database.DB = gormDB
	defer func() { database.DB = oldDB }()

	// 检查 code 不存在：SELECT 返回 ErrRecordNotFound（无行）
	mock.ExpectQuery("SELECT .* FROM `roles`").WithArgs("newcode").
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `roles`").
		WillReturnResult(sqlmock.NewResult(4, 1))
	mock.ExpectCommit()

	router := gin.New()
	router.POST("/admin/roles", NewRoleHandler().Create)
	body := `{"name":"新角色","code":"newcode","description":""}`
	req := httptest.NewRequest("POST", "/admin/roles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["success"].(bool))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoleHandler_Update_DuplicateCode(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	oldDB := database.DB
	database.DB = gormDB
	defer func() { database.DB = oldDB }()

	// 查询当前角色
	mock.ExpectQuery("SELECT .* FROM `roles`").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "description", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, "测试", "test", "", time.Now(), time.Now(), nil))

	// 检查新 code 是否被其他角色占用：返回 operator 表示已被占用
	mock.ExpectQuery("SELECT .* FROM `roles`").
		WithArgs("operator", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "description", "created_at", "updated_at", "deleted_at"}).
			AddRow(2, "运营员", "operator", "", time.Now(), time.Now(), nil))

	router := gin.New()
	router.PUT("/admin/roles/:id", NewRoleHandler().Update)
	body := `{"code":"operator"}`
	req := httptest.NewRequest("PUT", "/admin/roles/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
	assert.Equal(t, "编码已存在", resp["message"])
	require.NoError(t, mock.ExpectationsWereMet())
}
