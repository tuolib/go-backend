package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"

	"go-backend/internal/apperr"
	"go-backend/internal/middleware"
	"go-backend/internal/response"
	"go-backend/internal/user/dto"
	"go-backend/internal/user/service"
)

// validate 是 validator 的包级实例。
// validate is a package-level validator instance.
//
// validator.New() 内部会缓存 struct tag 的解析结果，所以创建一次、复用即可。
// 并发安全 — validator 的 Struct() 方法可以被多个 goroutine 同时调用。
// validator.New() internally caches parsed struct tags, so create once and reuse.
// Concurrency-safe — validator's Struct() method can be called from multiple goroutines simultaneously.
var validate = validator.New()

// AuthHandler 处理认证相关的 HTTP 请求。
// AuthHandler handles authentication-related HTTP requests.
//
// Handler 的职责是 HTTP 层面的事情：
//   - 解析 JSON body
//   - 校验输入字段
//   - 调用 service 方法
//   - 返回 JSON 响应
//
// Handler responsibilities are HTTP-layer concerns:
//   - Parse JSON body
//   - Validate input fields
//   - Call service methods
//   - Return JSON responses
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler 创建 AuthHandler 实例。
// NewAuthHandler creates an AuthHandler instance.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register 处理注册请求。
// Register handles the registration request.
//
// POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) error {
	var input dto.RegisterInput
	if err := decodeAndValidate(r, &input); err != nil {
		return err
	}

	result, err := h.authService.Register(r.Context(), input)
	if err != nil {
		return err
	}

	return response.Success(w, r, result)
}

// Login 处理登录请求。
// Login handles the login request.
//
// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) error {
	var input dto.LoginInput
	if err := decodeAndValidate(r, &input); err != nil {
		return err
	}

	result, err := h.authService.Login(r.Context(), input)
	if err != nil {
		return err
	}

	return response.Success(w, r, result)
}

// Refresh 处理刷新 token 请求。
// Refresh handles the token refresh request.
//
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) error {
	var input dto.RefreshInput
	if err := decodeAndValidate(r, &input); err != nil {
		return err
	}

	result, err := h.authService.Refresh(r.Context(), input)
	if err != nil {
		return err
	}

	return response.Success(w, r, result)
}

// Logout 处理登出请求。
// Logout handles the logout request.
//
// POST /api/v1/auth/logout
//
// 需要认证 — auth 中间件已经把 userId 和 JTI 注入了 context。
// Requires authentication — auth middleware has already injected userId and JTI into context.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) error {
	var input dto.LogoutInput
	// Logout 的 body 可能为空（refreshToken 是可选的），所以只 decode 不 validate。
	// Logout body may be empty (refreshToken is optional), so only decode, don't validate.
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		// 空 body 或无 body 时 Decode 会报错，但这对 Logout 是合法的。
		// Empty or missing body causes Decode error, but that's valid for Logout.
		input = dto.LogoutInput{}
	}

	// 从 context 中获取当前 access token 的 JTI — auth 中间件注入的。
	// Get the current access token's JTI from context — injected by auth middleware.
	accessJTI := middleware.TokenJTIFrom(r.Context())

	if err := h.authService.Logout(r.Context(), input, accessJTI); err != nil {
		return err
	}

	return response.Success(w, r, nil)
}

// ──────────────────────────────────────────────────────────────────────────────
// 通用辅助函数
// Common helper functions
// ──────────────────────────────────────────────────────────────────────────────

// decodeAndValidate 解析 JSON body 并校验 struct tag。
// decodeAndValidate parses the JSON body and validates struct tags.
//
// 合并了两个常见操作，避免每个 handler 都写一遍：
//
//	var input XxxInput
//	if err := json.NewDecoder(r.Body).Decode(&input); err != nil { ... }
//	if err := validate.Struct(input); err != nil { ... }
//
// Combined two common operations to avoid repeating in every handler.
func decodeAndValidate(r *http.Request, dst any) error {
	// ── Step 1: 解析 JSON ────────────────────────────────────
	// ── Step 1: Parse JSON ───────────────────────────────────
	//
	// json.NewDecoder 比 json.Unmarshal 更高效：
	//   - Decoder 直接从 io.Reader 流式读取，不需要先 ioutil.ReadAll 读到内存
	//   - 对于 HTTP body 这种流式数据源，Decoder 是最佳选择
	//
	// json.NewDecoder is more efficient than json.Unmarshal:
	//   - Decoder reads directly from io.Reader in a streaming fashion, no need for ioutil.ReadAll
	//   - For streaming data sources like HTTP bodies, Decoder is the best choice
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return apperr.NewBadRequest("invalid request body")
	}

	// ── Step 2: 校验 struct tag ──────────────────────────────
	// ── Step 2: Validate struct tags ─────────────────────────
	//
	// 例如 validate:"required,email" 会检查字段是否非空且是合法邮箱格式。
	// E.g., validate:"required,email" checks that the field is non-empty and a valid email format.
	if err := validate.Struct(dst); err != nil {
		return apperr.NewValidation(err.Error())
	}

	return nil
}
