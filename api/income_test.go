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

func TestIncomeHandler_Create(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `incomes`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	router := gin.New()
	router.Use(setUserIDMiddleware(1))
	router.POST("/incomes", NewIncomeHandler().Create)

	body := `{"amount":5000,"type":"工资","income_time":"2024-01-15 09:00:00"}`
	req := httptest.NewRequest("POST", "/incomes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "创建成功", resp["message"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIncomeHandler_GetIncomeCategories(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectQuery("SELECT .* FROM `income_categories`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "sort", "color", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, "工资", 10, "#10b981", time.Now(), time.Now(), nil).
			AddRow(2, "奖金", 20, "#3b82f6", time.Now(), time.Now(), nil))

	router := gin.New()
	router.GET("/income-categories", NewIncomeHandler().GetIncomeCategories)

	req := httptest.NewRequest("GET", "/income-categories", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
