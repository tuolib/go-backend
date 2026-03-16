-- 轮播图查询
-- Banner queries

-- name: ListActiveBanners :many
-- 获取当前活跃的轮播图（在展示时间范围内且已启用）。
-- Get currently active banners (within display time range and enabled).
-- COALESCE 处理 NULL：start_at 为 NULL 表示无开始限制，end_at 为 NULL 表示无结束限制。
-- COALESCE handles NULLs: NULL start_at means no start limit, NULL end_at means no end limit.
SELECT * FROM product_service.banners
WHERE is_active = true
  AND (start_at IS NULL OR start_at <= NOW())
  AND (end_at IS NULL OR end_at >= NOW())
ORDER BY sort_order ASC;

-- name: GetBannerByID :one
-- 按 ID 查轮播图。
-- Get banner by ID.
SELECT * FROM product_service.banners
WHERE id = $1;

-- name: CreateBanner :one
-- 创建轮播图。
-- Create a banner.
INSERT INTO product_service.banners (id, title, subtitle, image_url, link_type, link_value, sort_order, is_active, start_at, end_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
RETURNING *;

-- name: UpdateBanner :one
-- 更新轮播图。
-- Update a banner.
UPDATE product_service.banners
SET title = COALESCE($2, title),
    subtitle = $3,
    image_url = COALESCE($4, image_url),
    link_type = $5,
    link_value = $6,
    sort_order = COALESCE($7, sort_order),
    is_active = COALESCE($8, is_active),
    start_at = $9,
    end_at = $10,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteBanner :exec
-- 删除轮播图。
-- Delete a banner.
DELETE FROM product_service.banners WHERE id = $1;
