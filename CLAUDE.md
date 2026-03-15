# CLAUDE.md — 项目约定 & AI 协作指南（Go 版）

> **Claude Code CLI 会自动读取此文件。所有会话必须遵守以下约定。**

---

## 项目概述

企业级高并发电商平台（对标 Amazon）— Go 重写版
保持与 TypeScript 版相同的业务逻辑和 API 契约，采用 Go 惯用架构模式。
核心域：用户认证、商品管理、购物车、订单支付、库存并发控制。

## 技术栈

| 层级 | 选型 | 说明 |
|------|------|------|
| 语言 | Go 1.22+ | 高性能、强类型、原生并发 |
| 路由 | chi v5 | 基于 stdlib net/http，Go 惯用中间件模式 |
| DB 驱动 | pgx v5 + pgxpool | Go 生态最佳 PG 驱动，连接池 |
| 查询生成 | sqlc + squirrel | sqlc 处理固定 CRUD；squirrel 处理动态筛选/搜索 |
| 迁移 | goose | SQL 迁移文件，支持 up/down |
| Redis | go-redis v9 | Lua/Pipeline/Sentinel，context.Context |
| 校验 | validator v10 | struct tag 校验 `validate:"required,email"` |
| 配置 | koanf v2 | 无全局状态，类型安全，模块化 |
| 日志 | log/slog (stdlib) | Go 1.21+ 标准库 JSON 结构化日志 |
| JWT | golang-jwt v5 | HS256 双令牌（access + refresh） |
| 密码 | alexedwards/argon2id | Argon2id 哈希 |
| ID | go-nanoid v2 | 21 字符唯一 ID |
| 并发控制 | x/sync/singleflight | 进程内缓存击穿防护 |
| 测试 | testing + testify + testcontainers-go | 表驱动测试 + 断言 + 自动启停 PG/Redis |
| 反向代理 | Caddy | 自动 HTTPS、反向代理、负载均衡 |
| 容器化 | Docker + Compose | 本地开发 & 生产部署统一 |

## 双模式运行

本项目支持**单体模式**和**微服务模式**双模式运行：

```
cmd/
├── monolith/main.go       # 单体模式：所有服务跑在一个进程，内部调用零网络开销
├── gateway/main.go        # 微服务模式：API Gateway
├── user/main.go           # 微服务模式：用户服务
├── product/main.go        # 微服务模式：商品服务
├── cart/main.go           # 微服务模式：购物车服务
├── order/main.go          # 微服务模式：订单服务
└── migrate/main.go        # 迁移工具
```

- **本地开发**：`go run ./cmd/monolith` 一个进程搞定
- **生产部署**：每个服务独立编译、独立扩缩容

## 项目结构

```
go-backend/
├── CLAUDE.md
├── Makefile
├── go.mod / go.sum
├── .env.example
├── .gitignore
├── .golangci.yml
├── docker-compose.yml
├── Dockerfile
│
├── cmd/                               # 每个服务一个入口
│   ├── monolith/main.go               # 单体模式入口
│   ├── gateway/main.go                # :3000
│   ├── user/main.go                   # :3001
│   ├── product/main.go                # :3002
│   ├── cart/main.go                   # :3003
│   ├── order/main.go                  # :3004
│   └── migrate/main.go               # 迁移工具
│
├── internal/
│   ├── config/                        # koanf 配置加载（每个服务独立 Config struct）
│   │   ├── config.go                  # 公共字段
│   │   ├── gateway.go                 # GatewayConfig
│   │   ├── user.go                    # UserServiceConfig
│   │   ├── product.go                 # ProductServiceConfig
│   │   ├── cart.go                    # CartServiceConfig
│   │   └── order.go                   # OrderServiceConfig
│   ├── apperr/                        # AppError + 错误码
│   │   ├── errors.go
│   │   └── codes.go
│   ├── response/                      # Success/Error/Paginated JSON + HandleError
│   │   └── response.go
│   ├── handler/                       # AppHandler 类型 + Wrap 包装器
│   │   └── wrap.go
│   ├── middleware/                     # 共享中间件
│   │   ├── requestid.go
│   │   ├── logger.go
│   │   ├── cors.go
│   │   ├── auth.go
│   │   ├── ratelimit.go
│   │   ├── idempotent.go
│   │   └── internal_only.go
│   ├── auth/                          # JWT + Argon2 + SHA256
│   │   ├── jwt.go
│   │   ├── argon2.go
│   │   └── hash.go
│   ├── id/                            # nanoid 生成
│   │   └── id.go
│   ├── httpclient/                    # 内部服务 HTTP 客户端（微服务模式用）
│   │   └── client.go
│   ├── database/
│   │   ├── postgres.go                # pgxpool 连接
│   │   ├── redis.go                   # go-redis 客户端
│   │   ├── migrations/*.sql           # goose 迁移文件
│   │   ├── queries/*.sql              # sqlc 查询文件（固定 CRUD）
│   │   ├── sqlc.yaml
│   │   └── gen/                       # sqlc 生成代码（勿手动修改）
│   ├── lua/                           # Redis Lua 脚本 (go:embed)
│   │   ├── scripts.go
│   │   ├── stock-deduct.lua
│   │   ├── stock-deduct-multi.lua
│   │   ├── stock-release.lua
│   │   └── stock-release-multi.lua
│   │
│   ├── gateway/                       # API Gateway 服务
│   │   ├── server.go
│   │   ├── proxy.go
│   │   ├── registry.go
│   │   └── health.go
│   ├── user/                          # 用户服务
│   │   ├── handler/
│   │   │   ├── auth.go
│   │   │   ├── user.go
│   │   │   └── address.go
│   │   ├── service/
│   │   │   ├── auth.go
│   │   │   ├── user.go
│   │   │   └── address.go
│   │   ├── repository/
│   │   │   ├── user.go
│   │   │   ├── address.go
│   │   │   └── token.go
│   │   └── dto/
│   │       └── dto.go
│   ├── product/                       # 商品服务
│   │   ├── handler/
│   │   ├── service/
│   │   ├── repository/
│   │   └── dto/
│   ├── cart/                          # 购物车服务
│   │   ├── handler/
│   │   ├── service/
│   │   └── dto/
│   └── order/                         # 订单服务
│       ├── handler/
│       ├── service/
│       ├── repository/
│       ├── statemachine/
│       └── dto/
│
├── infra/                             # 基础设施配置
│   ├── caddy/Caddyfile
│   ├── postgres/
│   │   ├── init.sql
│   │   └── postgresql.conf
│   └── redis/redis.conf
├── scripts/
└── docs/
    ├── architecture.md
    └── api-reference.md
```

## 编码规范

### 命名

- 文件名：`snake_case.go`（例：`error_handler.go`）— Go 标准
- 包名：小写单词，不用下划线（例：`apperr`、`httpclient`）
- 类型/接口：`PascalCase`（例：`CreateOrderInput`、`UserRepository`）
- 函数/方法：`PascalCase`（导出）或 `camelCase`（未导出）
- 常量：`PascalCase`（导出）或 `camelCase`（未导出）
- 错误码常量：`ErrCode` 前缀（例：`ErrCodeStockInsufficient`）
- 数据库表名：`snake_case`（例：`order_items`）
- 数据库列名：`snake_case`（例：`created_at`）
- Redis Key：`{service}:{resource}:{id}`（例：`stock:sku123`）
- JSON tag：`camelCase`（与 TS 版 API 契约一致）

### 包组织

```
cmd/        — 入口点（main 包）
internal/   — 私有包（不可被外部导入）
```

### 依赖注入

使用构造函数注入，不用全局变量：

```go
// ✅ 正确 — 构造函数注入
type UserService struct {
    repo   UserRepository
    cache  *redis.Client
    hasher *auth.Argon2Hasher
}

func NewUserService(repo UserRepository, cache *redis.Client, hasher *auth.Argon2Hasher) *UserService {
    return &UserService{repo: repo, cache: cache, hasher: hasher}
}

// ❌ 错误 — 全局变量
var db *pgxpool.Pool
```

### 接口定义 — 由使用方定义

```go
// ✅ 在 service 包中定义 repository 接口
type UserRepository interface {
    FindByEmail(ctx context.Context, email string) (*User, error)
    Create(ctx context.Context, user *User) error
}
```

### 服务间调用 — 接口抽象

服务间依赖通过接口定义，支持双模式运行：

```go
// order 包内定义依赖接口
type ProductQuerier interface {
    BatchGetSKUs(ctx context.Context, skuIDs []string) ([]SKUInfo, error)
    ReserveStock(ctx context.Context, items []StockItem, orderID string) error
}

// 微服务模式：HTTP 实现
type httpProductClient struct { ... }

// 单体模式：直接调用
type localProductClient struct { service *product.StockService }
```

### Handler 模式 — 返回 error

Handler 使用返回 error 的签名 + Wrap 包装器统一处理错误：

```go
// internal/handler/wrap.go
type AppHandler func(w http.ResponseWriter, r *http.Request) error

func Wrap(h AppHandler) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if err := h(w, r); err != nil {
            response.HandleError(w, r, err)
        }
    }
}

// handler 写法 — 干净的 error 流
func (h *UserHandler) Profile(w http.ResponseWriter, r *http.Request) error {
    userID := middleware.UserIDFrom(r.Context())
    user, err := h.service.GetProfile(r.Context(), userID)
    if err != nil {
        return err  // 直接返回，Wrap 处理
    }
    return response.Success(w, r, user)
}

// 路由注册
r.Post("/api/v1/user/profile", handler.Wrap(h.Profile))
```

### 错误处理

- 使用 `error` 接口逐层返回，不用 panic
- 业务错误使用 `apperr.AppError`
- 使用 `errors.Is` / `errors.As` 判断错误类型
- Wrap 包装器自动将 AppError 转为 HTTP 响应，未知错误转为 500

```go
// ✅ 正确
if user == nil {
    return apperr.NewNotFound("user", email)
}

// ❌ 错误 — 不要 panic
panic("user not found")
```

### Context 使用

- 所有函数第一个参数为 `context.Context`
- 通过 context 传递 traceId、userId 等请求级数据

```go
type contextKey string

const (
    TraceIDKey contextKey = "traceId"
    UserIDKey  contextKey = "userId"
)
```

### 中间件模式

chi 标准签名：

```go
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := nanoid.New()
        ctx := context.WithValue(r.Context(), TraceIDKey, id)
        w.Header().Set("X-Trace-Id", id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### API 设计

- **全部使用 POST**，参数通过 JSON Body 传递（与 TS 版一致）
- 路由格式：`POST /api/v1/{domain}/{action}`
- 外部接口前缀 `/api/v1/`，内部接口前缀 `/internal/`
- 内部接口仅 Docker 内部网络可访问

### 响应格式

所有 API 统一返回（与 TS 版 JSON 格式完全一致）：

```go
type SuccessResponse[T any] struct {
    Code    int    `json:"code"`
    Success bool   `json:"success"`
    Data    T      `json:"data"`
    Message string `json:"message"`
    TraceID string `json:"traceId"`
}

type ErrorResponse struct {
    Code    int         `json:"code"`
    Success bool        `json:"success"`
    Message string      `json:"message"`
    Data    interface{} `json:"data"`
    Meta    *ErrorMeta  `json:"meta,omitempty"`
    TraceID string      `json:"traceId"`
}
```

### 数据库查询策略

双工具策略：

```go
// 固定 CRUD → sqlc（编译时类型安全）
// internal/database/queries/users.sql
// -- name: GetUserByEmail :one
// SELECT * FROM user_service.users WHERE email = $1 AND deleted_at IS NULL;

// 动态筛选/搜索 → squirrel（运行时构建）
qb := sq.Select("id", "title", "min_price").
    From("product_service.products").
    Where("status = 'active'")
if input.CategoryID != "" {
    qb = qb.Where("id IN (SELECT product_id FROM product_service.product_categories WHERE category_id = ?)", input.CategoryID)
}
```

### 缓存击穿防护

进程内用 singleflight，跨实例用 Redis 分布式锁：

```go
var group singleflight.Group

func (s *ProductService) GetDetail(ctx context.Context, id string) (*Product, error) {
    v, err, _ := group.Do("product:"+id, func() (interface{}, error) {
        return s.repo.FindByID(ctx, id)
    })
    return v.(*Product), err
}
```

### 环境变量

- 通过 `internal/config` 的 koanf 模块加载
- 每个服务有独立的 Config struct，只加载自己需要的变量
- 禁止在业务代码中直接 `os.Getenv()`

### 库存操作

- 预扣/释放：Redis Lua 脚本原子操作（`go:embed` 嵌入，编译进二进制）
- 确认：PG 乐观锁
- 所有操作记录到 `stock_operations` 表
- 禁止直接 `UPDATE skus SET stock = $value`

### 并发模式

```go
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { skus, err = productClient.BatchGetSKUs(ctx, skuIDs); return err })
g.Go(func() error { address, err = userClient.GetAddress(ctx, addressID, userID); return err })
if err := g.Wait(); err != nil { return nil, err }
```

### 优雅关停

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
// ... start server ...
<-ctx.Done()
srv.Shutdown(timeoutCtx)
```

### 测试

- 测试框架：`testing` + `testify/assert` + `testcontainers-go`
- 测试文件：与源码同目录，命名 `*_test.go`
- 表驱动测试（Table-Driven Tests）
- 集成测试用 testcontainers 自动启停 PG/Redis，不依赖外部实例
- 并发测试：`sync.WaitGroup` + `-race` flag
- 集成测试 build tag：`//go:build integration`

### 中间件顺序

API Gateway 中间件链：

```
requestid → logger → cors → ratelimit → auth → idempotent
```

错误由 Wrap 包装器处理（handler 返回 error → 自动转 JSON 响应）。

## Claude Code 协作规则

### 分阶段开发

按 `docs/architecture.md` 中定义的 Phase 0-11 路线图开发。
每个阶段使用**独立的 Claude Code 会话**。

### 每个会话的开始

1. 读取 `CLAUDE.md`（自动）
2. 读取 `docs/architecture.md` 中对应阶段的描述
3. 检查已完成阶段的代码，理解现有实现
4. 只做当前阶段的工作，不越界

### 接口文档同步

新增或修改 API 路由后，必须同步更新 `docs/api-reference.md`。

### 代码生成要求

- 先写类型定义（struct + interface），再写实现
- 先写测试骨架，再写业务逻辑
- 关键设计决策写在代码注释中
- 库存/支付等关键路径必须有并发安全注释

### 禁止事项

- 不要生成 `.env` 文件（使用 `.env.example`）
- 不要硬编码密钥、密码、端口号
- 不要引入未在技术栈中列出的依赖（需先讨论）
- 不要修改其他阶段的代码（除非修 bug）
- 不要直接 UPDATE 库存数值（必须走 Lua 脚本或乐观锁）
- 不要信任前端传来的金额（服务端必须重新计算）
- 不要使用 `init()` 函数做副作用初始化
- 不要使用全局可变状态
- 不要忽略 error 返回值
