package service

import (
	"time"
)

// SubscriptionCacheData represents cached subscription data
type SubscriptionCacheData struct {
	Status       string
	ExpiresAt    time.Time
	DailyUsage   float64
	WeeklyUsage  float64
	MonthlyUsage float64
	Version      int64
	// Token 配额使用量
	TokenUsageTotal   int64
	TokenUsageDaily   int64
	TokenUsageWeekly  int64
	TokenUsageMonthly int64
}
