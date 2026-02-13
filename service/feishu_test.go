package service

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAuthURL(t *testing.T) {
	appID := "cli_xxx"
	redirectURI := "https://myapp.com/admin/feishu/callback"

	// state 非空时使用传入的 state
	u := BuildAuthURL(appID, redirectURI, "bind:token123")
	assert.Contains(t, u, "https://www.feishu.cn/suite/passport/oauth/authorize?")
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	assert.Equal(t, appID, parsed.Query().Get("client_id"))
	assert.Equal(t, redirectURI, parsed.Query().Get("redirect_uri"))
	assert.Equal(t, "code", parsed.Query().Get("response_type"))
	assert.Equal(t, "bind:token123", parsed.Query().Get("state"))

	// state 为空时使用默认 "STATE"
	u2 := BuildAuthURL(appID, redirectURI, "")
	parsed2, err := url.Parse(u2)
	require.NoError(t, err)
	assert.Equal(t, "STATE", parsed2.Query().Get("state"))
}
