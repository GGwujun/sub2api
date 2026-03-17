-- +goose Up
-- +goose StatementBegin
-- Hotfix: ensure groups token quota columns exist to avoid startup query failures
ALTER TABLE groups ADD COLUMN IF NOT EXISTS token_quota BIGINT;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS token_quota_daily BIGINT;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS token_quota_weekly BIGINT;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS token_quota_monthly BIGINT;

COMMENT ON COLUMN groups.token_quota IS 'Total token quota limit (0 = unlimited)';
COMMENT ON COLUMN groups.token_quota_daily IS 'Daily token quota limit (0 = unlimited)';
COMMENT ON COLUMN groups.token_quota_weekly IS 'Weekly token quota limit (0 = unlimited)';
COMMENT ON COLUMN groups.token_quota_monthly IS 'Monthly token quota limit (0 = unlimited)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE groups DROP COLUMN IF EXISTS token_quota_monthly;
ALTER TABLE groups DROP COLUMN IF EXISTS token_quota_weekly;
ALTER TABLE groups DROP COLUMN IF EXISTS token_quota_daily;
ALTER TABLE groups DROP COLUMN IF EXISTS token_quota;
-- +goose StatementEnd
