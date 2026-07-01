package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type UserSubscriptionRepository interface {
	Create(ctx context.Context, sub *UserSubscription) error
	GetByID(ctx context.Context, id int64) (*UserSubscription, error)
	GetByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error)
	GetActiveByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error)
	Update(ctx context.Context, sub *UserSubscription) error
	Delete(ctx context.Context, id int64) error

	ListByUserID(ctx context.Context, userID int64) ([]UserSubscription, error)
	ListActiveByUserID(ctx context.Context, userID int64) ([]UserSubscription, error)
	ListByGroupID(ctx context.Context, groupID int64, params pagination.PaginationParams) ([]UserSubscription, *pagination.PaginationResult, error)
	List(ctx context.Context, params pagination.PaginationParams, userID, groupID *int64, status, platform, sortBy, sortOrder string) ([]UserSubscription, *pagination.PaginationResult, error)

	ExistsByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (bool, error)
	ExtendExpiry(ctx context.Context, subscriptionID int64, newExpiresAt time.Time) error
	UpdateStatus(ctx context.Context, subscriptionID int64, status string) error
	UpdateNotes(ctx context.Context, subscriptionID int64, notes string) error

	ActivateWindows(ctx context.Context, id int64, start time.Time) error
	ResetDailyUsage(ctx context.Context, id int64, newWindowStart time.Time) error
	ResetWeeklyUsage(ctx context.Context, id int64, newWindowStart time.Time) error
	ResetMonthlyUsage(ctx context.Context, id int64, newWindowStart time.Time) error
	IncrementUsage(ctx context.Context, id int64, costUSD float64) error
	IncrementTokenUsage(ctx context.Context, id int64, tokens int64) error
	// ResetTokenUsage 将 token 用量字段重置为指定值（兑换码续期回拨额度场景）。
	// newTotal 为已计算好的 token_usage_total 目标值（通常 max(0, total-quota)），
	// 周期字段(daily/weekly/monthly)清零、窗口起始时间重置为 now。
	// 注意：此方法已废弃，请使用 AccumulateTokenQuota 替代（支持额度累加）。
	ResetTokenUsage(ctx context.Context, id int64, newTotal int64) error
	// AccumulateTokenQuota 累加 Token 配额总额度（多次兑换额度累加场景）。
	// 不清零使用量，仅更新累计额度。
	AccumulateTokenQuota(ctx context.Context, id int64, newAccumulated int64) error

	BatchUpdateExpiredStatus(ctx context.Context) (int64, error)
}
