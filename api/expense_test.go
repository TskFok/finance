package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setUserIDMiddleware(userID uint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	}
}

func TestExpenseHandler_Create(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	// 查询类别
	mock.ExpectQuery("SELECT .* FROM `expense_categories`").
		WithArgs("餐饮").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "sort", "color", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, "餐饮", 10, "#ef4444", time.Now(), time.Now(), nil))

	// INSERT expense
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `expenses`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	router := gin.New()
	router.Use(setUserIDMiddleware(1))
	router.POST("/expenses", NewExpenseHandler().Create)

	body := `{"amount":99.99,"category":"餐饮","description":"午餐","expense_time":"2024-01-15 12:30:00"}`
	req := httptest.NewRequest("POST", "/expenses", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "创建成功", resp["message"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExpenseHandler_Create_InvalidCategory(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectQuery("SELECT .* FROM `expense_categories`").
		WithArgs("无效类别").
		WillReturnRows(sqlmock.NewRows([]string{}))

	router := gin.New()
	router.Use(setUserIDMiddleware(1))
	router.POST("/expenses", NewExpenseHandler().Create)

	body := `{"amount":99,"category":"无效类别","expense_time":"2024-01-15 12:30:00"}`
	req := httptest.NewRequest("POST", "/expenses", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
