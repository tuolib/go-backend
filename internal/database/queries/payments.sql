-- 支付记录查询
-- Payment record queries

-- name: CreatePaymentRecord :one
-- 创建支付记录。
-- Create a payment record.
INSERT INTO order_service.payment_records (id, order_id, payment_method, amount, status, idempotency_key, created_at, updated_at)
VALUES ($1, $2, $3, $4, 'pending', $5, NOW(), NOW())
RETURNING *;

-- name: GetPaymentByOrder :one
-- 按订单查最新支付记录。
-- Get the latest payment record for an order.
SELECT * FROM order_service.payment_records
WHERE order_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetPaymentByIdempotencyKey :one
-- 按幂等键查支付记录（去重）。
-- Get payment record by idempotency key (dedup).
SELECT * FROM order_service.payment_records
WHERE idempotency_key = $1;

-- name: UpdatePaymentSuccess :exec
-- 标记支付成功。
-- Mark payment as successful.
UPDATE order_service.payment_records
SET status = 'success',
    transaction_id = $2,
    raw_notify = $3,
    updated_at = NOW()
WHERE id = $1 AND status = 'pending';

-- name: UpdatePaymentFailed :exec
-- 标记支付失败。
-- Mark payment as failed.
UPDATE order_service.payment_records
SET status = 'failed',
    raw_notify = $2,
    updated_at = NOW()
WHERE id = $1 AND status = 'pending';
