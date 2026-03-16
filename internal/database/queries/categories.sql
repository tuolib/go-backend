-- 分类查询
-- Category queries

-- name: ListCategories :many
-- 获取所有分类（扁平列表），按 sort_order 排序。
-- Get all categories (flat list), sorted by sort_order.
SELECT * FROM product_service.categories
ORDER BY sort_order ASC, name ASC;

-- name: ListActiveCategories :many
-- 获取所有启用的分类。
-- Get all active categories.
SELECT * FROM product_service.categories
WHERE is_active = true
ORDER BY sort_order ASC, name ASC;

-- name: GetCategoryByID :one
-- 按 ID 查分类。
-- Get category by ID.
SELECT * FROM product_service.categories
WHERE id = $1;

-- name: GetCategoryBySlug :one
-- 按 slug 查分类。
-- Get category by slug.
SELECT * FROM product_service.categories
WHERE slug = $1;

-- name: CreateCategory :one
-- 创建分类。
-- Create a category.
INSERT INTO product_service.categories (id, parent_id, name, slug, icon_url, sort_order, is_active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
RETURNING *;

-- name: UpdateCategory :one
-- 更新分类。
-- Update a category.
UPDATE product_service.categories
SET name = COALESCE($2, name),
    slug = COALESCE($3, slug),
    parent_id = $4,
    icon_url = COALESCE($5, icon_url),
    sort_order = COALESCE($6, sort_order),
    is_active = COALESCE($7, is_active),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteCategory :exec
-- 删除分类（物理删除）。
-- Delete a category (hard delete).
DELETE FROM product_service.categories WHERE id = $1;
