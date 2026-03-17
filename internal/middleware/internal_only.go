package middleware

import (
	"crypto/subtle"
	"net/http"

	"go-backend/internal/apperr"
	"go-backend/internal/response"
)

// InternalOnly 验证内部服务间调用的共享密钥，阻止外部直接访问 /internal/ 接口。
// InternalOnly verifies the shared secret for inter-service calls, blocking external access to /internal/ endpoints.
//
// 注册在 /internal/ 路由组上，普通用户路由不受影响。
// Mounted on the /internal/ route group only — normal user routes are unaffected.
func InternalOnly(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get("X-Internal-Secret")

			// subtle.ConstantTimeCompare 防止时序攻击。
			// subtle.ConstantTimeCompare prevents timing attacks.
			//
			// 普通的 == 比较在发现第一个不同字节时就返回，攻击者可以通过测量响应时间
			// 逐字节猜出密钥。ConstantTimeCompare 无论是否匹配都花相同时间。
			// Normal == returns as soon as it finds the first differing byte — an attacker
			// could guess the secret byte-by-byte by measuring response times.
			// ConstantTimeCompare takes the same time regardless of match or mismatch.
			if subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
				response.HandleError(w, r, apperr.NewForbidden("internal access denied"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
