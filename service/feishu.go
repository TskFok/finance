package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// 使用 passport 体系接口（与 www.feishu.cn/passport.feishu.cn 授权页配套）
// 若使用 open.feishu.cn 的 token 接口会导致「飞书授权失败」（code 来自 passport，不兼容）
const (
	feishuTokenURL   = "https://passport.feishu.cn/suite/passport/oauth/token"
	feishuUserInfoURL = "https://passport.feishu.cn/suite/passport/oauth/userinfo"
)

// OAuthTokenRequest 飞书 OAuth token 请求
type OAuthTokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
}

// OAuthTokenResponse 飞书 OAuth token 响应
type OAuthTokenResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Data    *OAuthTokenData `json:"data,omitempty"`
}

// OAuthTokenData token 数据
type OAuthTokenData struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	Scope            string `json:"scope"`
}

// UserInfoResponse 飞书用户信息响应
type UserInfoResponse struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data *FeishuUserInfo `json:"data,omitempty"`
}

// FeishuUserInfo 飞书用户信息
type FeishuUserInfo struct {
	OpenID   string `json:"open_id"`
	UnionID  string `json:"union_id"`
	UserID   string `json:"user_id"`
	Name     string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Email    string `json:"email"`
}

// ExchangeToken 使用授权码换取 user_access_token
// passport 接口要求 application/x-www-form-urlencoded
func ExchangeToken(appID, appSecret, code, redirectURI string) (*OAuthTokenData, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", appID)
	form.Set("client_secret", appSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", feishuTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求飞书服务器失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// passport 接口直接返回 { access_token, token_type, expires_in, refresh_token, refresh_expires_in }
	var tokenData OAuthTokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if tokenData.AccessToken == "" {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(data, &errResp)
		msg := errResp.ErrorDescription
		if msg == "" {
			msg = string(data)
		}
		return nil, fmt.Errorf("飞书返回错误: %s", msg)
	}

	return &tokenData, nil
}

// GetUserInfo 使用 user_access_token 获取用户信息
// passport 接口直接返回用户对象 { open_id, union_id, name, avatar_url, email, ... }，无 code/msg/data 包装
func GetUserInfo(accessToken string) (*FeishuUserInfo, error) {
	req, err := http.NewRequest("GET", feishuUserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求飞书服务器失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var userInfo FeishuUserInfo
	if err := json.Unmarshal(data, &userInfo); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if userInfo.OpenID == "" {
		// 可能返回了 sub 作为 open_id
		var sub struct {
			Sub string `json:"sub"`
		}
		if json.Unmarshal(data, &sub) == nil && sub.Sub != "" {
			userInfo.OpenID = sub.Sub
		}
	}
	if userInfo.OpenID == "" {
		return nil, fmt.Errorf("飞书返回的用户信息中无 open_id")
	}
	return &userInfo, nil
}

// BuildAuthURL 构建飞书授权页面 URL（用于二维码 goto 参数）
// 参考官方示例：https://github.com/Feishu-Lark-Support/sample-node-js-webapp-qrcode-login
// 必须使用 www.feishu.cn/suite/passport/oauth/authorize，否则扫码会报 4401
func BuildAuthURL(appID, redirectURI, state string) string {
	params := url.Values{}
	params.Set("client_id", appID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	if state != "" {
		params.Set("state", state)
	} else {
		params.Set("state", "STATE")
	}
	return "https://www.feishu.cn/suite/passport/oauth/authorize?" + params.Encode()
}
