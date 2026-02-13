package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"finance/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestFeishuAuthHandler_GetFeishuConfig_Disabled(t *testing.T) {
	cfg := &config.Config{
		Feishu: config.FeishuConfig{Enabled: false, AppID: ""},
	}
	h := NewFeishuAuthHandler(cfg)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/feishu/config", h.GetFeishuConfig)

	req := httptest.NewRequest("GET", "/feishu/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp["success"].(bool))
	assert.Contains(t, resp["message"], "未启用")
}

func TestFeishuAuthHandler_GetFeishuConfig_Enabled(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Port: ":8080", BaseURL: ""},
		Feishu: config.FeishuConfig{Enabled: true, AppID: "cli_test123"},
	}
	h := NewFeishuAuthHandler(cfg)

	router := gin.New()
	router.GET("/feishu/config", h.GetFeishuConfig)

	req := httptest.NewRequest("GET", "/feishu/config?state=bind", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp["success"].(bool))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "cli_test123", data["app_id"])
	assert.Contains(t, data["auth_url"], "www.feishu.cn")
	assert.Contains(t, data["auth_url"], "bind")
}
