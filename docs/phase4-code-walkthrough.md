# Phase 4 代码逐行详解 — 共享中间件

> 本文档对 Phase 4 所有新增代码进行详细解释，帮助 Go 初学者理解每个中间件的作用和实现细节。

---

## 目录

1. [RequestID 中间件 — requestid.go](#1-requestid-中间件)
2. [Logger 中间件 — logger.go](#2-logger-中间件)
3. [CORS 中间件 — cors.go](#3-cors-中间件)
4. [InternalOnly 中间件 — internal_only.go](#4-internalonly-中间件)
5. [Auth 中间件 — auth.go](#5-auth-中间件)
6. [RateLimit 中间件 — ratelimit.go](#6-ratelimit-中间件)
7. [Idempotent 中间件 — idempotent.go](#7-idempotent-中间件)

---

## 1. RequestID 中间件

**文件：** `internal/middleware/requestid.go`

### 核心概念：中间件模式

Go 的 HTTP 中间件就是一个函数，接收下一个 handler，返回一个新 handler：

```go
func Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 前置逻辑（请求进来时执行）
        next.ServeHTTP(w, r)  // 调用下一层
        // 后置逻辑（响应返回时执行）
    })
}
```

类比 TS 的 Express/Hono 中间件：

```typescript
function middleware(req, res, next) {
    // 前置逻辑
    next();
    // 后置逻辑
}
```

### Context Key 类型安全

```go
type contextKey string
const UserIDKey contextKey = "userId"
```

Go 的 `context.Value` 按**类型+值**匹配。即使另一个包也定义了 `"userId"` 字符串，只要类型不同就不会冲突。这和 TS 的 `Map<string, any>` 不同 — TS 里相同字符串 key 会互相覆盖。

### TraceID 共享策略

TraceID 的 context key 定义在 `response` 包中（`response.TraceIDKey`），因为：
- `middleware` 包写入 traceId
- `response` 包读取 traceId（填入 JSON 响应）
- 两个包必须用同一个 key 类型和值
- `middleware` 已经依赖 `response`，所以 key 放在 `response` 中，避免循环依赖

### Getter 函数

```go
func UserIDFrom(ctx context.Context) string {
    if v, ok := ctx.Value(UserIDKey).(string); ok {
        return v
    }
    return ""
}
```

- `ctx.Value(key)` 返回 `any` 类型（Go 的 `interface{}`，类似 TS 的 `unknown`）
- `.(string)` 是类型断言，把 `any` 转成 `string`
- `ok` 表示断言是否成功（值不存在或类型不匹配时为 `false`）

---

## 2. Logger 中间件

**文件：** `internal/middleware/logger.go`

### 核心概念：responseRecorder

Go 的 `http.ResponseWriter` 接口只有 3 个方法：`Header()`、`Write()`、`WriteHeader()`。没有 `GetStatusCode()` 这种 getter。

解决方案：用 struct 嵌入包装一层。

```go
type responseRecorder struct {
    http.ResponseWriter    // 嵌入，自动继承所有方法
    statusCode int         // 新增字段
}
```

**接口嵌入**：`responseRecorder` 自动满足 `http.ResponseWriter` 接口，因为它继承了嵌入类型的所有方法。我们只覆盖 `WriteHeader`：

```go
func (rec *responseRecorder) WriteHeader(code int) {
    rec.statusCode = code                    // 记录
    rec.ResponseWriter.WriteHeader(code)     // 转发给原始 writer
}
```

用指针接收者（`*responseRecorder`）是因为需要修改 `statusCode` 字段 — 值接收者只会修改副本。

### 默认状态码 200

```go
rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
```

如果 handler 直接调 `w.Write(data)` 而不调 `w.WriteHeader()`，Go 会隐式发送 200，但我们的覆盖方法不会被触发。所以预设 200。

### 日志级别自动适配

```go
level := slog.LevelInfo
if rec.statusCode >= 500 { level = slog.LevelError }
else if rec.statusCode >= 400 { level = slog.LevelWarn }
```

- 2xx/3xx → Info（正常请求）
- 4xx → Warn（客户端错误，如参数不对）
- 5xx → Error（服务端错误，需要排查）

### slog.LogAttrs vs slog.Info

`slog.LogAttrs` 使用 `slog.Attr` 结构体直接传递参数，避免 `interface{}` 装箱开销。在高并发场景（每个请求都记日志），这个优化有意义。

---

## 3. CORS 中间件

**文件：** `internal/middleware/cors.go`

### 核心概念：工厂函数模式

CORS 需要配置（允许哪些域名），所以用工厂函数包一层：

```go
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
    // 初始化逻辑（只执行一次）
    return func(next http.Handler) http.Handler {
        // 这才是真正的中间件
    }
}
```

注册时：`r.Use(CORS(cfg))` — 先调用工厂函数，返回中间件。

### Origin 白名单查找优化

```go
allowed := make(map[string]bool, len(cfg.AllowedOrigins))
for _, o := range cfg.AllowedOrigins {
    allowed[o] = true
}
```

把 slice 转成 map，查找从 O(n) 变成 O(1)。类似 TS 的 `new Set(origins)` + `set.has(origin)`。

### 为什么不用 `*` 通配符？

带 credentials（cookie/token）的请求，浏览器不接受 `Access-Control-Allow-Origin: *`，必须回显具体的 origin。我们的 API 使用 JWT token（credentials），所以必须精确匹配。

### 预检请求（Preflight）

浏览器对跨域的非简单请求（如带 `Content-Type: application/json` 头的 POST）会先发 OPTIONS 请求。中间件回复允许的方法和头后直接返回 204，不走下游。

```go
if r.Method == http.MethodOptions {
    w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
    w.WriteHeader(http.StatusNoContent)
    return  // 不调用 next.ServeHTTP
}
```

---

## 4. InternalOnly 中间件

**文件：** `internal/middleware/internal_only.go`

### 核心概念：时序攻击防护

```go
subtle.ConstantTimeCompare([]byte(provided), []byte(secret))
```

普通的 `==` 比较遇到第一个不同字节就返回，攻击者可以通过测量响应时间逐字节猜出密钥。`ConstantTimeCompare` 无论匹配多少字节，耗时恒定。

### []byte() 类型转换

`subtle.ConstantTimeCompare` 的参数类型是 `[]byte`（字节切片），不是 `string`。Go 不允许隐式类型转换，所以需要 `[]byte(provided)` 显式转换。

---

## 5. Auth 中间件

**文件：** `internal/middleware/auth.go`

### 处理流程

```
Authorization: Bearer eyJhbGci...
  │
  ▼ Step 1: strings.CutPrefix 提取 token
  ▼ Step 2: JWTManager.VerifyAccessToken 验证签名+有效期
  ▼ Step 3: Redis 检查 blacklist:jti:<jti>（登出黑名单）
  ▼ Step 4: context.WithValue 注入 userId/email/jti
  │
  ▼ next.ServeHTTP(w, r.WithContext(ctx))
```

### strings.CutPrefix vs TrimPrefix

```go
// TrimPrefix — 没有前缀时静默返回原字符串（危险）
s := strings.TrimPrefix("NoBearer", "Bearer ")  // → "NoBearer"

// CutPrefix — 第二个返回值告诉你是否有前缀（安全）
s, ok := strings.CutPrefix("NoBearer", "Bearer ")  // → "NoBearer", false
```

### Redis 黑名单降级

```go
if err != nil {
    slog.WarnContext(r.Context(), "redis blacklist check failed, allowing request", "error", err)
    // 不 return，继续往下走
}
```

Redis 故障时放行。最坏情况：已登出的 token 在 Redis 恢复前仍可用，但 token 15 分钟后自然过期。

### r.WithContext(ctx)

```go
ctx = context.WithValue(ctx, UserIDKey, claims.Subject)
next.ServeHTTP(w, r.WithContext(ctx))
```

`r.WithContext(ctx)` 返回 request 的副本（不可变），带上新的 context。下游通过 `middleware.UserIDFrom(r.Context())` 读取。

---

## 6. RateLimit 中间件

**文件：** `internal/middleware/ratelimit.go`

### 核心概念：Redis ZSET 滑动窗口

ZSET（有序集合）的 score 存时间戳，member 存唯一 ID：

```
ZSET gateway:ratelimit:ip:192.168.1.1
┌──────────┬───────────────┐
│ member   │ score (时间戳)  │
├──────────┼───────────────┤
│ nanoid-1 │ 1710000001000 │
│ nanoid-2 │ 1710000002500 │
│ nanoid-3 │ 1710000003000 │
└──────────┴───────────────┘
```

每次请求：
1. `ZREMRANGEBYSCORE` — 删掉窗口外的旧记录
2. `ZADD` — 加入当前请求
3. `ZCARD` — 统计窗口内总数
4. `PEXPIRE` — 设置 key 过期（防止无请求后数据永驻）

### Pipeline 批处理

```go
pipe := cfg.RedisClient.Pipeline()
pipe.ZRemRangeByScore(...)
pipe.ZAdd(...)
countCmd := pipe.ZCard(...)
pipe.PExpire(...)
_, err := pipe.Exec(r.Context())
count := countCmd.Val()
```

4 条命令打包为 1 次网络往返。`countCmd` 在 `Exec` 后才能调用 `.Val()` 读取结果。

### 为什么 member 用 nanoid 而不是时间戳？

ZSET 的 member 必须唯一。同一毫秒的两个请求如果用时间戳做 member，会被 ZSET 去重，计数就少了。

---

## 7. Idempotent 中间件

**文件：** `internal/middleware/idempotent.go`

### 核心概念：SET NX 原子操作

```go
ok, err := cfg.RedisClient.SetNX(r.Context(), redisKey, "1", cfg.TTL).Result()
```

`SET NX` = "Set if Not eXists"：

| `ok` | 含义 | 动作 |
|------|------|------|
| `true` | key 不存在，SET 成功 | 第一次提交，放行 |
| `false` | key 已存在 | 重复提交，返回 409 |

### 为什么用 SET NX 而不是先 GET 再 SET？

```
请求 A:  GET key → 不存在
请求 B:  GET key → 不存在（A 还没 SET）
请求 A:  SET key → 成功
请求 B:  SET key → 成功（都通过了！）
```

GET + SET 不是原子的，存在竞态条件。SET NX 是 Redis 的原子操作，不会出现这个问题。

### 与 TS 版的差异

TS 版在请求完成后还缓存了响应体，重复提交时返回上次的结果。Go 版简化为直接返回 409 错误。后续有需求可以加上响应缓存。
