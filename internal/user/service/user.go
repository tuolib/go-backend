package service

import (
	"context"

	"go-backend/internal/database/gen"
	"go-backend/internal/user/dto"
	"go-backend/internal/user/repository"
)

// UserService 处理用户资料相关的业务逻辑。
// UserService handles user profile business logic.
//
// 和 AuthService 分开是因为职责不同：
//   - AuthService: 认证（注册/登录/登出）— 涉及密码、Token、Redis 黑名单
//   - UserService: 资料（查看/更新）— 只涉及用户表的读写
//
// Separated from AuthService due to different responsibilities:
//   - AuthService: authentication (register/login/logout) — involves passwords, tokens, Redis blacklist
//   - UserService: profile (view/update) — only involves user table reads/writes
type UserService struct {
	userRepo repository.UserRepository
}

// NewUserService 创建 UserService 实例。
// NewUserService creates a UserService instance.
func NewUserService(userRepo repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

// GetProfile 获取用户资料。
// GetProfile returns a user's profile.
//
// userID 来自 auth 中间件注入的 context（middleware.UserIDFrom）。
// userID comes from the auth middleware's context injection (middleware.UserIDFrom).
func (s *UserService) GetProfile(ctx context.Context, userID string) (*dto.UserResp, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toUserResp(user), nil
}

// UpdateProfile 更新用户资料。
// UpdateProfile updates a user's profile.
//
// 使用 COALESCE 策略：只更新传入的字段，未传入的保留原值。
// Uses a COALESCE strategy: only updates provided fields; unprovided fields keep their original values.
func (s *UserService) UpdateProfile(ctx context.Context, userID string, input dto.UpdateUserInput) (*dto.UserResp, error) {
	user, err := s.userRepo.Update(ctx, gen.UpdateUserParams{
		ID:        userID,
		Nickname:  repository.StringPtrToText(input.Nickname),
		AvatarUrl: repository.StringPtrToText(input.AvatarURL),
		Phone:     repository.StringPtrToText(input.Phone),
	})
	if err != nil {
		return nil, err
	}
	return toUserResp(user), nil
}

// GetByID 按 ID 获取用户（内部接口用）。
// GetByID gets a user by ID (for internal endpoints).
func (s *UserService) GetByID(ctx context.Context, userID string) (*dto.UserResp, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toUserResp(user), nil
}

// BatchGetByIDs 批量获取用户（内部接口用）。
// BatchGetByIDs gets multiple users by their IDs (for internal endpoints).
//
// 逐个查询而非批量 SQL — 用户数量少（订单最多关联几个用户），简单可靠。
// 未来如果需要大批量查询，可以加一个 BatchGetByIDs 的 sqlc 查询。
// Queries one by one instead of batch SQL — user count is small (an order involves a few users at most), simple and reliable.
// If large batch queries are needed in the future, add a BatchGetByIDs sqlc query.
func (s *UserService) BatchGetByIDs(ctx context.Context, userIDs []string) ([]*dto.UserResp, error) {
	results := make([]*dto.UserResp, 0, len(userIDs))
	for _, uid := range userIDs {
		user, err := s.userRepo.GetByID(ctx, uid)
		if err != nil {
			return nil, err
		}
		results = append(results, toUserResp(user))
	}
	return results, nil
}
