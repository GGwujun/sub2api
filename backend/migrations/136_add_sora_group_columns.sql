-- Migration: 136_add_sora_group_columns
-- Re-add Sora-related columns to groups table for sora platform support

-- ============================================================
-- 1. Add Sora pricing columns to groups table
-- ============================================================
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS sora_image_price_360 DECIMAL(20,8),
    ADD COLUMN IF NOT EXISTS sora_image_price_540 DECIMAL(20,8),
    ADD COLUMN IF NOT EXISTS sora_video_price_per_request DECIMAL(20,8),
    ADD COLUMN IF NOT EXISTS sora_video_price_per_request_hd DECIMAL(20,8);

-- ============================================================
-- 2. Add comments for documentation
-- ============================================================
COMMENT ON COLUMN groups.sora_image_price_360 IS 'Sora 360p 图片生成价格';
COMMENT ON COLUMN groups.sora_image_price_540 IS 'Sora 540p 图片生成价格';
COMMENT ON COLUMN groups.sora_video_price_per_request IS 'Sora 标准视频每次请求价格';
COMMENT ON COLUMN groups.sora_video_price_per_request_hd IS 'Sora HD 视频每次请求价格';