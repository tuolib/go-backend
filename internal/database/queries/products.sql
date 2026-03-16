-- 商品查询（固定 CRUD 部分，动态搜索用 squirrel）
-- Product queries (fixed CRUD part — dynamic search uses squirrel)

-- name: GetProductByID :one
-- 按 ID 查商品详情。
-- Get product detail by ID.
SELECT * FROM product_service.products
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetProductBySlug :one
-- 按 slug 查商品。
-- Get product by slug.
SELECT * FROM product_service.products
WHERE slug = $1 AND deleted_at IS NULL;

-- name: CreateProduct :one
-- 创建商品。
-- Create a product.
INSERT INTO product_service.products (id, title, slug, description, brand, status, attributes, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
RETURNING *;

-- name: UpdateProduct :one
-- 更新商品。
-- Update a product.
UPDATE product_service.products
SET title = COALESCE($2, title),
    slug = COALESCE($3, slug),
    description = COALESCE($4, description),
    brand = COALESCE($5, brand),
    status = COALESCE($6, status),
    attributes = COALESCE($7, attributes),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteProduct :exec
-- 软删除商品。
-- Soft-delete a product.
UPDATE product_service.products
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateProductPriceRange :exec
-- 更新商品的价格范围（从 SKU 聚合时调用）。
-- Update product price range (called when aggregating from SKUs).
UPDATE product_service.products
SET min_price = $2, max_price = $3, updated_at = NOW()
WHERE id = $1;

-- name: IncrementProductSales :exec
-- 增加商品销量。
-- Increment product sales count.
UPDATE product_service.products
SET total_sales = total_sales + $2, updated_at = NOW()
WHERE id = $1;

-- name: AddProductCategory :exec
-- 关联商品和分类。
-- Associate product with category.
INSERT INTO product_service.product_categories (product_id, category_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveProductCategory :exec
-- 移除商品和分类的关联。
-- Remove product-category association.
DELETE FROM product_service.product_categories
WHERE product_id = $1 AND category_id = $2;

-- name: ListProductCategories :many
-- 获取商品的所有分类。
-- Get all categories for a product.
SELECT c.* FROM product_service.categories c
JOIN product_service.product_categories pc ON c.id = pc.category_id
WHERE pc.product_id = $1;
