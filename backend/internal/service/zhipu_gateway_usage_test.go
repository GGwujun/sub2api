//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestExtractZhipuUsage_OpenAICompatible(t *testing.T) {
	usage := extractZhipuUsage([]byte(`{"usage":{"prompt_tokens":12,"completion_tokens":7,"total_tokens":19}}`))

	require.NotNil(t, usage)
	require.Equal(t, 12, usage.PromptTokens)
	require.Equal(t, 7, usage.CompletionTokens)
	require.Equal(t, 19, usage.TotalTokens)
}

func TestExtractZhipuUsage_AnthropicCompatible(t *testing.T) {
	usage := extractZhipuUsage([]byte(`{"id":"msg_123","usage":{"input_tokens":15,"output_tokens":8}}`))

	require.NotNil(t, usage)
	require.Equal(t, 15, usage.PromptTokens)
	require.Equal(t, 8, usage.CompletionTokens)
	require.Equal(t, 23, usage.TotalTokens)
}

func TestExtractZhipuRequestID_AnthropicMessageStart(t *testing.T) {
	requestID := extractZhipuRequestID([]byte(`{"type":"message_start","message":{"id":"msg_abc","usage":{"input_tokens":3}}}`))

	require.Equal(t, "msg_abc", requestID)
}

func TestZhipuGatewayServiceForward_ExtractsAnthropicUsage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/anthropic/v1/messages", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_456","type":"message","usage":{"input_tokens":20,"output_tokens":5}}`))
	}))
	defer ts.Close()

	svc := &ZhipuGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					AllowInsecureHTTP: true,
					AllowPrivateHosts: true,
				},
			},
		},
		httpClient: ts.Client(),
	}

	account := &Account{
		ID:   1,
		Name: "test-zhipu",
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-key",
			"base_url": makeAllowlistedBaseURL(ts.URL),
		},
		Platform: PlatformZAI,
	}

	result, err := svc.Forward(WithZhipuRouteVariant(context.Background(), ZhipuRouteVariantClaude), account, []byte(`{"stream":false}`))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "msg_456", result.RequestID)
	require.NotNil(t, result.Usage)
	require.Equal(t, 20, result.Usage.PromptTokens)
	require.Equal(t, 5, result.Usage.CompletionTokens)
	require.Equal(t, 25, result.Usage.TotalTokens)
}

func TestZhipuGatewayServiceForwardStream_CollectsAnthropicUsageAcrossEvents(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/anthropic/v1/messages", r.URL.Path)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_stream\",\"usage\":{\"input_tokens\":12}}}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hello\"}}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":7}}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer ts.Close()

	svc := &ZhipuGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					AllowInsecureHTTP: true,
					AllowPrivateHosts: true,
				},
			},
		},
		httpClient: ts.Client(),
	}

	account := &Account{
		ID:   1,
		Name: "test-zhipu",
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-key",
			"base_url": makeAllowlistedBaseURL(ts.URL),
		},
		Platform: PlatformZAI,
	}

	recorder := httptest.NewRecorder()
	result, err := svc.ForwardStream(WithZhipuRouteVariant(context.Background(), ZhipuRouteVariantClaude), account, []byte(`{"stream":true}`), recorder)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, result.StatusCode)
	require.Equal(t, "msg_stream", result.RequestID)
	require.NotNil(t, result.Usage)
	require.Equal(t, 12, result.Usage.PromptTokens)
	require.Equal(t, 7, result.Usage.CompletionTokens)
	require.Equal(t, 19, result.Usage.TotalTokens)
}
