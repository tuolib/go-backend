package apperr

// 错误码按服务域分段，避免跨服务冲突，前端可直接用 switch 判断。
// 为什么用数字而不是字符串？数字比较快、不会拼错、方便前端 switch。
//
// 分段规则：
// 1000-1099: 通用错误（所有服务共用）
// 1100-1199: User service 业务错误
// 2000-2999: Product service
// 3000-3999: Cart service
// 4000-4999: Order service
// 5000-5999: Admin
// 9000-9999: Gateway

const (
	// Common
	ErrCodeBadRequest   = 1000
	ErrCodeUnauthorized = 1001
	ErrCodeForbidden    = 1002
	ErrCodeNotFound     = 1003
	ErrCodeConflict     = 1004
	ErrCodeInternal     = 1500
	ErrCodeRateLimit    = 1006
	ErrCodeValidation   = 1007

	// User service (1xxx)
	ErrCodeUserNotFound       = 1101
	ErrCodeUserAlreadyExists  = 1102
	ErrCodeInvalidCredentials = 1103
	ErrCodeTokenExpired       = 1104
	ErrCodeTokenRevoked       = 1105
	ErrCodePasswordTooWeak    = 1106
	ErrCodeEmailNotVerified   = 1107
	ErrCodeAddressLimit       = 1108

	// Product service (2xxx)
	ErrCodeProductNotFound    = 2001
	ErrCodeSKUNotFound        = 2002
	ErrCodeStockInsufficient  = 2003
	ErrCodeCategoryNotFound   = 2004
	ErrCodeDuplicateSKUCode   = 2005
	ErrCodeInvalidPrice       = 2006
	ErrCodeProductUnavailable = 2007

	// Cart service (3xxx)
	ErrCodeCartItemNotFound   = 3001
	ErrCodeCartLimitExceeded  = 3002
	ErrCodeCartSKUUnavailable = 3003
	ErrCodeCartPriceChanged   = 3004

	// Order service (4xxx)
	ErrCodeOrderNotFound      = 4001
	ErrCodeOrderStatusInvalid = 4002
	ErrCodeOrderExpired       = 4003
	ErrCodeOrderAlreadyPaid   = 4004
	ErrCodeOrderCancelDenied  = 4005
	ErrCodePaymentFailed      = 4006
	ErrCodeIdempotentConflict = 4007

	// Admin (5xxx)
	ErrCodeAdminNotFound      = 5001
	ErrCodeAdminAlreadyExists = 5002
	ErrCodeAdminInvalidCreds  = 5003
	ErrCodeAdminTokenExpired  = 5004
	ErrCodeAdminForbidden     = 5005
	ErrCodeAdminTokenRevoked  = 5006

	// Gateway (9xxx)
	ErrCodeRateLimited        = 9001
	ErrCodeServiceUnavailable = 9002
)
