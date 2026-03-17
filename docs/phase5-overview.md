# Phase 5 架构纲领 — API Gateway

> **目标：** 网关启动，正确代理请求到下游服务，中间件链完整生效
> **状态：** 已完成

---

## 1. 本阶段在整体架构中的位置

```
Phase 0: 项目脚手架（go.mod, 目录结构, Makefile, Docker Compose）     ✅
Phase 1: 通用基础设施（apperr, response, handler/wrap, config, auth, id） ✅
Phase 2: 服务入口（cmd/monolith, gateway, user, product, cart, order）    ✅
Phase 3: 数据库层（PG 连接池, Redis 客户端, 迁移, sqlc, Lua 脚本）     ✅
Phase 4: 共享中间件（requestid, logger, cors, auth, ratelimit 等）     ✅
Phase 5: API Gateway（registry, proxy, health, server 组装）           ✅ ← 当前
Phase 6+: 各服务业务逻辑...                                             ⬜
```

Phase 5 是**请求的入口大门**。所有外部 HTTP 请求都通过 Gateway 进入系统，由 Gateway 决定转发到哪个下游服务。Gateway 不包含业务逻辑 — 它只负责：路由分发、中间件处理、反向代理。

---

## 2. Gateway 在微服务架构中的角色

```
                         ┌──────────────────────┐
                         │      Caddy           │  反向代理 + 自动 HTTPS
                         │  (生产环境入口)       │
                         └──────────┬───────────┘
                                    │
                         ┌──────────▼───────────┐
  客户端 ──────────────→ │    API Gateway :3000  │  路由分发 + 中间件链
                         │  RequestID → Logger   │
                         │  → CORS → RateLimit   │
                         │  → Proxy              │
                         └──┬────┬────┬────┬────┘
                            │    │    │    │
              ┌─────────────┘    │    │    └─────────────┐
              ▼                  ▼    ▼                  ▼
        ┌──────────┐      ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ User     │      │ Product  │ │ Cart     │ │ Order    │
        │ :3001    │      │ :3002    │ │ :3003    │ │ :3004    │
        └──────────┘      └──────────┘ └──────────┘ └──────────┘
```

### Gateway vs 直连的区别

| 特性 | 无 Gateway（直连） | 有 Gateway |
|------|-------------------|-----------|
| 客户端复杂度 | 需要知道每个服务的地址和端口 | 只需知道 Gateway 地址 |
| 跨域处理 | 每个服务都要配 CORS | Gateway 统一处理 |
| 限流 | 每个服务独立限流 | Gateway 全局限流 |
| 请求追踪 | 需要各服务自己生成 traceId | Gateway 统一注入 |
| SSL/TLS | 每个服务都要配证书 | Gateway（或 Caddy）统一处理 |

---

## 3. 整体设计决策

### 3.1 路由匹配策略：前缀匹配

```
/api/v1/user/login     → 匹配前缀 /api/v1/user/     → 转发到 user:3001
/api/v1/product/list   → 匹配前缀 /api/v1/product/  → 转发到 product:3002
/api/v1/cart/add       → 匹配前缀 /api/v1/cart/     → 转发到 cart:3003
/api/v1/order/create   → 匹配前缀 /api/v1/order/    → 转发到 order:3004
/health                → 直接命中 Gateway 自己的路由
/unknown/path          → 兜底路由 → Registry 查找失败 → 404
```

为什么用前缀匹配而不是精确匹配？
- 下游服务自己定义具体路由（如 `/api/v1/user/login`、`/api/v1/user/register`）
- Gateway 只需知道"以 `/api/v1/user/` 开头的请求归 user 服务管"
- 新增 API 不需要改 Gateway 配置

### 3.2 反向代理 header 注入

```
客户端请求 → Gateway 收到
  │
  ▼ Director 函数修改请求：
  │  ① 改目标地址：req.URL.Host = "user:3001"
  │  ② 注入 X-Trace-Id（请求追踪）
  │  ③ 注入 X-User-Id（已认证用户 ID）
  │  ④ 注入 X-Internal-Secret（内部鉴权密钥）
  │
  ▼ 转发到下游服务
```

| Header | 用途 | 来源 |
|--------|------|------|
| `X-Trace-Id` | 请求追踪，关联日志 | RequestID 中间件注入到 context |
| `X-User-Id` | 传递已认证用户 ID | Auth 中间件注入到 context |
| `X-Internal-Secret` | 下游验证请求来自可信 Gateway | 环境变量配置 |

### 3.3 中间件链分层

```
Gateway 全局链：RequestID → Logger → CORS → RateLimit → Proxy
                 ↓           ↓       ↓       ↓
                 所有请求都经过这 4 个中间件

下游服务按需挂载：Auth → Idempotent → Handler
                   ↓       ↓
                   仅需要认证/幂等的路由才走
```

为什么 Auth 不放在 Gateway 全局链？
- 登录、注册、公开商品页不需要认证
- 如果全局 Auth，这些路由需要复杂的白名单机制
- 让各下游服务自己决定哪些路由需要认证，更灵活

### 3.4 Redis 降级运行

```
Redis 可用   → 限流 + token 黑名单检查 正常工作
Redis 不可用 → redisClient = nil → 中间件跳过 Redis 逻辑
Redis 中途故障 → Pipeline 报错 → 放行 + Warn 日志
```

设计原则：**可用性 > 绝对安全**。Gateway 是系统入口，如果因为 Redis 故障拦截所有请求，整个系统瘫痪。

---

## 4. 文件清单

```
internal/gateway/
├── registry.go    — 服务注册表：路由前缀 → 下游服务 URL 映射
├── proxy.go       — 反向代理：httputil.ReverseProxy + Director header 注入
├── health.go      — 健康检查端点
└── server.go      — 组装中心：Registry + Proxy + 中间件链 → http.Handler

cmd/gateway/
└── main.go        — 入口：加载配置 → 连 Redis → NewServer → 启动 HTTP → 优雅关停
```

修改的文件：

```
cmd/gateway/main.go  — 从手写路由改为调用 gateway.NewServer() 组装
```

---

## 5. 依赖关系

```
cmd/gateway/main.go
  ├── config     — Load(), LoadGateway()
  ├── database   — NewRedis()
  └── gateway    — NewServer()

internal/gateway/
  ├── config     — GatewayConfig
  ├── auth       — JWTManager（预留）
  ├── middleware  — RequestID, Logger, CORS, RateLimit
  ├── handler    — Wrap()
  ├── response   — Success(), HandleError()
  └── apperr     — NewNotFound()
```

---

## 6. 启动流程

```
main()
  │
  ▼ config.Load()          读取所有环境变量到 koanf
  ▼ config.LoadGateway(k)  提取 Gateway 需要的配置字段
  │
  ▼ initRedis()            连接 Redis（失败则 nil，降级运行）
  │
  ▼ gateway.NewServer(cfg, redisClient)
  │   ├── NewRegistry()                注册 4 个下游服务
  │   ├── chi.NewRouter()              创建路由器
  │   ├── r.Use(RequestID, Logger, CORS, RateLimit)  挂载中间件
  │   ├── r.Post("/health", ...)       注册健康检查
  │   └── r.HandleFunc("/*", ...)      注册兜底代理
  │
  ▼ http.Server{Handler: handler}      创建 HTTP 服务
  ▼ go srv.ListenAndServe()            goroutine 中启动
  ▼ <-ctx.Done()                       主 goroutine 等待系统信号
  ▼ srv.Shutdown(10s)                  优雅关停
```

---

## 7. 请求生命周期

```
客户端 POST /api/v1/user/login
  │
  ▼ http.Server 接收连接
  ▼ chi 路由器
  │   不匹配 /health → 走 /* 兜底路由
  │
  ▼ 中间件链（按注册顺序执行）：
  │  1. RequestID — 生成 traceId，注入 context，设置 X-Trace-Id 响应头
  │  2. Logger   — 记录开始时间，包装 responseRecorder
  │  3. CORS     — 检查 Origin，设置 Access-Control-* 头
  │  4. RateLimit — Redis ZSET 检查限流（降级时跳过）
  │
  ▼ NewProxyHandler
  │  ├── reg.Lookup("/api/v1/user/login")
  │  │   → 前缀匹配 "/api/v1/user/" → entry{TargetURL: "http://user:3001"}
  │  │
  │  ├── httputil.ReverseProxy.Director:
  │  │   ① req.URL.Host = "user:3001"
  │  │   ② req.Header.Set("X-Trace-Id", traceId)
  │  │   ③ req.Header.Set("X-User-Id", "")       ← 登录前无用户 ID
  │  │   ④ req.Header.Set("X-Internal-Secret", secret)
  │  │
  │  └── proxy.ServeHTTP → 转发到 user:3001
  │
  ▼ user:3001 处理请求，返回响应
  │
  ▼ Logger ← 记录 status=200, duration=15ms
  │
  响应返回客户端
```

---

## 8. 与 TS 版对比

| 特性 | Go 版 | TS 版 |
|------|-------|-------|
| 反向代理 | `httputil.ReverseProxy`（标准库） | `hono` 的 proxy helper |
| 路由注册表 | 自定义 `Registry` struct（slice 遍历） | 直接在路由配置中定义 |
| 配置加载 | `koanf`（类型安全） | `dotenv` + `zod` |
| 中间件挂载 | `r.Use(middleware)` | `app.use(middleware)` |
| 健康检查 | `handler.Wrap(HealthHandler)` 返回 error | 直接返回 JSON |
| 优雅关停 | `signal.NotifyContext` + `srv.Shutdown` | `process.on('SIGTERM')` |
| Redis 降级 | `initRedis` 返回 nil，中间件检查 nil | 类似的 try/catch 模式 |

---

## 9. 与其他阶段的衔接

### 9.1 Phase 5 使用了哪些前序阶段的能力

| 来源阶段 | 使用的能力 |
|----------|-----------|
| Phase 1 | `apperr.NewNotFound`（路由未匹配）、`response.Success`（健康检查）、`handler.Wrap`（handler 包装） |
| Phase 1 | `auth.NewJWTManager`（预留认证能力）、`config.Load/LoadGateway`（配置加载） |
| Phase 3 | `database.NewRedis`（Redis 连接） |
| Phase 4 | `middleware.RequestID/Logger/CORS/RateLimit`（中间件链） |

### 9.2 Phase 5 为后续阶段提供的能力

| 后续阶段 | 使用的能力 |
|----------|-----------|
| Phase 6 用户服务 | Gateway 代理 `/api/v1/user/*` 到 user:3001 |
| Phase 7 商品服务 | Gateway 代理 `/api/v1/product/*` 到 product:3002 |
| Phase 8 购物车服务 | Gateway 代理 `/api/v1/cart/*` 到 cart:3003 |
| Phase 9 订单服务 | Gateway 代理 `/api/v1/order/*` 到 order:3004 |

---

## 10. 关键 Go 知识点（本阶段涉及）

| 知识点 | 出现位置 | 说明 |
|--------|----------|------|
| `httputil.ReverseProxy` | proxy.go | 标准库反向代理，Director 函数修改请求 |
| `net/url.URL` | registry.go | 结构化 URL 类型，避免每次请求重复解析字符串 |
| 匿名结构体 | server.go | `[]struct{prefix, url string}{...}` 一次性使用的类型 |
| 接口返回 | server.go | 返回 `http.Handler` 接口，调用方不关心具体实现 |
| nil 降级 | main.go | `redisClient` 为 nil 时，中间件跳过 Redis 逻辑 |
| 前缀匹配 | registry.go | 手动切片比较，比 `strings.HasPrefix` 更轻量 |
| `_ =` 赋值 | server.go | 告诉编译器"我知道这个值没用，但保留构建逻辑" |
