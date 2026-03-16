-- 订单明细查询
-- Order item queries

-- name: CreateOrderItem :one
-- 创建订单明细项。
-- Create an order item.
INSERT INTO order_service.order_items (id, order_id, product_id, sku_id, product_title, sku_attrs, image_url, unit_price, quantity, subtotal, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
RETURNING *;

-- name: ListOrderItems :many
-- 获取订单的所有明细项。
-- Get all items for an order.
SELECT * FROM order_service.order_items
WHERE order_id = $1
ORDER BY created_at ASC;

-- name: CreateOrderAddress :one
-- 创建订单地址快照。
-- Create an order address snapshot.
INSERT INTO order_service.order_addresses (id, order_id, recipient, phone, province, city, district, address, postal_code, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
RETURNING *;

-- name: GetOrderAddress :one
-- 获取订单的收货地址。
-- Get the delivery address for an order.
SELECT * FROM order_service.order_addresses
WHERE order_id = $1;
