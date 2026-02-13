package api

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportHandler_ExportCSV(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	// 查询 expenses
	mock.ExpectQuery("SELECT .* FROM `expenses`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "amount", "category", "description", "expense_time", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, 1, 99.99, "餐饮", "午餐", time.Now(), time.Now(), time.Now(), nil))

	router := gin.New()
	router.Use(setUserIDMiddleware(1))
	router.GET("/export/csv", NewExportHandler().ExportCSV)

	req := httptest.NewRequest("GET", "/export/csv?start_time=2024-01-01&end_time=2024-01-31", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Body.String(), "ID")
	assert.Contains(t, w.Body.String(), "金额")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExportHandler_ExportCSV_MissingParams(t *testing.T) {
	router := gin.New()
	router.Use(setUserIDMiddleware(1))
	router.GET("/export/csv", NewExportHandler().ExportCSV)

	req := httptest.NewRequest("GET", "/export/csv", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}
