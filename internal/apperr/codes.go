package apperr

// 错误码按服务域分段，避免跨服务冲突，前端可直接用 switch 判断。
// Error codes are segmented by service domain to avoid cross-service conflicts. Frontend can use switch directly.
//
// 为什么用数字而不是字符串？数字比较快、不会拼错、方便前端 switch。
// Why numbers instead of strings? Faster comparison, no typos, and easy to switch on in frontend code.
//
// 分段规则 / Segment rules:
// 1000-1099: 通用错误（所有服务共用）  Common errors (shared by all services)
// 1100-1199: User service 业务错误      User service business errors
// 2000-2999: Product service            Product service
// 3000-3999: Cart service               Cart service
// 4000-4999: Order service              Order service
// 5000-5999: Admin                      Admin
// 9000-9999: Gateway                    Gateway

const (
	// --- 通用错误 Common errors ---
	ErrCodeBadRequest   = 1000 // 请求格式错误 / Malformed request
	ErrCodeUnauthorized = 1001 // 未认证 / Not authenticated
	ErrCodeForbidden    = 1002 // 无权限 / Not authorized
	ErrCodeNotFound     = 1003 // 资源不存在 / Resource not found
	ErrCodeConflict     = 1004 // 资源冲突 / Resource conflict
	ErrCodeInternal     = 1500 // 服务器内部错误 / Internal server error
	ErrCodeRateLimit    = 1006 // 限流 / Rate limited
	ErrCodeValidation   = 1007 // 校验失败 / Validation failed

	// --- 用户服务 User service (1xxx) ---
	ErrCodeUserNotFound       = 1101 // 用户不存在 / User not found
	ErrCodeUserAlreadyExists  = 1102 // 用户已存在（邮箱重复）/ User already exists (duplicate email)
	ErrCodeInvalidCredentials = 1103 // 密码错误 / Wrong password
	ErrCodeTokenExpired       = 1104 // Token 已过期 / Token expired
	ErrCodeTokenRevoked       = 1105 // Token 已被吊销 / Token revoked
	ErrCodePasswordTooWeak    = 1106 // 密码强度不够 / Password too weak
	ErrCodeEmailNotVerified   = 1107 // 邮箱未验证 / Email not verified
	ErrCodeAddressLimit       = 1108 // 地址数量达到上限 / Address count limit reached

	// --- 商品服务 Product service (2xxx) ---
	ErrCodeProductNotFound    = 2001 // 商品不存在 / Product not found
	ErrCodeSKUNotFound        = 2002 // SKU 不存在 / SKU not found
	ErrCodeStockInsufficient  = 2003 // 库存不足 / Insufficient stock
	ErrCodeCategoryNotFound   = 2004 // 分类不存在 / Category not found
	ErrCodeDuplicateSKUCode   = 2005 // SKU 编码重复 / Duplicate SKU code
	ErrCodeInvalidPrice       = 2006 // 价格无效 / Invalid price
	ErrCodeProductUnavailable = 2007 // 商品不可用（已下架）/ Product unavailable (delisted)

	// --- 购物车服务 Cart service (3xxx) ---
	ErrCodeCartItemNotFound   = 3001 // 购物车项不存在 / Cart item not found
	ErrCodeCartLimitExceeded  = 3002 // 购物车数量超限 / Cart item count exceeded
	ErrCodeCartSKUUnavailable = 3003 // 购物车中 SKU 不可用 / SKU in cart unavailable
	ErrCodeCartPriceChanged   = 3004 // 购物车中商品价格已变动 / Cart item price changed

	// --- 订单服务 Order service (4xxx) ---
	ErrCodeOrderNotFound      = 4001 // 订单不存在 / Order not found
	ErrCodeOrderStatusInvalid = 4002 // 订单状态不允许此操作 / Order status doesn't allow this action
	ErrCodeOrderExpired       = 4003 // 订单已过期（超时未支付）/ Order expired (payment timeout)
	ErrCodeOrderAlreadyPaid   = 4004 // 订单已支付 / Order already paid
	ErrCodeOrderCancelDenied  = 4005 // 订单不允许取消 / Order cancellation denied
	ErrCodePaymentFailed      = 4006 // 支付失败 / Payment failed
	ErrCodeIdempotentConflict = 4007 // 幂等冲突（重复提交）/ Idempotent conflict (duplicate submission)

	// --- 管理员 Admin (5xxx) ---
	ErrCodeAdminNotFound      = 5001 // 管理员不存在 / Admin not found
	ErrCodeAdminAlreadyExists = 5002 // 管理员已存在 / Admin already exists
	ErrCodeAdminInvalidCreds  = 5003 // 管理员密码错误 / Admin wrong password
	ErrCodeAdminTokenExpired  = 5004 // 管理员 Token 过期 / Admin token expired
	ErrCodeAdminForbidden     = 5005 // 管理员无权限 / Admin forbidden
	ErrCodeAdminTokenRevoked  = 5006 // 管理员 Token 已吊销 / Admin token revoked

	// --- 网关 Gateway (9xxx) ---
	ErrCodeRateLimited        = 9001 // 网关限流 / Gateway rate limited
	ErrCodeServiceUnavailable = 9002 // 上游服务不可用 / Upstream service unavailable
)
