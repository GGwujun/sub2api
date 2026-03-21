//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func makeAllowlistedBaseURL(raw string) string {
	// URL 校验器对裸 IP（如 127.0.0.1）有更严格限制，
	// 测试场景统一替换为 localhost 以匹配 allowlist 规则。
	return strings.Replace(raw, "127.0.0.1", "localhost", 1)
}

// TestZhipuGatewayService_TestConnection_Success 测试连接成功场景
func TestZhipuGatewayService_TestConnection_Success(t *testing.T) {
	// 创建测试服务器
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求
		authHeader := r.Header.Get("Authorization")
		require.True(t, strings.HasPrefix(authHeader, "Bearer "))

		// 返回模拟响应
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "test response"
				}
			}]
		}`))
	}))
	defer ts.Close()

	// 创建服务
	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				AllowInsecureHTTP: true,
				AllowPrivateHosts: true,
			},
		},
	}

	account := &Account{
		ID:   1,
		Name: "test-account",
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-key-123",
			"base_url": makeAllowlistedBaseURL(ts.URL),
		},
		Concurrency: 1,
		Platform:    PlatformZAI,
	}

	svc := &ZhipuGatewayService{
		cfg: cfg,
	}

	// 测试
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := svc.TestConnection(ctx, account, "glm-4-flash")

	// 验证结果
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Text, "test response")
	require.Equal(t, "glm-4-flash", result.MappedModel)
}

// TestZhipuGatewayService_TestConnection_InvalidAPIKey 测试无效API Key场景
func TestZhipuGatewayService_TestConnection_InvalidAPIKey(t *testing.T) {
	// 创建测试服务器（返回401）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer ts.Close()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				AllowInsecureHTTP: true,
				AllowPrivateHosts: true,
			},
		},
	}

	account := &Account{
		ID:   1,
		Name: "test-account",
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "invalid-key",
			"base_url": makeAllowlistedBaseURL(ts.URL),
		},
		Concurrency: 1,
		Platform:    PlatformZAI,
	}

	svc := &ZhipuGatewayService{
		cfg: cfg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := svc.TestConnection(ctx, account, "glm-4-flash")

	// 验证结果
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "test request failed")
}

// TestZhipuGatewayService_TestConnection_DefaultModel 测试默认模型选择
func TestZhipuGatewayService_TestConnection_DefaultModel(t *testing.T) {
	// 创建测试服务器
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "test"
				}
			}]
		}`))
	}))
	defer ts.Close()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				AllowInsecureHTTP: true,
				AllowPrivateHosts: true,
			},
		},
	}

	account := &Account{
		ID:   1,
		Name: "test-account",
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-key",
			"base_url": makeAllowlistedBaseURL(ts.URL),
		},
		Concurrency: 1,
		Platform:    PlatformZAI,
	}

	svc := &ZhipuGatewayService{
		cfg: cfg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 不提供modelID，应该使用默认的glm-4-flash
	result, err := svc.TestConnection(ctx, account, "")

	// 验证结果
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "glm-4-flash", result.MappedModel)
}

// TestZhipuGatewayService_TestConnection_NilAccount 测试nil账号场景
func TestZhipuGatewayService_TestConnection_NilAccount(t *testing.T) {
	cfg := &config.Config{}
	svc := &ZhipuGatewayService{
		cfg: cfg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := svc.TestConnection(ctx, nil, "glm-4-flash")

	// 验证结果
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "account is nil")
}

// TestZhipuGatewayService_TestConnection_Timeout 测试超时场景
func TestZhipuGatewayService_TestConnection_Timeout(t *testing.T) {
	// 创建测试服务器（延迟响应）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟超时；同时监听客户端取消，避免测试结束时连接悬挂
		select {
		case <-r.Context().Done():
			return
		case <-time.After(35 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				AllowInsecureHTTP: true,
				AllowPrivateHosts: true,
			},
		},
	}

	account := &Account{
		ID:   1,
		Name: "test-account",
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-key",
			"base_url": makeAllowlistedBaseURL(ts.URL),
		},
		Concurrency: 1,
		Platform:    PlatformZAI,
	}

	svc := &ZhipuGatewayService{
		cfg: cfg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := svc.TestConnection(ctx, account, "glm-4-flash")

	// 验证结果
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "deadline exceeded")
}

// TestZhipuGatewayService_TestConnection_NoAPIKey 测试账号缺少API Key
func TestZhipuGatewayService_TestConnection_NoAPIKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				AllowInsecureHTTP: true,
				AllowPrivateHosts: true,
			},
		},
	}

	account := &Account{
		ID:   1,
		Name: "test-account",
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": makeAllowlistedBaseURL(ts.URL),
			// 缺少api_key
		},
		Concurrency: 1,
		Platform:    PlatformZAI,
	}

	svc := &ZhipuGatewayService{
		cfg: cfg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := svc.TestConnection(ctx, account, "glm-4-flash")

	// 验证结果
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "api key not found")
}

// TestZhipuGatewayService_TestConnection_ParseFailed 测试响应解析失败场景
func TestZhipuGatewayService_TestConnection_ParseFailed(t *testing.T) {
	// 创建测试服务器（返回无效JSON）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"invalid":`))
	}))
	defer ts.Close()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				AllowInsecureHTTP: true,
				AllowPrivateHosts: true,
			},
		},
	}

	account := &Account{
		ID:   1,
		Name: "test-account",
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-key",
			"base_url": makeAllowlistedBaseURL(ts.URL),
		},
		Concurrency: 1,
		Platform:    PlatformZAI,
	}

	svc := &ZhipuGatewayService{
		cfg: cfg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := svc.TestConnection(ctx, account, "glm-4-flash")

	// 验证结果：即使解析失败，如果状态码是200也应该返回
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Text, "Connected (response parsing failed)")
}

// TestZhipuGatewayService_TestConnection_EmptyResponse 测试空响应场景
func TestZhipuGatewayService_TestConnection_EmptyResponse(t *testing.T) {
	// 创建测试服务器（返回空响应）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": []
		}`))
	}))
	defer ts.Close()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				AllowInsecureHTTP: true,
				AllowPrivateHosts: true,
			},
		},
	}

	account := &Account{
		ID:   1,
		Name: "test-account",
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-key",
			"base_url": makeAllowlistedBaseURL(ts.URL),
		},
		Concurrency: 1,
		Platform:    PlatformZAI,
	}

	svc := &ZhipuGatewayService{
		cfg: cfg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := svc.TestConnection(ctx, account, "glm-4-flash")

	// 验证结果
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "Connected (empty response)", result.Text)
}

func TestZhipuGatewayService_BuildTargetURL_ByRouteVariant(t *testing.T) {
	svc := &ZhipuGatewayService{}

	openAIURL := svc.buildTargetURL(
		WithZhipuRouteVariant(context.Background(), ZhipuRouteVariantOpenAI),
		"https://open.bigmodel.cn/api",
	)
	require.Equal(t, "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions", openAIURL)

	claudeURL := svc.buildTargetURL(
		WithZhipuRouteVariant(context.Background(), ZhipuRouteVariantClaude),
		"https://open.bigmodel.cn/api",
	)
	require.Equal(t, "https://open.bigmodel.cn/api/anthropic/v1/messages", claudeURL)
}

func TestZhipuGatewayService_BuildTargetURL_LegacyBaseURLCompatibility(t *testing.T) {
	svc := &ZhipuGatewayService{}

	openAILegacy := svc.buildTargetURL(
		WithZhipuRouteVariant(context.Background(), ZhipuRouteVariantOpenAI),
		"https://open.bigmodel.cn/api/coding/paas/v4",
	)
	require.Equal(t, "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions", openAILegacy)

	claudeLegacy := svc.buildTargetURL(
		WithZhipuRouteVariant(context.Background(), ZhipuRouteVariantClaude),
		"https://open.bigmodel.cn/api/anthropic",
	)
	require.Equal(t, "https://open.bigmodel.cn/api/anthropic/v1/messages", claudeLegacy)
}

func TestZhipuGatewayService_RoutePathToFinalTargetURL(t *testing.T) {
	svc := &ZhipuGatewayService{}
	baseURL := "https://open.bigmodel.cn/api"

	claudeVariant := ResolveZhipuRouteVariantByPath("/zhipu/claude/v1/messages", "/zhipu/claude/v1/messages")
	claudeURL := svc.buildTargetURL(WithZhipuRouteVariant(context.Background(), claudeVariant), baseURL)
	require.Equal(t, ZhipuRouteVariantClaude, claudeVariant)
	require.Equal(t, "https://open.bigmodel.cn/api/anthropic/v1/messages", claudeURL)

	openAIVariant := ResolveZhipuRouteVariantByPath("/zhipu/v1/chat/completions", "/zhipu/v1/chat/completions")
	openAIURL := svc.buildTargetURL(WithZhipuRouteVariant(context.Background(), openAIVariant), baseURL)
	require.Equal(t, ZhipuRouteVariantOpenAI, openAIVariant)
	require.Equal(t, "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions", openAIURL)
}
