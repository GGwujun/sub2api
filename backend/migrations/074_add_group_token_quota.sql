-- +goose Up
-- +goose StatementBegin
-- Add token quota fields to groups table for token-based quota management

-- Use DROP+ADD pattern for idempotency
ALTER TABLE groups DROP COLUMN IF EXISTS token_quota;
ALTER TABLE groups ADD COLUMN token_quota BIGINT;

ALTER TABLE groups DROP COLUMN IF EXISTS token_quota_daily;
ALTER TABLE groups ADD COLUMN token_quota_daily BIGINT;

ALTER TABLE groups DROP COLUMN IF EXISTS token_quota_weekly;
ALTER TABLE groups ADD COLUMN token_quota_weekly BIGINT;

ALTER TABLE groups DROP COLUMN IF EXISTS token_quota_monthly;
ALTER TABLE groups ADD COLUMN token_quota_monthly BIGINT;

COMMENT ON COLUMN groups.token_quota IS 'Total token quota limit (0 = unlimited)';
COMMENT ON COLUMN groups.token_quota_daily IS 'Daily token quota limit (0 = unlimited)';
COMMENT ON COLUMN groups.token_quota_weekly IS 'Weekly token quota limit (0 = unlimited)';
COMMENT ON COLUMN groups.token_quota_monthly IS 'Monthly token quota limit (0 = unlimited)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE groups DROP COLUMN IF EXISTS token_quota;
ALTER TABLE groups DROP COLUMN IF EXISTS token_quota_daily;
ALTER TABLE groups DROP COLUMN IF EXISTS token_quota_weekly;
ALTER TABLE groups DROP COLUMN IF EXISTS token_quota_monthly;
-- +goose StatementEnd
