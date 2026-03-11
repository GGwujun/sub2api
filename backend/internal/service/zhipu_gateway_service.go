package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const (
	// Zhipu API endpoint (默认配置，可由config覆盖)
	defaultZhipuBaseURL = "https://open.bigmodel.cn/api/coding/paas/v4"
)

// ZhipuAccountSwitchError 账号切换信号
// 当账号限流时间超过阈值时，通知上层切换账号
type ZhipuAccountSwitchError struct {
	OriginalAccountID int64
	RateLimitedModel  string
	WaitDuration      time.Duration
	IsStickySession   bool // 是否为粘性会话切换（决定是否缓存计费）
}

func (e *ZhipuAccountSwitchError) Error() string {
	return fmt.Sprintf("account %d model %s rate limited for %v, need switch",
		e.OriginalAccountID, e.RateLimitedModel, e.WaitDuration)
}

// IsZhipuAccountSwitchError 检查错误是否为账号切换信号
func IsZhipuAccountSwitchError(err error) (*ZhipuAccountSwitchError, bool) {
	var switchErr *ZhipuAccountSwitchError
	if !errors.As(err, &switchErr) || switchErr == nil {
		return nil, false
	}
	return switchErr, true
}

// ZhipuRetryMetricsSnapshot Zhipu重试指标快照
type ZhipuRetryMetricsSnapshot struct {
	RetryAttemptsTotal            int64 `json:"retry_attempts_total"`
	RetryBackoffMsTotal           int64 `json:"retry_backoff_ms_total"`
	RetryExhaustedTotal           int64 `json:"retry_exhausted_total"`
	NonRetryableFastFallbackTotal int64 `json:"non_retryable_fast_fallback_total"`
}

// zhipuRetryMetrics Zhipu重试指标
type zhipuRetryMetrics struct {
	retryAttempts            atomic.Int64
	retryBackoffMs           atomic.Int64
	retryExhausted           atomic.Int64
	nonRetryableFastFallback atomic.Int64
}

// recordRetryAttempt 记录一次重试尝试
func (m *zhipuRetryMetrics) recordRetryAttempt(backoff time.Duration) {
	m.retryAttempts.Add(1)
	if backoff > 0 {
		m.retryBackoffMs.Add(backoff.Milliseconds())
	}
}

// recordRetryExhausted 记录重试用尽
func (m *zhipuRetryMetrics) recordRetryExhausted() {
	m.retryExhausted.Add(1)
}

// recordNonRetryableFastFallback 记录非重试性错误快速降级
func (m *zhipuRetryMetrics) recordNonRetryableFastFallback() {
	m.nonRetryableFastFallback.Add(1)
}

// SnapshotZhipuRetryMetrics 获取重试指标快照
func (m *zhipuRetryMetrics) SnapshotRetryMetrics() ZhipuRetryMetricsSnapshot {
	return ZhipuRetryMetricsSnapshot{
		RetryAttemptsTotal:            m.retryAttempts.Load(),
		RetryBackoffMsTotal:           m.retryBackoffMs.Load(),
		RetryExhaustedTotal:           m.retryExhausted.Load(),
		NonRetryableFastFallbackTotal: m.nonRetryableFastFallback.Load(),
	}
}

// GetMetrics 返回Zhipu网关指标（供监控使用）
func (s *ZhipuGatewayService) GetMetrics() ZhipuRetryMetricsSnapshot {
	if s.retryMetrics != nil {
		return s.retryMetrics.SnapshotRetryMetrics()
	}
	return ZhipuRetryMetricsSnapshot{}
}

// isRetryableZhipuError 判断是否应该重试Zhipu错误
// 支持的HTTP状态码：429, 500, 502, 503, 504
func isRetryableZhipuError(statusCode int) bool {
	switch statusCode {
	case 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

// calculateZhipuRetryBackoff 计算重试退避时间（指数退避+Jitter）
func calculateZhipuRetryBackoff(attempt int, baseDelay, maxDelay time.Duration, jitterRatio float64) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// 指数退避：baseDelay * 2^(attempt-1)
	backoff := baseDelay * time.Duration(1<<(attempt-1))
	if backoff > maxDelay {
		backoff = maxDelay
	}

	// 添加Jitter：避免所有请求同时重试
	if jitterRatio > 0 && jitterRatio <= 1 {
		jitter := time.Duration(float64(backoff) * jitterRatio)
		if jitter > 0 {
			delta := time.Duration(rand.Int63n(int64(jitter)*2+1)) - jitter
			backoff = backoff + delta
		}
	}

	return backoff
}

// Zhipu模型定价表（单位：元/千tokens）
// 参考智谱官方定价：https://open.bigmodel.cn/dev/api#prices
// 更新时间：2026-03-11
var defaultZhipuPricing = map[string]*zhipuModelPricing{
	// GLM-4 系列
	"glm-4":          {InputPrice: 0.05, OutputPrice: 0.05},
	"glm-4v":         {InputPrice: 0.0125, OutputPrice: 0.0125},
	"glm-4-plus":     {InputPrice: 0.005, OutputPrice: 0.005},
	"glm-4-0520":     {InputPrice: 0.05, OutputPrice: 0.05},
	"glm-4-air":      {InputPrice: 0.005, OutputPrice: 0.005},
	"glm-4-airx":     {InputPrice: 0.005, OutputPrice: 0.005},
	"glm-4-long":     {InputPrice: 0.05, OutputPrice: 0.05},
	"glm-4-flash":    {InputPrice: 0.0001, OutputPrice: 0.0002},
	"glm-4v-plus":    {InputPrice: 0.0125, OutputPrice: 0.0125},
	"glm-4-alltools": {InputPrice: 0.05, OutputPrice: 0.05},

	// GLM-4.5 系列
	"glm-4.5":     {InputPrice: 0.05, OutputPrice: 0.05},
	"glm-4.5-air": {InputPrice: 0.0008, OutputPrice: 0.002}, // 默认0-32k 0-0.2k价格

	// GLM-4.6 系列
	"glm-4.6":  {InputPrice: 0.05, OutputPrice: 0.05},
	"glm-4.6v": {InputPrice: 0.001, OutputPrice: 0.003},

	// GLM-4.7 系列（按输出token数量分段计价）
	// 0-32k 0-0.2k: 输入¥0.002, 输出¥0.008
	// 0-32k 0.2k+: 输入¥0.003, 输出¥0.014
	// 32k-200k: 输入¥0.004, 输出¥0.016
	"glm-4.7": {InputPrice: 0.002, OutputPrice: 0.008}, // 默认0-32k 0-0.2k价格

	// GLM-5 系列（按输出token数量分段计价）
	// 0-32k: 输入¥0.004, 输出¥0.018
	// 32k+: 输入¥0.006, 输出¥0.022
	"glm-5": {InputPrice: 0.004, OutputPrice: 0.018}, // 默认0-32k价格

	// GLM-3 系列
	"glm-3-turbo":   {InputPrice: 0.004, OutputPrice: 0.004},
	"chatglm_turbo": {InputPrice: 0.004, OutputPrice: 0.004},
	"chatglm_pro":   {InputPrice: 0.005, OutputPrice: 0.005},
	"chatglm_std":   {InputPrice: 0.001, OutputPrice: 0.001},
	"chatglm_lite":  {InputPrice: 0.0005, OutputPrice: 0.0005},

	// Embedding
	"embedding-3": {InputPrice: 0.0005, OutputPrice: 0.0005},

	// 多模态
	"cogview-3": {InputPrice: 0.025, OutputPrice: 0.025},
	"cogvideo":  {InputPrice: 0.15, OutputPrice: 0.15},
}

// zhipuModelPricing Zhipu模型定价配置
type zhipuModelPricing struct {
	InputPrice  float64 // 输入价格（元/千tokens）
	OutputPrice float64 // 输出价格（元/千tokens）
}

// getZhipuModelPricing 获取模型定价（支持模糊匹配和分段计价）
// completionTokens: 输出token数量，用于glm-4.7和glm-5的分段计价
func getZhipuModelPricing(model string, completionTokens int) *zhipuModelPricing {
	// 精确匹配
	if pricing, ok := defaultZhipuPricing[model]; ok {
		return pricing
	}

	// GLM-4.7 分段计价（按输出token数量）
	if strings.HasPrefix(model, "glm-4.7") {
		return getGLM47Pricing(completionTokens)
	}

	// GLM-5 分段计价（按输出token数量）
	if strings.HasPrefix(model, "glm-5") {
		return getGLM5Pricing(completionTokens)
	}

	// 模糊匹配：处理模型版本后缀
	if strings.HasPrefix(model, "glm-4-") || strings.HasPrefix(model, "glm-4.") {
		// GLM-4 系列使用基础定价
		if pricing, ok := defaultZhipuPricing["glm-4"]; ok {
			return pricing
		}
	}

	// 默认定价（防止未知模型）
	return &zhipuModelPricing{
		InputPrice:  0.01,
		OutputPrice: 0.01,
	}
}

// getGLM47Pricing 获取glm-4.7定价（按输出token数量分段）
// 0-32k 0-0.2k: 输入¥0.002, 输出¥0.008
// 0-32k 0.2k+: 输入¥0.003, 输出¥0.014
// 32k-200k: 输入¥0.004, 输出¥0.016
func getGLM47Pricing(completionTokens int) *zhipuModelPricing {
	if completionTokens <= 0 {
		// 默认最低档位
		return &zhipuModelPricing{InputPrice: 0.002, OutputPrice: 0.008}
	}

	// 输出token > 200 且 <= 32000: 0-32k 0.2k+ 档位
	if completionTokens > 200 && completionTokens <= 32000 {
		return &zhipuModelPricing{InputPrice: 0.003, OutputPrice: 0.014}
	}

	// 输出token > 32000: 32k-200k 档位
	if completionTokens > 32000 {
		return &zhipuModelPricing{InputPrice: 0.004, OutputPrice: 0.016}
	}

	// 默认: 0-32k 0-0.2k 档位
	return &zhipuModelPricing{InputPrice: 0.002, OutputPrice: 0.008}
}

// getGLM5Pricing 获取glm-5定价（按输出token数量分段）
// 0-32k: 输入¥0.004, 输出¥0.018
// 32k+: 输入¥0.006, 输出¥0.022
func getGLM5Pricing(completionTokens int) *zhipuModelPricing {
	if completionTokens <= 0 {
		// 默认0-32k档位
		return &zhipuModelPricing{InputPrice: 0.004, OutputPrice: 0.018}
	}

	// 输出token > 32000: 32k+ 档位
	if completionTokens > 32000 {
		return &zhipuModelPricing{InputPrice: 0.006, OutputPrice: 0.022}
	}

	// 默认: 0-32k 档位
	return &zhipuModelPricing{InputPrice: 0.004, OutputPrice: 0.018}
}

// ZhipuGatewayService handles Zhipu API forwarding
type ZhipuGatewayService struct {
	cfg                 *config.Config
	accountRepo         AccountRepository
	usageRepo           UsageLogRepository
	userRepo            UserRepository
	userSubRepo         UserSubscriptionRepository
	cache               GatewayCache
	concurrencyService  *ConcurrencyService
	schedulerSnapshot   *SchedulerSnapshotService
	billingService      *BillingService
	billingCacheService *BillingCacheService
	deferredService     *DeferredService
	retryMetrics        *zhipuRetryMetrics
	httpClient          *http.Client // 连接池复用
}

// NewZhipuGatewayService creates a new Zhipu gateway service
func NewZhipuGatewayService(
	cfg *config.Config,
	accountRepo AccountRepository,
	usageRepo UsageLogRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	cache GatewayCache,
	concurrencyService *ConcurrencyService,
	schedulerSnapshot *SchedulerSnapshotService,
	billingService *BillingService,
	billingCacheService *BillingCacheService,
	deferredService *DeferredService,
) *ZhipuGatewayService {
	// 创建带连接池的HTTP客户端
	httpClient := &http.Client{
		Timeout: 300 * time.Second, // 默认5分钟超时
		Transport: &http.Transport{
			MaxIdleConns:        100,              // 最大空闲连接数
			MaxIdleConnsPerHost: 20,               // 每个主机最大空闲连接数
			IdleConnTimeout:     90 * time.Second, // 空闲连接超时
			TLSHandshakeTimeout: 10 * time.Second, // TLS握手超时
		},
	}

	return &ZhipuGatewayService{
		cfg:                 cfg,
		accountRepo:         accountRepo,
		usageRepo:           usageRepo,
		userRepo:            userRepo,
		userSubRepo:         userSubRepo,
		cache:               cache,
		concurrencyService:  concurrencyService,
		schedulerSnapshot:   schedulerSnapshot,
		billingService:      billingService,
		billingCacheService: billingCacheService,
		deferredService:     deferredService,
		retryMetrics:        &zhipuRetryMetrics{},
		httpClient:          httpClient,
	}
}

// billingDeps returns billing dependencies for postUsageBilling
func (s *ZhipuGatewayService) billingDeps() *billingDeps {
	return &billingDeps{
		accountRepo:         s.accountRepo,
		userRepo:            s.userRepo,
		userSubRepo:         s.userSubRepo,
		billingCacheService: s.billingCacheService,
		deferredService:     s.deferredService,
	}
}

// ZhipuForwardResult represents the result of forwarding a request to Zhipu
type ZhipuForwardResult struct {
	StatusCode   int
	Body         []byte
	Headers      http.Header
	RequestID    string
	FirstTokenMs *int
	DurationMs   *int
	Stream       bool
	Usage        *ZhipuUsage // Token usage information
}

// ZhipuUsage represents token usage from Zhipu API response
type ZhipuUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Forward forwards a request to Zhipu API with retry support
func (s *ZhipuGatewayService) Forward(ctx context.Context, account *Account, body []byte) (*ZhipuForwardResult, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}

	// 日志：开始转发请求
	logger.L().Debug("zhipu.forward_start",
		zap.Int64("account_id", account.ID),
		zap.String("account_name", account.Name),
	)

	// 获取重试配置
	maxRetries := s.getZhipuMaxRetries()
	baseDelay := s.getZhipuRetryBaseDelay()
	maxDelay := s.getZhipuRetryMaxDelay()
	jitterRatio := s.getZhipuRetryJitterRatio()

	// 重试循环
	var lastErr error
	for attempt := 1; attempt <= maxRetries+1; attempt++ {
		// 执行请求
		result, err := s.doForward(ctx, account, body, attempt)

		// 如果成功，直接返回
		if err == nil {
			return result, nil
		}

		lastErr = err

		// 检查错误是否可重试
		isRetryable := s.isRequestRetryable(err, result)
		if !isRetryable || attempt > maxRetries {
			// 不可重试或已到达最大重试次数
			if attempt > maxRetries {
				// 记录重试耗尽
				if s.retryMetrics != nil {
					s.retryMetrics.recordRetryExhausted()
				}
				logger.L().Warn("zhipu.retry_exhausted",
					zap.Int64("account_id", account.ID),
					zap.Int("attempts", attempt),
					zap.Error(err),
				)
			}
			return result, err
		}

		// 计算退避时间
		backoff := calculateZhipuRetryBackoff(attempt, baseDelay, maxDelay, jitterRatio)

		// 记录重试
		if s.retryMetrics != nil {
			s.retryMetrics.recordRetryAttempt(backoff)
		}

		logger.L().Warn("zhipu.retry_attempt",
			zap.Int64("account_id", account.ID),
			zap.Int("attempt", attempt),
			zap.Int("max_retries", maxRetries),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)

		// 等待后重试
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			// 继续下一次重试
		}
	}

	return nil, lastErr
}

// isRequestRetryable 判断请求错误是否可重试
func (s *ZhipuGatewayService) isRequestRetryable(err error, result *ZhipuForwardResult) bool {
	if err != nil {
		// 网络错误总是可重试
		return true
	}
	if result != nil && result.StatusCode > 0 {
		// 检查HTTP状态码
		return isRetryableZhipuError(result.StatusCode)
	}
	return false
}

// doForward 执行实际的转发请求（单次尝试）
func (s *ZhipuGatewayService) doForward(ctx context.Context, account *Account, body []byte, attempt int) (*ZhipuForwardResult, error) {
	// Get base URL from account or use default
	baseURL := s.getAccountBaseURL(account)

	// Build target URL (Zhipu uses /chat/completions)
	targetURL := strings.TrimSuffix(baseURL, "/") + "/chat/completions"

	// Validate URL
	allowInsecure := s.cfg != nil && s.cfg.Security.URLAllowlist.AllowInsecureHTTP
	validatedURL, err := urlvalidator.ValidateHTTPURL(targetURL, allowInsecure, urlvalidator.ValidationOptions{})
	if err != nil {
		return nil, fmt.Errorf("invalid zhipu base url: %w", err)
	}

	// Get API key from account
	apiKey := s.getAccountAPIKey(account)
	if apiKey == "" {
		return nil, errors.New("api key not found in account credentials")
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, validatedURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Copy relevant headers from original request if available
	if ginCtx, ok := ctx.Value("gin_context").(*gin.Context); ok {
		s.copyRequestHeaders(ginCtx.Request, req)
	}

	// Execute request using shared HTTP client with connection pooling
	startTime := time.Now()
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Calculate first token time (approximation)
	durationMs := int(time.Since(startTime).Milliseconds())
	firstTokenMs := durationMs

	// Extract usage from response (Zhipu API is OpenAI compatible)
	var usage *ZhipuUsage
	requestID := extractZhipuRequestID(respBody)
	if resp.StatusCode == http.StatusOK && len(respBody) > 0 {
		usage = extractZhipuUsage(respBody)
	}

	// 日志：转发完成
	logger.L().Debug("zhipu.forward_complete",
		zap.Int64("account_id", account.ID),
		zap.Int("status_code", resp.StatusCode),
		zap.Int("duration_ms", durationMs),
		zap.String("request_id", requestID),
	)

	return &ZhipuForwardResult{
		StatusCode:   resp.StatusCode,
		Body:         respBody,
		Headers:      resp.Header,
		RequestID:    requestID,
		FirstTokenMs: &firstTokenMs,
		DurationMs:   &durationMs,
		Stream:       false,
		Usage:        usage,
	}, nil
}

// extractZhipuUsage extracts usage information from Zhipu API response
func extractZhipuUsage(body []byte) *ZhipuUsage {
	usage := &ZhipuUsage{}

	if promptTokens := gjson.GetBytes(body, "usage.prompt_tokens").Int(); promptTokens > 0 {
		usage.PromptTokens = int(promptTokens)
	}
	if completionTokens := gjson.GetBytes(body, "usage.completion_tokens").Int(); completionTokens > 0 {
		usage.CompletionTokens = int(completionTokens)
	}
	if totalTokens := gjson.GetBytes(body, "usage.total_tokens").Int(); totalTokens > 0 {
		usage.TotalTokens = int(totalTokens)
	}

	// Return nil if no usage info found
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0 {
		return nil
	}

	return usage
}

func extractZhipuRequestID(body []byte) string {
	requestID := strings.TrimSpace(gjson.GetBytes(body, "request_id").String())
	if requestID != "" {
		return requestID
	}
	return strings.TrimSpace(gjson.GetBytes(body, "id").String())
}

// ForwardStream forwards a streaming request to Zhipu API with retry support
func (s *ZhipuGatewayService) ForwardStream(ctx context.Context, account *Account, body []byte, w http.ResponseWriter) (*ZhipuForwardResult, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}

	// 日志：开始流式转发
	logger.L().Debug("zhipu.forward_stream_start",
		zap.Int64("account_id", account.ID),
		zap.String("account_name", account.Name),
	)

	// 获取重试配置
	maxRetries := s.getZhipuMaxRetries()
	baseDelay := s.getZhipuRetryBaseDelay()
	maxDelay := s.getZhipuRetryMaxDelay()
	jitterRatio := s.getZhipuRetryJitterRatio()

	// 重试循环
	var lastErr error
	for attempt := 1; attempt <= maxRetries+1; attempt++ {
		// 执行请求
		result, err := s.doForwardStream(ctx, account, body, w, attempt)

		// 如果成功，直接返回
		if err == nil {
			return result, nil
		}

		lastErr = err

		// 检查是否是流式错误（不可重试）
		isStreamError := s.isStreamError(err, result)
		if isStreamError {
			// 流式错误不可重试，直接返回
			logger.L().Warn("zhipu.forward_stream_error_non_retryable",
				zap.Int64("account_id", account.ID),
				zap.Error(err),
			)
			return result, err
		}

		// 检查HTTP状态码是否可重试
		if result != nil && !isRetryableZhipuError(result.StatusCode) {
			// 不可重试的状态码
			return result, err
		}

		if attempt > maxRetries {
			// 记录重试耗尽
			if s.retryMetrics != nil {
				s.retryMetrics.recordRetryExhausted()
			}
			logger.L().Warn("zhipu.retry_exhausted_stream",
				zap.Int64("account_id", account.ID),
				zap.Int("attempts", attempt),
				zap.Error(err),
			)
			return result, err
		}

		// 计算退避时间
		backoff := calculateZhipuRetryBackoff(attempt, baseDelay, maxDelay, jitterRatio)

		// 记录重试
		if s.retryMetrics != nil {
			s.retryMetrics.recordRetryAttempt(backoff)
		}

		logger.L().Warn("zhipu.retry_attempt_stream",
			zap.Int64("account_id", account.ID),
			zap.Int("attempt", attempt),
			zap.Int("max_retries", maxRetries),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)

		// 等待后重试
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			// 继续下一次重试
		}
	}

	return nil, lastErr
}

// isStreamError 判断流式错误是否可重试
func (s *ZhipuGatewayService) isStreamError(err error, result *ZhipuForwardResult) bool {
	// 流式传输中如果已经开始写入，无法重试
	if err != nil && strings.Contains(err.Error(), "failed to write to response") {
		return true
	}
	return false
}

// doForwardStream 执行实际的流式转发请求（单次尝试）
func (s *ZhipuGatewayService) doForwardStream(ctx context.Context, account *Account, body []byte, w http.ResponseWriter, attempt int) (*ZhipuForwardResult, error) {
	// 日志：开始流式转发
	logger.L().Debug("zhipu.forward_stream_attempt",
		zap.Int64("account_id", account.ID),
		zap.String("account_name", account.Name),
		zap.Int("attempt", attempt),
	)

	// Get base URL from account or use default
	baseURL := s.getAccountBaseURL(account)

	// Build target URL
	targetURL := strings.TrimSuffix(baseURL, "/") + "/chat/completions"

	// Validate URL
	allowInsecure := s.cfg != nil && s.cfg.Security.URLAllowlist.AllowInsecureHTTP
	validatedURL, err := urlvalidator.ValidateHTTPURL(targetURL, allowInsecure, urlvalidator.ValidationOptions{})
	if err != nil {
		return nil, fmt.Errorf("invalid zhipu base url: %w", err)
	}

	// Get API key from account
	apiKey := s.getAccountAPIKey(account)
	if apiKey == "" {
		return nil, errors.New("api key not found in account credentials")
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, validatedURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	// Copy relevant headers from original request
	if ginCtx, ok := ctx.Value("gin_context").(*gin.Context); ok {
		s.copyRequestHeaders(ginCtx.Request, req)
	}

	// Execute request using shared HTTP client with connection pooling
	startTime := time.Now()
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	var firstTokenMs *int
	var durationMs *int

	// Set response headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(resp.StatusCode)

	// Stream response and collect usage
	var collectedUsage *ZhipuUsage
	requestID := ""
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := w.Write([]byte(line + "\n")); err != nil {
			return nil, fmt.Errorf("failed to write to response: %w", err)
		}

		// Try to extract usage from this line
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload != "" && payload != "[DONE]" {
				payloadBytes := []byte(payload)
				if firstTokenMs == nil {
					content := strings.TrimSpace(gjson.GetBytes(payloadBytes, "choices.0.delta.content").String())
					if content == "" {
						content = strings.TrimSpace(gjson.GetBytes(payloadBytes, "choices.0.message.content").String())
					}
					if content != "" {
						v := int(time.Since(startTime).Milliseconds())
						firstTokenMs = &v
					}
				}
				usage := extractZhipuUsage(payloadBytes)
				if usage != nil {
					collectedUsage = usage
				}
				if requestID == "" {
					requestID = extractZhipuRequestID(payloadBytes)
				}
			}
		}

		flusher, ok := w.(http.Flusher)
		if ok {
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	durationValue := int(time.Since(startTime).Milliseconds())
	durationMs = &durationValue

	return &ZhipuForwardResult{
		StatusCode:   resp.StatusCode,
		Headers:      resp.Header,
		RequestID:    requestID,
		FirstTokenMs: firstTokenMs,
		DurationMs:   durationMs,
		Stream:       true,
		Usage:        collectedUsage,
	}, nil
}

// SelectAccount selects a Zhipu account for the request
// 支持粘性会话和负载感知调度
func (s *ZhipuGatewayService) SelectAccount(ctx context.Context, groupID *int64, sessionHash string) (*Account, error) {
	gid := derefGroupID(groupID)

	// 日志记录账号选择开始
	logger.L().Info("zhipu.account_select_start",
		zap.Int64("group_id", gid),
		zap.String("session_hash", sessionHash),
	)

	// 1. 尝试获取粘性会话绑定的账号
	if sessionHash != "" && s.cache != nil {
		stickyAccountID, err := s.getStickySessionAccountID(ctx, gid, sessionHash)
		if err == nil && stickyAccountID > 0 {
			// 日志：发现粘性会话绑定
			logger.L().Debug("zhipu.sticky_session_found",
				zap.Int64("group_id", gid),
				zap.String("session_hash", sessionHash),
				zap.Int64("sticky_account_id", stickyAccountID),
			)

			// 验证绑定账号是否仍然可用
			if s.schedulerSnapshot != nil {
				accounts, _, err := s.schedulerSnapshot.ListSchedulableAccounts(ctx, groupID, PlatformZhipu, false)
				if err == nil {
					for i := range accounts {
						acc := &accounts[i]
						if acc.ID == stickyAccountID && acc.IsSchedulable() && acc.Platform == PlatformZhipu {
							// 刷新粘性会话TTL
							_ = s.refreshStickySessionTTL(ctx, gid, sessionHash)

							// 日志：粘性会话有效
							logger.L().Info("zhipu.sticky_session_valid",
								zap.Int64("account_id", stickyAccountID),
								zap.Int64("group_id", gid),
								zap.String("session_hash", sessionHash),
							)
							return acc, nil
						}
					}
				}
			}
			// 账号不可用，清除绑定
			_ = s.deleteStickySessionAccountID(ctx, gid, sessionHash)

			// 日志：粘性会话失效，清除
			logger.L().Info("zhipu.sticky_session_invalid_cleared",
				zap.Int64("sticky_account_id", stickyAccountID),
				zap.Int64("group_id", gid),
				zap.String("session_hash", sessionHash),
			)
		}
	}

	// 2. 查询可用账号
	var accounts []Account
	var err error
	if s.schedulerSnapshot != nil {
		accounts, _, err = s.schedulerSnapshot.ListSchedulableAccounts(ctx, groupID, PlatformZhipu, false)
	} else if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		accounts, err = s.accountRepo.ListSchedulableByPlatform(ctx, PlatformZhipu)
	} else if groupID != nil {
		accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformZhipu)
	} else {
		accounts, err = s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, PlatformZhipu)
	}

	if err != nil {
		logger.L().Error("zhipu.account_query_failed",
			zap.Error(err),
			zap.Int64("group_id", gid),
		)
		return nil, fmt.Errorf("failed to query zhipu accounts: %w", err)
	}

	if len(accounts) == 0 {
		logger.L().Warn("zhipu.no_available_accounts",
			zap.Int64("group_id", gid),
		)
		return nil, errors.New("no available zhipu accounts")
	}

	// 3. 负载感知调度：优先级 + LRU（最后使用时间）
	// 排序规则：优先级降序 -> LastUsedAt升序（LRU）
	schedulableAccounts := make([]*Account, 0, len(accounts))
	for i := range accounts {
		acc := &accounts[i]
		if acc.IsSchedulable() && acc.Platform == PlatformZhipu {
			schedulableAccounts = append(schedulableAccounts, acc)
		}
	}

	if len(schedulableAccounts) == 0 {
		logger.L().Warn("zhipu.no_schedulable_accounts",
			zap.Int64("group_id", gid),
			zap.Int("total_accounts", len(accounts)),
		)
		return nil, errors.New("no schedulable zhipu accounts")
	}

	// 选择账号：优先级最高且最久未使用的
	selectedAccount := schedulableAccounts[0]
	for _, acc := range schedulableAccounts[1:] {
		if acc.Priority > selectedAccount.Priority {
			selectedAccount = acc
		} else if acc.Priority == selectedAccount.Priority {
			// 优先级相同，选择最久未使用的（LRU）
			if selectedAccount.LastUsedAt == nil {
				selectedAccount = acc
			} else if acc.LastUsedAt != nil && acc.LastUsedAt.Before(*selectedAccount.LastUsedAt) {
				selectedAccount = acc
			}
		}
	}

	// 4. 设置粘性会话绑定
	if sessionHash != "" && s.cache != nil {
		err := s.setStickySessionAccountID(ctx, gid, sessionHash, selectedAccount.ID)
		if err != nil {
			logger.L().Warn("zhipu.sticky_session_set_failed",
				zap.Error(err),
				zap.Int64("account_id", selectedAccount.ID),
				zap.Int64("group_id", gid),
				zap.String("session_hash", sessionHash),
			)
		} else {
			// 日志：设置粘性会话绑定成功
			logger.L().Info("zhipu.sticky_session_set",
				zap.Int64("account_id", selectedAccount.ID),
				zap.Int64("group_id", gid),
				zap.String("session_hash", sessionHash),
				zap.Int("priority", selectedAccount.Priority),
			)
		}
	}

	logger.L().Info("zhipu.account_selected",
		zap.Int64("account_id", selectedAccount.ID),
		zap.Int64("group_id", gid),
		zap.Int("priority", selectedAccount.Priority),
		zap.Bool("has_sticky_session", sessionHash != ""),
	)

	return selectedAccount, nil
}

// getAccountBaseURL returns the base URL for the account
func (s *ZhipuGatewayService) getAccountBaseURL(account *Account) string {
	if account.Type == AccountTypeAPIKey {
		baseURL := strings.TrimSpace(account.GetCredential("base_url"))
		if baseURL != "" {
			return baseURL
		}
	}
	// 使用配置中的默认BaseURL
	if s.cfg != nil && s.cfg.Gateway.Zhipu.BaseURL != "" {
		return s.cfg.Gateway.Zhipu.BaseURL
	}
	return defaultZhipuBaseURL
}

// getAccountAPIKey returns the API key for the account
func (s *ZhipuGatewayService) getAccountAPIKey(account *Account) string {
	if account.Type == AccountTypeAPIKey {
		return strings.TrimSpace(account.GetCredential("api_key"))
	}
	return ""
}

// getTimeout returns the HTTP client timeout（从配置读取）
func (s *ZhipuGatewayService) getTimeout() time.Duration {
	return s.getZhipuRequestTimeout()
}

// copyRequestHeaders copies relevant headers from the original request
func (s *ZhipuGatewayService) copyRequestHeaders(original *http.Request, target *http.Request) {
	// Copy user-agent if present
	if ua := original.Header.Get("User-Agent"); ua != "" {
		target.Header.Set("User-Agent", ua)
	}

	// Copy X-Request-ID for tracing
	if reqID := original.Header.Get("X-Request-ID"); reqID != "" {
		target.Header.Set("X-Request-ID", reqID)
	}
}

// DefaultZhipuModels returns the default list of Zhipu models
func DefaultZhipuModels() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "glm-4", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-4v", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-4-plus", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-4-0520", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-4-air", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-4-airx", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-4-long", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-4-flash", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-4v-plus", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-4.5", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-4.6", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-3-turbo", "object": "model", "owned_by": "zhipu"},
		{"id": "glm-4-alltools", "object": "model", "owned_by": "zhipu"},
		{"id": "chatglm_turbo", "object": "model", "owned_by": "zhipu"},
		{"id": "chatglm_pro", "object": "model", "owned_by": "zhipu"},
		{"id": "chatglm_std", "object": "model", "owned_by": "zhipu"},
		{"id": "chatglm_lite", "object": "model", "owned_by": "zhipu"},
		{"id": "cogview-3", "object": "model", "owned_by": "zhipu"},
		{"id": "cogvideo", "object": "model", "owned_by": "zhipu"},
	}
}

// ListModels returns the list of available Zhipu models
func (s *ZhipuGatewayService) ListModels() []map[string]interface{} {
	return DefaultZhipuModels()
}

// LogRequest logs Zhipu gateway request details
func (s *ZhipuGatewayService) LogRequest(ctx context.Context, accountID int64, model string, stream bool) {
	logger.L().Info("zhipu.request",
		zap.Int64("account_id", accountID),
		zap.String("model", model),
		zap.Bool("stream", stream),
	)
}

// LogResponse logs Zhipu gateway response details
func (s *ZhipuGatewayService) LogResponse(ctx context.Context, accountID int64, statusCode int, durationMs int) {
	logger.L().Info("zhipu.response",
		zap.Int64("account_id", accountID),
		zap.Int("status_code", statusCode),
		zap.Int("duration_ms", durationMs),
	)
}

// ZhipuRecordUsageInput input for recording usage
type ZhipuRecordUsageInput struct {
	APIKey        *APIKey
	Account       *Account
	Model         string
	Usage         *ZhipuUsage
	RequestID     string
	Stream        bool
	DurationMs    *int
	FirstTokenMs  *int
	UserAgent     string
	IPAddress     string
	APIKeyService APIKeyQuotaUpdater
}

// RecordUsage records usage and deducts balance
func (s *ZhipuGatewayService) RecordUsage(ctx context.Context, input *ZhipuRecordUsageInput) error {
	if input == nil || input.Usage == nil {
		return nil
	}

	usage := input.Usage

	// Skip if no tokens
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return nil
	}

	apiKey := input.APIKey
	account := input.Account
	user := apiKey.User

	// Load user if not preloaded
	if user == nil {
		if s.userRepo == nil {
			return errors.New("zhipu user repository not initialized")
		}
		loadedUser, err := s.userRepo.GetByID(ctx, apiKey.UserID)
		if err != nil {
			return fmt.Errorf("load user for zhipu usage: %w", err)
		}
		user = loadedUser
	}

	// Calculate rate multiplier
	multiplier := s.cfg.Default.RateMultiplier
	if apiKey.Group != nil {
		multiplier = apiKey.Group.RateMultiplier
	}

	// Calculate cost based on Zhipu pricing
	// 单位：千tokens（1000），而非百万tokens
	// 注意：根据输出token数量进行分段计价（glm-4.7, glm-5）
	pricing := getZhipuModelPricing(input.Model, usage.CompletionTokens)
	inputCost := float64(usage.PromptTokens) / 1000 * pricing.InputPrice
	outputCost := float64(usage.CompletionTokens) / 1000 * pricing.OutputPrice
	totalCost := (inputCost + outputCost) * multiplier

	// 记录定价信息（便于调试）
	logger.L().Info("zhipu.cost_calculation",
		zap.String("model", input.Model),
		zap.Int("prompt_tokens", usage.PromptTokens),
		zap.Int("completion_tokens", usage.CompletionTokens),
		zap.Float64("input_price_per_1k", pricing.InputPrice),
		zap.Float64("output_price_per_1k", pricing.OutputPrice),
		zap.Float64("input_cost", inputCost),
		zap.Float64("output_cost", outputCost),
		zap.Float64("total_cost", totalCost),
		zap.Float64("multiplier", multiplier),
	)

	// Determine billing type (subscription vs balance)
	var subscription *UserSubscription
	isSubscriptionBilling := apiKey.GroupID != nil && apiKey.Group != nil && apiKey.Group.IsSubscriptionType()
	if isSubscriptionBilling && s.userSubRepo != nil {
		sub, subErr := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, user.ID, *apiKey.GroupID)
		if subErr != nil {
			logger.FromContext(ctx).Warn("zhipu.subscription_not_found",
				zap.Error(subErr),
				zap.Int64("user_id", user.ID),
				zap.Int64("group_id", *apiKey.GroupID),
			)
			isSubscriptionBilling = false
		} else {
			subscription = sub
		}
	}

	billingType := BillingTypeBalance
	if isSubscriptionBilling {
		billingType = BillingTypeSubscription
	}

	// Create usage log
	ua := input.UserAgent
	ip := input.IPAddress
	accountRateMultiplier := account.BillingRateMultiplier()
	usageLog := &UsageLog{
		UserID:                user.ID,
		APIKeyID:              apiKey.ID,
		AccountID:             account.ID,
		RequestID:             normalizeZhipuRequestID(input.RequestID, apiKey.ID),
		Model:                 input.Model,
		InputTokens:           usage.PromptTokens,
		OutputTokens:          usage.CompletionTokens,
		InputCost:             inputCost * multiplier,
		OutputCost:            outputCost * multiplier,
		TotalCost:             totalCost,
		ActualCost:            totalCost,
		RateMultiplier:        multiplier,
		AccountRateMultiplier: &accountRateMultiplier,
		BillingType:           billingType,
		RequestType:           RequestTypeSync,
		Stream:                input.Stream,
		DurationMs:            input.DurationMs,
		FirstTokenMs:          input.FirstTokenMs,
	}
	if input.Stream {
		usageLog.RequestType = RequestTypeStream
	}
	if usageLog.FirstTokenMs == nil && input.DurationMs != nil {
		fallback := *input.DurationMs
		usageLog.FirstTokenMs = &fallback
	}
	if apiKey.GroupID != nil {
		usageLog.GroupID = apiKey.GroupID
	}
	if subscription != nil {
		usageLog.SubscriptionID = &subscription.ID
	}
	if strings.TrimSpace(ua) != "" {
		usageLog.UserAgent = &ua
	}
	if strings.TrimSpace(ip) != "" {
		usageLog.IPAddress = &ip
	}

	// Save usage log using repository
	inserted := false
	if s.usageRepo != nil {
		var err error
		inserted, err = s.usageRepo.Create(ctx, usageLog)
		if err != nil {
			return fmt.Errorf("create usage log: %w", err)
		}
	} else {
		logger.L().Warn("zhipu.usage_repo_not_available")
	}

	// SIMPLE MODE: skip billing, only update last used
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		logger.LegacyPrintf("service.zhipu_gateway", "[SIMPLE MODE] Usage recorded (not billed): user=%d, tokens=%d", usageLog.UserID, usageLog.TotalTokens())
		if s.deferredService != nil {
			s.deferredService.ScheduleLastUsedUpdate(account.ID)
		}
		return nil
	}

	// PRODUCTION MODE: apply billing (balance deduction or subscription usage increment)
	if inserted {
		postUsageBilling(ctx, &postUsageBillingParams{
			Cost: &CostBreakdown{
				InputCost:     inputCost * multiplier,
				OutputCost:    outputCost * multiplier,
				CacheReadCost: 0,
				TotalCost:     totalCost,
				ActualCost:    totalCost,
			},
			User:                  user,
			APIKey:                apiKey,
			Account:               account,
			Subscription:          subscription,
			IsSubscriptionBill:    isSubscriptionBilling,
			AccountRateMultiplier: accountRateMultiplier,
			APIKeyService:         input.APIKeyService,
		}, s.billingDeps())
	} else if s.deferredService != nil {
		s.deferredService.ScheduleLastUsedUpdate(account.ID)
	}

	logger.L().Info("zhipu.usage_recorded",
		zap.Int64("api_key_id", apiKey.ID),
		zap.Int64("account_id", account.ID),
		zap.String("model", input.Model),
		zap.Int("prompt_tokens", usage.PromptTokens),
		zap.Int("completion_tokens", usage.CompletionTokens),
		zap.Float64("cost", totalCost),
		zap.Bool("subscription_billing", isSubscriptionBilling),
	)

	return nil
}

// ZhipuTestConnectionResult Zhipu连接测试结果
type ZhipuTestConnectionResult struct {
	Text        string // 响应文本
	MappedModel string // 实际使用的模型
}

// TestConnection 测试Zhipu账号连接
// 发送一个最小的测试请求验证账号可用性（非流式、无重试、无计费）
func (s *ZhipuGatewayService) TestConnection(ctx context.Context, account *Account, modelID string) (*ZhipuTestConnectionResult, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}

	// 默认使用glm-4-flash（低成本模型）
	if modelID == "" {
		modelID = "glm-4-flash"
	}

	// 获取base URL
	baseURL := s.getAccountBaseURL(account)

	// 构建测试请求URL
	targetURL := strings.TrimSuffix(baseURL, "/") + "/chat/completions"

	// 验证URL
	allowInsecure := s.cfg != nil && s.cfg.Security.URLAllowlist.AllowInsecureHTTP
	validatedURL, err := urlvalidator.ValidateHTTPURL(targetURL, allowInsecure, urlvalidator.ValidationOptions{})
	if err != nil {
		return nil, fmt.Errorf("invalid zhipu base url: %w", err)
	}

	// 获取API Key
	apiKey := s.getAccountAPIKey(account)
	if apiKey == "" {
		return nil, errors.New("api key not found in account credentials")
	}

	// 构建测试请求体
	testRequest := map[string]interface{}{
		"model": modelID,
		"messages": []map[string]string{
			{"role": "user", "content": "Hi"},
		},
		"max_tokens": 1, // 最小化成本
	}

	requestBody, err := json.Marshal(testRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal test request: %w", err)
	}

	// 创建HTTP请求（设置较短超时）
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, validatedURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create test request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// 发送请求（使用短超时，不使用代理）
	client := &http.Client{
		Timeout: 30 * time.Second, // 测试请求使用30秒超时
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("test request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read test response: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		errorMsg := string(respBody)
		return nil, fmt.Errorf("test request failed with status %d: %s", resp.StatusCode, errorMsg)
	}

	// 解析响应
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		// 即使解析失败，如果状态码是200也认为连接成功
		logger.L().Info("zhipu.test_connection_parse_failed",
			zap.Error(err),
			zap.Int("status_code", resp.StatusCode),
		)
		return &ZhipuTestConnectionResult{
			Text:        "Connected (response parsing failed)",
			MappedModel: modelID,
		}, nil
	}

	// 提取响应内容
	var responseText string
	if len(result.Choices) > 0 {
		responseText = result.Choices[0].Message.Content
	} else {
		responseText = "Connected (empty response)"
	}

	return &ZhipuTestConnectionResult{
		Text:        responseText,
		MappedModel: modelID,
	}, nil
}

func normalizeZhipuRequestID(requestID string, apiKeyID int64) string {
	trimmed := strings.TrimSpace(requestID)
	if trimmed == "" {
		trimmed = fmt.Sprintf("zhipu-%d-%d", apiKeyID, time.Now().UnixNano())
	}
	if len(trimmed) > 64 {
		return trimmed[:64]
	}
	return trimmed
}

// getZhipuStickySessionTTL 获取粘性会话TTL（从配置读取）
func (s *ZhipuGatewayService) getZhipuStickySessionTTL() time.Duration {
	if s.cfg != nil && s.cfg.Gateway.Zhipu.StickySessionTTLSeconds > 0 {
		return time.Duration(s.cfg.Gateway.Zhipu.StickySessionTTLSeconds) * time.Second
	}
	return time.Hour // 默认1小时
}

// getZhipuMaxRetries 获取最大重试次数（从配置读取）
func (s *ZhipuGatewayService) getZhipuMaxRetries() int {
	if s.cfg != nil && s.cfg.Gateway.Zhipu.MaxRetries > 0 {
		return s.cfg.Gateway.Zhipu.MaxRetries
	}
	return 3 // 默认3次
}

// getZhipuRetryBaseDelay 获取重试基础延迟（从配置读取）
func (s *ZhipuGatewayService) getZhipuRetryBaseDelay() time.Duration {
	if s.cfg != nil && s.cfg.Gateway.Zhipu.RetryBaseDelaySeconds > 0 {
		return time.Duration(s.cfg.Gateway.Zhipu.RetryBaseDelaySeconds) * time.Second
	}
	return 1 * time.Second // 默认1秒
}

// getZhipuRetryMaxDelay 获取重试最大延迟（从配置读取）
func (s *ZhipuGatewayService) getZhipuRetryMaxDelay() time.Duration {
	if s.cfg != nil && s.cfg.Gateway.Zhipu.RetryMaxDelaySeconds > 0 {
		return time.Duration(s.cfg.Gateway.Zhipu.RetryMaxDelaySeconds) * time.Second
	}
	return 16 * time.Second // 默认16秒
}

// getZhipuRetryJitterRatio 获取重试抖动比例（从配置读取）
func (s *ZhipuGatewayService) getZhipuRetryJitterRatio() float64 {
	if s.cfg != nil && s.cfg.Gateway.Zhipu.RetryJitterRatio >= 0 && s.cfg.Gateway.Zhipu.RetryJitterRatio <= 1 {
		return s.cfg.Gateway.Zhipu.RetryJitterRatio
	}
	return 0.2 // 默认0.2
}

// getZhipuRequestTimeout 获取请求超时时间（从配置读取）
func (s *ZhipuGatewayService) getZhipuRequestTimeout() time.Duration {
	if s.cfg != nil && s.cfg.Gateway.Zhipu.RequestTimeoutSeconds > 0 {
		return time.Duration(s.cfg.Gateway.Zhipu.RequestTimeoutSeconds) * time.Second
	}
	return 300 * time.Second // 默认5分钟
}

// getStickySessionAccountID 获取粘性会话绑定的账号ID
func (s *ZhipuGatewayService) getStickySessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	if s.cache == nil || sessionHash == "" {
		return 0, nil
	}
	return s.cache.GetSessionAccountID(ctx, groupID, sessionHash)
}

// setStickySessionAccountID 设置粘性会话与账号的绑定关系
func (s *ZhipuGatewayService) setStickySessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64) error {
	if s.cache == nil || sessionHash == "" {
		return nil
	}
	return s.cache.SetSessionAccountID(ctx, groupID, sessionHash, accountID, s.getZhipuStickySessionTTL())
}

// refreshStickySessionTTL 刷新粘性会话的过期时间
func (s *ZhipuGatewayService) refreshStickySessionTTL(ctx context.Context, groupID int64, sessionHash string) error {
	if s.cache == nil || sessionHash == "" {
		return nil
	}
	return s.cache.RefreshSessionTTL(ctx, groupID, sessionHash, s.getZhipuStickySessionTTL())
}

// DeleteStickySessionAccountID 删除粘性会话绑定（导出方法，供handler调用）
func (s *ZhipuGatewayService) DeleteStickySessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	return s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
}

// deleteStickySessionAccountID 删除粘性会话绑定
func (s *ZhipuGatewayService) deleteStickySessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	if s.cache == nil || sessionHash == "" {
		return nil
	}
	return s.cache.DeleteSessionAccountID(ctx, groupID, sessionHash)
}

// ZhipuHealthCheckResult 健康检查结果
type ZhipuHealthCheckResult struct {
	Status       string                    `json:"status"`        // healthy/unhealthy
	AccountCount int                       `json:"account_count"` // 可用账号数量
	HealthyCount int                       `json:"healthy_count"` // 健康账号数量
	RetryMetrics ZhipuRetryMetricsSnapshot `json:"retry_metrics"` // 重试指标
	BaseURL      string                    `json:"base_url"`      // 配置的Base URL
}

// HealthCheck 执行Zhipu网关健康检查
// 检查可用账号数量、重试指标等
func (s *ZhipuGatewayService) HealthCheck(ctx context.Context) (*ZhipuHealthCheckResult, error) {
	result := &ZhipuHealthCheckResult{
		Status:  "unhealthy",
		BaseURL: defaultZhipuBaseURL,
	}

	// 获取配置的Base URL
	if s.cfg != nil && s.cfg.Gateway.Zhipu.BaseURL != "" {
		result.BaseURL = s.cfg.Gateway.Zhipu.BaseURL
	}

	// 获取账号数量
	var accounts []Account
	var err error

	if s.schedulerSnapshot != nil {
		accounts, _, err = s.schedulerSnapshot.ListSchedulableAccounts(ctx, nil, PlatformZhipu, false)
	} else if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		accounts, err = s.accountRepo.ListSchedulableByPlatform(ctx, PlatformZhipu)
	} else {
		accounts, err = s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, PlatformZhipu)
	}

	if err != nil {
		logger.L().Warn("zhipu.health_check_failed",
			zap.Error(err),
		)
		return result, fmt.Errorf("failed to query accounts: %w", err)
	}

	result.AccountCount = len(accounts)

	// 统计健康账号数量（状态为Active）
	for _, acc := range accounts {
		if acc.Status == StatusActive && acc.IsSchedulable() {
			result.HealthyCount++
		}
	}

	// 获取重试指标
	result.RetryMetrics = s.GetMetrics()

	// 判断健康状态
	if result.AccountCount > 0 && result.HealthyCount > 0 {
		result.Status = "healthy"
	}

	logger.L().Debug("zhipu.health_check_complete",
		zap.String("status", result.Status),
		zap.Int("account_count", result.AccountCount),
		zap.Int("healthy_count", result.HealthyCount),
	)

	return result, nil
}
