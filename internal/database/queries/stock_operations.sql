-- 库存操作记录查询（审计用）
-- Stock operation record queries (for auditing)

-- name: CreateStockOperation :one
-- 记录一次库存操作。
-- Record a stock operation.
INSERT INTO order_service.stock_operations (id, sku_id, order_id, type, quantity, created_at)
VALUES ($1, $2, $3, $4, $5, NOW())
RETURNING *;

-- name: ListStockOperationsBySKU :many
-- 按 SKU 查库存操作记录（审计时用）。
-- List stock operations by SKU (for auditing).
SELECT * FROM order_service.stock_operations
WHERE sku_id = $1
ORDER BY created_at DESC;

-- name: ListStockOperationsByOrder :many
-- 按订单查库存操作记录（排查问题时用）。
-- List stock operations by order (for troubleshooting).
SELECT * FROM order_service.stock_operations
WHERE order_id = $1
ORDER BY created_at ASC;
