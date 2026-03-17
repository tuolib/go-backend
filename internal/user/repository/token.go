package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-backend/internal/apperr"
	"go-backend/internal/database/gen"
)

// TokenRepository 定义 refresh token 的数据访问接口。
// TokenRepository defines the interface for refresh token data access.
//
// 为什么 refresh token 要存数据库？
// access token 是无状态的（靠过期时间自动失效），但 refresh token 需要支持主动吊销：
//   - 用户登出 → 吊销当前 refresh token
//   - 密码修改 → 吊销该用户所有 refresh token（强制重新登录）
//   - 安全事件 → 管理员可以吊销任意用户的所有 token
//
// Why store refresh tokens in the database?
// Access tokens are stateless (expire automatically), but refresh tokens need active revocation:
//   - User logout → revoke current refresh token
//   - Password change → revoke all user refresh tokens (force re-login)
//   - Security incident → admin can revoke any user's tokens
type TokenRepository interface {
	// Create 存储新的 refresh token 哈希。
	// Create stores a new refresh token hash.
	Create(ctx context.Context, arg gen.CreateRefreshTokenParams) (gen.UserServiceRefreshToken, error)

	// GetByHash 按 token 哈希查找未吊销的记录。
	// GetByHash finds an unrevoked token record by its hash.
	GetByHash(ctx context.Context, tokenHash string) (gen.UserServiceRefreshToken, error)

	// Revoke 吊销指定的 refresh token（设置 revoked_at）。
	// Revoke revokes a specific refresh token (sets revoked_at).
	Revoke(ctx context.Context, tokenHash string) error

	// RevokeAllByUser 吊销用户的所有 refresh token。
	// RevokeAllByUser revokes all refresh tokens for a user.
	RevokeAllByUser(ctx context.Context, userID string) error

	// DeleteExpired 清理已过期的 token 记录。定时任务调用，避免表无限膨胀。
	// DeleteExpired removes expired token records. Called by scheduled jobs to prevent table bloat.
	DeleteExpired(ctx context.Context) error
}

// tokenRepository 是 TokenRepository 的 PostgreSQL 实现。
// tokenRepository is the PostgreSQL implementation of TokenRepository.
type tokenRepository struct {
	q *gen.Queries
}

// NewTokenRepository 创建 TokenRepository 实例。
// NewTokenRepository creates a TokenRepository instance.
func NewTokenRepository(db gen.DBTX) TokenRepository {
	return &tokenRepository{q: gen.New(db)}
}

func (r *tokenRepository) Create(ctx context.Context, arg gen.CreateRefreshTokenParams) (gen.UserServiceRefreshToken, error) {
	return r.q.CreateRefreshToken(ctx, arg)
}

func (r *tokenRepository) GetByHash(ctx context.Context, tokenHash string) (gen.UserServiceRefreshToken, error) {
	token, err := r.q.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// token 不存在或已被吊销 — 都算 token 无效。
			// Token doesn't exist or already revoked — both mean invalid token.
			return gen.UserServiceRefreshToken{}, apperr.New(
				apperr.ErrCodeTokenRevoked,
				401,
				"refresh token is invalid or revoked",
			)
		}
		return gen.UserServiceRefreshToken{}, err
	}

	// 检查 token 是否已过期。数据库 WHERE 条件只过滤了 revoked_at IS NULL，
	// 没有过滤 expires_at，所以需要在应用层检查。
	// Check if the token has expired. The DB WHERE clause only filters revoked_at IS NULL,
	// not expires_at, so we check expiration at the application layer.
	if token.ExpiresAt.Valid && token.ExpiresAt.Time.Before(time.Now()) {
		return gen.UserServiceRefreshToken{}, apperr.New(
			apperr.ErrCodeTokenExpired,
			401,
			"refresh token has expired",
		)
	}

	return token, nil
}

func (r *tokenRepository) Revoke(ctx context.Context, tokenHash string) error {
	return r.q.RevokeRefreshToken(ctx, tokenHash)
}

func (r *tokenRepository) RevokeAllByUser(ctx context.Context, userID string) error {
	return r.q.RevokeAllUserTokens(ctx, userID)
}

func (r *tokenRepository) DeleteExpired(ctx context.Context) error {
	return r.q.DeleteExpiredTokens(ctx)
}

// ──────────────────────────────────────────────────────────────────────────────
// pgtype 构造辅助
// pgtype construction helpers
// ──────────────────────────────────────────────────────────────────────────────

// NewTimestamptz 将 time.Time 包装为 pgtype.Timestamptz。
// NewTimestamptz wraps a time.Time into pgtype.Timestamptz.
//
// pgx 的时间类型不能直接用 time.Time，需要包装一层标记 Valid=true。
// pgx's time types can't use time.Time directly — they need a wrapper with Valid=true.
func NewTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
