-- Refresh token 查询
-- Refresh token queries

-- name: CreateRefreshToken :one
-- 存储 refresh token 哈希。
-- Store a refresh token hash.
INSERT INTO user_service.refresh_tokens (id, user_id, token_hash, expires_at, created_at)
VALUES ($1, $2, $3, $4, NOW())
RETURNING *;

-- name: GetRefreshTokenByHash :one
-- 按 token 哈希查找（验证 refresh token 时用）。
-- Find by token hash (used when verifying a refresh token).
SELECT * FROM user_service.refresh_tokens
WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshToken :exec
-- 吊销指定的 refresh token（登出时用）。
-- Revoke a specific refresh token (used during logout).
UPDATE user_service.refresh_tokens
SET revoked_at = NOW()
WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: RevokeAllUserTokens :exec
-- 吊销用户的所有 refresh token（强制登出所有设备时用）。
-- Revoke all refresh tokens for a user (force logout from all devices).
UPDATE user_service.refresh_tokens
SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredTokens :exec
-- 清理过期的 token 记录（定时任务用）。
-- Clean up expired token records (for scheduled cleanup jobs).
DELETE FROM user_service.refresh_tokens
WHERE expires_at < NOW();
