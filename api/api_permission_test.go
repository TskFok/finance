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

func setupAPIMockDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	oldDB := database.DB
	database.DB = gormDB
	return mock, func() {
		database.DB = oldDB
		sqlDB.Close()
	}
}

func TestAPIPermissionHandler_Create_DuplicateMethodPath(t *testing.T) {
	mock, cleanup := setupAPIMockDB(t)
	defer cleanup()

	// 方法+路径已存在
	mock.ExpectQuery("SELECT .* FROM `api_permissions`").
		WithArgs("GET", "/admin/expenses").
		WillReturnRows(sqlmock.NewRows([]string{"id", "method", "path", "desc", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, "GET", "/admin/expenses", "", time.Now(), time.Now(), nil))

	router := gin.New()
	router.POST("/admin/apis", NewAPIPermissionHandler().Create)
	body := `{"method":"GET","path":"/admin/expenses","desc":""}`
	req := httptest.NewRequest("POST", "/admin/apis", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
	assert.Equal(t, "方法+路径已存在", resp["message"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIPermissionHandler_Create_Success(t *testing.T) {
	mock, cleanup := setupAPIMockDB(t)
	defer cleanup()

	mock.ExpectQuery("SELECT .* FROM `api_permissions`").
		WithArgs("POST", "/admin/custom").
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `api_permissions`").
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectCommit()

	router := gin.New()
	router.POST("/admin/apis", NewAPIPermissionHandler().Create)
	body := `{"method":"POST","path":"/admin/custom","desc":"自定义"}`
	req := httptest.NewRequest("POST", "/admin/apis", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["success"].(bool))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIPermissionHandler_Update_DuplicateMethodPath(t *testing.T) {
	mock, cleanup := setupAPIMockDB(t)
	defer cleanup()

	// 查询当前接口
	mock.ExpectQuery("SELECT .* FROM `api_permissions`").WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "method", "path", "desc", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, "GET", "/admin/users", "", time.Now(), time.Now(), nil))

	// 修改为已存在的 method+path（排除自身后仍找到其他记录）
	mock.ExpectQuery("SELECT .* FROM `api_permissions`").
		WithArgs("GET", "/admin/expenses", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "method", "path", "desc", "created_at", "updated_at", "deleted_at"}).
			AddRow(2, "GET", "/admin/expenses", "", time.Now(), time.Now(), nil))

	router := gin.New()
	router.PUT("/admin/apis/:id", NewAPIPermissionHandler().Update)
	body := `{"method":"GET","path":"/admin/expenses"}`
	req := httptest.NewRequest("PUT", "/admin/apis/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
	assert.Equal(t, "方法+路径已存在", resp["message"])
	require.NoError(t, mock.ExpectationsWereMet())
}
