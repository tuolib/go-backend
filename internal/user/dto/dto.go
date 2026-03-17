package dto

import "time"

// ──────────────────────────────────────────────────────────────────────────────
// Auth 请求输入
// Auth request inputs
// ──────────────────────────────────────────────────────────────────────────────

// RegisterInput 注册请求参数。
// RegisterInput holds the registration request payload.
//
// validate tag 说明：
//   - required  — 必填字段，不能为空
//   - email     — 必须是合法邮箱格式
//   - min=8     — 最小长度 8 字符
//   - max=100   — 最大长度 100 字符
//   - omitempty — 如果字段为空则跳过后续校验规则
//
// validate tag reference:
//   - required  — field is mandatory and cannot be empty
//   - email     — must be a valid email format
//   - min=8     — minimum length 8 characters
//   - max=100   — maximum length 100 characters
//   - omitempty — skip subsequent rules if field is empty
type RegisterInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=100"`
	Nickname string `json:"nickname" validate:"omitempty,min=1,max=50"`
}

// LoginInput 登录请求参数。
// LoginInput holds the login request payload.
type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshInput 刷新 token 请求参数。
// RefreshInput holds the token refresh request payload.
type RefreshInput struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

// LogoutInput 登出请求参数。refreshToken 可选，传入则吊销该 token。
// LogoutInput holds the logout request payload. refreshToken is optional; if provided, that token is revoked.
type LogoutInput struct {
	RefreshToken string `json:"refreshToken"`
}

// ──────────────────────────────────────────────────────────────────────────────
// User 请求输入
// User request inputs
// ──────────────────────────────────────────────────────────────────────────────

// UpdateUserInput 更新用户资料请求参数。所有字段可选（指针类型，nil 表示不更新）。
// UpdateUserInput holds the update profile request payload. All fields are optional (pointer types; nil means no update).
//
// 为什么用 *string（指针）而不是 string？
// Go 的 string 零值是 ""（空字符串），无法区分「用户没传这个字段」和「用户传了空字符串」。
// 用指针：nil = 没传（不更新），non-nil = 传了（即使是空字符串也更新）。
//
// Why *string (pointer) instead of string?
// Go's string zero value is "" (empty string), making it impossible to distinguish "field not provided" from "field set to empty".
// With pointers: nil = not provided (don't update), non-nil = provided (update even if empty).
type UpdateUserInput struct {
	Nickname  *string `json:"nickname" validate:"omitempty,min=1,max=50"`
	AvatarURL *string `json:"avatarUrl" validate:"omitempty,url"`
	Phone     *string `json:"phone" validate:"omitempty,max=20"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Address 请求输入
// Address request inputs
// ──────────────────────────────────────────────────────────────────────────────

// CreateAddressInput 新建地址请求参数。
// CreateAddressInput holds the create address request payload.
type CreateAddressInput struct {
	Label      string `json:"label" validate:"omitempty,max=50"`
	Recipient  string `json:"recipient" validate:"required,min=1,max=100"`
	Phone      string `json:"phone" validate:"required,min=1,max=20"`
	Province   string `json:"province" validate:"required,min=1,max=50"`
	City       string `json:"city" validate:"required,min=1,max=50"`
	District   string `json:"district" validate:"required,min=1,max=50"`
	Address    string `json:"address" validate:"required,min=1,max=200"`
	PostalCode string `json:"postalCode" validate:"omitempty,max=10"`
	IsDefault  bool   `json:"isDefault"`
}

// UpdateAddressInput 更新地址请求参数。ID 必填，其余字段可选。
// UpdateAddressInput holds the update address request payload. ID is required; other fields are optional.
type UpdateAddressInput struct {
	ID         string  `json:"id" validate:"required"`
	Label      *string `json:"label" validate:"omitempty,max=50"`
	Recipient  *string `json:"recipient" validate:"omitempty,min=1,max=100"`
	Phone      *string `json:"phone" validate:"omitempty,min=1,max=20"`
	Province   *string `json:"province" validate:"omitempty,min=1,max=50"`
	City       *string `json:"city" validate:"omitempty,min=1,max=50"`
	District   *string `json:"district" validate:"omitempty,min=1,max=50"`
	Address    *string `json:"address" validate:"omitempty,min=1,max=200"`
	PostalCode *string `json:"postalCode" validate:"omitempty,max=10"`
	IsDefault  *bool   `json:"isDefault"`
}

// DeleteAddressInput 删除地址请求参数。
// DeleteAddressInput holds the delete address request payload.
type DeleteAddressInput struct {
	ID string `json:"id" validate:"required"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal 请求输入（服务间调用）
// Internal request inputs (inter-service calls)
// ──────────────────────────────────────────────────────────────────────────────

// GetUserDetailInput 内部获取用户详情请求。
// GetUserDetailInput holds the internal get-user-detail request payload.
type GetUserDetailInput struct {
	ID string `json:"id" validate:"required"`
}

// BatchGetUsersInput 内部批量获取用户请求。
// BatchGetUsersInput holds the internal batch-get-users request payload.
type BatchGetUsersInput struct {
	IDs []string `json:"ids" validate:"required,min=1,max=100"`
}

// GetAddressDetailInput 内部获取地址详情请求。
// GetAddressDetailInput holds the internal get-address-detail request payload.
type GetAddressDetailInput struct {
	AddressID string `json:"addressId" validate:"required"`
	UserID    string `json:"userId" validate:"required"`
}

// ──────────────────────────────────────────────────────────────────────────────
// 响应结构体
// Response structs
// ──────────────────────────────────────────────────────────────────────────────

// UserResp 用户信息响应。注意不包含 password 字段 — 永远不把密码返回给客户端。
// UserResp is the user info response. Note: no password field — never send passwords to the client.
//
// JSON tag 使用 camelCase 以匹配 TS 版的 API 契约（如 avatarUrl, lastLogin）。
// JSON tags use camelCase to match the TS version's API contract (e.g., avatarUrl, lastLogin).
type UserResp struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Nickname  *string    `json:"nickname"`
	AvatarURL *string    `json:"avatarUrl"`
	Phone     *string    `json:"phone"`
	Status    string     `json:"status"`
	LastLogin *time.Time `json:"lastLogin"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// AuthResp 认证响应（注册/登录/刷新 共用）。
// AuthResp is the authentication response (shared by register/login/refresh).
type AuthResp struct {
	User                  *UserResp `json:"user,omitempty"`
	AccessToken           string    `json:"accessToken"`
	RefreshToken          string    `json:"refreshToken"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
}

// AddressResp 地址信息响应。
// AddressResp is the address info response.
type AddressResp struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId"`
	Label      *string   `json:"label"`
	Recipient  string    `json:"recipient"`
	Phone      string    `json:"phone"`
	Province   string    `json:"province"`
	City       string    `json:"city"`
	District   string    `json:"district"`
	Address    string    `json:"address"`
	PostalCode *string   `json:"postalCode"`
	IsDefault  bool      `json:"isDefault"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
