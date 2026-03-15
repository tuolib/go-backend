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
| DB 驱动 | pgx v5 + pgxpool | Go 生态最佳 PG 驱动，连接池、LISTEN/COPY |
| 查询生成 | sqlc | SQL 编译生成类型安全 Go 代码，SQL-first |
| 迁移 | goose | SQL 迁移文件，支持 up/down |
| Redis | go-redis v9 | Lua/Pipeline/Sentinel，context.Context |
| 校验 | validator v10 | struct tag 校验 `validate:"required,email"` |
| 配置 | koanf v2 | 无全局状态，类型安全，模块化 |
| 日志 | log/slog (stdlib) | Go 1.21+ 标准库 JSON 结构化日志 |
| JWT | golang-jwt v5 | HS256 双令牌（access + refresh） |
| 密码 | alexedwards/argon2id | Argon2id 哈希 |
| ID | go-nanoid v2 | 21 字符唯一 ID |
| 测试 | testing + testify | 表驱动测试 + 断言 |
| 反向代理 | Caddy | 自动 HTTPS、反向代理、负载均衡 |
| 容器化 | Docker + Compose | 本地开发 & 生产部署统一 |

## 项目结构

```
my-backend-go/
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
│   ├── gateway/main.go                # :3000
│   ├── user/main.go                   # :3001
│   ├── product/main.go                # :3002
│   ├── cart/main.go                   # :3003
│   ├── order/main.go                  # :3004
│   └── migrate/main.go               # 迁移工具
│
├── internal/
│   ├── config/                        # koanf 配置加载
│   │   └── config.go
│   ├── apperr/                        # AppError + 错误码
│   │   ├── errors.go                  # AppError struct + 工厂函数
│   │   └── codes.go                   # 业务错误码常量
│   ├── response/                      # Success/Error/Paginated JSON
│   │   └── response.go
│   ├── middleware/                     # 共享中间件
│   │   ├── requestid.go
│   │   ├── logger.go
│   │   ├── recovery.go
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
│   ├── httpclient/                    # 内部服务 HTTP 客户端
│   │   └── client.go
│   ├── database/
│   │   ├── postgres.go                # pgxpool 连接
│   │   ├── redis.go                   # go-redis 客户端
│   │   ├── migrations/*.sql           # goose 迁移文件
│   │   ├── queries/*.sql              # sqlc 查询文件
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
│   ├── caddy/
│   │   └── Caddyfile
│   ├── postgres/
│   │   ├── init.sql
│   │   └── postgresql.conf
│   └── redis/
│       └── redis.conf
├── scripts/                           # 运维脚本
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
- 错误码常量：`PascalCase` 前缀（例：`ErrCodeStockInsufficient`）
- 数据库表名：`snake_case`（例：`order_items`）
- 数据库列名：`snake_case`（例：`created_at`）
- Redis Key：`{service}:{resource}:{id}`（例：`stock:sku123`）
- JSON tag：`camelCase`（与 TS 版 API 契约一致）

### 包组织

遵循 Go 标准项目布局：

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

### 接口定义

接口由使用方定义，不由实现方定义（Go 惯例）：

```go
// ✅ 在 service 包中定义 repository 接口
type UserRepository interface {
    FindByEmail(ctx context.Context, email string) (*User, error)
    Create(ctx context.Context, user *User) error
}

// ❌ 不要在 repository 包中定义接口
```

### 错误处理

- 使用 `error` 接口逐层返回，不用 panic
- 业务错误使用 `apperr.AppError`
- 使用 `errors.Is` / `errors.As` 判断错误类型
- 工厂函数创建特定错误：`apperr.NewNotFound("user", userId)`

```go
// ✅ 正确
func (s *UserService) FindByEmail(ctx context.Context, email string) (*User, error) {
    user, err := s.repo.FindByEmail(ctx, email)
    if err != nil {
        return nil, fmt.Errorf("find user by email: %w", err)
    }
    if user == nil {
        return nil, apperr.NewNotFound("user", email)
    }
    return user, nil
}

// ❌ 错误 — 不要 panic
func (s *UserService) FindByEmail(ctx context.Context, email string) *User {
    user, err := s.repo.FindByEmail(ctx, email)
    if err != nil {
        panic(err) // NEVER
    }
    return user
}
```

### Context 使用

- 所有函数第一个参数为 `context.Context`
- 通过 context 传递 traceId、userId 等请求级数据
- 使用 `context.WithTimeout` 控制超时

```go
type contextKey string

const (
    TraceIDKey contextKey = "traceId"
    UserIDKey  contextKey = "userId"
)
```

### 中间件模式

使用 chi 标准中间件签名：

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
// 成功
type SuccessResponse[T any] struct {
    Code    int    `json:"code"`
    Success bool   `json:"success"`
    Data    T      `json:"data"`
    Message string `json:"message"`
    TraceID string `json:"traceId"`
}

// 失败
type ErrorResponse struct {
    Code    int         `json:"code"`
    Success bool        `json:"success"`
    Message string      `json:"message"`
    Data    interface{} `json:"data"`
    Meta    *ErrorMeta  `json:"meta,omitempty"`
    TraceID string      `json:"traceId"`
}

type ErrorMeta struct {
    Code    string      `json:"code"`
    Message string      `json:"message"`
    Details interface{} `json:"details,omitempty"`
}
```

### 环境变量

- 所有环境变量通过 `internal/config` 的 koanf 模块加载
- 使用 struct tag 校验必填项
- 启动时校验失败直接 fatal
- 禁止在业务代码中直接 `os.Getenv()`

### 数据库

- 使用 sqlc 从 SQL 生成类型安全的 Go 代码
- 迁移文件放在 `internal/database/migrations/`
- 查询文件放在 `internal/database/queries/`
- 所有表使用 PG schema 隔离（`user_service.users` 等）
- 所有表必须有 `id`、`created_at`、`updated_at` 字段
- `id` 使用 nanoid（21位）
- 时间字段统一 `timestamp with time zone`
- 软删除使用 `deleted_at` 字段
- 并发安全的表使用 `version` 字段（乐观锁）

### 库存操作

- 预扣/释放：通过 Redis Lua 脚本原子操作（`go:embed` 加载）
- 确认：通过 PG 乐观锁（`WHERE version = $currentVersion`）
- 所有操作记录到 `stock_operations` 表
- 禁止直接 `UPDATE skus SET stock = $value`

### 幂等设计

- 订单创建、支付发起必须携带 `X-Idempotency-Key` header
- Gateway 层和 Service 层双重检查
- 幂等键存储在 Redis，TTL 24h

### 并发模式

使用 Go 原生并发原语：

```go
// errgroup 并发调用多个服务
g, ctx := errgroup.WithContext(ctx)

var skus []*SKU
var address *Address

g.Go(func() error {
    var err error
    skus, err = productClient.BatchGetSKUs(ctx, skuIDs)
    return err
})

g.Go(func() error {
    var err error
    address, err = userClient.GetAddress(ctx, addressID, userID)
    return err
})

if err := g.Wait(); err != nil {
    return nil, err
}
```

### 优雅关停

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

srv := &http.Server{Addr: ":3001", Handler: router}

go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        slog.Error("server error", "err", err)
    }
}()

<-ctx.Done()
slog.Info("shutting down...")

shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
srv.Shutdown(shutdownCtx)
```

### 测试

- 测试框架：`testing` + `testify/assert`
- 测试文件：与源码同目录，命名 `*_test.go`
- 使用表驱动测试（Table-Driven Tests）
- 集成测试使用 build tag：`//go:build integration`
- 并发测试使用 `sync.WaitGroup` + `testing.T.Parallel()`

```go
func TestFindUserByEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        want    *User
        wantErr error
    }{
        {"found", "alice@example.com", &User{Email: "alice@example.com"}, nil},
        {"not found", "unknown@example.com", nil, apperr.ErrNotFound},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := svc.FindByEmail(ctx, tt.email)
            if tt.wantErr != nil {
                assert.ErrorIs(t, err, tt.wantErr)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.want.Email, got.Email)
            }
        })
    }
}
```

### 中间件顺序

API Gateway 的中间件链按以下顺序挂载：

```
requestid → logger → recovery → cors → ratelimit → auth → idempotent
```

错误由 recovery 中间件兜底捕获（panic → 500），业务错误由 handler 层处理。

## Claude Code 协作规则

### 分阶段开发

本项目按 `docs/architecture.md` 中定义的 Phase 0-11 路线图开发。
每个阶段使用**独立的 Claude Code 会话**，避免长对话上下文退化。

### 每个会话的开始

1. 读取 `CLAUDE.md`（自动）
2. 读取 `docs/architecture.md` 中对应阶段的描述
3. 检查已完成阶段的代码，理解现有实现
4. 只做当前阶段的工作，不越界

### 接口文档同步

每当新增或修改 API 路由后，必须同步更新 `docs/api-reference.md` 中对应的接口文档。

### 代码生成要求

- 先写类型定义（struct + interface），再写实现
- 先写测试骨架，再写业务逻辑
- 每个文件头部注释说明用途
- 关键设计决策写在代码注释中
- 库存/支付等关键路径必须有并发安全注释

### SQL 文件约定

- 迁移文件：`internal/database/migrations/YYYYMMDDHHMMSS_description.sql`
- 查询文件按域拆分：`queries/users.sql`、`queries/products.sql` 等
- sqlc 生成代码放在 `internal/database/gen/`，不要手动修改

### 禁止事项

- 不要生成 `.env` 文件（使用 `.env.example`）
- 不要硬编码密钥、密码、端口号
- 不要引入未在技术栈中列出的依赖（需先讨论）
- 不要修改其他阶段的代码（除非修 bug）
- 不要直接 UPDATE 库存数值（必须走 Lua 脚本或乐观锁）
- 不要信任前端传来的金额（服务端必须重新计算）
- 不要使用 `init()` 函数做副作用初始化
- 不要使用全局可变状态
- 不要忽略 error 返回值（`_ = someFunc()` 仅限确实不需要处理的场景）
