package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"go-backend/internal/apperr"
	"go-backend/internal/auth"
	"go-backend/internal/database/gen"
	"go-backend/internal/id"
	"go-backend/internal/user/dto"
	"go-backend/internal/user/repository"
)

// AuthService 处理认证相关的业务逻辑。
// AuthService handles authentication-related business logic.
//
// 它是一个编排者（orchestrator）— 自己不做底层操作，而是协调多个组件完成流程：
//   - UserRepository: 查/建用户
//   - TokenRepository: 存/查/吊销 refresh token
//   - JWTManager: 签发/验证 JWT
//   - Argon2Hasher: 哈希/验证密码
//   - RedisClient: access token 黑名单
//
// It's an orchestrator — it doesn't do low-level operations itself, but coordinates multiple components:
//   - UserRepository: find/create users
//   - TokenRepository: store/find/revoke refresh tokens
//   - JWTManager: sign/verify JWTs
//   - Argon2Hasher: hash/verify passwords
//   - RedisClient: access token blacklist
type AuthService struct {
	userRepo  repository.UserRepository
	tokenRepo repository.TokenRepository
	jwt       *auth.JWTManager
	hasher    *auth.Argon2Hasher
	redis     *redis.Client // 可能为 nil（降级运行）/ May be nil (degraded mode)
}

// NewAuthService 通过构造函数注入所有依赖。
// NewAuthService injects all dependencies via constructor.
func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	jwt *auth.JWTManager,
	hasher *auth.Argon2Hasher,
	redisClient *redis.Client,
) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		jwt:       jwt,
		hasher:    hasher,
		redis:     redisClient,
	}
}

// Register 用户注册。
// Register creates a new user account.
//
// 流程 / Flow:
//  1. 检查邮箱是否已注册 → 409 Conflict
//  2. 哈希密码（Argon2id）
//  3. 生成 nanoid 作为用户 ID
//  4. 创建用户记录
//  5. 签发双 Token
func (s *AuthService) Register(ctx context.Context, input dto.RegisterInput) (*dto.AuthResp, error) {
	// ── 1. 检查邮箱唯一性 ─────────────────────────────────────
	// ── 1. Check email uniqueness ─────────────────────────────
	exists, err := s.userRepo.EmailExists(ctx, input.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.New(apperr.ErrCodeUserAlreadyExists, 409, "email already registered")
	}

	// ── 2. 哈希密码 ──────────────────────────────────────────
	// ── 2. Hash password ─────────────────────────────────────
	//
	// 绝不存明文密码。Argon2id 自动生成随机盐，同一密码每次哈希结果不同。
	// Never store plaintext passwords. Argon2id auto-generates a random salt; same password → different hash each time.
	hashed, err := s.hasher.HashPassword(input.Password)
	if err != nil {
		return nil, apperr.NewInternal("failed to hash password")
	}

	// ── 3. 创建用户 ──────────────────────────────────────────
	// ── 3. Create user ───────────────────────────────────────
	userID, err := id.GenerateID()
	if err != nil {
		return nil, apperr.NewInternal("failed to generate user id")
	}

	user, err := s.userRepo.Create(ctx, gen.CreateUserParams{
		ID:       userID,
		Email:    input.Email,
		Password: hashed,
		Nickname: repository.StringPtrToText(ptrIfNotEmpty(input.Nickname)),
	})
	if err != nil {
		return nil, err
	}

	// ── 4. 签发双 Token ──────────────────────────────────────
	// ── 4. Issue dual tokens ─────────────────────────────────
	return s.issueTokens(ctx, user)
}

// Login 用户登录。
// Login authenticates a user.
//
// 流程 / Flow:
//  1. 按邮箱查用户 → 不存在返回 401（不是 404，防止邮箱枚举）
//  2. 验证密码 → 不匹配返回 401
//  3. 更新最后登录时间
//  4. 签发双 Token
func (s *AuthService) Login(ctx context.Context, input dto.LoginInput) (*dto.AuthResp, error) {
	// ── 1. 查用户 ────────────────────────────────────────────
	// ── 1. Find user ─────────────────────────────────────────
	//
	// 安全设计：无论是"邮箱不存在"还是"密码错误"，都返回同一个错误消息。
	// 防止攻击者通过不同错误消息枚举有效邮箱。
	// Security: return the same error message for both "email not found" and "wrong password".
	// Prevents attackers from enumerating valid emails via different error messages.
	user, err := s.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		var appErr *apperr.AppError
		if errors.As(err, &appErr) && appErr.Code == apperr.ErrCodeUserNotFound {
			return nil, apperr.New(apperr.ErrCodeInvalidCredentials, 401, "invalid email or password")
		}
		return nil, err
	}

	// ── 2. 验证密码 ──────────────────────────────────────────
	// ── 2. Verify password ───────────────────────────────────
	match, err := s.hasher.VerifyPassword(input.Password, user.Password)
	if err != nil {
		return nil, apperr.NewInternal("failed to verify password")
	}
	if !match {
		return nil, apperr.New(apperr.ErrCodeInvalidCredentials, 401, "invalid email or password")
	}

	// ── 3. 更新登录时间 ──────────────────────────────────────
	// ── 3. Update last login ─────────────────────────────────
	//
	// 非关键操作 — 失败只记日志，不影响登录。
	// Non-critical operation — only log on failure, don't block login.
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		slog.WarnContext(ctx, "failed to update last login", "error", err, "userId", user.ID)
	}

	// ── 4. 签发双 Token ──────────────────────────────────────
	// ── 4. Issue dual tokens ─────────────────────────────────
	return s.issueTokens(ctx, user)
}

// Refresh 刷新访问令牌。
// Refresh rotates the access and refresh tokens.
//
// 流程 / Flow:
//  1. SHA-256 哈希客户端发来的 refresh token
//  2. 在数据库中查找该哈希 → 不存在或已吊销返回 401
//  3. 验证 refresh token 的 JWT 签名和有效期
//  4. 吊销旧 refresh token（一次性使用，防重放攻击）
//  5. 签发全新的双 Token
//
// 为什么每次刷新都生成新的 refresh token（旋转策略）？
// 如果旧 refresh token 被窃取，攻击者使用它刷新后，合法用户的 token 就失效了 —
// 用户会发现登录失效，从而意识到账号可能被盗。
// Why generate a new refresh token on every refresh (rotation strategy)?
// If the old refresh token is stolen and used by an attacker, the legitimate user's token is invalidated —
// the user notices they're logged out, alerting them to a potential compromise.
func (s *AuthService) Refresh(ctx context.Context, input dto.RefreshInput) (*dto.AuthResp, error) {
	// ── 1. 哈希 token ────────────────────────────────────────
	// ── 1. Hash the token ────────────────────────────────────
	tokenHash := auth.HashToken(input.RefreshToken)

	// ── 2. 查数据库 ──────────────────────────────────────────
	// ── 2. Find in database ──────────────────────────────────
	tokenRecord, err := s.tokenRepo.GetByHash(ctx, tokenHash)
	if err != nil {
		return nil, err // 已包含 AppError（TokenRevoked / TokenExpired）/ Already contains AppError
	}

	// ── 3. 验证 JWT ──────────────────────────────────────────
	// ── 3. Verify JWT ────────────────────────────────────────
	//
	// 数据库记录存在只代表 token 没被吊销，还需要验证 JWT 签名确保 token 没被篡改。
	// DB record existence only means the token isn't revoked — we still need to verify the JWT signature to ensure it hasn't been tampered with.
	claims, err := s.jwt.VerifyRefreshToken(input.RefreshToken)
	if err != nil {
		return nil, apperr.New(apperr.ErrCodeTokenExpired, 401, "invalid or expired refresh token")
	}

	// ── 4. 吊销旧 token ─────────────────────────────────────
	// ── 4. Revoke old token ──────────────────────────────────
	if err := s.tokenRepo.Revoke(ctx, tokenHash); err != nil {
		return nil, err
	}

	// ── 5. 查用户（确保用户仍然存在且未被删除）────────────────
	// ── 5. Find user (ensure user still exists and isn't deleted) ──
	user, err := s.userRepo.GetByID(ctx, tokenRecord.UserID)
	if err != nil {
		return nil, err
	}

	// ── 6. 签发新双 Token ────────────────────────────────────
	// ── 6. Issue new dual tokens ─────────────────────────────
	_ = claims // claims 已在 Step 3 验证，这里用 user 信息签发新 token / claims verified in step 3; use user info for new tokens
	return s.issueTokens(ctx, user)
}

// Logout 用户登出。
// Logout signs the user out.
//
// 流程 / Flow:
//  1. 如果传了 refreshToken → 哈希后吊销数据库记录
//  2. 把当前 access token 的 JTI 加入 Redis 黑名单
//
// 为什么要两步？refresh token 吊销防止换取新 access token；
// access token 加黑名单让当前 token 立即失效（否则要等 15 分钟自然过期）。
// Why two steps? Revoking refresh token prevents getting new access tokens;
// blacklisting access token JTI makes the current token immediately invalid (otherwise wait 15 min for natural expiry).
func (s *AuthService) Logout(ctx context.Context, input dto.LogoutInput, accessJTI string) error {
	// ── 1. 吊销 refresh token ────────────────────────────────
	// ── 1. Revoke refresh token ──────────────────────────────
	if input.RefreshToken != "" {
		tokenHash := auth.HashToken(input.RefreshToken)
		if err := s.tokenRepo.Revoke(ctx, tokenHash); err != nil {
			// 吊销失败只记日志，不阻断登出。token 最终会自然过期。
			// Log revocation failure but don't block logout. Token will eventually expire naturally.
			slog.WarnContext(ctx, "failed to revoke refresh token", "error", err)
		}
	}

	// ── 2. Access token JTI 加入 Redis 黑名单 ────────────────
	// ── 2. Add access token JTI to Redis blacklist ───────────
	//
	// TTL 设为 access token 的有效期（15分钟）。过期后 token 本身也失效了，
	// 黑名单记录自动清理，不会无限膨胀。
	// TTL is set to the access token's lifetime (15 min). After expiry, the token itself is invalid,
	// and the blacklist entry auto-cleans — no unbounded growth.
	if s.redis != nil && accessJTI != "" {
		blacklistKey := "blacklist:jti:" + accessJTI
		if err := s.redis.Set(ctx, blacklistKey, "1", s.jwt.AccessExpiresIn()).Err(); err != nil {
			slog.WarnContext(ctx, "failed to blacklist access token", "error", err)
		}
	}

	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// 内部方法
// Internal methods
// ──────────────────────────────────────────────────────────────────────────────

// issueTokens 签发 access + refresh 双 Token，并将 refresh token 哈希存入数据库。
// issueTokens signs access + refresh dual tokens and stores the refresh token hash in the database.
//
// 被 Register、Login、Refresh 三个方法复用。
// Shared by Register, Login, and Refresh.
func (s *AuthService) issueTokens(ctx context.Context, user gen.UserServiceUser) (*dto.AuthResp, error) {
	// 为每个 token 生成唯一的 JTI（JSON Token ID），用于吊销追踪。
	// Generate a unique JTI (JSON Token ID) for each token, used for revocation tracking.
	accessJTI, err := id.GenerateID()
	if err != nil {
		return nil, apperr.NewInternal("failed to generate access jti")
	}
	refreshJTI, err := id.GenerateID()
	if err != nil {
		return nil, apperr.NewInternal("failed to generate refresh jti")
	}

	// 签发 access token（短命：15 分钟）。
	// Sign access token (short-lived: 15 min).
	accessToken, err := s.jwt.SignAccessToken(user.ID, user.Email, accessJTI)
	if err != nil {
		return nil, apperr.NewInternal("failed to sign access token")
	}

	// 签发 refresh token（长命：7 天）。
	// Sign refresh token (long-lived: 7 days).
	refreshToken, err := s.jwt.SignRefreshToken(user.ID, user.Email, refreshJTI)
	if err != nil {
		return nil, apperr.NewInternal("failed to sign refresh token")
	}

	// 将 refresh token 的 SHA-256 哈希存入数据库。
	// Store the refresh token's SHA-256 hash in the database.
	tokenID, err := id.GenerateID()
	if err != nil {
		return nil, apperr.NewInternal("failed to generate token id")
	}

	now := time.Now()
	_, err = s.tokenRepo.Create(ctx, gen.CreateRefreshTokenParams{
		ID:        tokenID,
		UserID:    user.ID,
		TokenHash: auth.HashToken(refreshToken),
		ExpiresAt: repository.NewTimestamptz(now.Add(s.jwt.RefreshExpiresIn())),
	})
	if err != nil {
		return nil, err
	}

	return &dto.AuthResp{
		User:                  toUserResp(user),
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  now.Add(s.jwt.AccessExpiresIn()),
		RefreshTokenExpiresAt: now.Add(s.jwt.RefreshExpiresIn()),
	}, nil
}

// toUserResp 将数据库 model 转为 API 响应结构体。
// toUserResp converts a database model to an API response struct.
//
// 这是 model → DTO 的转换层。关键作用：
//   - 过滤敏感字段（Password 不出现在响应中）
//   - 转换类型（pgtype.Text → *string，pgtype.Timestamptz → *time.Time）
//   - 统一 JSON 命名（数据库 snake_case → API camelCase）
//
// This is the model → DTO conversion layer. Key purposes:
//   - Filter sensitive fields (Password never appears in responses)
//   - Convert types (pgtype.Text → *string, pgtype.Timestamptz → *time.Time)
//   - Unify JSON naming (DB snake_case → API camelCase)
func toUserResp(u gen.UserServiceUser) *dto.UserResp {
	return &dto.UserResp{
		ID:        u.ID,
		Email:     u.Email,
		Nickname:  repository.TextToStringPtr(u.Nickname),
		AvatarURL: repository.TextToStringPtr(u.AvatarUrl),
		Phone:     repository.TextToStringPtr(u.Phone),
		Status:    u.Status,
		LastLogin: repository.TimestamptzToTimePtr(u.LastLogin),
		CreatedAt: u.CreatedAt.Time,
		UpdatedAt: u.UpdatedAt.Time,
	}
}

// ptrIfNotEmpty 如果字符串非空则返回指针，空字符串返回 nil。
// ptrIfNotEmpty returns a pointer to the string if non-empty, or nil if empty.
//
// 用于处理可选字段：JSON 传了空字符串视为"没传"。
// Used for optional fields: an empty string in JSON is treated as "not provided".
func ptrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
