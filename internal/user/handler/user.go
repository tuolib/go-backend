package handler

import (
	"net/http"

	"go-backend/internal/middleware"
	"go-backend/internal/response"
	"go-backend/internal/user/dto"
	"go-backend/internal/user/service"
)

// UserHandler 处理用户资料相关的 HTTP 请求。
// UserHandler handles user profile HTTP requests.
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler 创建 UserHandler 实例。
// NewUserHandler creates a UserHandler instance.
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// Profile 获取当前登录用户的资料。
// Profile returns the current authenticated user's profile.
//
// POST /api/v1/user/profile（需要认证）
// POST /api/v1/user/profile (requires auth)
func (h *UserHandler) Profile(w http.ResponseWriter, r *http.Request) error {
	// userID 由 auth 中间件从 JWT 中提取并注入 context。
	// userID is extracted from JWT by auth middleware and injected into context.
	userID := middleware.UserIDFrom(r.Context())

	result, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		return err
	}

	return response.Success(w, r, result)
}

// Update 更新当前登录用户的资料。
// Update modifies the current authenticated user's profile.
//
// POST /api/v1/user/update（需要认证）
// POST /api/v1/user/update (requires auth)
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) error {
	userID := middleware.UserIDFrom(r.Context())

	var input dto.UpdateUserInput
	if err := decodeAndValidate(r, &input); err != nil {
		return err
	}

	result, err := h.userService.UpdateProfile(r.Context(), userID, input)
	if err != nil {
		return err
	}

	return response.Success(w, r, result)
}

// ──────────────────────────────────────────────────────────────────────────────
// 内部接口 Handler
// Internal endpoint handlers
// ──────────────────────────────────────────────────────────────────────────────
//
// 这些接口不经过 auth 中间件，而是通过 InternalOnly 中间件校验 X-Internal-Secret。
// These endpoints bypass auth middleware and use InternalOnly middleware to verify X-Internal-Secret.
//
// 调用方：其他微服务（如订单服务查用户地址）。
// Callers: other microservices (e.g., order service fetching user address).

// Detail 内部接口：按 ID 获取用户。
// Detail is an internal endpoint: get user by ID.
//
// POST /internal/user/detail
func (h *UserHandler) Detail(w http.ResponseWriter, r *http.Request) error {
	var input dto.GetUserDetailInput
	if err := decodeAndValidate(r, &input); err != nil {
		return err
	}

	result, err := h.userService.GetByID(r.Context(), input.ID)
	if err != nil {
		return err
	}

	return response.Success(w, r, result)
}

// Batch 内部接口：批量获取用户。
// Batch is an internal endpoint: batch get users.
//
// POST /internal/user/batch
func (h *UserHandler) Batch(w http.ResponseWriter, r *http.Request) error {
	var input dto.BatchGetUsersInput
	if err := decodeAndValidate(r, &input); err != nil {
		return err
	}

	result, err := h.userService.BatchGetByIDs(r.Context(), input.IDs)
	if err != nil {
		return err
	}

	return response.Success(w, r, result)
}
