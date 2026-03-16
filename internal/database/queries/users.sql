-- 用户表 CRUD 查询（sqlc 固定查询）
-- User table CRUD queries (sqlc fixed queries)

-- name: GetUserByID :one
-- 按 ID 查用户（排除已删除）。
-- Get user by ID (excluding deleted).
SELECT * FROM user_service.users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
-- 按邮箱查用户（登录时用）。
-- Get user by email (used during login).
SELECT * FROM user_service.users
WHERE email = $1 AND deleted_at IS NULL;

-- name: CreateUser :one
-- 创建新用户，RETURNING * 返回包含 created_at 等数据库生成字段的完整记录。
-- Create a new user. RETURNING * returns the full record including DB-generated fields like created_at.
INSERT INTO user_service.users (id, email, password, nickname, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, 'active', NOW(), NOW())
RETURNING *;

-- name: UpdateUser :one
-- 更新用户资料。只更新传入的字段（利用 COALESCE 保留原值）。
-- Update user profile. Only updates provided fields (COALESCE preserves original values).
-- COALESCE($2, nickname) 含义：如果 $2 非 NULL 就用 $2，否则保留原来的 nickname。
-- COALESCE($2, nickname) means: use $2 if non-NULL, otherwise keep the original nickname.
UPDATE user_service.users
SET nickname = COALESCE($2, nickname),
    avatar_url = COALESCE($3, avatar_url),
    phone = COALESCE($4, phone),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateLastLogin :exec
-- 更新最后登录时间。:exec 表示不返回结果。
-- Update last login time. :exec means no result returned.
UPDATE user_service.users
SET last_login = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: SoftDeleteUser :exec
-- 软删除用户（设置 deleted_at 而不是物理删除）。
-- Soft-delete a user (set deleted_at instead of physically deleting).
UPDATE user_service.users
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;
