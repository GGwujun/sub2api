-- 137_add_token_quota_accumulated.sql
-- 新增 token_quota_accumulated 字段，支持多次兑换额度累加
-- 业务场景：同一订阅多次兑换，额度累加，已使用量保持不变

-- 添加 token_quota_accumulated 字段
ALTER TABLE user_subscriptions
ADD COLUMN IF NOT EXISTS token_quota_accumulated BIGINT NOT NULL DEFAULT 0;

-- 添加字段注释
COMMENT ON COLUMN user_subscriptions.token_quota_accumulated IS '累计获得的 Token 配额总额度（多次兑换累加）';

-- 为现有数据初始化：将已存在的 token_quota 订阅的 accumulated 设为分组的 token_quota
-- 这样现有订阅也能正确使用累计额度判断
UPDATE user_subscriptions us
SET token_quota_accumulated = COALESCE(g.token_quota, 0)
FROM groups g
WHERE us.group_id = g.id
  AND g.subscription_type = 'token_quota'
  AND g.token_quota IS NOT NULL
  AND us.deleted_at IS NULL
  AND us.token_quota_accumulated = 0;

-- 创建索引优化查询
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_token_quota_accumulated
ON user_subscriptions(token_quota_accumulated)
WHERE deleted_at IS NULL;