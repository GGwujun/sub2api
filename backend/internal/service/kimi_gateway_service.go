package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const (
	// Kimi API endpoint (默认配置，可由config覆盖)
	// Kimi Coding Plan 使用 Anthropic 兼容端点
	defaultKimiBaseURL = "https://api.kimi.com/coding/v1"
)

// Kimi模型定价表（单位：元/千tokens）
// 参考 Kimi 官方定价
// 更新时间：2026-03-11
var defaultKimiPricing = map[string]*kimiModelPricing{
	// K2.5 系列
	"k2p5":             {InputPrice: 0.005, OutputPrice: 0.015},
	"kimi-k2-thinking": {InputPrice: 0.005, OutputPrice: 0.015},
}

// kimiModelPricing Kimi模型定价配置
type kimiModelPricing struct {
	InputPrice  float64 // 输入价格（元/千tokens）
	OutputPrice float64 // 输出价格（元/千tokens）
}

// getKimiModelPricing 获取模型定价（支持模糊匹配）
func getKimiModelPricing(model string) *kimiModelPricing {
	// 精确匹配
	if pricing, ok := defaultKimiPricing[model]; ok {
		return pricing
	}

	// 前缀模糊匹配 - 使用最长前缀匹配
	var longestMatch *kimiModelPricing
	longestLen := 0
	for prefix, pricing := range defaultKimiPricing {
		if strings.HasPrefix(model, prefix) && len(prefix) > longestLen {
			longestMatch = pricing
			longestLen = len(prefix)
		}
	}
	if longestMatch != nil {
		return longestMatch
	}

	// 默认使用 k2p5 定价
	return defaultKimiPricing["k2p5"]
}

// KimiUsage Kimi API使用量
type KimiUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// KimiForwardResult Kimi请求转发结果
type KimiForwardResult struct {
	StatusCode   int
	Headers      http.Header
	Body         []byte
	Usage        *KimiUsage
	RequestID    string
	Stream       bool
	DurationMs   *int64
	FirstTokenMs *int64
}

// KimiRecordUsageInput Kimi使用量记录输入
type KimiRecordUsageInput struct {
	APIKey        *APIKey
	Account       *Account
	Model         string
	Usage         *KimiUsage
	RequestID     string
	Stream        bool
	DurationMs    *int64
	FirstTokenMs  *int64
	UserAgent     string
	IPAddress     string
	APIKeyService *APIKeyService
}

// KimiGatewayService Kimi网关服务
type KimiGatewayService struct {
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
	httpClient          *http.Client
}

// NewKimiGatewayService creates a new KimiGatewayService
func NewKimiGatewayService(
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
) *KimiGatewayService {
	return &KimiGatewayService{
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
		httpClient: &http.Client{
			Timeout: time.Duration(getKimiRequestTimeout(cfg)) * time.Second,
		},
	}
}

// getKimiRequestTimeout 获取Kimi请求超时时间
func getKimiRequestTimeout(cfg *config.Config) int {
	if cfg != nil && cfg.Gateway.Kimi.RequestTimeoutSeconds > 0 {
		return cfg.Gateway.Kimi.RequestTimeoutSeconds
	}
	return 300 // 默认5分钟
}

// getKimiBaseURL 获取Kimi API基础URL
func (s *KimiGatewayService) getKimiBaseURL(account *Account) string {
	// 优先使用账号的BaseURL
	if account != nil && account.GetBaseURL() != "" {
		if url := account.GetBaseURL(); url != "" {
			return url
		}
	}

	// 其次使用全局配置
	if s.cfg != nil && s.cfg.Gateway.Kimi.BaseURL != "" {
		return s.cfg.Gateway.Kimi.BaseURL
	}

	return defaultKimiBaseURL
}

// Forward 转发非流式请求
func (s *KimiGatewayService) Forward(ctx context.Context, account *Account, body []byte) (*KimiForwardResult, error) {
	startTime := time.Now()

	baseURL := s.getKimiBaseURL(account)
	url := fmt.Sprintf("%s/messages", baseURL)

	// Debug: 打印上游请求信息
	logger.FromContext(ctx).Info("kimi.upstream_request",
		zap.String("url", url),
		zap.String("base_url", baseURL),
		zap.Int64("account_id", account.ID),
		zap.String("account_base_url", account.GetBaseURL()),
		zap.Bool("api_key_present", account.GetCredential("api_key") != ""),
	)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+account.GetCredential("api_key"))

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	durationMs := time.Since(startTime).Milliseconds()

	// 解析使用量
	usage := s.extractUsage(respBody)

	// 如果没有获取到实际 usage，使用估算值
	if usage == nil || (usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0) {
		// 打印原始响应用于调试 - 显示所有可能的 usage 字段
		respStr := string(respBody)
		if len(respStr) > 1000 {
			respStr = respStr[:1000]
		}
		logger.FromContext(ctx).Warn("kimi.usage_not_found",
			zap.String("reason", "API did not return usage in response"),
			zap.String("response_body", respStr),
			zap.Int64("usage_prompt_tokens", gjson.GetBytes(respBody, "usage.prompt_tokens").Int()),
			zap.Int64("usage_input_tokens", gjson.GetBytes(respBody, "usage.input_tokens").Int()),
			zap.Int64("usage_completion_tokens", gjson.GetBytes(respBody, "usage.completion_tokens").Int()),
			zap.Int64("usage_output_tokens", gjson.GetBytes(respBody, "usage.output_tokens").Int()),
			zap.Int64("message_usage_prompt_tokens", gjson.GetBytes(respBody, "message.usage.prompt_tokens").Int()),
			zap.Int64("message_usage_input_tokens", gjson.GetBytes(respBody, "message.usage.input_tokens").Int()),
		)

		// 从请求 body 中提取 max_tokens 用于估算输出 token
		maxTokens := int(gjson.GetBytes(body, "max_tokens").Int())
		if maxTokens == 0 {
			maxTokens = 4096 // 默认值
		}
		// 估算 input tokens（根据请求 body 大小估算）
		estimatedInputTokens := len(body) / 4 // 简单估算：每个字符约 4 字节
		if estimatedInputTokens > maxTokens {
			estimatedInputTokens = maxTokens
		}

		usage = &KimiUsage{
			PromptTokens:     estimatedInputTokens,
			CompletionTokens: maxTokens,
			TotalTokens:      estimatedInputTokens + maxTokens,
		}
		logger.FromContext(ctx).Warn("kimi.usage_estimated",
			zap.Int("estimated_prompt_tokens", estimatedInputTokens),
			zap.Int("estimated_completion_tokens", maxTokens),
		)
	}

	result := &KimiForwardResult{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       respBody,
		Usage:      usage,
		RequestID:  resp.Header.Get("X-Request-ID"),
		Stream:     false,
		DurationMs: &durationMs,
	}

	// 记录响应日志
	s.LogResponse(ctx, account.ID, resp.StatusCode, int(durationMs))

	// 检查错误状态码
	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("upstream error: %d - %s", resp.StatusCode, string(respBody))
	}

	return result, nil
}

// ForwardStream 转发流式请求
func (s *KimiGatewayService) ForwardStream(ctx context.Context, account *Account, body []byte, writer gin.ResponseWriter) (*KimiForwardResult, error) {
	startTime := time.Now()
	firstTokenTime := time.Time{}

	baseURL := s.getKimiBaseURL(account)
	url := fmt.Sprintf("%s/messages", baseURL)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+account.GetCredential("api_key"))
	req.Header.Set("Accept", "text/event-stream")

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 设置响应头
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(resp.StatusCode)
	writer.Flush()

	// 从请求 body 中提取 max_tokens 用于估算输出 token
	maxTokens := int(gjson.GetBytes(body, "max_tokens").Int())
	if maxTokens == 0 {
		maxTokens = 4096 // 默认值
	}

	// 流式传输
	var collectedUsage *KimiUsage
	requestID := ""
	scanner := bufio.NewScanner(resp.Body)
	// 设置 scanner 的 buffer 大小，避免长行被截断
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// 记录首token时间 - 更宽松的匹配，支持 "data:", "data:", "data :" 等格式
		if firstTokenTime.IsZero() && (strings.HasPrefix(line, "data:") || strings.HasPrefix(line, "data ")) {
			firstTokenTime = time.Now()
		}

		// 解析token使用量（包括 [DONE] 消息中的 usage）
		// 支持 "data: " 和 "data:" 两种格式
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			// 即使是 [DONE] 也要尝试解析 usage（usage 通常在最后消息中）
			if data != "" && data != "[DONE]" {
				// 尝试解析usage
				usage := s.extractUsage([]byte(data))
				// 流式响应中，需要累加 usage 而不是覆盖
				// 因为 input_tokens 在 message_start，output_tokens 在 message_delta
				if usage != nil {
					if collectedUsage == nil {
						collectedUsage = &KimiUsage{}
					}
					// 累加 input_tokens（通常只在 message_start 时出现）
					if usage.PromptTokens > 0 && collectedUsage.PromptTokens == 0 {
						collectedUsage.PromptTokens = usage.PromptTokens
					}
					// 累加 output_tokens（可能在多个事件中出现，取最新的值）
					if usage.CompletionTokens > 0 {
						collectedUsage.CompletionTokens = usage.CompletionTokens
					}
					// 累加 total_tokens
					if usage.TotalTokens > 0 {
						collectedUsage.TotalTokens = usage.TotalTokens
					}
				}
				if requestID == "" {
					requestID = gjson.Get(data, "id").String()
				}
			}
			// 如果是 [DONE]，也尝试解析 usage（某些 API 会在最后返回 usage）
			if data == "[DONE]" {
				usage := s.extractUsage([]byte(line))
				if usage != nil {
					if collectedUsage == nil {
						collectedUsage = &KimiUsage{}
					}
					if usage.PromptTokens > 0 && collectedUsage.PromptTokens == 0 {
						collectedUsage.PromptTokens = usage.PromptTokens
					}
					if usage.CompletionTokens > 0 {
						collectedUsage.CompletionTokens = usage.CompletionTokens
					}
					if usage.TotalTokens > 0 {
						collectedUsage.TotalTokens = usage.TotalTokens
					}
				}
			}
		} else if strings.HasPrefix(line, "data:") && !strings.HasPrefix(line, "data: ") {
			// 支持 "data:" 格式（无空格）
			data := strings.TrimPrefix(line, "data:")
			if data != "" && data != "[DONE]" {
				// 尝试解析usage
				usage := s.extractUsage([]byte(data))
				if usage != nil {
					if collectedUsage == nil {
						collectedUsage = &KimiUsage{}
					}
					if usage.PromptTokens > 0 && collectedUsage.PromptTokens == 0 {
						collectedUsage.PromptTokens = usage.PromptTokens
					}
					if usage.CompletionTokens > 0 {
						collectedUsage.CompletionTokens = usage.CompletionTokens
					}
					if usage.TotalTokens > 0 {
						collectedUsage.TotalTokens = usage.TotalTokens
					}
				}
				if requestID == "" && len(data) > 5 {
					requestID = gjson.Get(data, "id").String()
				}
			}
		}

		// 写入响应
		writer.Write([]byte(line + "\n"))
		writer.Flush()
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read stream: %w", err)
	}

	durationMs := time.Since(startTime).Milliseconds()
	var firstTokenMs int64
	if !firstTokenTime.IsZero() {
		firstTokenMs = firstTokenTime.Sub(startTime).Milliseconds()
	}

	// 如果没有获取到实际 usage，使用估算值
	if collectedUsage == nil || (collectedUsage.PromptTokens == 0 && collectedUsage.CompletionTokens == 0) {
		// 打印调试信息
		logger.FromContext(ctx).Warn("kimi.usage_stream_not_found",
			zap.String("reason", "API did not return usage in stream response"),
			zap.Int("collected_prompt_tokens", 0),
			zap.Int("collected_completion_tokens", 0),
			zap.Int("max_tokens_from_request", maxTokens),
		)
		collectedUsage = &KimiUsage{
			PromptTokens:     0,
			CompletionTokens: maxTokens, // 使用请求中的 max_tokens 作为估算
			TotalTokens:      maxTokens,
		}
		logger.FromContext(ctx).Warn("kimi.usage_estimated",
			zap.Int("estimated_completion_tokens", maxTokens),
			zap.String("reason", "API did not return usage in stream response"),
		)
	}

	result := &KimiForwardResult{
		StatusCode:   resp.StatusCode,
		Headers:      resp.Header,
		RequestID:    requestID,
		Stream:       true,
		DurationMs:   &durationMs,
		FirstTokenMs: &firstTokenMs,
		Usage:        collectedUsage,
	}

	// 记录响应日志
	s.LogResponse(ctx, account.ID, resp.StatusCode, int(durationMs))

	return result, nil
}

// SelectAccount 选择Kimi账号（支持粘性会话）
func (s *KimiGatewayService) SelectAccount(ctx context.Context, groupID *int64, sessionHash string) (*Account, error) {
	gid := int64(0)
	if groupID != nil {
		gid = *groupID
	}

	logger.FromContext(ctx).Info("kimi.select_account",
		zap.Int64("group_id", gid),
		zap.String("session_hash", sessionHash),
	)

	// 1. 尝试获取粘性会话绑定的账号
	if sessionHash != "" && s.cache != nil {
		stickyAccountID, err := s.getStickySessionAccountID(ctx, gid, sessionHash)
		if err == nil && stickyAccountID > 0 {
			// 验证账号仍然可用
			account, err := s.accountRepo.GetByID(ctx, stickyAccountID)
			if err == nil && account != nil && account.Status == StatusActive && account.Schedulable {
				logger.FromContext(ctx).Info("kimi.sticky_session_hit",
					zap.Int64("account_id", account.ID),
					zap.String("session_hash", sessionHash),
				)
				return account, nil
			}
		}
	}

	// 2. 查询可用账号 - 使用schedulerSnapshot优先
	var accounts []Account
	var err error
	if s.schedulerSnapshot != nil {
		accounts, _, err = s.schedulerSnapshot.ListSchedulableAccounts(ctx, groupID, PlatformKimi, false)
	} else if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		accounts, err = s.accountRepo.ListSchedulableByPlatform(ctx, PlatformKimi)
	} else if groupID != nil {
		accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformKimi)
	} else {
		accounts, err = s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, PlatformKimi)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}

	if len(accounts) == 0 {
		return nil, errors.New("no available accounts")
	}

	// 3. 选择最佳账号（优先级 + LRU）
	selectedAccount := s.selectBestAccount(accounts)

	// 4. 设置粘性会话绑定
	if sessionHash != "" && s.cache != nil {
		s.setStickySessionAccountID(ctx, gid, sessionHash, selectedAccount.ID)
	}

	logger.FromContext(ctx).Info("kimi.account_selected",
		zap.Int64("account_id", selectedAccount.ID),
		zap.String("account_name", selectedAccount.Name),
	)

	return selectedAccount, nil
}

// selectBestAccount 选择最佳账号（优先级 + LRU）
func (s *KimiGatewayService) selectBestAccount(accounts []Account) *Account {
	if len(accounts) == 0 {
		return nil
	}

	// 过滤出可调度的账号
	var schedulableAccounts []*Account
	for i := range accounts {
		acc := &accounts[i]
		if acc.IsSchedulable() && acc.Platform == PlatformKimi {
			schedulableAccounts = append(schedulableAccounts, acc)
		}
	}

	if len(schedulableAccounts) == 0 {
		return nil
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

	return selectedAccount
}

// getStickySessionAccountID 获取粘性会话绑定的账号ID
func (s *KimiGatewayService) getStickySessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	if s.cache == nil {
		return 0, errors.New("cache not available")
	}

	return s.cache.GetSessionAccountID(ctx, groupID, sessionHash)
}

// setStickySessionAccountID 设置粘性会话绑定的账号ID
func (s *KimiGatewayService) setStickySessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64) {
	if s.cache == nil {
		return
	}

	// 默认TTL 1小时
	ttl := time.Hour
	if s.cfg != nil && s.cfg.Gateway.Kimi.StickySessionTTLSeconds > 0 {
		ttl = time.Duration(s.cfg.Gateway.Kimi.StickySessionTTLSeconds) * time.Second
	}

	_ = s.cache.SetSessionAccountID(ctx, groupID, sessionHash, accountID, ttl)
}

// DeleteStickySessionAccountID 删除粘性会话绑定
func (s *KimiGatewayService) DeleteStickySessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	if s.cache == nil {
		return errors.New("cache not available")
	}

	return s.cache.DeleteSessionAccountID(ctx, groupID, sessionHash)
}

// checkHeadersForUsage 检查响应头中是否包含 usage 信息
func checkHeadersForUsage(headers http.Header) *KimiUsage {
	// 常见的 usage 相关响应头
	// 有些 API 会返回 x-usage-input-tokens, x-usage-output-tokens 等响应头
	inputTokens := headers.Get("x-usage-input-tokens")
	outputTokens := headers.Get("x-usage-output-tokens")
	totalTokens := headers.Get("x-usage-total-tokens")

	if inputTokens == "" && outputTokens == "" && totalTokens == "" {
		return nil
	}

	usage := &KimiUsage{}
	if inputTokens != "" {
		if val, err := strconv.Atoi(inputTokens); err == nil {
			usage.PromptTokens = val
		}
	}
	if outputTokens != "" {
		if val, err := strconv.Atoi(outputTokens); err == nil {
			usage.CompletionTokens = val
		}
	}
	if totalTokens != "" {
		if val, err := strconv.Atoi(totalTokens); err == nil {
			usage.TotalTokens = val
		}
	} else {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	return usage
}

// getHeaderKeys 获取响应头的 key 列表（用于调试）
func getHeaderKeys(headers http.Header) []string {
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	return keys
}

// extractUsage 提取使用量信息
// 兼容 Anthropic 格式的多种 usage 位置：
// 1. usage.input_tokens / usage.output_tokens (最常用)
// 2. usage.prompt_tokens / usage.completion_tokens
// 3. message.usage.input_tokens / message.usage.output_tokens
// 4. message.usage.prompt_tokens / message.usage.completion_tokens
func (s *KimiGatewayService) extractUsage(body []byte) *KimiUsage {
	usage := &KimiUsage{}

	// 尝试多种路径获取 input_tokens / prompt_tokens
	// 优先使用 input_tokens（Anthropic 格式）
	inputPaths := []string{
		"usage.input_tokens",
		"usage.prompt_tokens",
		"message.usage.input_tokens",
		"message.usage.prompt_tokens",
	}
	for _, path := range inputPaths {
		val := gjson.GetBytes(body, path).Int()
		if val > 0 {
			usage.PromptTokens = int(val)
			break
		}
	}

	// 尝试多种路径获取 output_tokens / completion_tokens
	outputPaths := []string{
		"usage.output_tokens",
		"usage.completion_tokens",
		"message.usage.output_tokens",
		"message.usage.completion_tokens",
	}
	for _, path := range outputPaths {
		val := gjson.GetBytes(body, path).Int()
		if val > 0 {
			usage.CompletionTokens = int(val)
			break
		}
	}

	// 如果 total_tokens 未提供，计算总和
	if totalTokens := gjson.GetBytes(body, "usage.total_tokens").Int(); totalTokens > 0 {
		usage.TotalTokens = int(totalTokens)
	} else if totalTokens := gjson.GetBytes(body, "message.usage.total_tokens").Int(); totalTokens > 0 {
		usage.TotalTokens = int(totalTokens)
	} else if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	return usage
}

// RecordUsage 记录使用量
func (s *KimiGatewayService) RecordUsage(ctx context.Context, input *KimiRecordUsageInput) error {
	if input == nil || input.Usage == nil {
		return errors.New("usage data is nil")
	}

	logger.FromContext(ctx).Info("kimi.record_usage",
		zap.Int64("api_key_id", input.APIKey.ID),
		zap.Int64("account_id", input.Account.ID),
		zap.String("model", input.Model),
		zap.Int("prompt_tokens", input.Usage.PromptTokens),
		zap.Int("completion_tokens", input.Usage.CompletionTokens),
	)

	// 获取用户信息
	user, err := s.userRepo.GetByID(ctx, input.APIKey.UserID)
	if err != nil {
		logger.FromContext(ctx).Warn("kimi.record_usage_get_user_failed",
			zap.Error(err),
			zap.Int64("user_id", input.APIKey.UserID),
		)
		// 继续，不因为获取用户失败而中断
	}

	// 获取定价
	pricing := getKimiModelPricing(input.Model)

	// 计算费用
	inputCost := float64(input.Usage.PromptTokens) / 1000 * pricing.InputPrice
	outputCost := float64(input.Usage.CompletionTokens) / 1000 * pricing.OutputPrice
	totalCost := inputCost + outputCost

	// 应用分组倍率
	multiplier := 1.0
	if input.APIKey.Group != nil {
		multiplier = input.APIKey.Group.RateMultiplier
	}
	totalCost = totalCost * multiplier

	logger.FromContext(ctx).Info("kimi.cost_calculation",
		zap.String("model", input.Model),
		zap.Int("prompt_tokens", input.Usage.PromptTokens),
		zap.Int("completion_tokens", input.Usage.CompletionTokens),
		zap.Float64("input_price_per_1k", pricing.InputPrice),
		zap.Float64("output_price_per_1k", pricing.OutputPrice),
		zap.Float64("input_cost", inputCost),
		zap.Float64("output_cost", outputCost),
		zap.Float64("total_cost", totalCost),
		zap.Float64("multiplier", multiplier),
	)

	// 获取账号计费倍率
	accountRateMultiplier := input.Account.BillingRateMultiplier()

	// 创建使用记录
	usageLog := &UsageLog{
		UserID:                input.APIKey.UserID,
		APIKeyID:              input.APIKey.ID,
		AccountID:             input.Account.ID,
		RequestID:             input.RequestID,
		Model:                 input.Model,
		InputTokens:           input.Usage.PromptTokens,
		OutputTokens:          input.Usage.CompletionTokens,
		InputCost:             inputCost * multiplier,
		OutputCost:            outputCost * multiplier,
		TotalCost:             totalCost,
		ActualCost:            totalCost,
		RateMultiplier:        multiplier,
		AccountRateMultiplier: &accountRateMultiplier,
		RequestType:           RequestTypeSync,
		Stream:                input.Stream,
	}

	// 根据是否流式设置请求类型
	if input.Stream {
		usageLog.RequestType = RequestTypeStream
	}

	if input.DurationMs != nil {
		duration := int(*input.DurationMs)
		usageLog.DurationMs = &duration
	}

	if input.FirstTokenMs != nil {
		firstTokenMs := int(*input.FirstTokenMs)
		usageLog.FirstTokenMs = &firstTokenMs
	} else if input.DurationMs != nil {
		// 如果没有首token时间，使用总时长作为回退
		durationMs := int(*input.DurationMs)
		usageLog.FirstTokenMs = &durationMs
	}

	// 设置 UserAgent 和 IPAddress
	if input.UserAgent != "" {
		usageLog.UserAgent = &input.UserAgent
	}
	if input.IPAddress != "" {
		usageLog.IPAddress = &input.IPAddress
	}

	// 保存使用记录
	_, err = s.usageRepo.Create(ctx, usageLog)
	if err != nil {
		return fmt.Errorf("failed to create usage log: %w", err)
	}

	// 更新账号最后使用时间
	now := time.Now()
	input.Account.LastUsedAt = &now
	err = s.accountRepo.Update(ctx, input.Account)
	if err != nil {
		logger.FromContext(ctx).Warn("kimi.update_last_used_at_failed",
			zap.Error(err),
			zap.Int64("account_id", input.Account.ID),
		)
	}

	_ = user // silence unused variable warning

	return nil
}

// LogResponse 记录响应日志
func (s *KimiGatewayService) LogResponse(ctx context.Context, accountID int64, statusCode int, durationMs int) {
	logger.FromContext(ctx).Info("kimi.response",
		zap.Int64("account_id", accountID),
		zap.Int("status_code", statusCode),
		zap.Int("duration_ms", durationMs),
	)
}

// ListModels 返回可用的模型列表
func (s *KimiGatewayService) ListModels() []map[string]interface{} {
	return DefaultKimiModels()
}

// DefaultKimiModels 返回默认的 Kimi 模型列表
func DefaultKimiModels() []map[string]interface{} {
	models := make([]map[string]interface{}, 0, len(defaultKimiPricing))

	for model := range defaultKimiPricing {
		models = append(models, map[string]interface{}{
			"id":       model,
			"object":   "model",
			"created":  time.Now().Unix(),
			"owned_by": "kimi",
		})
	}

	return models
}

// TestConnection 测试账号连接
func (s *KimiGatewayService) TestConnection(ctx context.Context, account *Account) error {
	if account == nil {
		return errors.New("account is nil")
	}

	baseURL := s.getKimiBaseURL(account)
	url := fmt.Sprintf("%s/models", baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+account.GetCredential("api_key"))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("connection test failed: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// ValidateAndSetBaseURL 验证并设置BaseURL
func (s *KimiGatewayService) ValidateAndSetBaseURL(account *Account, baseURL string) error {
	if account == nil {
		return errors.New("account is nil")
	}

	// 如果为空，使用默认值
	if baseURL == "" {
		baseURL = defaultKimiBaseURL
	}

	// 设置到账号extra中
	if account.Extra == nil {
		account.Extra = make(map[string]interface{})
	}
	account.Extra["base_url"] = baseURL

	return nil
}
