-- Repair token quota schema after earlier goose-style migrations were executed by the custom runner as full files.
-- Forward-only: restore required token columns/indexes if they were dropped on fresh installs.

ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota BIGINT NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota_used BIGINT NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota_daily BIGINT;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota_daily_used BIGINT NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota_daily_start TIMESTAMP WITH TIME ZONE;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota_weekly BIGINT;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota_weekly_used BIGINT NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota_weekly_start TIMESTAMP WITH TIME ZONE;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota_monthly BIGINT;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota_monthly_used BIGINT NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota_monthly_start TIMESTAMP WITH TIME ZONE;

COMMENT ON COLUMN api_keys.token_quota IS 'Token quota limit (0 = unlimited)';
COMMENT ON COLUMN api_keys.token_quota_used IS 'Used token quota amount';
COMMENT ON COLUMN api_keys.token_quota_daily IS 'Daily token quota limit (0 = unlimited)';
COMMENT ON COLUMN api_keys.token_quota_daily_used IS 'Used daily token quota amount';
COMMENT ON COLUMN api_keys.token_quota_daily_start IS 'Daily token quota window start time';
COMMENT ON COLUMN api_keys.token_quota_weekly IS 'Weekly token quota limit (0 = unlimited)';
COMMENT ON COLUMN api_keys.token_quota_weekly_used IS 'Used weekly token quota amount';
COMMENT ON COLUMN api_keys.token_quota_weekly_start IS 'Weekly token quota window start time';
COMMENT ON COLUMN api_keys.token_quota_monthly IS 'Monthly token quota limit (0 = unlimited)';
COMMENT ON COLUMN api_keys.token_quota_monthly_used IS 'Used monthly token quota amount';
COMMENT ON COLUMN api_keys.token_quota_monthly_start IS 'Monthly token quota window start time';

CREATE INDEX IF NOT EXISTS idx_api_keys_token_quota ON api_keys(token_quota, token_quota_used) WHERE deleted_at IS NULL AND token_quota > 0;
CREATE INDEX IF NOT EXISTS idx_api_keys_token_quota_daily ON api_keys(token_quota_daily, token_quota_daily_used) WHERE deleted_at IS NULL AND token_quota_daily IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_token_quota_weekly ON api_keys(token_quota_weekly, token_quota_weekly_used) WHERE deleted_at IS NULL AND token_quota_weekly IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_token_quota_monthly ON api_keys(token_quota_monthly, token_quota_monthly_used) WHERE deleted_at IS NULL AND token_quota_monthly IS NOT NULL;

ALTER TABLE groups ADD COLUMN IF NOT EXISTS token_quota BIGINT;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS token_quota_daily BIGINT;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS token_quota_weekly BIGINT;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS token_quota_monthly BIGINT;

COMMENT ON COLUMN groups.token_quota IS 'Total token quota limit (0 = unlimited)';
COMMENT ON COLUMN groups.token_quota_daily IS 'Daily token quota limit (0 = unlimited)';
COMMENT ON COLUMN groups.token_quota_weekly IS 'Weekly token quota limit (0 = unlimited)';
COMMENT ON COLUMN groups.token_quota_monthly IS 'Monthly token quota limit (0 = unlimited)';

ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS token_usage_total BIGINT NOT NULL DEFAULT 0;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS token_usage_daily BIGINT NOT NULL DEFAULT 0;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS token_usage_weekly BIGINT NOT NULL DEFAULT 0;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS token_usage_monthly BIGINT NOT NULL DEFAULT 0;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS token_daily_window_start TIMESTAMPTZ;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS token_weekly_window_start TIMESTAMPTZ;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS token_monthly_window_start TIMESTAMPTZ;

COMMENT ON COLUMN user_subscriptions.token_usage_total IS 'Total token usage (for token quota subscriptions)';
COMMENT ON COLUMN user_subscriptions.token_usage_daily IS 'Daily token usage';
COMMENT ON COLUMN user_subscriptions.token_usage_weekly IS 'Weekly token usage';
COMMENT ON COLUMN user_subscriptions.token_usage_monthly IS 'Monthly token usage';
COMMENT ON COLUMN user_subscriptions.token_daily_window_start IS 'Daily token window start time';
COMMENT ON COLUMN user_subscriptions.token_weekly_window_start IS 'Weekly token window start time';
COMMENT ON COLUMN user_subscriptions.token_monthly_window_start IS 'Monthly token window start time';
