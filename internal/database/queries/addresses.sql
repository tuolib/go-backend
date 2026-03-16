-- 收货地址 CRUD 查询
-- User address CRUD queries

-- name: ListAddressesByUser :many
-- 获取用户的所有收货地址，默认地址排在前面。
-- Get all addresses for a user, default address first.
SELECT * FROM user_service.user_addresses
WHERE user_id = $1
ORDER BY is_default DESC, created_at DESC;

-- name: GetAddressByID :one
-- 按 ID 查地址（同时校验 user_id，防止越权访问别人的地址）。
-- Get address by ID (also verify user_id to prevent accessing another user's address).
SELECT * FROM user_service.user_addresses
WHERE id = $1 AND user_id = $2;

-- name: CreateAddress :one
-- 创建新地址。
-- Create a new address.
INSERT INTO user_service.user_addresses (id, user_id, label, recipient, phone, province, city, district, address, postal_code, is_default, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
RETURNING *;

-- name: UpdateAddress :one
-- 更新地址。COALESCE 保留未传入字段的原值。
-- Update address. COALESCE preserves original values for unprovided fields.
UPDATE user_service.user_addresses
SET label = COALESCE($3, label),
    recipient = COALESCE($4, recipient),
    phone = COALESCE($5, phone),
    province = COALESCE($6, province),
    city = COALESCE($7, city),
    district = COALESCE($8, district),
    address = COALESCE($9, address),
    postal_code = COALESCE($10, postal_code),
    updated_at = NOW()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteAddress :exec
-- 删除地址（物理删除）。
-- Delete an address (hard delete).
DELETE FROM user_service.user_addresses
WHERE id = $1 AND user_id = $2;

-- name: ClearDefaultAddress :exec
-- 清除用户的所有默认地址标记（设置新默认前调用）。
-- Clear all default flags for a user (called before setting a new default).
UPDATE user_service.user_addresses
SET is_default = false, updated_at = NOW()
WHERE user_id = $1 AND is_default = true;

-- name: SetDefaultAddress :exec
-- 将指定地址设为默认。
-- Set a specific address as default.
UPDATE user_service.user_addresses
SET is_default = true, updated_at = NOW()
WHERE id = $1 AND user_id = $2;

-- name: CountAddressesByUser :one
-- 统计用户地址数量（用于限制最大地址数）。
-- Count user addresses (used to enforce maximum address limit).
SELECT COUNT(*) FROM user_service.user_addresses
WHERE user_id = $1;
