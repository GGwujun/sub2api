package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// MiniMaxGatewayHandler handles MiniMax Coding Plan chat completions requests
type MiniMaxGatewayHandler struct {
	minimaxGatewayService   *service.MiniMaxGatewayService
	billingCacheService     *service.BillingCacheService
	apiKeyService           *service.APIKeyService
	usageRecordWorkerPool   *service.UsageRecordWorkerPool
	gatewayService          *service.GatewayService
	errorPassthroughService *service.ErrorPassthroughService
	maxAccountSwitches      int
}

// NewMiniMaxGatewayHandler creates a new MiniMaxGatewayHandler
func NewMiniMaxGatewayHandler(
	minimaxGatewayService *service.MiniMaxGatewayService,
	billingCacheService *service.BillingCacheService,
	apiKeyService *service.APIKeyService,
	usageRecordWorkerPool *service.UsageRecordWorkerPool,
	gatewayService *service.GatewayService,
	errorPassthroughService *service.ErrorPassthroughService,
	cfg *config.Config,
) *MiniMaxGatewayHandler {
	maxAccountSwitches := 3
	if cfg != nil && cfg.Gateway.MaxAccountSwitches > 0 {
		maxAccountSwitches = cfg.Gateway.MaxAccountSwitches
	}

	return &MiniMaxGatewayHandler{
		minimaxGatewayService:   minimaxGatewayService,
		billingCacheService:     billingCacheService,
		apiKeyService:           apiKeyService,
		usageRecordWorkerPool:   usageRecordWorkerPool,
		gatewayService:          gatewayService,
		errorPassthroughService: errorPassthroughService,
		maxAccountSwitches:      maxAccountSwitches,
	}
}

// ChatCompletions handles MiniMax /v1/chat/completions endpoint
func (h *MiniMaxGatewayHandler) ChatCompletions(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}

	// 获取订阅信息
	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	ctx := c.Request.Context()
	logger.FromContext(ctx).Info("minimax.chat_completions",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	// Check billing eligibility
	if err := h.billingCacheService.CheckBillingEligibility(ctx, apiKey.User, apiKey, apiKey.Group, subscription); err != nil {
		logger.FromContext(ctx).Warn("minimax.billing_eligibility_check_failed", zap.Error(err))
		h.handleBillingError(c, err)
		return
	}

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	defer c.Request.Body.Close()

	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	// Parse model and stream flag
	model := gjson.GetBytes(body, "model").String()
	stream := gjson.GetBytes(body, "stream").Bool()

	if model == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}

	logger.FromContext(ctx).Info("minimax.request_parsed",
		zap.String("model", model),
		zap.Bool("stream", stream),
	)

	// Parse request for session hash generation
	sessionHash := ""
	if h.gatewayService != nil {
		parsedRequest, err := service.ParseGatewayRequest(body, service.PlatformMiniMaxCode)
		if err == nil {
			parsedRequest.SessionContext = &service.SessionContext{
				ClientIP:  getClientIP(c),
				UserAgent: c.GetHeader("User-Agent"),
				APIKeyID:  apiKey.ID,
			}
			sessionHash = h.gatewayService.GenerateSessionHash(parsedRequest)
		}
	}

	// Select account and forward request
	maxSwitches := h.maxAccountSwitches
	failedAccountIDs := make(map[int64]struct{})

	for i := 0; i <= maxSwitches; i++ {
		// Select MiniMax account
		account, err := h.minimaxGatewayService.SelectAccount(ctx, apiKey.GroupID, sessionHash)
		if err != nil {
			logger.FromContext(ctx).Warn("minimax.account_select_failed", zap.Error(err), zap.Int("attempt", i))
			if len(failedAccountIDs) == 0 {
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable")
				return
			}
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "All accounts failed")
			return
		}

		// Skip if account was already tried
		if _, failed := failedAccountIDs[account.ID]; failed {
			continue
		}

		// ✅ 新增：添加 stream_options.include_usage=true 以获取完整 usage
		if stream {
			// 解析请求 body 为 JSON 对象
			var reqBody map[string]any
			if parseErr := json.Unmarshal(body, &reqBody); parseErr == nil {
				// 添加 stream_options
				reqBody["stream_options"] = map[string]any{
					"include_usage": true,
				}
				// 重新序列化
				body, _ = json.Marshal(reqBody)
			}
		}

		// Forward request
		var result *service.MiniMaxForwardResult
		if stream {
			result, err = h.forwardStream(c, account, body)
		} else {
			result, err = h.forwardNonStream(c, account, body)
		}

		if err == nil {
			// Success - record usage if available
			if result != nil && result.Usage != nil && result.StatusCode == http.StatusOK {
				h.recordUsage(c, apiKey, account, model, result, subscription)
			}
			return
		}

		// Check if error is retryable
		if h.isRetryableError(err) {
			failedAccountIDs[account.ID] = struct{}{}
			// Clear sticky session binding for failed account (if sticky session was enabled)
			if sessionHash != "" && i == 0 {
				// First attempt failed, clear sticky session to allow account switching
				groupID := int64(0)
				if apiKey.GroupID != nil {
					groupID = *apiKey.GroupID
				}
				_ = h.minimaxGatewayService.DeleteStickySessionAccountID(ctx, groupID, sessionHash)
			}
			logger.FromContext(ctx).Warn("minimax.forward_failed_retryable",
				zap.Error(err),
				zap.Int64("account_id", account.ID),
				zap.Int("switch_count", i+1),
				zap.String("session_hash", sessionHash),
			)
			continue
		}

		// Non-retryable error
		// 应用错误透传规则
		if h.errorPassthroughService != nil && result != nil {
			status, code, message := h.applyErrorPassthrough(
				c,
				result.StatusCode,
				result.Body,
				http.StatusInternalServerError,
				"api_error",
				err.Error(),
			)
			h.errorResponse(c, status, code, message)
		} else {
			logger.FromContext(ctx).Error("minimax.forward_failed",
				zap.Error(err),
				zap.Int64("account_id", account.ID),
			)
			h.errorResponse(c, http.StatusInternalServerError, "api_error", "Service error")
		}
		return
	}

	// All retries exhausted
	h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "All accounts failed after retries")
}

// forwardNonStream forwards a non-streaming request
func (h *MiniMaxGatewayHandler) forwardNonStream(c *gin.Context, account *service.Account, body []byte) (*service.MiniMaxForwardResult, error) {
	result, err := h.minimaxGatewayService.Forward(c.Request.Context(), account, body)
	if err != nil {
		return nil, err
	}

	duration := int64(0)
	if result.DurationMs != nil {
		duration = int64(*result.DurationMs)
	}

	// Log response
	h.minimaxGatewayService.LogResponse(c.Request.Context(), account.ID, result.StatusCode, int(duration))

	// Set response headers
	for key, values := range result.Headers {
		if len(values) > 0 && key != "Content-Length" {
			c.Header(key, values[0])
		}
	}

	c.Status(result.StatusCode)
	c.Writer.Write(result.Body)

	return result, nil
}

// forwardStream forwards a streaming request
func (h *MiniMaxGatewayHandler) forwardStream(c *gin.Context, account *service.Account, body []byte) (*service.MiniMaxForwardResult, error) {
	result, err := h.minimaxGatewayService.ForwardStream(c.Request.Context(), account, body, c.Writer)
	if err != nil {
		return nil, err
	}

	duration := int64(0)
	if result.DurationMs != nil {
		duration = int64(*result.DurationMs)
	}
	logger.FromContext(c.Request.Context()).Info("minimax.stream_completed",
		zap.Int64("account_id", account.ID),
		zap.Int64("duration_ms", duration),
	)

	return result, nil
}

// Models returns the list of available MiniMax models
func (h *MiniMaxGatewayHandler) Models(c *gin.Context) {
	models := h.minimaxGatewayService.ListModels()

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   models,
	})
}

// isRetryableError checks if an error should trigger a retry with another account
func (h *MiniMaxGatewayHandler) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check for rate limit errors
	if strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") {
		return true
	}

	// Check for timeout errors
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") {
		return true
	}

	// Check for connection errors
	if strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "no available") {
		return true
	}

	return false
}

// handleBillingError handles billing eligibility errors
func (h *MiniMaxGatewayHandler) handleBillingError(c *gin.Context, err error) {
	status := http.StatusForbidden
	code := "billing_error"
	message := "计费检查失败"

	if err != nil {
		errStr := err.Error()
		message = errStr
		switch {
		case strings.Contains(strings.ToLower(errStr), "token") && strings.Contains(strings.ToLower(errStr), "quota"):
			code = "token_quota_exceeded"
			message = "令牌额度已用完，请联系管理员续费"
			status = http.StatusTooManyRequests
		case strings.Contains(errStr, "insufficient"):
			code = "insufficient_balance"
			message = "余额不足"
			status = http.StatusPaymentRequired
		case strings.Contains(errStr, "quota"):
			code = "quota_exceeded"
			message = "额度已用完，请联系管理员续费"
			status = http.StatusTooManyRequests
		case strings.Contains(errStr, "concurrent"):
			code = "concurrency_limit"
			message = "并发限制已达上限"
			status = http.StatusTooManyRequests
		}
	}

	h.errorResponse(c, status, code, message)
}

// errorResponse sends an error response
func (h *MiniMaxGatewayHandler) errorResponse(c *gin.Context, status int, code, message string) {
	response.Error(c, status, message)
}

// applyErrorPassthrough 应用错误透传规则到错误响应
func (h *MiniMaxGatewayHandler) applyErrorPassthrough(
	c *gin.Context,
	upstreamStatus int,
	responseBody []byte,
	defaultStatus int,
	defaultCode string,
	defaultMessage string,
) (status int, code string, message string) {
	// 如果没有错误透传服务，直接返回默认值
	if h.errorPassthroughService == nil {
		return defaultStatus, defaultCode, defaultMessage
	}

	// 使用错误透传规则
	status = defaultStatus
	code = defaultCode
	message = defaultMessage

	rule := h.errorPassthroughService.MatchRule(service.PlatformMiniMaxCode, upstreamStatus, responseBody)
	if rule == nil {
		return defaultStatus, defaultCode, defaultMessage
	}

	// 应用规则中的状态码
	status = upstreamStatus
	if !rule.PassthroughCode && rule.ResponseCode != nil {
		status = *rule.ResponseCode
	}

	// 应用规则中的错误消息
	message = service.ExtractUpstreamErrorMessage(responseBody)
	if !rule.PassthroughBody && rule.CustomMessage != nil {
		message = *rule.CustomMessage
	}

	// 命中skip_monitoring时在context中标记，供ops_error_logger跳过记录
	if rule.SkipMonitoring {
		c.Set(service.OpsSkipPassthroughKey, true)
	}

	// 命中规则时统一返回upstream_error
	code = "upstream_error"
	return status, code, message
}

// submitUsageRecordTask submits a usage recording task to the worker pool
func (h *MiniMaxGatewayHandler) submitUsageRecordTask(task service.UsageRecordTask) {
	if task == nil {
		return
	}
	if h.usageRecordWorkerPool != nil {
		h.usageRecordWorkerPool.Submit(task)
		return
	}
	// Fallback: execute synchronously to avoid unbounded goroutines
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().With(
				zap.String("component", "handler.minimax_gateway"),
			).Error("minimax.usage_record_task_panic_recovered")
		}
	}()
	task(ctx)
}

// recordUsage records the token usage for a successful request
func (h *MiniMaxGatewayHandler) recordUsage(c *gin.Context, apiKey *service.APIKey, account *service.Account, model string, result *service.MiniMaxForwardResult, subscription *service.UserSubscription) {
	if result.Usage == nil {
		logger.FromContext(c.Request.Context()).Warn("minimax.record_usage_nil",
			zap.Int64("api_key_id", apiKey.ID),
			zap.Int64("account_id", account.ID),
			zap.String("model", model),
		)
		return
	}

	logger.FromContext(c.Request.Context()).Info("minimax.record_usage_start",
		zap.Int64("api_key_id", apiKey.ID),
		zap.Int64("account_id", account.ID),
		zap.String("model", model),
		zap.Int("prompt_tokens", result.Usage.PromptTokens),
		zap.Int("completion_tokens", result.Usage.CompletionTokens),
	)

	// Capture required info before submitting async task
	userAgent := c.GetHeader("User-Agent")
	clientIP := getClientIP(c)

	h.submitUsageRecordTask(func(ctx context.Context) {
		// Delegate to gateway service for usage recording
		if err := h.minimaxGatewayService.RecordUsage(ctx, &service.MiniMaxRecordUsageInput{
			APIKey:        apiKey,
			Subscription:  subscription,
			Account:       account,
			Model:         model,
			Usage:         result.Usage,
			RequestID:     result.RequestID,
			Stream:        result.Stream,
			DurationMs:    result.DurationMs,
			FirstTokenMs:  result.FirstTokenMs,
			UserAgent:     userAgent,
			IPAddress:     clientIP,
			APIKeyService: h.apiKeyService,
		}); err != nil {
			logger.FromContext(ctx).Error("minimax.record_usage_failed",
				zap.Error(err),
				zap.Int64("api_key_id", apiKey.ID),
				zap.Int64("account_id", account.ID),
			)
		}
	})
}
