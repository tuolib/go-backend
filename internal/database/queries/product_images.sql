-- 商品图片查询
-- Product image queries

-- name: ListProductImages :many
-- 获取商品的所有图片，主图排前面。
-- Get all images for a product, primary image first.
SELECT * FROM product_service.product_images
WHERE product_id = $1
ORDER BY is_primary DESC, sort_order ASC;

-- name: CreateProductImage :one
-- 创建商品图片。
-- Create a product image.
INSERT INTO product_service.product_images (id, product_id, url, alt_text, is_primary, sort_order, created_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW())
RETURNING *;

-- name: DeleteProductImage :exec
-- 删除商品图片。
-- Delete a product image.
DELETE FROM product_service.product_images
WHERE id = $1 AND product_id = $2;

-- name: ClearPrimaryImage :exec
-- 清除商品的所有主图标记（设置新主图前调用）。
-- Clear all primary flags for a product (called before setting a new primary).
UPDATE product_service.product_images
SET is_primary = false
WHERE product_id = $1 AND is_primary = true;
