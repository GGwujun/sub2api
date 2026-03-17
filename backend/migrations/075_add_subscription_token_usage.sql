-- +goose Up
-- +goose StatementBegin
-- Add token usage fields to user_subscriptions table for token quota subscription tracking

-- Token usage total
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_usage_total;
ALTER TABLE user_subscriptions ADD COLUMN token_usage_total BIGINT NOT NULL DEFAULT 0;

-- Token usage daily/weekly/monthly
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_usage_daily;
ALTER TABLE user_subscriptions ADD COLUMN token_usage_daily BIGINT NOT NULL DEFAULT 0;

ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_usage_weekly;
ALTER TABLE user_subscriptions ADD COLUMN token_usage_weekly BIGINT NOT NULL DEFAULT 0;

ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_usage_monthly;
ALTER TABLE user_subscriptions ADD COLUMN token_usage_monthly BIGINT NOT NULL DEFAULT 0;

-- Token window start times
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_daily_window_start;
ALTER TABLE user_subscriptions ADD COLUMN token_daily_window_start TIMESTAMPTZ;

ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_weekly_window_start;
ALTER TABLE user_subscriptions ADD COLUMN token_weekly_window_start TIMESTAMPTZ;

ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_monthly_window_start;
ALTER TABLE user_subscriptions ADD COLUMN token_monthly_window_start TIMESTAMPTZ;

COMMENT ON COLUMN user_subscriptions.token_usage_total IS 'Total token usage (for token quota subscriptions)';
COMMENT ON COLUMN user_subscriptions.token_usage_daily IS 'Daily token usage';
COMMENT ON COLUMN user_subscriptions.token_usage_weekly IS 'Weekly token usage';
COMMENT ON COLUMN user_subscriptions.token_usage_monthly IS 'Monthly token usage';
COMMENT ON COLUMN user_subscriptions.token_daily_window_start IS 'Daily token window start time';
COMMENT ON COLUMN user_subscriptions.token_weekly_window_start IS 'Weekly token window start time';
COMMENT ON COLUMN user_subscriptions.token_monthly_window_start IS 'Monthly token window start time';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_usage_total;
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_usage_daily;
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_usage_weekly;
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_usage_monthly;
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_daily_window_start;
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_weekly_window_start;
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS token_monthly_window_start;
-- +goose StatementEnd
