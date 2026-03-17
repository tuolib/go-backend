# Phase 4 架构纲领 — 共享中间件

> **目标：** 7 个中间件全部就绪，覆盖请求追踪、日志、跨域、内部鉴权、JWT 认证、限流、幂等
> **状态：** 已完成

---

## 1. 本阶段在整体架构中的位置

```
Phase 0: 项目脚手架（go.mod, 目录结构, Makefile, Docker Compose）     ✅
Phase 1: 通用基础设施（apperr, response, handler/wrap, config, auth, id） ✅
Phase 2: 服务入口（cmd/monolith, gateway, user, product, cart, order）    ✅
Phase 3: 数据库层（PG 连接池, Redis 客户端, 迁移, sqlc, Lua 脚本）     ✅
Phase 4: 共享中间件（requestid, logger, cors, auth, ratelimit 等）     ✅ ← 当前
Phase 5+: 各服务业务逻辑...                                             ⬜
```

Phase 4 是**请求处理的安全屏障**。所有 HTTP 请求在到达 handler 之前，都要经过中间件链的层层处理：追踪、日志、跨域、认证、限流、幂等。

---

## 2. 中间件链顺序

```
请求进入
  │
  ▼ RequestID    — 生成/提取 traceId，注入 context，设置响应头
  ▼ Logger       — 记录请求方法、路径、状态码、耗时
  ▼ CORS         — 处理跨域预检，设置 Access-Control-* 头
  ▼ RateLimit    — Redis ZSET 滑动窗口限流
  ▼ Auth         — JWT 验证 + Redis 黑名单检查（仅需认证的路由）
  ▼ Idempotent   — Redis SET NX 防重复提交（仅特定路由）
  │
  ▼ Handler      — 业务逻辑
  │
  响应返回（Logger 记录最终状态码和耗时）
```

### 为什么是这个顺序？

| 顺序 | 中间件 | 原因 |
|------|--------|------|
| 1 | RequestID | 后续所有中间件和 handler 都依赖 traceId 做日志关联，必须第一个 |
| 2 | Logger | 包住后续所有处理，利用"先进后出"记录完整耗时 |
| 3 | CORS | 预检请求（OPTIONS）在这里直接返回 204，不需要走后续链 |
| 4 | RateLimit | 在认证之前拦截，防止暴力破解密码等攻击 |
| 5 | Auth | 验证用户身份，注入 userId/email/jti |
| 6 | Idempotent | 必须在 Auth 之后（可能需要 userId 构建 key） |

### 内部接口的中间件链

```
/internal/ 路由组：
  RequestID → Logger → InternalOnly（验证共享密钥）→ Handler
```

InternalOnly 不需要 CORS（不是浏览器调用）、不需要 RateLimit（信任内部服务）、不需要 Auth（用共享密钥代替 JWT）。

---

## 3. 整体设计决策

### 3.1 两种中间件签名

```go
// 无配置 — 直接就是中间件
func RequestID(next http.Handler) http.Handler
func Logger(next http.Handler) http.Handler

// 有配置 — 工厂函数，返回中间件
func CORS(cfg CORSConfig) func(http.Handler) http.Handler
func Auth(cfg AuthConfig) func(http.Handler) http.Handler
func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler
func Idempotent(cfg IdempotentConfig) func(http.Handler) http.Handler
func InternalOnly(secret string) func(http.Handler) http.Handler
```

注册时写法：

```go
r.Use(RequestID)                    // 直接传
r.Use(CORS(CORSConfig{...}))       // 先调用工厂函数
```

### 3.2 Context Key 类型安全

定义了两套 context key 类型，避免跨包 key 冲突：

```go
// response 包：导出给 middleware 共用
type ContextKey string
const TraceIDKey ContextKey = "traceId"

// middleware 包：仅 middleware 和 handler 间使用
type contextKey string
const UserIDKey contextKey = "userId"
const UserEmailKey contextKey = "userEmail"
const TokenJTIKey contextKey = "tokenJti"
```

Go 的 `context.Value` 按**类型+值**匹配，即使两个包都用字符串 `"traceId"`，只要类型不同就不会冲突。

### 3.3 Redis 降级策略

所有 Redis 依赖的中间件（auth、ratelimit、idempotent）在 Redis 不可用时**降级放行**：

```
Redis 正常 → 正常检查（黑名单/限流/幂等）
Redis 故障 → 放行 + 记录 Warn 日志
Redis nil  → 放行（测试环境可能不启动 Redis）
```

设计原则：**可用性 > 绝对安全**。Redis 故障影响范围有限（token 15 分钟自然过期），但如果拦截所有请求则整个系统瘫痪。

### 3.4 responseRecorder 模式

Logger 中间件需要记录状态码，但 Go 的 `http.ResponseWriter` 没有 getter。解决方案：

```go
type responseRecorder struct {
    http.ResponseWriter    // 嵌入原始 writer
    statusCode int         // 额外记录
}

func (rec *responseRecorder) WriteHeader(code int) {
    rec.statusCode = code                    // 记下来
    rec.ResponseWriter.WriteHeader(code)     // 转发
}
```

下游 handler 不知道自己用的是 recorder 还是原始 writer — 接口相同，透明代理。

---

## 4. 文件清单

```
internal/middleware/
├── requestid.go       — RequestID 中间件 + context key 定义 + getter 函数
├── logger.go          — Logger 中间件 + responseRecorder
├── cors.go            — CORS 中间件 + 预检处理
├── internal_only.go   — InternalOnly 内部接口守卫
├── auth.go            — Auth JWT 认证 + Redis 黑名单
├── ratelimit.go       — RateLimit Redis ZSET 滑动窗口
└── idempotent.go      — Idempotent Redis SET NX 防重复
```

修改的文件：

```
internal/response/response.go  — 导出 ContextKey 类型和 TraceIDKey 常量
```

---

## 5. 依赖关系

```
middleware 包依赖：
  ├── response    — ContextKey, TraceIDKey, HandleError
  ├── apperr      — NewUnauthorized, NewForbidden, NewRateLimited, AppError codes
  ├── auth        — JWTManager, Claims
  ├── id          — GenerateID (nanoid)
  └── go-redis    — Redis client

response 包不依赖 middleware（单向依赖，无循环）
```

---

## 6. 与 TS 版对比

| 特性 | Go 版 | TS 版 |
|------|-------|-------|
| 中间件签名 | `func(http.Handler) http.Handler` | `MiddlewareHandler<AppEnv>` |
| 用户信息传递 | `context.WithValue` | `req.userId = ...` |
| 密钥比较 | `subtle.ConstantTimeCompare`（防时序攻击） | `===`（无防护） |
| 限流算法 | Redis ZSET 滑动窗口 | Redis ZSET 滑动窗口（相同） |
| 幂等实现 | SET NX 拒绝重复 | SET NX + 缓存响应体 |
| Redis 降级 | 放行 + Warn 日志 | 放行 + Warn 日志（相同） |
