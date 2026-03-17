-- +goose Up
-- +goose StatementBegin
-- Add daily/weekly/monthly token quota fields to api_keys table
-- Using DROP+ADD pattern for idempotency

-- Daily token quota
ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_daily;
ALTER TABLE api_keys ADD COLUMN token_quota_daily BIGINT;

ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_daily_used;
ALTER TABLE api_keys ADD COLUMN token_quota_daily_used BIGINT NOT NULL DEFAULT 0;

ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_daily_start;
ALTER TABLE api_keys ADD COLUMN token_quota_daily_start TIMESTAMP WITH TIME ZONE;

-- Weekly token quota
ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_weekly;
ALTER TABLE api_keys ADD COLUMN token_quota_weekly BIGINT;

ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_weekly_used;
ALTER TABLE api_keys ADD COLUMN token_quota_weekly_used BIGINT NOT NULL DEFAULT 0;

ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_weekly_start;
ALTER TABLE api_keys ADD COLUMN token_quota_weekly_start TIMESTAMP WITH TIME ZONE;

-- Monthly token quota
ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_monthly;
ALTER TABLE api_keys ADD COLUMN token_quota_monthly BIGINT;

ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_monthly_used;
ALTER TABLE api_keys ADD COLUMN token_quota_monthly_used BIGINT NOT NULL DEFAULT 0;

ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_monthly_start;
ALTER TABLE api_keys ADD COLUMN token_quota_monthly_start TIMESTAMP WITH TIME ZONE;

COMMENT ON COLUMN api_keys.token_quota_daily IS 'Daily token quota limit (0 = unlimited)';
COMMENT ON COLUMN api_keys.token_quota_daily_used IS 'Used daily token quota amount';
COMMENT ON COLUMN api_keys.token_quota_daily_start IS 'Daily token quota window start time';

COMMENT ON COLUMN api_keys.token_quota_weekly IS 'Weekly token quota limit (0 = unlimited)';
COMMENT ON COLUMN api_keys.token_quota_weekly_used IS 'Used weekly token quota amount';
COMMENT ON COLUMN api_keys.token_quota_weekly_start IS 'Weekly token quota window start time';

COMMENT ON COLUMN api_keys.token_quota_monthly IS 'Monthly token quota limit (0 = unlimited)';
COMMENT ON COLUMN api_keys.token_quota_monthly_used IS 'Used monthly token quota amount';
COMMENT ON COLUMN api_keys.token_quota_monthly_start IS 'Monthly token quota window start time';
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin
-- Create indexes for token quota window queries
CREATE INDEX IF NOT EXISTS idx_api_keys_token_quota_daily ON api_keys(token_quota_daily, token_quota_daily_used) WHERE deleted_at IS NULL AND token_quota_daily IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_token_quota_weekly ON api_keys(token_quota_weekly, token_quota_weekly_used) WHERE deleted_at IS NULL AND token_quota_weekly IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_token_quota_monthly ON api_keys(token_quota_monthly, token_quota_monthly_used) WHERE deleted_at IS NULL AND token_quota_monthly IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_api_keys_token_quota_monthly;
DROP INDEX IF EXISTS idx_api_keys_token_quota_weekly;
DROP INDEX IF EXISTS idx_api_keys_token_quota_daily;

ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_monthly_start;
ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_monthly_used;
ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_monthly;

ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_weekly_start;
ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_weekly_used;
ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_weekly;

ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_daily_start;
ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_daily_used;
ALTER TABLE api_keys DROP COLUMN IF EXISTS token_quota_daily;
-- +goose StatementEnd
