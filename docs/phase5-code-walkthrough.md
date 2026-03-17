# Phase 5 代码逐行详解 — API Gateway

> 本文档对 Phase 5 所有新增/修改代码进行详细解释，帮助 Go 初学者理解 API Gateway 的实现细节。

---

## 目录

1. [Registry 服务注册表 — registry.go](#1-registry-服务注册表)
2. [Proxy 反向代理 — proxy.go](#2-proxy-反向代理)
3. [Health 健康检查 — health.go](#3-health-健康检查)
4. [Server 组装中心 — server.go](#4-server-组装中心)
5. [Main 入口 — cmd/gateway/main.go](#5-main-入口)

---

## 1. Registry 服务注册表

**文件：** `internal/gateway/registry.go`

### 核心概念：路由前缀映射

Registry 就是一张"路由表"，告诉 Gateway："以这个前缀开头的请求，转发到那个地址"。

```
/api/v1/user/    → http://user:3001
/api/v1/product/ → http://product:3002
/api/v1/cart/    → http://cart:3003
/api/v1/order/   → http://order:3004
```

### ServiceEntry 结构体

```go
type ServiceEntry struct {
    Prefix    string   // 路由前缀，如 "/api/v1/user/"
    TargetURL *url.URL // 下游服务地址，如 "http://user:3001"
}
```

- `Prefix` 是普通字符串
- `TargetURL` 是 `*url.URL` 类型 — Go 标准库提供的结构化 URL 对象，把 `"http://user:3001"` 解析成 `Scheme="http"`, `Host="user:3001"` 等字段

为什么用 `*url.URL` 而不是 `string`？
- 注册时解析一次，缓存结构体
- 代理时直接读取 `.Scheme` 和 `.Host` 字段，不需要每次请求重新解析字符串
- 启动时就能发现 URL 格式错误（早发现、早修复）

### Registry 为什么用 slice

```go
type Registry struct {
    entries []ServiceEntry
}
```

用 `[]ServiceEntry`（切片）而不是 `map[string]ServiceEntry`，因为：
- 需要**有序匹配** — 先注册的前缀优先匹配
- 前缀数量很少（< 10 个），遍历比 map 更简单
- map 无法保证遍历顺序

### Register 方法

```go
func (reg *Registry) Register(prefix, rawURL string) error {
    target, err := url.Parse(rawURL)
    if err != nil {
        return err
    }
    reg.entries = append(reg.entries, ServiceEntry{
        Prefix:    prefix,
        TargetURL: target,
    })
    return nil
}
```

- `url.Parse(rawURL)` — 把字符串 `"http://user:3001"` 解析为 `*url.URL` 结构体
- `append(reg.entries, ...)` — 向切片末尾追加元素（类似 TS 的 `array.push()`）
- 返回 `error` — 如果 URL 格式不合法，调用方可以处理错误

### Lookup 前缀匹配

```go
func (reg *Registry) Lookup(path string) *ServiceEntry {
    for i := range reg.entries {
        if len(path) >= len(reg.entries[i].Prefix) && path[:len(reg.entries[i].Prefix)] == reg.entries[i].Prefix {
            return &reg.entries[i]
        }
    }
    return nil
}
```

- `for i := range reg.entries` — 遍历切片，`i` 是索引
- `path[:len(prefix)]` — 切片语法，取 path 的前 N 个字符（类似 TS 的 `path.slice(0, prefix.length)`）
- `return &reg.entries[i]` — 返回指针，避免复制整个 struct
- `return nil` — 没找到匹配的前缀

等价于 `strings.HasPrefix(path, prefix)`，但直接用 `len` 比较更轻量（少一次函数调用开销）。

---

## 2. Proxy 反向代理

**文件：** `internal/gateway/proxy.go`

### 核心概念：httputil.ReverseProxy

Go 标准库内置了反向代理，不需要第三方库。工作流程：

```
客户端 → Gateway 收到请求
          │
          ▼ Director 函数修改请求（改目标地址、加 header）
          │
          ▼ ReverseProxy 自动转发到下游
          │
          ▼ 下游返回响应
          │
          ▼ ReverseProxy 自动把响应传回客户端
```

### NewProxyHandler 函数

```go
func NewProxyHandler(reg *Registry, internalSecret string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ...
    }
}
```

这是一个工厂函数 — 接收配置（注册表、密钥），返回一个 handler 函数。和 Phase 4 中间件的工厂函数模式相同。

### 路由查找

```go
entry := reg.Lookup(r.URL.Path)
if entry == nil {
    response.HandleError(w, r, apperr.NewNotFound("route", r.URL.Path))
    return
}
```

- 在注册表中查找匹配的下游服务
- 没找到返回 404 — 客户端请求了不存在的路由

### Director 函数

```go
proxy := &httputil.ReverseProxy{
    Director: func(req *http.Request) {
        req.URL.Scheme = entry.TargetURL.Scheme  // "http"
        req.URL.Host = entry.TargetURL.Host      // "user:3001"
        req.Host = entry.TargetURL.Host           // Host header 也要改
        ...
    },
}
```

`Director` 是 `ReverseProxy` 在转发前调用的"修改器"。它做 4 件事：

1. **改目标地址** — 把请求从 Gateway 重定向到下游服务
2. **注入 X-Trace-Id** — 请求追踪，下游服务的日志可以关联同一个请求
3. **注入 X-User-Id** — 传递已认证用户 ID，下游不需要重新解析 JWT
4. **注入 X-Internal-Secret** — 下游验证请求来自可信的 Gateway

### 为什么要设置 req.Host？

```go
req.URL.Host = entry.TargetURL.Host  // URL 里的 host
req.Host = entry.TargetURL.Host      // HTTP Host header
```

这是两个不同的东西：
- `req.URL.Host` — Go 内部用来决定连接哪个服务器
- `req.Host` — HTTP 请求头里的 `Host` 字段，下游服务看到的

如果只改 `req.URL.Host` 不改 `req.Host`，下游服务收到的 Host header 还是 Gateway 的地址（如 `gateway:3000`），可能导致下游路由匹配出错。

---

## 3. Health 健康检查

**文件：** `internal/gateway/health.go`

### 代码

```go
func HealthHandler(w http.ResponseWriter, r *http.Request) error {
    return response.Success(w, r, map[string]string{
        "service": "gateway",
        "status":  "ok",
    })
}
```

最简单的一个文件 — 只有一个 handler，返回 Gateway 的存活状态。

为什么要独立文件？
- 之前 `healthHandler` 写在 `cmd/gateway/main.go` 里（小写，未导出）
- 移到 `internal/gateway/` 包后，大写导出为 `HealthHandler`，可以被 `server.go` 和其他入口复用
- 未来可扩展为检查下游服务和 Redis 的连通性

### 函数签名 `error` 返回值

```go
func HealthHandler(w http.ResponseWriter, r *http.Request) error
```

符合项目的 `handler.AppHandler` 类型签名：

```go
type AppHandler func(w http.ResponseWriter, r *http.Request) error
```

配合 `handler.Wrap()` 使用：`handler.Wrap(HealthHandler)` 把返回 error 的函数包装成标准的 `http.HandlerFunc`。

---

## 4. Server 组装中心

**文件：** `internal/gateway/server.go`

### 核心概念：组装模式

`server.go` 不包含业务逻辑，它的唯一职责是把各个零件拼在一起：

```
输入：
  - config.GatewayConfig（所有配置）
  - *redis.Client（Redis 连接，可能为 nil）

组装过程：
  1. 创建 Registry，注册 4 个下游服务
  2. 创建 chi Router
  3. 挂载全局中间件链
  4. 注册健康检查路由
  5. 注册兜底代理路由

输出：
  - http.Handler（可以直接喂给 http.Server）
```

### 匿名结构体切片

```go
services := []struct {
    prefix string
    url    string
}{
    {"/api/v1/user/", cfg.UserServiceURL},
    {"/api/v1/product/", cfg.ProductServiceURL},
    {"/api/v1/cart/", cfg.CartServiceURL},
    {"/api/v1/order/", cfg.OrderServiceURL},
}
```

Go 的匿名结构体 — 不需要提前定义类型名，一次性使用。等价于 TS 的：

```typescript
const services: { prefix: string; url: string }[] = [
    { prefix: "/api/v1/user/", url: config.userServiceURL },
    // ...
];
```

用循环注册避免 4 次重复调用，也方便未来加新服务（只需加一行）。

### 空 URL 跳过

```go
for _, s := range services {
    if s.url == "" {
        continue
    }
    ...
}
```

`continue` — 跳过当前循环迭代（等价于 TS 的 `continue`）。

当某个服务 URL 为空时跳过注册。这个场景出现在：
- 本地开发只启动了部分服务
- 单体模式下不需要配置服务 URL

### `_ =` 赋值

```go
_ = auth.NewJWTManager(
    cfg.JWT.AccessSecret,
    cfg.JWT.RefreshSecret,
    cfg.JWT.AccessExpiresIn,
    cfg.JWT.RefreshExpiresIn,
)
```

Go 不允许"声明了变量但不使用"（编译错误）。`_ =` 的意思是"我知道返回值没用到，但保留这段代码"。

JWTManager 暂时没用 — Auth 中间件由下游服务各自挂载。保留构建逻辑是为了：
- 后续 Gateway 可能需要对特定路由做认证
- 确保 JWT 配置在启动时就被验证

### 兜底路由

```go
r.HandleFunc("/*", NewProxyHandler(reg, cfg.Internal.Secret))
```

`/*` 是 chi 的通配符模式，匹配所有未被前面路由命中的路径。

路由匹配优先级：
1. `/health` — 精确匹配，直接命中
2. `/*` — 通配符，其他所有路径走这里

---

## 5. Main 入口

**文件：** `cmd/gateway/main.go`

### 对比旧版

旧版（Phase 2）：
```go
port := envOr("API_GATEWAY_PORT", "3000")
r := chi.NewRouter()
r.Post("/health", handler.Wrap(healthHandler))
srv := &http.Server{Addr: ":" + port, Handler: r}
```

新版（Phase 5）：
```go
k, _ := config.Load()
cfg := config.LoadGateway(k)
redisClient := initRedis(cfg.Redis.URL)
handler, _ := gateway.NewServer(cfg, redisClient)
srv := &http.Server{Addr: ":" + cfg.Port, Handler: handler}
```

核心变化：从"手写路由"变成"配置驱动 + 工厂函数组装"。

### initRedis 降级函数

```go
func initRedis(redisURL string) *redis.Client {
    if redisURL == "" {
        slog.Warn("redis url not configured, running without redis")
        return nil
    }
    client, err := database.NewRedis(redisURL)
    if err != nil {
        slog.Warn("failed to connect to redis, running without redis", "error", err)
        return nil
    }
    slog.Info("redis connected")
    return client
}
```

三种结果：

| 情况 | 行为 | 返回值 |
|------|------|--------|
| URL 为空 | Warn 日志，降级 | `nil` |
| 连接失败 | Warn 日志，降级 | `nil` |
| 连接成功 | Info 日志 | `*redis.Client` |

返回 `nil` 不是错误 — 中间件会检查 `redisClient == nil` 并跳过 Redis 逻辑。这样本地开发不启动 Redis 也能正常运行 Gateway。

### 优雅关停

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

go func() {
    srv.ListenAndServe()
}()

<-ctx.Done()
srv.Shutdown(shutdownCtx)
```

执行流程：

```
main goroutine                    server goroutine
     │                                 │
     ├── signal.NotifyContext ──→ 注册信号监听
     ├── go func() ──────────────→ srv.ListenAndServe()
     │                                 │ 处理请求...
     ├── <-ctx.Done() 阻塞等待         │ 处理请求...
     │                                 │ 处理请求...
     │ ← SIGINT/SIGTERM ─────────────→ │
     ├── srv.Shutdown() ──────────→ 停止接受新连接
     │                                 │ 等待处理中的请求完成
     │                                 │ 返回
     └── main() 结束                   └── goroutine 结束
```

- `SIGINT` — Ctrl+C 触发
- `SIGTERM` — Docker/K8s 发送的停止信号
- `Shutdown(10s)` — 最多等 10 秒让处理中的请求完成，超时后强制关闭

### 为什么用 slog.Error + os.Exit(1) 而不是 log.Fatal？

```go
slog.Error("failed to load config", "error", err)
os.Exit(1)
```

`log.Fatal` 内部也是 `os.Exit(1)`，但它用的是旧的 `log` 包。项目统一使用 `slog`（结构化日志），所以手动 `slog.Error` + `os.Exit(1)`，保持日志格式一致。
