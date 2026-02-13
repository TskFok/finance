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

func setupMenuMockDB(t *testing.T) (sqlmock.Sqlmock, func()) {
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

func TestMenuHandler_Create_ParentNotExists(t *testing.T) {
	mock, cleanup := setupMenuMockDB(t)
	defer cleanup()

	// 父级菜单不存在：SELECT parent 返回 ErrRecordNotFound
	mock.ExpectQuery("SELECT .* FROM `menus`").WithArgs(999).
		WillReturnError(gorm.ErrRecordNotFound)

	router := gin.New()
	router.POST("/admin/menus", NewMenuHandler().Create)
	body := `{"parent_id":999,"name":"子菜单","path":"child","icon":"","sort_order":0}`
	req := httptest.NewRequest("POST", "/admin/menus", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
	assert.Equal(t, "父级菜单不存在", resp["message"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuHandler_Update_SelfAsParent(t *testing.T) {
	mock, cleanup := setupMenuMockDB(t)
	defer cleanup()

	// 查询当前菜单
	mock.ExpectQuery("SELECT .* FROM `menus`").WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "name", "path", "icon", "sort_order", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, 0, "菜单1", "m1", "", 0, time.Now(), time.Now(), nil))

	router := gin.New()
	router.PUT("/admin/menus/:id", NewMenuHandler().Update)
	body := `{"parent_id":1}`
	req := httptest.NewRequest("PUT", "/admin/menus/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
	assert.Equal(t, "不能将父级设为自己", resp["message"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuHandler_Create_SuccessWithParent(t *testing.T) {
	mock, cleanup := setupMenuMockDB(t)
	defer cleanup()

	// 父级存在
	mock.ExpectQuery("SELECT .* FROM `menus`").WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "name", "path", "icon", "sort_order", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, 0, "父菜单", "parent", "", 0, time.Now(), time.Now(), nil))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `menus`").
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	router := gin.New()
	router.POST("/admin/menus", NewMenuHandler().Create)
	body := `{"parent_id":1,"name":"子菜单","path":"child","icon":"","sort_order":0}`
	req := httptest.NewRequest("POST", "/admin/menus", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["success"].(bool))
	require.NoError(t, mock.ExpectationsWereMet())
}
