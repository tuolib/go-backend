-- 订单查询
-- Order queries

-- name: GetOrderByID :one
-- 按 ID 查订单。
-- Get order by ID.
SELECT * FROM order_service.orders
WHERE id = $1;

-- name: GetOrderByIDAndUser :one
-- 按 ID + 用户查订单（防止越权查看他人订单）。
-- Get order by ID + user (prevents unauthorized access to others' orders).
SELECT * FROM order_service.orders
WHERE id = $1 AND user_id = $2;

-- name: GetOrderByIdempotencyKey :one
-- 按幂等键查订单（创建订单时去重）。
-- Get order by idempotency key (dedup on order creation).
SELECT * FROM order_service.orders
WHERE idempotency_key = $1;

-- name: CreateOrder :one
-- 创建订单。
-- Create an order.
INSERT INTO order_service.orders (id, order_no, user_id, status, total_amount, discount_amount, pay_amount, remark, idempotency_key, expires_at, created_at, updated_at)
VALUES ($1, $2, $3, 'pending', $4, $5, $6, $7, $8, $9, NOW(), NOW())
RETURNING *;

-- name: UpdateOrderStatus :exec
-- 更新订单状态（使用乐观锁）。
-- Update order status (with optimistic locking).
UPDATE order_service.orders
SET status = $2,
    version = version + 1,
    updated_at = NOW()
WHERE id = $1 AND version = $3;

-- name: UpdateOrderPaid :exec
-- 标记订单为已支付。
-- Mark order as paid.
UPDATE order_service.orders
SET status = 'paid',
    payment_method = $2,
    payment_no = $3,
    paid_at = NOW(),
    version = version + 1,
    updated_at = NOW()
WHERE id = $1 AND status = 'pending' AND version = $4;

-- name: UpdateOrderCancelled :exec
-- 取消订单。
-- Cancel an order.
UPDATE order_service.orders
SET status = 'cancelled',
    cancelled_at = NOW(),
    cancel_reason = $2,
    version = version + 1,
    updated_at = NOW()
WHERE id = $1 AND version = $3;

-- name: ListExpiredPendingOrders :many
-- 查找已超时的待支付订单（定时取消任务用，每次最多 100 条）。
-- Find expired pending orders (for auto-cancellation job, max 100 per batch).
SELECT * FROM order_service.orders
WHERE status = 'pending' AND expires_at < NOW()
ORDER BY expires_at ASC
LIMIT 100;
