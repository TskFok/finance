package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"finance/config"
	"finance/database"
	"finance/middleware"
	"finance/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (sqlmock.Sqlmock, func()) {
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

func TestAuthHandler_Register(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "debug"},
		JWT:    config.JWTConfig{Secret: "test-secret", ExpireTime: time.Hour},
	}
	config.GlobalConfig = cfg
	middleware.InitJWT(cfg)
	defer func() { config.GlobalConfig = nil }()

	// 检查用户名不存在：SELECT 返回无记录
	mock.ExpectQuery("SELECT .* FROM `users`").
		WithArgs("newuser").
		WillReturnRows(sqlmock.NewRows([]string{}))

	// GORM Create 使用事务
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `users`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewAuthHandler(cfg)
	router.POST("/register", h.Register)

	body := `{"username":"newuser","password":"password123","email":"test@example.com"}`
	req := httptest.NewRequest("POST", "/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(200), resp["code"])
	assert.Equal(t, "注册成功", resp["message"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandler_Register_UsernameExists(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "debug"},
		JWT:    config.JWTConfig{Secret: "test-secret"},
	}
	config.GlobalConfig = cfg
	defer func() { config.GlobalConfig = nil }()

	// SELECT 返回已有用户
	mock.ExpectQuery("SELECT .* FROM `users`").
		WithArgs("existinguser").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "email", "is_admin", "status", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, "existinguser", "hash", "e@x.com", false, "locked", time.Now(), time.Now(), nil))

	router := gin.New()
	h := NewAuthHandler(cfg)
	router.POST("/register", h.Register)

	body := `{"username":"existinguser","password":"password123"}`
	req := httptest.NewRequest("POST", "/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "用户名已存在", resp["message"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandler_Login(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "debug"},
		JWT:    config.JWTConfig{Secret: "test-secret", ExpireTime: time.Hour},
	}
	config.GlobalConfig = cfg
	middleware.InitJWT(cfg)
	defer func() { config.GlobalConfig = nil }()

	// SELECT 用户（username OR email）
	mock.ExpectQuery("SELECT .* FROM `users`").
		WithArgs("loginuser", "loginuser").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "email", "is_admin", "status", "feishu_open_id", "feishu_union_id", "created_at", "updated_at", "deleted_at"}).
			AddRow(1, "loginuser", string(hashed), "login@x.com", false, models.UserStatusActive, nil, "", time.Now(), time.Now(), nil))

	router := gin.New()
	h := NewAuthHandler(cfg)
	router.POST("/login", h.Login)

	body := `{"username":"loginuser","password":"password123"}`
	req := httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(200), resp["code"])
	assert.NotEmpty(t, resp["data"])
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["token"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandler_Login_UserNotFound(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	cfg := &config.Config{Server: config.ServerConfig{Mode: "debug"}, JWT: config.JWTConfig{Secret: "x"}}
	config.GlobalConfig = cfg
	defer func() { config.GlobalConfig = nil }()

	mock.ExpectQuery("SELECT .* FROM `users`").
		WithArgs("nouser", "nouser").
		WillReturnRows(sqlmock.NewRows([]string{}))

	router := gin.New()
	h := NewAuthHandler(cfg)
	router.POST("/login", h.Login)

	body := `{"username":"nouser","password":"any"}`
	req := httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
