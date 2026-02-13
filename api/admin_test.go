package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"finance/config"
	"finance/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestAdminHandler_AdminLogin(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	config.GlobalConfig = &config.Config{Server: config.ServerConfig{Mode: "debug"}}
	defer func() { config.GlobalConfig = nil }()

	hashed, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	mock.ExpectQuery("SELECT .* FROM `users`").
		WithArgs("adminuser", "adminuser").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "email", "is_admin", "status", "feishu_open_id", "feishu_union_id", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, "adminuser", string(hashed), "admin@x.com", true, models.UserStatusActive, nil, "", time.Now(), time.Now(), nil))

	router := gin.New()
	router.POST("/admin/login", NewAdminHandler().AdminLogin)

	body := `{"username":"adminuser","password":"admin123"}`
	req := httptest.NewRequest("POST", "/admin/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["success"].(bool))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminHandler_AdminLogin_AccountLocked(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	hashed, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	mock.ExpectQuery("SELECT .* FROM `users`").
		WithArgs("lockeduser", "lockeduser").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "email", "is_admin", "status", "feishu_open_id", "feishu_union_id", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, "lockeduser", string(hashed), "l@x.com", false, models.UserStatusLocked, nil, "", time.Now(), time.Now(), nil))

	router := gin.New()
	router.POST("/admin/login", NewAdminHandler().AdminLogin)

	body := `{"username":"lockeduser","password":"pass"}`
	req := httptest.NewRequest("POST", "/admin/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
