-- SKU 查询
-- SKU queries

-- name: GetSKUByID :one
-- 按 ID 查 SKU。
-- Get SKU by ID.
SELECT * FROM product_service.skus
WHERE id = $1;

-- name: ListSKUsByProduct :many
-- 获取商品的所有 SKU。
-- Get all SKUs for a product.
SELECT * FROM product_service.skus
WHERE product_id = $1
ORDER BY created_at ASC;

-- name: BatchGetSKUs :many
-- 批量查 SKU（下单时用，一次查多个 SKU 的信息）。
-- Batch get SKUs (used when placing an order, fetching multiple SKU info at once).
-- ANY($1::varchar[]) 是 PG 的数组 IN 查询语法。
-- ANY($1::varchar[]) is PG's array-based IN query syntax.
SELECT * FROM product_service.skus
WHERE id = ANY($1::varchar[]);

-- name: CreateSKU :one
-- 创建 SKU。
-- Create a SKU.
INSERT INTO product_service.skus (id, product_id, sku_code, price, compare_price, cost_price, stock, low_stock, weight, attributes, barcode, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
RETURNING *;

-- name: UpdateSKU :one
-- 更新 SKU 信息（不含库存，库存走 Lua 脚本 + 乐观锁）。
-- Update SKU info (excludes stock — stock goes through Lua scripts + optimistic locking).
UPDATE product_service.skus
SET price = COALESCE($2, price),
    compare_price = $3,
    cost_price = $4,
    low_stock = COALESCE($5, low_stock),
    weight = $6,
    attributes = COALESCE($7, attributes),
    barcode = $8,
    status = COALESCE($9, status),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ConfirmStockDeduction :exec
-- 确认库存扣减（支付成功后调用，使用乐观锁防并发）。
-- Confirm stock deduction (called after payment success, uses optimistic lock for concurrency safety).
-- WHERE version = $3 是乐观锁：如果版本号不匹配说明有并发修改，操作失败。
-- WHERE version = $3 is optimistic locking: version mismatch means concurrent modification, operation fails.
UPDATE product_service.skus
SET stock = stock - $2,
    version = version + 1,
    updated_at = NOW()
WHERE id = $1 AND version = $3 AND stock >= $2;

-- name: GetSKUByCode :one
-- 按 SKU 编码查找。
-- Get SKU by code.
SELECT * FROM product_service.skus
WHERE sku_code = $1;
