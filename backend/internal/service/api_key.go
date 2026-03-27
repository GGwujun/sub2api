package service

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
)

// API Key status constants
const (
	StatusAPIKeyActive         = "active"
	StatusAPIKeyDisabled       = "disabled"
	StatusAPIKeyQuotaExhausted = "quota_exhausted"
	StatusAPIKeyExpired        = "expired"
)

// Rate limit window durations
const (
	RateLimitWindow5h = 5 * time.Hour
	RateLimitWindow1d = 24 * time.Hour
	RateLimitWindow7d = 7 * 24 * time.Hour
)

// IsWindowExpired returns true if the window starting at windowStart has exceeded the given duration.
// A nil windowStart is treated as expired — no initialized window means any accumulated usage is stale.
func IsWindowExpired(windowStart *time.Time, duration time.Duration) bool {
	return windowStart == nil || time.Since(*windowStart) >= duration
}

type APIKey struct {
	ID          int64
	UserID      int64
	Key         string
	Name        string
	GroupID     *int64
	Status      string
	IPWhitelist []string
	IPBlacklist []string
	// 预编译的 IP 规则，用于认证热路径避免重复 ParseIP/ParseCIDR。
	CompiledIPWhitelist *ip.CompiledIPRules `json:"-"`
	CompiledIPBlacklist *ip.CompiledIPRules `json:"-"`
	LastUsedAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	User                *User
	Group               *Group

	// Quota fields
	Quota     float64    // Quota limit in USD (0 = unlimited)
	QuotaUsed float64    // Used quota amount
	ExpiresAt *time.Time // Expiration time (nil = never expires)

	// Token Quota fields
	TokenQuota     int64 // Token quota limit (0 = unlimited)
	TokenQuotaUsed int64 // Used token quota amount

	// Token Quota Daily/Weekly/Monthly fields
	TokenQuotaDaily      *int64     // Daily token quota limit (0 = unlimited)
	TokenQuotaDailyUsed  *int64     // Used daily token quota amount
	TokenQuotaDailyStart *time.Time // Daily token quota window start

	TokenQuotaWeekly      *int64     // Weekly token quota limit (0 = unlimited)
	TokenQuotaWeeklyUsed  *int64     // Used weekly token quota amount
	TokenQuotaWeeklyStart *time.Time // Weekly token quota window start

	TokenQuotaMonthly      *int64     // Monthly token quota limit (0 = unlimited)
	TokenQuotaMonthlyUsed  *int64     // Used monthly token quota amount
	TokenQuotaMonthlyStart *time.Time // Monthly token quota window start

	// Rate limit fields
	RateLimit5h   float64    // Rate limit in USD per 5h (0 = unlimited)
	RateLimit1d   float64    // Rate limit in USD per 1d (0 = unlimited)
	RateLimit7d   float64    // Rate limit in USD per 7d (0 = unlimited)
	Usage5h       float64    // Used amount in current 5h window
	Usage1d       float64    // Used amount in current 1d window
	Usage7d       float64    // Used amount in current 7d window
	Window5hStart *time.Time // Start of current 5h window
	Window1dStart *time.Time // Start of current 1d window
	Window7dStart *time.Time // Start of current 7d window
}

func (k *APIKey) IsActive() bool {
	return k.Status == StatusActive
}

// HasRateLimits returns true if any rate limit window is configured
func (k *APIKey) HasRateLimits() bool {
	return k.RateLimit5h > 0 || k.RateLimit1d > 0 || k.RateLimit7d > 0
}

// IsExpired checks if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsQuotaExhausted checks if the API key quota is exhausted
func (k *APIKey) IsQuotaExhausted() bool {
	if k.Quota <= 0 {
		return false // unlimited
	}
	return k.QuotaUsed >= k.Quota
}

// GetQuotaRemaining returns remaining quota (-1 for unlimited)
func (k *APIKey) GetQuotaRemaining() float64 {
	if k.Quota <= 0 {
		return -1 // unlimited
	}
	remaining := k.Quota - k.QuotaUsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsTokenQuotaExhausted checks if the API key token quota is exhausted
func (k *APIKey) IsTokenQuotaExhausted() bool {
	effectiveQuota := k.GetEffectiveTokenQuota()
	if effectiveQuota <= 0 {
		return false // unlimited
	}
	return k.TokenQuotaUsed >= effectiveQuota
}

// GetTokenQuotaRemaining returns remaining token quota (-1 for unlimited)
func (k *APIKey) GetTokenQuotaRemaining() int64 {
	effectiveQuota := k.GetEffectiveTokenQuota()
	if effectiveQuota <= 0 {
		return -1 // unlimited
	}
	remaining := effectiveQuota - k.TokenQuotaUsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetEffectiveTokenQuota returns the effective token quota (API Key > Group > 0)
// Priority: API Key token_quota > Group token_quota
func (k *APIKey) GetEffectiveTokenQuota() int64 {
	if k.TokenQuota > 0 {
		return k.TokenQuota
	}
	if k.Group != nil && k.Group.TokenQuota != nil && *k.Group.TokenQuota > 0 {
		return *k.Group.TokenQuota
	}
	return 0
}

// GetEffectiveTokenQuotaDaily returns the effective daily token quota
func (k *APIKey) GetEffectiveTokenQuotaDaily() int64 {
	// API Key level daily quota
	if k.TokenQuotaDaily != nil && *k.TokenQuotaDaily > 0 {
		return *k.TokenQuotaDaily
	}
	// Fall back to Group level
	if k.Group != nil && k.Group.TokenQuotaDaily != nil && *k.Group.TokenQuotaDaily > 0 {
		return *k.Group.TokenQuotaDaily
	}
	return 0
}

// GetEffectiveTokenQuotaWeekly returns the effective weekly token quota
func (k *APIKey) GetEffectiveTokenQuotaWeekly() int64 {
	// API Key level weekly quota
	if k.TokenQuotaWeekly != nil && *k.TokenQuotaWeekly > 0 {
		return *k.TokenQuotaWeekly
	}
	// Fall back to Group level
	if k.Group != nil && k.Group.TokenQuotaWeekly != nil && *k.Group.TokenQuotaWeekly > 0 {
		return *k.Group.TokenQuotaWeekly
	}
	return 0
}

// GetEffectiveTokenQuotaMonthly returns the effective monthly token quota
func (k *APIKey) GetEffectiveTokenQuotaMonthly() int64 {
	// API Key level monthly quota
	if k.TokenQuotaMonthly != nil && *k.TokenQuotaMonthly > 0 {
		return *k.TokenQuotaMonthly
	}
	// Fall back to Group level
	if k.Group != nil && k.Group.TokenQuotaMonthly != nil && *k.Group.TokenQuotaMonthly > 0 {
		return *k.Group.TokenQuotaMonthly
	}
	return 0
}

// HasTokenQuota returns true if API Key or Group has any token quota configured
func (k *APIKey) HasTokenQuota() bool {
	return k.GetEffectiveTokenQuota() > 0 || k.GetEffectiveTokenQuotaDaily() > 0 ||
		k.GetEffectiveTokenQuotaWeekly() > 0 || k.GetEffectiveTokenQuotaMonthly() > 0
}

// TokenQuota window helpers

// EffectiveTokenQuotaDailyUsed returns effective daily used amount, considering window expiry
func (k *APIKey) EffectiveTokenQuotaDailyUsed() int64 {
	if k.TokenQuotaDailyUsed == nil {
		return 0
	}
	if k.TokenQuotaDailyStart != nil && time.Since(*k.TokenQuotaDailyStart) >= 24*time.Hour {
		return 0 // Window expired
	}
	return *k.TokenQuotaDailyUsed
}

// EffectiveTokenQuotaWeeklyUsed returns effective weekly used amount, considering window expiry
func (k *APIKey) EffectiveTokenQuotaWeeklyUsed() int64 {
	if k.TokenQuotaWeeklyUsed == nil {
		return 0
	}
	if k.TokenQuotaWeeklyStart != nil && time.Since(*k.TokenQuotaWeeklyStart) >= 7*24*time.Hour {
		return 0 // Window expired
	}
	return *k.TokenQuotaWeeklyUsed
}

// EffectiveTokenQuotaMonthlyUsed returns effective monthly used amount, considering window expiry
func (k *APIKey) EffectiveTokenQuotaMonthlyUsed() int64 {
	if k.TokenQuotaMonthlyUsed == nil {
		return 0
	}
	if k.TokenQuotaMonthlyStart != nil {
		// Check if we're still in the same month
		start := *k.TokenQuotaMonthlyStart
		now := time.Now()
		if start.Year() != now.Year() || start.Month() != now.Month() {
			return 0 // Window expired (new month)
		}
	}
	return *k.TokenQuotaMonthlyUsed
}

// IsTokenQuotaDailyExhausted checks if daily token quota is exhausted
func (k *APIKey) IsTokenQuotaDailyExhausted() bool {
	quota := k.GetEffectiveTokenQuotaDaily()
	if quota <= 0 {
		return false // No limit
	}
	return k.EffectiveTokenQuotaDailyUsed() >= quota
}

// IsTokenQuotaWeeklyExhausted checks if weekly token quota is exhausted
func (k *APIKey) IsTokenQuotaWeeklyExhausted() bool {
	quota := k.GetEffectiveTokenQuotaWeekly()
	if quota <= 0 {
		return false // No limit
	}
	return k.EffectiveTokenQuotaWeeklyUsed() >= quota
}

// IsTokenQuotaMonthlyExhausted checks if monthly token quota is exhausted
func (k *APIKey) IsTokenQuotaMonthlyExhausted() bool {
	quota := k.GetEffectiveTokenQuotaMonthly()
	if quota <= 0 {
		return false // No limit
	}
	return k.EffectiveTokenQuotaMonthlyUsed() >= quota
}

// AnyTokenQuotaExhausted checks if any token quota (total, daily, weekly, or monthly) is exhausted
func (k *APIKey) AnyTokenQuotaExhausted() bool {
	return k.IsTokenQuotaExhausted() || k.IsTokenQuotaDailyExhausted() ||
		k.IsTokenQuotaWeeklyExhausted() || k.IsTokenQuotaMonthlyExhausted()
}

// GetDaysUntilExpiry returns days until expiry (-1 for never expires)
func (k *APIKey) GetDaysUntilExpiry() int {
	if k.ExpiresAt == nil {
		return -1 // never expires
	}
	duration := time.Until(*k.ExpiresAt)
	if duration < 0 {
		return 0
	}
	return int(duration.Hours() / 24)
}

// EffectiveUsage5h returns the 5h window usage, or 0 if the window has expired.
func (k *APIKey) EffectiveUsage5h() float64 {
	if IsWindowExpired(k.Window5hStart, RateLimitWindow5h) {
		return 0
	}
	return k.Usage5h
}

// EffectiveUsage1d returns the 1d window usage, or 0 if the window has expired.
func (k *APIKey) EffectiveUsage1d() float64 {
	if IsWindowExpired(k.Window1dStart, RateLimitWindow1d) {
		return 0
	}
	return k.Usage1d
}

// EffectiveUsage7d returns the 7d window usage, or 0 if the window has expired.
func (k *APIKey) EffectiveUsage7d() float64 {
	if IsWindowExpired(k.Window7dStart, RateLimitWindow7d) {
		return 0
	}
	return k.Usage7d
}

// APIKeyListFilters holds optional filtering parameters for listing API keys.
type APIKeyListFilters struct {
	Search  string
	Status  string
	GroupID *int64 // nil=不筛选, 0=无分组, >0=指定分组
}
