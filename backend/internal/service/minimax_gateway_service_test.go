//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ==================== extractUsage 测试 ====================

// TestMiniMaxGatewayService_ExtractUsage_StandardFormat 测试标准 usage 格式
func TestMiniMaxGatewayService_ExtractUsage_StandardFormat(t *testing.T) {
	svc := &MiniMaxGatewayService{}

	// 测试标准格式: usage.prompt_tokens
	body := []byte(`{
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50,
			"total_tokens": 150
		}
	}`)

	usage := svc.extractUsage(body)

	require.NotNil(t, usage)
	require.Equal(t, 100, usage.PromptTokens)
	require.Equal(t, 50, usage.CompletionTokens)
	require.Equal(t, 150, usage.TotalTokens)
}

// TestMiniMaxGatewayService_ExtractUsage_InputOutputNames 测试 input_tokens/output_tokens 命名
func TestMiniMaxGatewayService_ExtractUsage_InputOutputNames(t *testing.T) {
	svc := &MiniMaxGatewayService{}

	// 测试 input_tokens/output_tokens 格式（优先级最高）
	body := []byte(`{
		"usage": {
			"input_tokens": 200,
			"output_tokens": 100,
			"total_tokens": 300
		}
	}`)

	usage := svc.extractUsage(body)

	require.NotNil(t, usage)
	require.Equal(t, 200, usage.PromptTokens)
	require.Equal(t, 100, usage.CompletionTokens)
	require.Equal(t, 300, usage.TotalTokens)
}

// TestMiniMaxGatewayService_ExtractUsage_MessageUsageFormat 测试 message.usage 格式（SSE 流）
func TestMiniMaxGatewayService_ExtractUsage_MessageUsageFormat(t *testing.T) {
	svc := &MiniMaxGatewayService{}

	// 测试 message.usage 格式（SSE 流式响应中 message_start 事件）
	body := []byte(`{
		"type": "message_start",
		"message": {
			"usage": {
				"prompt_tokens": 150
			}
		}
	}`)

	usage := svc.extractUsage(body)

	require.NotNil(t, usage)
	require.Equal(t, 150, usage.PromptTokens)
	require.Equal(t, 0, usage.CompletionTokens)
}

// TestMiniMaxGatewayService_ExtractUsage_MessageUsageInputTokens 测试 message.usage.input_tokens
func TestMiniMaxGatewayService_ExtractUsage_MessageUsageInputTokens(t *testing.T) {
	svc := &MiniMaxGatewayService{}

	// 测试 message.usage.input_tokens 格式
	body := []byte(`{
		"type": "message_start",
		"message": {
			"usage": {
				"input_tokens": 180
			}
		}
	}`)

	usage := svc.extractUsage(body)

	require.NotNil(t, usage)
	require.Equal(t, 180, usage.PromptTokens)
}

// TestMiniMaxGatewayService_ExtractUsage_StreamingDelta 测试流式 delta 事件
func TestMiniMaxGatewayService_ExtractUsage_StreamingDelta(t *testing.T) {
	svc := &MiniMaxGatewayService{}

	// 测试流式响应中的 message_delta 事件
	body := []byte(`{
		"type": "message_delta",
		"usage": {
			"output_tokens": 75
		}
	}`)

	usage := svc.extractUsage(body)

	require.NotNil(t, usage)
	require.Equal(t, 75, usage.CompletionTokens)
}

// TestMiniMaxGatewayService_ExtractUsage_Priority 测试路径优先级
// input_tokens 优先级高于 prompt_tokens
func TestMiniMaxGatewayService_ExtractUsage_Priority(t *testing.T) {
	svc := &MiniMaxGatewayService{}

	// 同时存在多个路径时，应该优先使用 usage.input_tokens
	body := []byte(`{
		"usage": {
			"input_tokens": 100
		},
		"message": {
			"usage": {
				"prompt_tokens": 200
			}
		}
	}`)

	usage := svc.extractUsage(body)

	require.NotNil(t, usage)
	require.Equal(t, 100, usage.PromptTokens, "should prefer usage.input_tokens over message.usage.prompt_tokens")
}

// TestMiniMaxGatewayService_ExtractUsage_Empty 测试空响应
func TestMiniMaxGatewayService_ExtractUsage_Empty(t *testing.T) {
	svc := &MiniMaxGatewayService{}

	// 测试空响应
	body := []byte(`{}`)

	usage := svc.extractUsage(body)

	require.NotNil(t, usage)
	require.Equal(t, 0, usage.PromptTokens)
	require.Equal(t, 0, usage.CompletionTokens)
	require.Equal(t, 0, usage.TotalTokens)
}

// TestMiniMaxGatewayService_ExtractUsage_CalculateTotal 测试 total_tokens 自动计算
func TestMiniMaxGatewayService_ExtractUsage_CalculateTotal(t *testing.T) {
	svc := &MiniMaxGatewayService{}

	// 不返回 total_tokens 时应该自动计算
	body := []byte(`{
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50
		}
	}`)

	usage := svc.extractUsage(body)

	require.NotNil(t, usage)
	require.Equal(t, 150, usage.TotalTokens)
}

// ==================== 定价测试 ====================

// TestMiniMaxGatewayService_Pricing_M25 测试 MiniMax-M2.5 定价
func TestMiniMaxGatewayService_Pricing_M25(t *testing.T) {
	pricing := getMiniMaxModelPricing("MiniMax-M2.5")

	require.NotNil(t, pricing)
	require.Equal(t, 0.01, pricing.InputPrice)
	require.Equal(t, 0.03, pricing.OutputPrice)
}

// TestMiniMaxGatewayService_Pricing_M25Highspeed 测试 MiniMax-M2.5-highspeed 定价
func TestMiniMaxGatewayService_Pricing_M25Highspeed(t *testing.T) {
	pricing := getMiniMaxModelPricing("MiniMax-M2.5-highspeed")

	require.NotNil(t, pricing)
	require.Equal(t, 0.02, pricing.InputPrice)
	require.Equal(t, 0.06, pricing.OutputPrice)
}

// TestMiniMaxGatewayService_Pricing_M21 测试 MiniMax-M2.1 定价
func TestMiniMaxGatewayService_Pricing_M21(t *testing.T) {
	pricing := getMiniMaxModelPricing("MiniMax-M2.1")

	require.NotNil(t, pricing)
	require.Equal(t, 0.008, pricing.InputPrice)
	require.Equal(t, 0.024, pricing.OutputPrice)
}

// TestMiniMaxGatewayService_Pricing_M2 测试 MiniMax-M2 定价
func TestMiniMaxGatewayService_Pricing_M2(t *testing.T) {
	pricing := getMiniMaxModelPricing("MiniMax-M2")

	require.NotNil(t, pricing)
	require.Equal(t, 0.005, pricing.InputPrice)
	require.Equal(t, 0.015, pricing.OutputPrice)
}

// TestMiniMaxGatewayService_Pricing_PrefixMatch 测试前缀匹配
func TestMiniMaxGatewayService_Pricing_PrefixMatch(t *testing.T) {
	// 测试前缀匹配
	pricing := getMiniMaxModelPricing("MiniMax-M2.5-long")

	require.NotNil(t, pricing)
	require.Equal(t, 0.01, pricing.InputPrice)
}

// TestMiniMaxGatewayService_Pricing_Default 测试默认定价
func TestMiniMaxGatewayService_Pricing_Default(t *testing.T) {
	// 未知模型使用默认定价
	pricing := getMiniMaxModelPricing("unknown-model")

	require.NotNil(t, pricing)
	require.Equal(t, 0.01, pricing.InputPrice)
	require.Equal(t, 0.03, pricing.OutputPrice)
}

// TestMiniMaxGatewayService_CostCalculation 测试费用计算
func TestMiniMaxGatewayService_CostCalculation(t *testing.T) {
	// 测试 1000 input tokens + 500 output tokens 的费用
	inputTokens := 1000
	outputTokens := 500
	model := "MiniMax-M2.5"

	pricing := getMiniMaxModelPricing(model)
	inputCost := float64(inputTokens) / 1000 * pricing.InputPrice
	outputCost := float64(outputTokens) / 1000 * pricing.OutputPrice
	totalCost := inputCost + outputCost

	// M2.5: input=0.01元/千, output=0.03元/千
	// 1000 tokens = 1k * 0.01 = 0.01 元
	// 500 tokens = 0.5k * 0.03 = 0.015 元
	// total = 0.025 元
	require.Equal(t, 0.01, inputCost)
	require.Equal(t, 0.015, outputCost)
	require.Equal(t, 0.025, totalCost)
}
