package apperr

// Error code ranges:
// 1000-1999: Common errors
// 2000-2999: User service
// 3000-3999: Product service
// 4000-4999: Cart service
// 5000-5999: Order service
const (
	// Common errors (1000-1999)
	ErrCodeBadRequest   = 1000
	ErrCodeUnauthorized = 1001
	ErrCodeForbidden    = 1002
	ErrCodeNotFound     = 1003
	ErrCodeConflict     = 1004
	ErrCodeInternal     = 1500
	ErrCodeRateLimit    = 1006
)
