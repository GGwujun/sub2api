-- Add token quota fields to api_keys table
-- Token quota allows limiting API keys by token usage (input + output) instead of USD cost

ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota BIGINT NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_quota_used BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN api_keys.token_quota IS 'Token quota limit (0 = unlimited)';
COMMENT ON COLUMN api_keys.token_quota_used IS 'Used token quota amount';

-- Index for token quota queries
CREATE INDEX IF NOT EXISTS idx_api_keys_token_quota ON api_keys(token_quota, token_quota_used) WHERE deleted_at IS NULL AND token_quota > 0;
