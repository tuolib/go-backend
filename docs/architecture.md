# Architecture Decision Record — 企业级高并发电商平台（Go 版）

> 本文档是所有开发阶段的**唯一架构真相来源（Single Source of Truth）**。
> Claude Code CLI 每个新会话应首先阅读本文档中对应阶段的内容。

---

## 1. 系统全景

### 1.1 设计目标

| 维度 | 目标 | 实现手段 |
|------|------|----------|
| 高并发 | 单节点 50K+ RPS | Go 原生高并发 + goroutine + 连接池 + 多级缓存 |
| 高可用 | 服务独立部署、独立扩缩容 | 微服务拆分 + Docker + 健康检查 |
| 数据一致 | 库存零超卖、订单状态机严格 | Redis 预扣 + PG 事务 + 乐观锁 + Lua 脚本 |
| 可演进 | 新业务域可快速接入 | 统一项目结构 + 共享内部包 + 接口抽象 |
| 类型安全 | 编译时发现错误 | Go 强类型 + sqlc 生成 + struct tag 校验 |
| 安全 | 零信任、最小权限 | JWT + Caddy TLS + 服务间鉴权 + 幂等设计 |

### 1.2 架构拓扑

```
                              ┌──────────────┐
                              │    Caddy      │
                              │  (TLS终止)    │
                              │  :443 / :80   │
                              └──────┬───────┘
                                     │
                              ┌──────▼───────┐
                              │ API Gateway  │
                              │   (chi)      │
                              │   :3000      │
                              └──┬──┬──┬──┬──┘
                                 │  │  │  │
              ┌──────────────────┘  │  │  └──────────────────┐
              │          ┌─────────┘  └─────────┐           │
              ▼          ▼                      ▼           ▼
        ┌──────────┐ ┌──────────┐       ┌──────────┐ ┌──────────┐
        │  User    │ │ Product  │       │  Cart    │ │  Order   │
        │ Service  │ │ Service  │       │ Service  │ │ Service  │
        │  :3001   │ │  :3002   │       │  :3003   │ │  :3004   │
        └────┬─────┘ └────┬─────┘       └────┬─────┘ └────┬─────┘
             │            │                  │            │
    ┌────────▼────────────▼──────────────────▼────────────▼────────┐
    │                    PostgreSQL :5432                          │
    │       user_service | product_service | order_service         │
    │                  (schema 级隔离)                              │
    └─────────────────────────────────────────────────────────────┘
    ┌─────────────────────────────────────────────────────────────┐
    │                      Redis :6379                            │
    │  购物车 | 库存预扣 | 会话 | 缓存 | 分布式锁 | 限流 | 事件总线  │
    └─────────────────────────────────────────────────────────────┘
```

### 1.3 技术选型理由

**Go over TypeScript：** 学习 Go 语言，同时获得原生并发（goroutine）、静态编译（单二进制部署）、更低的内存占用和更高的吞吐量。

**chi over Gin/Echo：** 基于 stdlib `net/http`，不隐藏标准库，中间件模式与标准库兼容。Go 社区推荐的"薄框架"理念。

**sqlc + squirrel（双查询策略）：** sqlc 从 SQL 编译生成类型安全 Go 代码，处理固定 CRUD 查询；squirrel 运行时构建动态筛选/搜索 SQL（解决 sqlc 无法处理可选 WHERE 条件的局限）。两者配合覆盖所有查询场景。

**pgx over database/sql：** Go 生态最佳 PostgreSQL 驱动，内置连接池（pgxpool），支持 LISTEN/NOTIFY、COPY、自定义类型，性能优于标准库。

**koanf over Viper：** 无全局状态，类型安全，更模块化。Viper 的全局单例模式不符合 Go 的依赖注入最佳实践。

**singleflight：** 进程内缓存击穿防护。当多个 goroutine 同时请求同一个缓存 key（miss），singleflight 确保只有一个 goroutine 执行回源查询，其余等待结果共享。与 Redis 分布式锁互补——singleflight 防单进程内的重复回源，分布式锁防跨实例的缓存击穿。

**testcontainers-go：** 集成测试自动启停真实 PG/Redis 容器，无需预装外部依赖，测试结果可靠且可复现。

**Caddy：** 复用 TS 版配置，自动 HTTPS，Go 编写，配置极简。

**Redis（go-redis）：** 支持 Lua 脚本（EVALSHA）、Pipeline、Sentinel、`context.Context` 传递，API 设计符合 Go 惯例。

---

## 2. 双模式运行架构

本项目支持**单体模式（monolith）**和**微服务模式（microservices）**双模式运行，利用 Go 的优势在同一套代码上实现两种部署形态。

### 2.1 模式对比

| 方面 | 单体模式 | 微服务模式 |
|------|---------|-----------|
| 入口 | `cmd/monolith/main.go` | `cmd/{gateway,user,product,cart,order}/main.go` |
| 服务间调用 | 直接函数调用（零网络开销） | HTTP 内部接口 |
| 适用场景 | 本地开发、单机部署 | 生产环境独立扩缩容 |
| 启动命令 | `make dev` | Docker Compose 启动 5 个容器 |

### 2.2 服务接口抽象

服务间依赖通过 Go 接口定义。每个接口有两种实现：

```go
// order 包内定义依赖接口（消费方定义）
type ProductQuerier interface {
    BatchGetSKUs(ctx context.Context, skuIDs []string) ([]SKUInfo, error)
    ReserveStock(ctx context.Context, items []StockItem, orderID string) error
    ReleaseStock(ctx context.Context, items []StockItem, orderID string) error
}

type UserQuerier interface {
    GetAddress(ctx context.Context, addressID, userID string) (*Address, error)
    GetUser(ctx context.Context, userID string) (*UserInfo, error)
}

type CartCleaner interface {
    ClearItems(ctx context.Context, userID string, skuIDs []string) error
}
```

```go
// 微服务模式：HTTP 实现（通过 httpclient 调用 /internal/* 端点）
type httpProductClient struct {
    client *httpclient.InternalClient
}

func (c *httpProductClient) BatchGetSKUs(ctx context.Context, skuIDs []string) ([]SKUInfo, error) {
    var result []SKUInfo
    err := c.client.Post(ctx, "/internal/product/sku/batch", map[string]any{"skuIds": skuIDs}, &result)
    return result, err
}

// 单体模式：直接调用（零网络开销）
type localProductClient struct {
    stockService *product.StockService
    skuRepo      *product.SKURepository
}

func (c *localProductClient) BatchGetSKUs(ctx context.Context, skuIDs []string) ([]SKUInfo, error) {
    return c.skuRepo.BatchGet(ctx, skuIDs)
}
```

### 2.3 单体模式入口

```go
// cmd/monolith/main.go
func main() {
    // 加载配置
    cfg := config.LoadAll()

    // 初始化共享基础设施
    pool := database.NewPool(cfg.Database)
    rdb := database.NewRedis(cfg.Redis)

    // 初始化所有 service（共享连接池）
    userService := user.NewService(pool, rdb, cfg.User)
    productService := product.NewService(pool, rdb, cfg.Product)
    cartService := cart.NewService(rdb, cfg.Cart)
    orderService := order.NewService(pool, rdb, cfg.Order,
        // 注入 local 实现（直接函数调用）
        &localProductClient{stockService: productService.Stock, skuRepo: productService.SKURepo},
        &localUserClient{service: userService},
        &localCartClient{service: cartService},
    )

    // 组装路由（所有服务挂载到同一个 chi.Router）
    r := chi.NewRouter()
    r.Use(middleware.RequestID, middleware.Logger, middleware.CORS, middleware.RateLimit)

    user.MountRoutes(r, userService)
    product.MountRoutes(r, productService)
    cart.MountRoutes(r, cartService)
    order.MountRoutes(r, orderService)

    // 启动单个 HTTP 服务器
    srv := &http.Server{Addr: ":3000", Handler: r}
    // ... 优雅关停 ...
}
```

### 2.4 微服务模式入口

```go
// cmd/order/main.go
func main() {
    cfg := config.LoadOrder()
    pool := database.NewPool(cfg.Database)
    rdb := database.NewRedis(cfg.Redis)

    orderService := order.NewService(pool, rdb, cfg,
        // 注入 HTTP 实现（通过网络调用其他服务）
        &httpProductClient{client: httpclient.New(cfg.ProductServiceURL)},
        &httpUserClient{client: httpclient.New(cfg.UserServiceURL)},
        &httpCartClient{client: httpclient.New(cfg.CartServiceURL)},
    )

    r := chi.NewRouter()
    order.MountRoutes(r, orderService)

    srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
    // ... 优雅关停 ...
}
```

---

## 3. Go 与 TS 版本的关键差异

| 方面 | TS (Hono + Bun) | Go (chi + stdlib) |
|------|-----------------|-------------------|
| 错误处理 | `throw AppError` → 全局 catch | `return error` → Handler Wrap 包装器 |
| 上下文传递 | AsyncLocalStorage（隐式） | `context.Context`（显式第一参数） |
| 校验 | Zod schema 对象 | struct tag `validate:"required,email"` |
| 中间件 | `async (c, next) => {}` | `func(next http.Handler) http.Handler` |
| 数据库查询 | Drizzle 运行时构建 | sqlc 编译时生成 + squirrel 动态构建 |
| 依赖注入 | 模块导入 | 构造函数注入（接口） |
| 并发 | `Promise.all` | `goroutine + errgroup` |
| 关停 | `process.on('SIGTERM')` | `signal.NotifyContext + WaitGroup` |
| JSON 序列化 | 原生支持 | `encoding/json` + struct tag |
| 部署产物 | Docker + Bun 运行时 | 静态编译单二进制（go:embed 嵌入 Lua） |
| 包管理 | npm workspace（monorepo） | 单 `go.mod`（所有服务共享） |
| 运行模式 | 只有微服务模式 | 单体 + 微服务双模式 |
| 缓存击穿 | 无专门处理 | singleflight 进程内防护 |
| 集成测试 | 外部 PG/Redis | testcontainers 自动启停 |

### Go 特有模式

**Handler Wrap 模式（替代 recovery 中间件 + 全局 catch）：**
```go
// internal/handler/wrap.go
type AppHandler func(w http.ResponseWriter, r *http.Request) error

func Wrap(h AppHandler) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if err := h(w, r); err != nil {
            response.HandleError(w, r, err)  // AppError → HTTP 响应，未知 error → 500
        }
    }
}

// handler 不需要自己处理错误格式化
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

**错误 wrapping：**
```go
if err != nil {
    return fmt.Errorf("create order: %w", err)
}
```

**Table-driven tests：**
```go
tests := []struct {
    name    string
    input   CreateOrderInput
    wantErr bool
}{
    {"valid order", validInput, false},
    {"empty items", emptyItems, true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

**Lua 脚本嵌入（go:embed，编译进二进制）：**
```go
import _ "embed"

//go:embed stock-deduct.lua
var stockDeductScript string
```

**优雅关停：**
```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
// ... start server ...
<-ctx.Done()
srv.Shutdown(timeoutCtx)
```

**singleflight 缓存击穿防护：**
```go
var group singleflight.Group

func (s *ProductService) GetDetail(ctx context.Context, id string) (*Product, error) {
    v, err, _ := group.Do("product:"+id, func() (interface{}, error) {
        // 只有一个 goroutine 执行，其余等待共享结果
        return s.loadFromDBAndCache(ctx, id)
    })
    return v.(*Product), err
}
```

---

## 3. 服务边界定义

### 3.1 User Service（用户域）— :3001

**职责边界：** 用户身份全生命周期管理

| 能力 | 说明 |
|------|------|
| 注册 / 登录 | 邮箱+密码注册，JWT 签发 |
| 用户资料 CRUD | 昵称、头像、联系方式 |
| 地址管理 | 收货地址增删改查，默认地址 |
| 会话管理 | Token 刷新、登出（Redis 黑名单） |
| 密码安全 | Argon2id 哈希 |

**不负责：** 订单、支付、购物车、权限策略

### 3.2 Product Service（商品域）— :3002

**职责边界：** 商品信息与库存全生命周期管理

| 能力 | 说明 |
|------|------|
| 商品 CRUD | 标题、描述、价格、图片、属性 |
| 分类体系 | 多级分类树，商品-分类多对多 |
| SKU 管理 | 规格组合（颜色/尺码）、独立定价 |
| 库存管理 | Redis 预扣 + DB 最终一致 + 乐观锁 |
| 搜索 | PostgreSQL 全文搜索 + 分类筛选 + Redis 缓存 |

**不负责：** 购物车、订单、定价策略、促销活动

### 3.3 Cart Service（购物车域）— :3003

**职责边界：** 购物车全生命周期

| 能力 | 说明 |
|------|------|
| 购物车 CRUD | 添加/修改数量/删除商品 |
| 存储策略 | Redis Hash（cart:{userId}） |
| 商品快照 | 添加时记录价格快照，结算时实时校验 |
| 勾选状态 | 支持部分商品勾选结算 |
| 库存预校验 | 加入购物车时检查库存（仅提示，不锁定） |

**不负责：** 库存扣减、订单创建、支付

### 3.4 Order Service（订单与支付域）— :3004

**职责边界：** 订单全生命周期 + 支付集成

| 能力 | 说明 |
|------|------|
| 订单创建 | 购物车结算 → 库存预扣 → 生成订单 |
| 订单状态机 | pending → paid → shipped → delivered → completed / cancelled / refunded |
| 支付集成 | 支付网关对接预留（Stripe / 支付宝 / 微信） |
| 支付回调 | 异步通知处理 + 幂等校验 |
| 订单超时 | goroutine + time.Ticker，ZRANGEBYSCORE 自动取消 |
| 幂等设计 | X-Idempotency-Key + Redis 去重 |

**不负责：** 物流追踪、退款审核、发票

### 3.5 API Gateway — :3000

**职责边界：** 唯一外部入口，横切关注点

| 能力 | 说明 |
|------|------|
| 路由转发 | httputil.ReverseProxy 转发 + header 注入 |
| 鉴权 | JWT 验证 + 用户上下文注入 |
| 限流 | Redis ZSET 滑动窗口（IP + 用户双维度） |
| 幂等层 | X-Idempotency-Key 网关级去重 |
| 请求追踪 | traceId 生成 & 向下游透传 |
| 日志 | 统一请求/响应日志 |
| CORS | 跨域策略管理 |

---

## 4. 数据库设计

### 4.1 Schema 隔离策略

每个 service 使用独立 PostgreSQL schema，共享连接池但逻辑隔离：

```sql
CREATE SCHEMA IF NOT EXISTS user_service;
CREATE SCHEMA IF NOT EXISTS product_service;
CREATE SCHEMA IF NOT EXISTS order_service;
-- Cart 纯 Redis，不需要 PG schema
```

### 4.2 公共字段约定

所有表必须包含：

```sql
id          VARCHAR(21)    PRIMARY KEY,   -- nanoid
created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
```

### 4.3 查询策略 — sqlc + squirrel

**固定 CRUD → sqlc（编译时类型安全）：**

```sql
-- internal/database/queries/users.sql

-- name: GetUserByEmail :one
SELECT * FROM user_service.users WHERE email = $1 AND deleted_at IS NULL;

-- name: CreateUser :one
INSERT INTO user_service.users (id, email, password, nickname, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
RETURNING *;
```

**动态筛选/搜索 → squirrel（运行时构建，参数化防注入）：**

```go
import sq "github.com/Masterminds/squirrel"

// squirrel 使用 PostgreSQL 风格占位符
var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

func (r *ProductRepo) Search(ctx context.Context, input SearchInput) ([]Product, error) {
    qb := psql.Select("id", "title", "min_price", "total_sales").
        From("product_service.products").
        Where("status = 'active'").
        Where("deleted_at IS NULL")

    if input.Keyword != "" {
        qb = qb.Where(
            "to_tsvector('simple', title || ' ' || coalesce(description,'') || ' ' || coalesce(brand,'')) @@ plainto_tsquery('simple', ?)",
            input.Keyword,
        )
    }
    if input.CategoryID != "" {
        qb = qb.Where("id IN (SELECT product_id FROM product_service.product_categories WHERE category_id = ?)", input.CategoryID)
    }
    if input.PriceMin > 0 {
        qb = qb.Where("min_price >= ?", input.PriceMin)
    }
    if input.PriceMax > 0 {
        qb = qb.Where("max_price <= ?", input.PriceMax)
    }

    sql, args, _ := qb.ToSql()
    rows, err := r.pool.Query(ctx, sql, args...)
    // ...
}
```

### 4.4 User Service 表结构

```
┌─────────────────────────────────────┐
│              users                   │
├─────────────────────────────────────┤
│ id          VARCHAR(21)    PK        │
│ email       VARCHAR(255)   UNIQUE    │
│ password    VARCHAR(255)   NOT NULL  │  ← Argon2id hash
│ nickname    VARCHAR(50)              │
│ avatar_url  TEXT                     │
│ phone       VARCHAR(20)              │
│ status      VARCHAR(20)    DEFAULT   │  ← active / suspended / deleted
│ last_login  TIMESTAMPTZ              │
│ created_at  TIMESTAMPTZ    NOT NULL  │
│ updated_at  TIMESTAMPTZ    NOT NULL  │
│ deleted_at  TIMESTAMPTZ              │  ← 软删除
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│          user_addresses              │
├─────────────────────────────────────┤
│ id          VARCHAR(21)    PK        │
│ user_id     VARCHAR(21)    FK→users  │
│ label       VARCHAR(50)              │  ← "家", "公司"
│ recipient   VARCHAR(100)   NOT NULL  │
│ phone       VARCHAR(20)    NOT NULL  │
│ province    VARCHAR(50)    NOT NULL  │
│ city        VARCHAR(50)    NOT NULL  │
│ district    VARCHAR(50)    NOT NULL  │
│ address     TEXT           NOT NULL  │
│ postal_code VARCHAR(10)              │
│ is_default  BOOLEAN        DEFAULT   │
│ created_at  TIMESTAMPTZ    NOT NULL  │
│ updated_at  TIMESTAMPTZ    NOT NULL  │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│          refresh_tokens              │
├─────────────────────────────────────┤
│ id          VARCHAR(21)    PK        │
│ user_id     VARCHAR(21)    FK→users  │
│ token_hash  VARCHAR(255)   UNIQUE    │  ← SHA-256 of token
│ expires_at  TIMESTAMPTZ    NOT NULL  │
│ revoked_at  TIMESTAMPTZ              │
│ created_at  TIMESTAMPTZ    NOT NULL  │
└─────────────────────────────────────┘
```

### 4.5 Product Service 表结构

```
┌─────────────────────────────────────┐
│           categories                 │
├─────────────────────────────────────┤
│ id          VARCHAR(21)    PK        │
│ parent_id   VARCHAR(21)    FK→self   │  ← 多级分类
│ name        VARCHAR(100)   NOT NULL  │
│ slug        VARCHAR(100)   UNIQUE    │
│ icon_url    TEXT                     │
│ sort_order  INTEGER        DEFAULT 0 │
│ is_active   BOOLEAN        DEFAULT   │
│ created_at  TIMESTAMPTZ    NOT NULL  │
│ updated_at  TIMESTAMPTZ    NOT NULL  │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│            products                  │
├─────────────────────────────────────┤
│ id          VARCHAR(21)    PK        │
│ title       VARCHAR(200)   NOT NULL  │
│ slug        VARCHAR(200)   UNIQUE    │
│ description TEXT                     │
│ brand       VARCHAR(100)             │
│ status      VARCHAR(20)    DEFAULT   │  ← draft / active / archived
│ attributes  JSONB                    │
│ min_price   DECIMAL(12,2)            │  ← 冗余，列表展示用
│ max_price   DECIMAL(12,2)            │
│ total_sales INTEGER        DEFAULT 0 │
│ created_at  TIMESTAMPTZ    NOT NULL  │
│ updated_at  TIMESTAMPTZ    NOT NULL  │
│ deleted_at  TIMESTAMPTZ              │
└─────────────────────────────────────┘
  │
  │  多对多
  ▼
┌─────────────────────────────────────┐
│      product_categories              │
├─────────────────────────────────────┤
│ product_id  VARCHAR(21)    FK        │
│ category_id VARCHAR(21)    FK        │
│ PRIMARY KEY (product_id, category_id)│
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│          product_images              │
├─────────────────────────────────────┤
│ id          VARCHAR(21)    PK        │
│ product_id  VARCHAR(21)    FK        │
│ url         TEXT           NOT NULL  │
│ alt_text    VARCHAR(200)             │
│ is_primary  BOOLEAN        DEFAULT   │
│ sort_order  INTEGER        DEFAULT 0 │
│ created_at  TIMESTAMPTZ    NOT NULL  │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│              skus                    │
├─────────────────────────────────────┤
│ id          VARCHAR(21)    PK        │
│ product_id  VARCHAR(21)    FK        │
│ sku_code    VARCHAR(50)    UNIQUE    │
│ price       DECIMAL(12,2)  NOT NULL  │
│ compare_price DECIMAL(12,2)          │  ← 划线价
│ cost_price  DECIMAL(12,2)            │  ← 成本价
│ stock       INTEGER        DEFAULT 0 │  ← DB 真实库存
│ low_stock   INTEGER        DEFAULT 5 │
│ weight      DECIMAL(8,2)             │
│ attributes  JSONB                    │  ← {"color":"红","size":"XL"}
│ barcode     VARCHAR(50)              │
│ status      VARCHAR(20)    DEFAULT   │  ← active / inactive
│ version     INTEGER        DEFAULT 0 │  ← 乐观锁
│ created_at  TIMESTAMPTZ    NOT NULL  │
│ updated_at  TIMESTAMPTZ    NOT NULL  │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│             banners                  │
├─────────────────────────────────────┤
│ id          VARCHAR(21)    PK        │
│ title       VARCHAR(200)   NOT NULL  │
│ subtitle    VARCHAR(200)             │
│ image_url   TEXT           NOT NULL  │
│ link_type   VARCHAR(20)              │  ← product / category
│ link_value  VARCHAR(200)             │
│ sort_order  INTEGER        DEFAULT 0 │
│ is_active   BOOLEAN        DEFAULT   │
│ start_at    TIMESTAMPTZ              │
│ end_at      TIMESTAMPTZ              │
│ created_at  TIMESTAMPTZ    NOT NULL  │
│ updated_at  TIMESTAMPTZ    NOT NULL  │
└─────────────────────────────────────┘
```

### 4.6 Order Service 表结构

```
┌─────────────────────────────────────────┐
│               orders                     │
├─────────────────────────────────────────┤
│ id              VARCHAR(21)    PK        │
│ order_no        VARCHAR(32)    UNIQUE    │  ← 业务订单号
│ user_id         VARCHAR(21)    NOT NULL  │
│ status          VARCHAR(20)    NOT NULL  │  ← pending/paid/shipped/delivered/completed/cancelled/refunded
│ total_amount    DECIMAL(12,2)  NOT NULL  │
│ discount_amount DECIMAL(12,2)  DEFAULT 0 │
│ pay_amount      DECIMAL(12,2)  NOT NULL  │
│ payment_method  VARCHAR(20)              │
│ payment_no      VARCHAR(100)             │
│ paid_at         TIMESTAMPTZ              │
│ shipped_at      TIMESTAMPTZ              │
│ delivered_at    TIMESTAMPTZ              │
│ completed_at    TIMESTAMPTZ              │
│ cancelled_at    TIMESTAMPTZ              │
│ cancel_reason   TEXT                     │
│ remark          TEXT                     │
│ idempotency_key VARCHAR(64)    UNIQUE    │
│ expires_at      TIMESTAMPTZ    NOT NULL  │  ← 支付截止
│ version         INTEGER        DEFAULT 0 │  ← 乐观锁
│ created_at      TIMESTAMPTZ    NOT NULL  │
│ updated_at      TIMESTAMPTZ    NOT NULL  │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│            order_items                   │
├─────────────────────────────────────────┤
│ id              VARCHAR(21)    PK        │
│ order_id        VARCHAR(21)    FK→orders │
│ product_id      VARCHAR(21)    NOT NULL  │
│ sku_id          VARCHAR(21)    NOT NULL  │
│ product_title   VARCHAR(200)   NOT NULL  │  ← 快照
│ sku_attrs       JSONB          NOT NULL  │  ← 快照
│ image_url       TEXT                     │  ← 快照
│ unit_price      DECIMAL(12,2)  NOT NULL  │
│ quantity        INTEGER        NOT NULL  │
│ subtotal        DECIMAL(12,2)  NOT NULL  │
│ created_at      TIMESTAMPTZ    NOT NULL  │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│          order_addresses                 │
├─────────────────────────────────────────┤
│ id              VARCHAR(21)    PK        │
│ order_id        VARCHAR(21)    FK UNIQUE │  ← 一单一地址
│ recipient       VARCHAR(100)   NOT NULL  │
│ phone           VARCHAR(20)    NOT NULL  │
│ province        VARCHAR(50)    NOT NULL  │
│ city            VARCHAR(50)    NOT NULL  │
│ district        VARCHAR(50)    NOT NULL  │
│ address         TEXT           NOT NULL  │
│ postal_code     VARCHAR(10)              │
│ created_at      TIMESTAMPTZ    NOT NULL  │
└─────────────────────────────────────────┘
  ← 快照！不 FK 到 user_addresses

┌─────────────────────────────────────────┐
│         payment_records                  │
├─────────────────────────────────────────┤
│ id              VARCHAR(21)    PK        │
│ order_id        VARCHAR(21)    FK→orders │
│ payment_method  VARCHAR(20)    NOT NULL  │
│ amount          DECIMAL(12,2)  NOT NULL  │
│ status          VARCHAR(20)    NOT NULL  │  ← pending / success / failed / refunded
│ transaction_id  VARCHAR(100)             │
│ raw_notify      JSONB                    │  ← 原始回调报文
│ idempotency_key VARCHAR(64)    UNIQUE    │
│ created_at      TIMESTAMPTZ    NOT NULL  │
│ updated_at      TIMESTAMPTZ    NOT NULL  │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│         stock_operations                 │
├─────────────────────────────────────────┤
│ id              VARCHAR(21)    PK        │
│ sku_id          VARCHAR(21)    NOT NULL  │
│ order_id        VARCHAR(21)              │
│ type            VARCHAR(20)    NOT NULL  │  ← reserve / confirm / release / adjust
│ quantity        INTEGER        NOT NULL  │
│ created_at      TIMESTAMPTZ    NOT NULL  │
└─────────────────────────────────────────┘
```

### 4.7 索引策略

```sql
-- User Service
CREATE INDEX idx_users_email ON user_service.users(email);
CREATE INDEX idx_users_status ON user_service.users(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_user_addresses_user ON user_service.user_addresses(user_id);
CREATE INDEX idx_refresh_tokens_user ON user_service.refresh_tokens(user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_refresh_tokens_expires ON user_service.refresh_tokens(expires_at) WHERE revoked_at IS NULL;

-- Product Service
CREATE INDEX idx_products_status ON product_service.products(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_slug ON product_service.products(slug);
CREATE INDEX idx_products_brand ON product_service.products(brand) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_sales ON product_service.products(total_sales DESC) WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX idx_products_fulltext ON product_service.products
  USING GIN(to_tsvector('simple', title || ' ' || coalesce(description, '') || ' ' || coalesce(brand, '')));
CREATE INDEX idx_products_attrs ON product_service.products USING GIN(attributes);
CREATE INDEX idx_skus_product ON product_service.skus(product_id);
CREATE INDEX idx_skus_code ON product_service.skus(sku_code);
CREATE INDEX idx_skus_stock_low ON product_service.skus(product_id) WHERE stock <= low_stock AND status = 'active';
CREATE INDEX idx_categories_parent ON product_service.categories(parent_id);
CREATE INDEX idx_categories_slug ON product_service.categories(slug);

-- Order Service
CREATE INDEX idx_orders_user ON order_service.orders(user_id);
CREATE INDEX idx_orders_user_status ON order_service.orders(user_id, status);
CREATE INDEX idx_orders_status ON order_service.orders(status);
CREATE INDEX idx_orders_no ON order_service.orders(order_no);
CREATE INDEX idx_orders_expires ON order_service.orders(expires_at) WHERE status = 'pending';
CREATE INDEX idx_orders_idempotency ON order_service.orders(idempotency_key);
CREATE INDEX idx_order_items_order ON order_service.order_items(order_id);
CREATE INDEX idx_order_items_sku ON order_service.order_items(sku_id);
CREATE INDEX idx_payment_records_order ON order_service.payment_records(order_id);
CREATE INDEX idx_stock_ops_sku ON order_service.stock_operations(sku_id);
CREATE INDEX idx_stock_ops_order ON order_service.stock_operations(order_id);
```

---

## 5. Redis 使用规范

### 5.1 Key 命名约定

```
{service}:{resource}:{id}:{sub}

示例：
user:session:blacklist:{tokenJti}      → JWT 黑名单（SET，TTL = token 剩余有效期）
user:profile:{userId}                  → 用户信息缓存（STRING JSON）

product:detail:{productId}             → 商品详情缓存（STRING JSON）
product:hot:list                       → 热门商品列表缓存（STRING JSON）
product:category:tree                  → 分类树缓存（STRING JSON）
product:search:{queryHash}             → 搜索结果缓存（STRING JSON）

stock:{skuId}                          → SKU 可用库存（STRING INT，Lua 原子操作）
stock:lock:{skuId}                     → 库存操作分布式锁（STRING，SET NX EX）

cart:{userId}                          → 购物车（HASH，field=skuId，value=JSON{qty,snapshot}）

order:timeout:{orderId}                → 订单超时延迟队列（ZSET，score=过期时间戳）
order:idempotent:{key}                 → 幂等键（STRING，TTL=24h）
order:lock:{orderId}                   → 订单操作锁（STRING，SET NX EX）

gateway:ratelimit:{ip}                 → IP 限流（ZSET，滑动窗口）
gateway:ratelimit:user:{userId}        → 用户级限流
```

### 5.2 TTL 策略

| Key 类型 | TTL | 说明 |
|----------|-----|------|
| session blacklist | = access token 剩余有效期 | 登出后阻止旧 token |
| user profile cache | 30 min | 低频变更 |
| product detail cache | 10 min | 中频变更 |
| hot products list | 5 min | 定期刷新 |
| category tree | 60 min | 极低频变更 |
| search result cache | 3 min | 高频变更 |
| stock counter | 无 TTL | 与 DB 同步 |
| cart (logged in) | 30 days | 长期保留 |
| order timeout ZSET | 无 TTL | 消费后删除 |
| idempotent key | 24h | 防止重复提交 |
| rate limit | 滑动窗口 60s | 自动过期 |
| distributed lock | 10-30s | 根据操作时长 |

### 5.3 缓存策略

```
Cache-Aside 模式（默认）：
  读：先 Redis → miss → 查 DB → 写 Redis
  写：先写 DB → 删 Redis（不是更新 Redis）

防缓存穿透：
  DB 查无结果 → Redis 写入空值 { "empty": true }，TTL = 60s

防缓存雪崩：
  TTL 加随机抖动：baseTTL + random(0, baseTTL * 0.2)

防缓存击穿（热 key）：
  进程内：singleflight — 同一进程内多个 goroutine 请求同一 key，仅一个回源
  跨实例：Redis SET NX 分布式锁 — 多实例中仅一个获得锁回源
  两者组合：singleflight 减少本进程发起的锁竞争，分布式锁解决跨实例问题
```

---

## 6. 认证 & 鉴权设计

### 6.1 JWT 双 Token 机制

```
Access Token:   短期（15 min），无状态验证
Refresh Token:  长期（7 days），存储在 DB + HttpOnly Cookie

流程：
1. 登录 → 签发 access + refresh token
2. 请求 → Gateway 用 access token 验证
3. 过期 → 客户端用 refresh token 换新 access token
4. 登出 → refresh token 写入 revoked_at + access token JTI 加入 Redis 黑名单
```

### 6.2 JWT Payload

```go
type Claims struct {
    jwt.RegisteredClaims
    Email string `json:"email"`
}
// RegisteredClaims includes: sub (userId), jti, iat, exp
```

---

## 7. 错误码体系

### 7.1 HTTP 状态码映射

| 状态码 | 错误类型 | 场景 |
|--------|---------|------|
| 400 | BadRequest | 参数格式错误 |
| 401 | Unauthorized | 未登录 / Token 无效 |
| 403 | Forbidden | 无权限访问 |
| 404 | NotFound | 资源不存在 |
| 409 | Conflict | 资源冲突 |
| 422 | Validation | 业务校验失败 |
| 429 | RateLimit | 请求过于频繁 |
| 500 | Internal | 系统内部错误 |

### 7.2 业务错误码

```go
const (
    // User 域 (1xxx)
    ErrCodeUserNotFound      = "USER_1001"
    ErrCodeUserAlreadyExists = "USER_1002"
    ErrCodeInvalidCredentials = "USER_1003"
    ErrCodeTokenExpired      = "USER_1004"
    ErrCodeTokenRevoked      = "USER_1005"
    ErrCodePasswordTooWeak   = "USER_1006"
    ErrCodeEmailNotVerified  = "USER_1007"
    ErrCodeAddressLimit      = "USER_1008"

    // Product 域 (2xxx)
    ErrCodeProductNotFound    = "PRODUCT_2001"
    ErrCodeSKUNotFound        = "PRODUCT_2002"
    ErrCodeStockInsufficient  = "PRODUCT_2003"
    ErrCodeCategoryNotFound   = "PRODUCT_2004"
    ErrCodeDuplicateSKUCode   = "PRODUCT_2005"
    ErrCodeInvalidPrice       = "PRODUCT_2006"
    ErrCodeProductUnavailable = "PRODUCT_2007"

    // Cart 域 (3xxx)
    ErrCodeCartItemNotFound   = "CART_3001"
    ErrCodeCartLimitExceeded  = "CART_3002"
    ErrCodeCartSKUUnavailable = "CART_3003"
    ErrCodeCartPriceChanged   = "CART_3004"

    // Order 域 (4xxx)
    ErrCodeOrderNotFound      = "ORDER_4001"
    ErrCodeOrderStatusInvalid = "ORDER_4002"
    ErrCodeOrderExpired       = "ORDER_4003"
    ErrCodeOrderAlreadyPaid   = "ORDER_4004"
    ErrCodeOrderCancelDenied  = "ORDER_4005"
    ErrCodePaymentFailed      = "ORDER_4006"
    ErrCodeIdempotentConflict = "ORDER_4007"

    // Admin 域 (5xxx)
    ErrCodeAdminNotFound       = "ADMIN_5001"
    ErrCodeAdminAlreadyExists  = "ADMIN_5002"
    ErrCodeAdminInvalidCreds   = "ADMIN_5003"
    ErrCodeAdminTokenExpired   = "ADMIN_5004"
    ErrCodeAdminForbidden      = "ADMIN_5005"
    ErrCodeAdminTokenRevoked   = "ADMIN_5006"

    // Gateway (9xxx)
    ErrCodeRateLimited        = "GATEWAY_9001"
    ErrCodeServiceUnavailable = "GATEWAY_9002"
)
```

---

## 8. API 路由规范

### 8.1 全 POST 约定

所有接口统一使用 `POST` 方法，参数通过 JSON Body 传递。
路由路径通过动词后缀区分操作类型，资源 ID 也放入 Body。

```
POST /api/v1/{domain}/{action}

动作后缀约定：
  /list     → 列表查询（分页）
  /detail   → 单条详情
  /create   → 创建
  /update   → 更新
  /delete   → 删除（软删除）
```

### 8.2 路由表

```
# ──── 公开路由（无需认证）────────────────────────────────
POST   /api/v1/auth/register
POST   /api/v1/auth/login
POST   /api/v1/auth/refresh

POST   /api/v1/product/list
POST   /api/v1/product/detail
POST   /api/v1/product/search
POST   /api/v1/product/sku/list

POST   /api/v1/category/list
POST   /api/v1/category/detail
POST   /api/v1/category/tree

POST   /api/v1/banner/list

# ──── 需要认证 ──────────────────────────────────────────
POST   /api/v1/auth/logout

POST   /api/v1/user/profile
POST   /api/v1/user/update
POST   /api/v1/user/address/list
POST   /api/v1/user/address/create
POST   /api/v1/user/address/update
POST   /api/v1/user/address/delete

POST   /api/v1/cart/list
POST   /api/v1/cart/add
POST   /api/v1/cart/update
POST   /api/v1/cart/remove
POST   /api/v1/cart/clear
POST   /api/v1/cart/select
POST   /api/v1/cart/checkout/preview

POST   /api/v1/order/create              # + X-Idempotency-Key
POST   /api/v1/order/list
POST   /api/v1/order/detail
POST   /api/v1/order/cancel

POST   /api/v1/payment/create            # + X-Idempotency-Key
POST   /api/v1/payment/notify            # 公开（第三方回调）
POST   /api/v1/payment/query

# ──── 管理端 ──────────────────────────────
POST   /api/v1/admin/auth/login
POST   /api/v1/admin/product/create
POST   /api/v1/admin/product/update
POST   /api/v1/admin/product/delete
POST   /api/v1/admin/product/sku/create
POST   /api/v1/admin/product/sku/update
POST   /api/v1/admin/category/create
POST   /api/v1/admin/category/update
POST   /api/v1/admin/order/list
POST   /api/v1/admin/order/ship
POST   /api/v1/admin/stock/adjust

# ──── 内部接口（Docker 内部）──────────────
POST   /internal/user/detail
POST   /internal/user/batch
POST   /internal/user/address/detail
POST   /internal/product/sku/batch
POST   /internal/stock/reserve
POST   /internal/stock/release
POST   /internal/stock/confirm
POST   /internal/stock/sync
POST   /internal/cart/clear-items
```

---

## 9. 库存与并发控制

### 9.1 库存扣减流程（下单时）

```
1. 用户提交订单
       │
2. [Redis Lua 脚本] 原子扣减库存
       │  → 成功：stock:{skuId} -= quantity
       │  → 失败：返回 STOCK_INSUFFICIENT
       │
3. [PostgreSQL 事务]
       │  → 创建 order + order_items + order_address
       │  → 创建 stock_operation (type=reserve)
       │
4. 返回 orderId
       │
5. [goroutine + time.Ticker] 检查超时：
       │  → 已支付：stock_operation (type=confirm)
       │           → UPDATE skus SET stock = stock - qty, version = version + 1
       │              WHERE id = $id AND version = $version (乐观锁)
       │  → 未支付：stock_operation (type=release)
       │           → [Redis Lua] stock:{skuId} += quantity
       │           → 订单状态 → cancelled
```

### 9.2 Redis Lua 库存扣减脚本

```lua
-- stock-deduct.lua: 单 SKU
-- KEYS[1] = stock:{skuId}, ARGV[1] = quantity
local stock = tonumber(redis.call('GET', KEYS[1]))
if stock == nil then return -1 end
if stock < tonumber(ARGV[1]) then return 0 end
redis.call('DECRBY', KEYS[1], ARGV[1])
return 1

-- stock-deduct-multi.lua: 多 SKU 原子扣减
-- KEYS = [stock:sku1, stock:sku2, ...], ARGV = [qty1, qty2, ...]
-- Phase 1: 检查所有库存
for i = 1, #KEYS do
  local stock = tonumber(redis.call('GET', KEYS[i]))
  if stock == nil or stock < tonumber(ARGV[i]) then
    return i  -- 返回不足的 SKU 索引
  end
end
-- Phase 2: 全部扣减
for i = 1, #KEYS do
  redis.call('DECRBY', KEYS[i], ARGV[i])
end
return 0
```

### 9.3 Go 实现要点

```go
// Lua 脚本通过 go:embed 嵌入，编译进二进制
//go:embed stock-deduct.lua
var stockDeductScript string

// 服务启动时加载脚本到 Redis（获取 SHA）
sha, err := rdb.ScriptLoad(ctx, stockDeductScript).Result()

// 调用时使用 EVALSHA（省带宽）
result, err := rdb.EvalSha(ctx, sha, []string{"stock:" + skuID}, quantity).Int()
switch result {
case 1:  // 成功
case 0:  // 库存不足
case -1: // key 不存在
}
```

### 9.4 订单超时自动取消

```go
// 使用 goroutine + time.Ticker 定时轮询
func (s *TimeoutService) Start(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.processExpiredOrders(ctx)
        }
    }
}

func (s *TimeoutService) processExpiredOrders(ctx context.Context) {
    now := float64(time.Now().Unix())
    // ZRANGEBYSCORE order:timeout 0 {now} LIMIT 0 100
    orderIDs, _ := s.rdb.ZRangeByScore(ctx, "order:timeout", &redis.ZRangeBy{
        Min: "0", Max: fmt.Sprintf("%f", now), Count: 100,
    }).Result()

    for _, orderID := range orderIDs {
        s.cancelExpiredOrder(ctx, orderID)
        s.rdb.ZRem(ctx, "order:timeout", orderID)
    }
}
```

---

## 10. 购物车设计

### 10.1 存储模型

```
Redis Hash: cart:{userId}
  field: {skuId}
  value: JSON {
    "quantity": 2,
    "selected": true,
    "addedAt": "2025-01-01T00:00:00Z",
    "snapshot": {
      "productId": "xxx",
      "productTitle": "...",
      "skuAttrs": {"color":"红"},
      "price": 99.00,
      "imageUrl": "..."
    }
  }

单用户购物车上限：50 个 SKU
```

### 10.2 购物车列表查询

```
1. HGETALL cart:{userId}
2. 批量查询 SKU 最新信息（内部接口 / 直接调用）
3. 对比快照，标记变化
4. 返回合并后的购物车列表
```

---

## 11. 服务间通信

### 11.1 双模式服务间调用

```go
// ── 接口定义（消费方定义）──
// order 包中
type ProductQuerier interface {
    BatchGetSKUs(ctx context.Context, skuIDs []string) ([]SKUInfo, error)
    ReserveStock(ctx context.Context, items []StockItem, orderID string) error
}

// ── 微服务模式：HTTP 调用 ──
type httpProductClient struct {
    client *httpclient.InternalClient
}

func (c *httpProductClient) BatchGetSKUs(ctx context.Context, skuIDs []string) ([]SKUInfo, error) {
    var result []SKUInfo
    err := c.client.Post(ctx, "/internal/product/sku/batch", map[string]any{"skuIds": skuIDs}, &result)
    return result, err
}

// ── 单体模式：直接调用 ──
type localProductClient struct {
    skuRepo      product.SKURepository
    stockService *product.StockService
}

func (c *localProductClient) BatchGetSKUs(ctx context.Context, skuIDs []string) ([]SKUInfo, error) {
    return c.skuRepo.BatchGet(ctx, skuIDs)  // 直接数据库查询，无 HTTP
}
```

### 11.2 HTTP 内部客户端

```go
// 仅微服务模式使用
type InternalClient struct {
    httpClient *http.Client
    baseURL    string
    secret     string
}

func (c *InternalClient) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
    // 注入 x-trace-id, x-user-id, x-internal-token
    // JSON 编解码
    // 错误处理
}
```

### 11.3 内部接口约定

```
/internal/ 前缀，仅 Docker 内部网络可访问
Gateway 配置 internal_only 中间件拦截外部请求
单体模式下不需要 /internal/ 端点（直接函数调用）
```

---

## 12. 搜索与性能优化

### 12.1 商品搜索

```
PostgreSQL 全文搜索：
  → ts_vector + GIN 索引
  → to_tsvector('simple', title || ' ' || coalesce(description, '') || ' ' || coalesce(brand, ''))
  → 搜索结果按 ts_rank 排序
  → 动态条件使用 squirrel 构建（参数化查询防注入）

缓存：搜索结果按 queryHash 缓存 3 分钟
```

### 12.2 多级缓存

```
L1: singleflight（进程内去重，防单进程缓存击穿）
L2: Redis 缓存（热数据，TTL + 随机抖动）
L3: PostgreSQL（冷数据，连接池限制并发）
写入后回写 Redis（Cache-Aside）
```

### 12.3 数据库连接池

```go
config := pgxpool.Config{
    MaxConns:          20,    // 单 service 最大连接数
    MinConns:          5,
    MaxConnLifetime:   time.Hour,
    MaxConnIdleTime:   30 * time.Minute,
    HealthCheckPeriod: time.Minute,
}
// 微服务模式：5 个 service × 20 = 100 ≈ PG max_connections(200)
// 单体模式：1 个进程共享连接池，MaxConns 可适当调高
```

---

## 13. 安全清单

| 项目 | 实现 | 阶段 |
|------|------|------|
| HTTPS 终止 | Caddy 自动证书 | Phase 1 |
| 密码哈希 | Argon2id | Phase 2 |
| JWT 短期 Token + JTI | 15 min + refresh + 黑名单 | Phase 2 |
| CORS 白名单 | middleware/cors.go | Phase 4 |
| 限流 | Redis 滑动窗口 | Phase 4 |
| SQL 注入防护 | sqlc 参数化 + squirrel 参数化 | Phase 3 |
| XSS 防护 | response JSON-only | Phase 2 |
| 环境变量隔离 | .env 不入仓库 | Phase 0 |
| 请求追踪 | traceId 全链路 | Phase 2 |
| 幂等设计 | X-Idempotency-Key + Redis | Phase 4 |
| 服务间鉴权 | x-internal-token + 网络隔离 | Phase 5 |
| 订单金额校验 | 服务端重算 | Phase 9 |
| 库存防超卖 | Redis Lua + 乐观锁 | Phase 7 |

---

## 14. 项目结构

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
├── cmd/
│   ├── monolith/main.go               # 单体模式入口（本地开发）
│   ├── gateway/main.go                # :3000（微服务模式）
│   ├── user/main.go                   # :3001
│   ├── product/main.go                # :3002
│   ├── cart/main.go                   # :3003
│   ├── order/main.go                  # :3004
│   └── migrate/main.go               # 迁移工具
│
├── internal/
│   ├── config/                        # koanf 配置（每个服务独立 Config struct）
│   ├── apperr/                        # AppError + 错误码
│   ├── response/                      # Success/Error/Paginated JSON + HandleError
│   ├── handler/                       # AppHandler 类型 + Wrap 包装器
│   ├── middleware/                    # 共享中间件
│   ├── auth/                          # JWT + Argon2 + SHA256
│   ├── id/                            # nanoid 生成
│   ├── httpclient/                    # 内部服务 HTTP 客户端（微服务模式用）
│   ├── database/
│   │   ├── postgres.go                # pgxpool 连接
│   │   ├── redis.go                   # go-redis 客户端
│   │   ├── migrations/*.sql           # goose 迁移文件
│   │   ├── queries/*.sql              # sqlc 查询文件（固定 CRUD）
│   │   ├── sqlc.yaml
│   │   └── gen/                       # sqlc 生成代码（勿手动修改）
│   ├── lua/                           # Redis Lua 脚本 (go:embed 嵌入)
│   │
│   ├── gateway/                       # API Gateway 服务
│   ├── user/                          # 用户服务
│   │   ├── handler/ / service/ / repository/ / dto/
│   ├── product/                       # 商品服务
│   │   ├── handler/ / service/ / repository/ / dto/
│   ├── cart/                          # 购物车服务
│   │   ├── handler/ / service/ / dto/
│   └── order/                         # 订单服务
│       ├── handler/ / service/ / repository/ / statemachine/ / dto/
│
├── infra/
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

---

## 15. 分阶段开发路线图

> **每个阶段 = 一个独立的 Claude Code CLI 会话**
> 阶段间通过文件传递上下文，不依赖对话历史

---

### Phase 0: 项目骨架 + 工具链

**目标：** `go build ./...` 成功，6 个 health 端点响应（含 monolith）

**Claude Code 提示词：**
```
请参考 CLAUDE.md 和 docs/architecture.md Phase 0。
搭建 Go 项目骨架：
1. go mod init, 创建全部目录结构
2. 每个服务最小 main.go（chi 路由 + /health POST 端点）
3. cmd/monolith/main.go — 单体模式入口（挂载所有服务路由到 :3000）
4. internal/handler/wrap.go — AppHandler 类型 + Wrap 包装器
5. Makefile (build/test/lint/run/dev/generate/migrate)
6. .gitignore, .env.example, .golangci.yml, CLAUDE.md
不写任何业务代码。
```

**产出物：**
- `go.mod` / `go.sum`
- `cmd/*/main.go` — 每个服务的入口（含 /health）+ monolith
- `internal/handler/wrap.go`
- `Makefile`
- `.gitignore`, `.env.example`, `.golangci.yml`

**验收标准：**
- [ ] `go build ./...` 无错误
- [ ] 6 个服务 /health 端点响应 200（含 monolith）
- [ ] `make build-all` 生成 7 个二进制（含 monolith）
- [ ] `make dev` 启动单体模式

**预估：** 1 个会话

---

### Phase 1: Docker 基础设施

**目标：** `docker compose up` 启动 PG + Redis + Caddy

**Claude Code 提示词：**
```
请参考 CLAUDE.md 和 docs/architecture.md Phase 1。
创建 Docker 基础设施：
1. docker-compose.yml (PG 16 + Redis 7 + Caddy 2 + 5 个 Go 服务)
2. infra/ 配置文件（init.sql 创建 3 个 PG schema）
3. 多阶段 Dockerfile（Go 编译 + Alpine 运行，Lua 用 go:embed 不需要 COPY）
4. infra/postgres/postgresql.conf 调优
5. infra/redis/redis.conf 调优
6. infra/caddy/Caddyfile 反向代理
```

**验收标准：**
- [ ] `docker compose up -d` 启动全部基础设施
- [ ] PG 3 个 schema 创建成功
- [ ] Redis ping 成功
- [ ] Caddy 反向代理到 gateway

**预估：** 1 个会话

---

### Phase 2: 共享工具包

**目标：** internal/ 共享包全部就绪

**Claude Code 提示词：**
```
请参考 CLAUDE.md 和 docs/architecture.md Phase 2。
实现 internal/ 共享包：
1. config/ — koanf 配置加载 + 每个服务独立 Config struct
2. apperr/ — AppError struct + 工厂函数 + 错误码常量
3. response/ — Success[T] / Error / Paginated[T] 构建器 + HandleError
4. handler/ — AppHandler 类型 + Wrap 包装器（Phase 0 骨架基础上完善）
5. auth/ — JWT 签发/验证 + Argon2id + SHA256
6. id/ — GenerateID (nanoid 21) + GenerateOrderNo
7. httpclient/ — 内部服务 HTTP 客户端（注入 traceId）
每个包写表驱动测试。
```

**验收标准：**
- [ ] 所有包编译通过
- [ ] 单元测试全绿
- [ ] apperr.NewNotFound("user", "xxx") 返回 404 + USER_1001
- [ ] JWT 签发/验证/黑名单流程正确
- [ ] Handler Wrap 正确捕获 AppError 和 unknown error

**预估：** 1-2 个会话

---

### Phase 3: 数据库层

**目标：** 迁移应用成功，sqlc 生成代码，Redis 连接就绪

**Claude Code 提示词：**
```
请参考 CLAUDE.md 和 docs/architecture.md Phase 3 + 数据库设计（4.3-4.6）+ 查询策略（4.3）。
实现数据库层：
1. database/postgres.go — pgxpool 连接池 + 健康检查
2. database/redis.go — go-redis 客户端
3. 6 个 goose 迁移文件（翻译自 TS 版 Drizzle schema）
4. sqlc 查询文件（固定 CRUD）+ squirrel 示例（动态查询）
5. lua/ — 4 个 Lua 脚本 + go:embed 封装
6. cmd/migrate/main.go — 迁移工具
7. database/sqlc.yaml 配置
```

**验收标准：**
- [ ] goose 迁移成功创建所有表和索引
- [ ] sqlc generate 生成类型安全 Go 代码
- [ ] Lua 脚本通过 go:embed 加载 + EVALSHA 可调用
- [ ] squirrel 动态查询编译通过

**预估：** 1-2 个会话

---

### Phase 4: 共享中间件

**目标：** 全部共享中间件就绪

**产出物：**
- `middleware/requestid.go` — nanoid traceId + context 注入
- `middleware/logger.go` — slog 结构化日志（method, path, status, duration）
- `middleware/cors.go` — 可配置白名单
- `middleware/auth.go` — JWT 验证 + Redis 黑名单 + context 注入 userId
- `middleware/ratelimit.go` — Redis ZSET 滑动窗口
- `middleware/idempotent.go` — X-Idempotency-Key 检查
- `middleware/internal_only.go` — 拦截外部 /internal/* 请求

注：不需要 recovery 中间件 — Handler Wrap 模式已处理所有错误。

**验收标准：**
- [ ] 表驱动测试全绿

**预估：** 1 个会话

---

### Phase 5: API Gateway

**目标：** 网关启动，正确代理请求

**产出物：**
- `internal/gateway/registry.go` — 路由前缀 → 服务 URL 映射
- `internal/gateway/proxy.go` — httputil.ReverseProxy 转发 + header 注入
- `internal/gateway/health.go` — /health/live, /health/ready, /health
- `internal/gateway/server.go` — chi 路由 + 中间件链组装
- `cmd/gateway/main.go` — 入口 + 优雅关停

**验收标准：**
- [ ] 网关正确代理请求到下游服务
- [ ] 中间件链生效（requestid → logger → cors → ratelimit → auth → idempotent）
- [ ] 公开路由白名单正确
- [ ] /internal/ 外部不可达

**预估：** 1 个会话

---

### Phase 6: 用户服务

**目标：** 完整的用户注册/登录/JWT/资料/地址管理

**产出物：**
- `internal/user/dto/` — RegisterInput, LoginInput, RefreshInput 等
- `internal/user/repository/` — user, address, token 仓储（接口 + sqlc 实现）
- `internal/user/service/` — auth, user, address 业务逻辑
- `internal/user/handler/` — HTTP handler（使用 Wrap 包装器）
- `cmd/user/main.go` — 组装依赖 + 路由挂载 + 优雅关停

**验收标准：**
- [ ] 注册 → 登录 → 获取 profile → 更新 → 登出 全流程
- [ ] JWT 双 Token 机制完整
- [ ] 地址 CRUD + 默认地址切换
- [ ] 内部端点 /internal/user/detail, /internal/user/batch
- [ ] 错误码正确

**预估：** 1-2 个会话

---

### Phase 7: 商品服务

**目标：** 商品 CRUD、分类、SKU、库存、搜索、缓存

**产出物：**
- Category handler/service/repository — 树形 CRUD
- Product handler/service/repository — 列表/详情/搜索/CRUD
  - 搜索使用 squirrel 动态构建
  - 详情使用 singleflight 防缓存击穿
- SKU 管理
- Stock Service — Reserve/Release/Confirm（Lua 脚本）
- Cache Service — Redis 缓存 + 防穿透/雪崩/击穿
- Banner handler/service
- Internal 端点：sku/batch, stock/reserve/release/confirm/sync

**验收标准：**
- [ ] 商品 CRUD 全流程
- [ ] 全文搜索正确（squirrel 动态条件）
- [ ] 库存并发扣减无超卖
- [ ] 缓存策略生效（singleflight + Redis）

**预估：** 2-3 个会话

---

### Phase 8: 购物车服务

**目标：** 购物车全功能

**产出物：**
- 全 Redis 实现（HASH cart:{userId}）
- Add/List/Update/Remove/Clear/Select handler（使用 Wrap 包装器）
- CheckoutPreview（服务端重算价格）
- Internal ClearItems

**验收标准：**
- [ ] 购物车 CRUD 全流程
- [ ] 结算预览正确计算金额
- [ ] 价格变动/库存不足正确检测

**预估：** 1 个会话

---

### Phase 9: 订单服务

**目标：** 订单完整生命周期 + 支付 + 超时

**产出物：**
- 状态机（statemachine 包）
- Order Service — 9 步创建编排（通过接口调用其他服务）
- Payment Service — 创建/回调/查询
- Timeout Service — goroutine + time.Ticker

**验收标准：**
- [ ] 订单全流程正确
- [ ] 幂等检查生效
- [ ] 支付回调幂等
- [ ] 超时自动取消 + 库存释放
- [ ] 并发下单无超卖
- [ ] 单体模式和微服务模式均可运行

**预估：** 2-3 个会话

---

### Phase 10: Admin 端点

**目标：** 管理后台全部 API

**产出物：**
- Admin 认证中间件（staff JWT）
- 商品/分类/库存管理
- 用户管理
- 订单管理（发货/取消/退款）
- Dashboard 统计

**验收标准：**
- [ ] Admin 认证流程
- [ ] 全部管理端点功能正确

**预估：** 1-2 个会话

---

### Phase 11: 测试 + 优化

**目标：** 全面测试 + 性能优化

**产出物：**
- 端到端集成测试（testcontainers 自动启停 PG/Redis）
- 并发压力测试（库存扣减竞争）
- Benchmark（`go test -bench`）
- pprof 集成
- SQL 查询优化
- Seed 脚本

**验收标准：**
- [ ] >80% 覆盖率
- [ ] 基准测试记录
- [ ] 无数据竞争（`-race` 通过）
- [ ] testcontainers 集成测试全绿

**预估：** 1-2 个会话

---

### Phase 12+: 未来演进

| 方向 | 说明 |
|------|------|
| gRPC | 服务间通信升级（HTTP → gRPC） |
| Notification Service | 邮件、短信、站内信 |
| File Service | 图片上传 + CDN（S3/R2） |
| RBAC | 角色权限管理 |
| 促销 & 优惠券 | 优惠计算引擎 |
| 监控 | Prometheus + Grafana |
| CI/CD | GitHub Actions |
| 消息队列 | Redis Streams → NATS/Kafka |
