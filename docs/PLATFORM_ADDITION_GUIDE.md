# Sub2API 平台接入指南

本文档详细介绍如何在 Sub2API 中添加一个新的 AI 平台支持。以 Zhipu (智谱AI) 为例进行说明，其他平台可参照此方案开发。

## 目录

1. [方案概览](#1-方案概览)
2. [后端改动](#2-后端改动)
3. [前端改动](#3-前端改动)
4. [定价配置](#4-定价配置)
5. [关键代码示例](#5-关键代码示例)
6. [测试验证](#6-测试验证)

---

## 1. 方案概览

### 1.1 新增文件

| 文件 | 说明 |
|------|------|
| `backend/internal/handler/zhipu_gateway_handler.go` | HTTP 请求处理 |
| `backend/internal/service/zhipu_gateway_service.go` | 核心业务逻辑、API 转发、计费 |
| `backend/internal/service/zhipu_gateway_service_test.go` | 单元测试 |

### 1.2 修改文件统计

- **后端**: 30+ 文件
- **前端**: 20+ 文件

---

## 2. 后端改动

### 2.1 平台常量定义

**文件**: `backend/internal/domain/constants.go`

```go
// Platform constants
const (
    PlatformAnthropic   = "anthropic"
    PlatformOpenAI      = "openai"
    PlatformGemini      = "gemini"
    PlatformAntigravity = "antigravity"
    PlatformSora        = "sora"
    PlatformZhipu       = "zhipu"  // 新增
)
```

**文件**: `backend/internal/service/domain_constants.go`

```go
var PlatformZhipu = domain.PlatformZhipu
```

### 2.2 网关配置

**文件**: `backend/internal/config/config.go`

```go
// GatewayZhipuConfig Zhipu平台配置
type GatewayZhipuConfig struct {
    BaseURL             string `mapstructure:"base_url"`              // API 基础地址
    RequestTimeoutSeconds int   `mapstructure:"request_timeout_seconds"` // 请求超时
    MaxRetries          int    `mapstructure:"max_retries"`           // 最大重试次数
    RetryBaseDelaySeconds int  `mapstructure:"retry_base_delay_seconds"` // 重试基础延迟
    RetryMaxDelaySeconds int   `mapstructure:"retry_max_delay_seconds"`  // 重试最大延迟
    RetryJitterRatio    float64 `mapstructure:"retry_jitter_ratio"`   // 重试抖动比例
    StickySessionTTLSeconds int `mapstructure:"sticky_session_ttl_seconds"` // 粘性会话 TTL
}
```

在 `GatewayConfig` 中添加:

```go
type GatewayConfig struct {
    // ... 其他平台
    Zhipu GatewayZhipuConfig `mapstructure:"zhipu"`
}
```

### 2.3 路由注册

**文件**: `backend/internal/server/routes/gateway.go`

```go
// Zhipu 专用路由（强制使用 zhipu 平台）
zhipuV1 := r.Group("/zhipu/v1")
zhipuV1.Use(bodyLimit)
zhipuV1.Use(clientRequestID)
zhipuV1.Use(opsErrorLogger)
zhipuV1.Use(middleware.ForcePlatform(service.PlatformZhipu))
zhipuV1.Use(gin.HandlerFunc(apiKeyAuth))
zhipuV1.Use(requireGroupAnthropic)
zhipuV1.POST("/chat/completions", h.ZhipuGateway.ChatCompletions)
zhipuV1.GET("/models", h.ZhipuGateway.Models)
```

### 2.4 Handler 层

**文件**: `backend/internal/handler/zhipu_gateway_handler.go`

实现以下接口方法:

```go
type ZhipuGatewayHandlerInterface interface {
    ChatCompletions(c *gin.Context)
    Models(c *gin.Context)
}
```

主要职责:
- 解析 API Key 和请求
- 调用 Service 选择账号
- 转发请求到上游
- 记录使用量

### 2.5 Service 层

**文件**: `backend/internal/service/zhipu_gateway_service.go`

核心结构:

```go
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
    httpClient          *http.Client
}
```

核心方法:

| 方法 | 说明 |
|------|------|
| `Forward()` | 非流式请求转发 |
| `ForwardStream()` | 流式请求转发 |
| `SelectAccount()` | 账号选择（支持粘性会话） |
| `RecordUsage()` | 使用量记录与计费 |
| `TestConnection()` | 连接测试 |
| `HealthCheck()` | 健康检查 |

### 2.6 定价配置

**文件**: `backend/internal/service/zhipu_gateway_service.go`

```go
var defaultZhipuPricing = map[string]*zhipuModelPricing{
    // 固定定价模型
    "glm-4-plus":     {InputPrice: 0.005, OutputPrice: 0.005},
    "glm-4-flash":    {InputPrice: 0.0001, OutputPrice: 0.0002},
    "glm-4.7":        {InputPrice: 0.002, OutputPrice: 0.008},  // 默认档位
    "glm-5":          {InputPrice: 0.004, OutputPrice: 0.018}, // 默认档位
    
    // 分段计价模型（需根据输出token数量动态选择）
    // 详见 2.6.1
}

// 分段计价函数
func getGLM47Pricing(completionTokens int) *zhipuModelPricing { ... }
func getGLM5Pricing(completionTokens int) *zhipuModelPricing { ... }
```

#### 2.6.1 分段计价实现

对于支持按输出 token 数量分段的模型（如 glm-4.7、glm-5），实现动态定价:

```go
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

    // ... 其他模糊匹配
}

// glm-4.7 分段定价
func getGLM47Pricing(completionTokens int) *zhipuModelPricing {
    if completionTokens <= 200 {
        return &zhipuModelPricing{InputPrice: 0.002, OutputPrice: 0.008}
    }
    if completionTokens <= 32000 {
        return &zhipuModelPricing{InputPrice: 0.003, OutputPrice: 0.014}
    }
    return &zhipuModelPricing{InputPrice: 0.004, OutputPrice: 0.016}
}
```

### 2.7 Wire 依赖注入

**文件**: `backend/internal/handler/wire.go`

```go
type HandlerSet struct {
    // ... 其他
    ZhipuGatewayHandler *ZhipuGatewayHandler
}
```

**文件**: `backend/cmd/server/wire_gen.go` (自动生成)

```go
zhipuGatewayService := service.NewZhipuGatewayService(...)
zhipuGatewayHandler := handler.NewZhipuGatewayHandler(...)
```

### 2.8 其他需要修改的文件

| 文件 | 修改内容 |
|------|----------|
| `backend/internal/model/error_passthrough_rule.go` | 添加平台到错误透传列表 |
| `backend/internal/repository/simple_mode_default_groups.go` | 简单模式默认分组 |
| `backend/internal/service/settings_view.go` | 设置视图添加平台配置 |
| `backend/internal/handler/admin/group_handler.go` | 分组管理添加平台校验 |
| `backend/internal/handler/admin/account_handler.go` | 账号管理平台筛选 |

---

## 3. 前端改动

### 3.1 类型定义

**文件**: `frontend/src/types/index.ts`

```typescript
export type GroupPlatform = 'anthropic' | 'openai' | 'gemini' | 'antigravity' | 'sora' | 'zhipu'
export type AccountPlatform = 'anthropic' | 'openai' | 'gemini' | 'antigravity' | 'sora' | 'zhipu'
```

### 3.2 国际化

**文件**: `frontend/src/i18n/locales/zh.ts`

```typescript
zhipu: '智谱',
```

**文件**: `frontend/src/i18n/locales/en.ts`

```typescript
zhipu: 'Zhipu',
```

### 3.3 平台图标

**文件**: `frontend/src/components/common/PlatformIcon.vue`

添加 Zhipu 图标组件。

### 3.4 平台徽章

**文件**: `frontend/src/components/common/PlatformTypeBadge.vue`

```vue
<template v-else-if="platform === 'zhipu'">
  <span class="platform-badge zhipu">Zhipu</span>
</template>
```

### 3.5 模型图标识别

**文件**: `frontend/src/components/common/ModelIcon.vue`

```typescript
if (modelLower.includes('glm') || modelLower.includes('chatglm') ||
    modelLower.includes('cogview') || modelLower.includes('cogvideo')) {
    return 'zhipu'
}
```

### 3.6 模型白名单

**文件**: `frontend/src/composables/useModelWhitelist.ts`

```typescript
const zhipuModels = [
    'glm-4', 'glm-4v', 'glm-4-plus', 'glm-4-air', 'glm-4-flash',
    'glm-4.5', 'glm-4.5-air', 'glm-4.6v', 'glm-4.7', 'glm-5',
    'glm-3-turbo', 'chatglm_turbo', 'chatglm_pro', 'chatglm_lite',
    'cogview-3', 'cogvideo', 'embedding-3'
]
```

### 3.7 账号管理

| 文件 | 修改内容 |
|------|----------|
| `CreateAccountModal.vue` | 添加 Zhipu 平台选项、默认 BaseURL |
| `EditAccountModal.vue` | 编辑时显示 Zhipu 默认 BaseURL |
| `BulkEditAccountModal.vue` | 批量编辑支持 Zhipu 模型前缀 |

```typescript
// 默认 BaseURL
if (form.platform === 'zhipu') {
    return 'https://open.bigmodel.cn/api/paas/v4'
}

// 模型前缀
zhipu: ['glm-', 'chatglm', 'cogview', 'cogvideo']
```

### 3.8 分组管理

**文件**: `frontend/src/views/admin/GroupsView.vue`

```typescript
{ value: 'zhipu', label: 'Zhipu' }
```

### 3.9 设置页面

**文件**: `frontend/src/views/admin/SettingsView.vue`

```typescript
fallback_model_zhipu: 'glm-4',
```

### 3.10 API 接口

**文件**: `frontend/src/api/admin/settings.ts`

```typescript
fallback_model_zhipu: string
```

---

## 4. 定价配置

### 4.1 官方定价获取

1. 登录智谱 AI开放平台
2. 进入「费用明细」导出账单
3. 从账单中提取各模型的输入/输出单价

### 4.2 定价表结构

```go
type zhipuModelPricing struct {
    InputPrice  float64  // 输入价格（元/千tokens）
    OutputPrice float64  // 输出价格（元/千tokens）
}
```

### 4.3 定价更新日志

在定价表上方添加更新说明:

```go
// Zhipu模型定价表（单位：元/千tokens）
// 参考智谱官方定价：https://open.bigmodel.cn/dev/api#prices
// 更新时间：2026-03-11
var defaultZhipuPricing = map[string]*zhipuModelPricing{
    // ...
}
```

---

## 5. 关键代码示例

### 5.1 Token 使用量提取

```go
func extractZhipuUsage(body []byte) *ZhipuUsage {
    usage := &ZhipuUsage{}
    
    if promptTokens := gjson.GetBytes(body, "usage.prompt_tokens").Int(); promptTokens > 0 {
        usage.PromptTokens = int(promptTokens)
    }
    if completionTokens := gjson.GetBytes(body, "usage.completion_tokens").Int(); completionTokens > 0 {
        usage.CompletionTokens = int(completionTokens)
    }
    
    return usage
}
```

### 5.2 费用计算

```go
// 根据模型和输出token数量获取定价
pricing := getZhipuModelPricing(input.Model, usage.CompletionTokens)

// 计算费用
inputCost := float64(usage.PromptTokens) / 1000 * pricing.InputPrice
outputCost := float64(usage.CompletionTokens) / 1000 * pricing.OutputPrice
totalCost := (inputCost + outputCost) * multiplier

// 记录日志
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
```

### 5.3 账号选择（支持粘性会话）

```go
func (s *ZhipuGatewayService) SelectAccount(ctx context.Context, groupID *int64, sessionHash string) (*Account, error) {
    // 1. 尝试获取粘性会话绑定的账号
    if sessionHash != "" {
        if stickyAccountID, err := s.getStickySessionAccountID(ctx, groupID, sessionHash); err == nil && stickyAccountID > 0 {
            // 验证并返回绑定的账号
            // ...
        }
    }

    // 2. 查询可用账号
    accounts, err := s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformZhipu)
    
    // 3. 负载感知调度：优先级 + LRU
    selectedAccount := selectBestAccount(accounts)
    
    // 4. 设置粘性会话绑定
    if sessionHash != "" {
        s.setStickySessionAccountID(ctx, groupID, sessionHash, selectedAccount.ID)
    }
    
    return selectedAccount, nil
}
```

---

## 6. 测试验证

### 6.1 单元测试

**文件**: `backend/internal/service/zhipu_gateway_service_test.go`

```go
func TestZhipuGatewayService_TestConnection(t *testing.T) {
    // 测试连接
}

func TestZhipuGatewayService_Pricing(t *testing.T) {
    // 测试定价计算
}
```

### 6.2 手动测试

1. **账号添加**: 在管理后台添加 Zhipu 平台的测试账号
2. **API 测试**: 使用 API Key 调用 `/zhipu/v1/chat/completions`
3. **计费验证**: 检查使用量记录和余额扣费是否正确

---

## 附录：文件清单

### 新增文件 (3个)

```
backend/internal/handler/zhipu_gateway_handler.go
backend/internal/service/zhipu_gateway_service.go
backend/internal/service/zhipu_gateway_service_test.go
```

### 修改文件 (52个)

**后端 (30+)**:
- `backend/cmd/server/main.go`
- `backend/cmd/server/wire_gen.go`
- `backend/go.mod` / `backend/go.sum`
- `backend/internal/config/config.go`
- `backend/internal/domain/constants.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/wire.go`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/admin/group_handler.go`
- `backend/internal/handler/admin/setting_handler.go`
- `backend/internal/handler/dto/settings.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/model/error_passthrough_rule.go`
- `backend/internal/repository/simple_mode_default_groups.go`
- `backend/internal/server/api_contract_test.go`
- `backend/internal/server/http.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/service/account.go`
- `backend/internal/service/account_service.go`
- `backend/internal/service/domain_constants.go`
- `backend/internal/service/openai_account_scheduler.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/ops_retry.go`
- `backend/internal/service/scheduler_snapshot_service.go`
- `backend/internal/service/setting_service.go`
- `backend/internal/service/settings_view.go`
- `backend/internal/service/wire.go`

**前端 (20+)**:
- `frontend/pnpm-lock.yaml`
- `frontend/src/api/admin/settings.ts`
- `frontend/src/assets/base-model-logo.svg`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/BulkEditAccountModal.vue`
- `frontend/src/components/admin/ErrorPassthroughRulesModal.vue`
- `frontend/src/components/admin/account/AccountTableFilters.vue`
- `frontend/src/components/common/PlatformIcon.vue`
- `frontend/src/components/common/PlatformTypeBadge.vue`
- `frontend/src/components/common/ModelIcon.vue`
- `frontend/src/components/common/GroupBadge.vue`
- `frontend/src/components/common/GroupOptionItem.vue`
- `frontend/src/components/keys/UseKeyModal.vue`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/types/index.ts`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/admin/ops/components/OpsDashboardHeader.vue`
- `frontend/src/views/user/UsageView.vue`

---

## 更新记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-03-11 | v1.0 | 初始文档，基于 Zhipu 平台接入 |
