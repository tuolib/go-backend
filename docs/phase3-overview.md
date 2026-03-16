# Phase 3 架构纲领 — 数据库层

> **目标：** 迁移应用成功，sqlc 生成代码，Redis 连接就绪
> **状态：** 已完成

---

## 1. 本阶段在整体架构中的位置

```
Phase 0: 项目脚手架（go.mod, 目录结构, Makefile, Docker Compose）     ✅
Phase 1: 通用基础设施（apperr, response, handler/wrap, config, auth, id） ✅
Phase 2: 服务入口（cmd/monolith, gateway, user, product, cart, order）    ✅
Phase 3: 数据库层（PG 连接池, Redis 客户端, 迁移, sqlc, Lua 脚本）     ✅ ← 当前
Phase 4: 共享中间件（requestid, logger, cors, auth, ratelimit 等）       ⬜
Phase 5+: 各服务业务逻辑...                                             ⬜
```

Phase 3 是**数据存储的基石**。后续所有业务逻辑（用户注册、商品管理、下单支付）都依赖此阶段建立的：
- PostgreSQL 连接池 + 表结构
- Redis 客户端 + Lua 脚本
- sqlc 类型安全查询

---

## 2. 整体设计决策

### 2.1 为什么需要三种数据库交互方式？

| 交互方式 | 用途 | 选型理由 |
|----------|------|----------|
| **sqlc** | 固定 CRUD 查询 | SQL 编译时生成类型安全 Go 代码，编译器帮你检查 SQL 错误 |
| **squirrel** | 动态搜索/筛选 | 运行时构建 SQL，处理可选 WHERE 条件（sqlc 做不到） |
| **Redis Lua** | 库存原子操作 | Lua 在 Redis 内原子执行，无竞态条件 |

```
用户请求
  ├─ 固定查询（GetUserByEmail, CreateOrder 等）→ sqlc 生成的 Go 函数
  ├─ 动态搜索（按关键词/分类/价格筛选商品）→ squirrel 构建 SQL
  └─ 库存扣减/释放 → Redis Lua 脚本原子执行
```

### 2.2 Schema 隔离策略

```
PostgreSQL (单实例)
  ├── user_service schema    → users, user_addresses, refresh_tokens
  ├── product_service schema → categories, products, product_categories,
  │                            product_images, skus, banners
  └── order_service schema   → orders, order_items, order_addresses,
                               payment_records, stock_operations
```

**为什么用 schema 而不是分库？**
- 单库多 schema：共享连接池，JOIN 方便，运维简单
- 逻辑隔离：每个服务只访问自己的 schema，职责清晰
- 未来可拆分：如果需要分库，只需改连接配置

### 2.3 Redis 用途一览

```
Redis
  ├── 库存预扣/释放 → Lua 脚本原子操作（stock:{skuId}）
  ├── 购物车 → Hash 结构（cart:{userId}）
  ├── JWT 黑名单 → SET 结构（user:session:blacklist:{jti}）
  ├── 缓存 → 商品详情、分类树、搜索结果
  ├── 分布式锁 → SET NX EX
  ├── 限流 → ZSET 滑动窗口
  └── 幂等去重 → order:idempotent:{key}
```

---

## 3. 产出物清单

### 3.1 文件列表

```
internal/database/
  ├── postgres.go                     ← pgxpool 连接池
  ├── redis.go                        ← go-redis 客户端
  ├── squirrel.go                     ← PostgreSQL 风格 SQL 构建器
  ├── sqlc.yaml                       ← sqlc 配置
  ├── migrations/
  │   ├── embed.go                    ← go:embed 嵌入所有 .sql 到二进制
  │   ├── 001_create_schemas.sql      ← 3 个 schema
  │   ├── 002_user_service_tables.sql ← users, user_addresses, refresh_tokens
  │   ├── 003_product_service_tables.sql ← categories, products, skus 等 6 表
  │   ├── 004_order_service_tables.sql   ← orders, order_items 等 5 表
  │   ├── 005_create_indexes.sql      ← 25+ 索引
  │   └── 006_fulltext_search.sql     ← GIN 全文搜索索引
  ├── queries/
  │   ├── users.sql                   ← 6 个查询
  │   ├── addresses.sql               ← 8 个查询
  │   ├── tokens.sql                  ← 5 个查询
  │   ├── categories.sql              ← 7 个查询
  │   ├── products.sql                ← 10 个查询
  │   ├── product_images.sql          ← 4 个查询
  │   ├── skus.sql                    ← 7 个查询
  │   ├── banners.sql                 ← 5 个查询
  │   ├── orders.sql                  ← 8 个查询
  │   ├── order_items.sql             ← 4 个查询
  │   ├── payments.sql                ← 5 个查询
  │   └── stock_operations.sql        ← 3 个查询
  └── gen/                            ← sqlc 生成的代码（待运行 sqlc generate）

internal/lua/
  ├── scripts.go                      ← go:embed + EVALSHA 封装
  ├── stock-deduct.lua                ← 单 SKU 扣减
  ├── stock-deduct-multi.lua          ← 多 SKU 原子扣减
  ├── stock-release.lua               ← 单 SKU 释放
  └── stock-release-multi.lua         ← 多 SKU 原子释放

cmd/migrate/main.go                   ← goose 迁移工具
```

### 3.2 数量统计

| 类别 | 数量 |
|------|------|
| Go 源文件 | 5 个新增 |
| SQL 迁移文件 | 6 个 |
| SQL 查询文件 | 12 个（共 72 个查询） |
| Lua 脚本 | 4 个 |
| 数据库表 | 14 个 |
| 索引 | 25+ 个 |

---

## 4. 数据模型概览

### 4.1 User Service（3 表）

```
users ──1:N──→ user_addresses    （一个用户多个收货地址）
users ──1:N──→ refresh_tokens    （一个用户多个登录设备）
```

**关键设计：**
- `users.deleted_at`：软删除，保留历史数据
- `refresh_tokens.token_hash`：存 SHA-256 哈希，不存原始 token（防数据库泄露）
- 密码用 Argon2id 哈希存储

### 4.2 Product Service（6 表）

```
categories ──self──→ categories     （多级分类树，parent_id 自引用）
products ──M:N──→ categories        （通过 product_categories 关联）
products ──1:N──→ product_images    （多图，is_primary 标记主图）
products ──1:N──→ skus              （多规格变体，如颜色/尺码）
banners                             （首页轮播图，独立表）
```

**关键设计：**
- `products.min_price/max_price`：冗余字段，从 SKU 聚合，列表页展示用（避免 JOIN）
- `skus.version`：乐观锁，库存确认时防并发
- `skus.stock`：DB 真实库存，Redis 为预扣缓存

### 4.3 Order Service（5 表）

```
orders ──1:N──→ order_items         （订单明细，快照商品信息）
orders ──1:1──→ order_addresses     （快照收货地址）
orders ──1:N──→ payment_records     （支付尝试记录）
stock_operations                     （库存操作审计日志）
```

**关键设计：**
- 订单明细/地址都是**快照**：下单后商品改价、用户改地址不影响历史订单
- `orders.idempotency_key`：防重复提交
- `orders.expires_at`：超时自动取消
- `orders.version`：乐观锁，状态流转防并发

---

## 5. 索引策略

### 5.1 索引类型分布

| 索引类型 | 数量 | 用途 |
|----------|------|------|
| 普通 B-Tree | 15+ | 精确查找（user_id, order_id 等） |
| 部分索引（WHERE） | 6 | 只索引有意义的行（如未删除的用户、pending 状态的订单） |
| GIN (JSONB) | 1 | 商品属性 JSON 查询 |
| GIN (全文搜索) | 1 | 商品标题+描述+品牌全文搜索 |
| 复合索引 | 2 | 多列联合查询（user_id + status） |
| 降序索引 | 1 | 热门商品按销量排序 |

### 5.2 部分索引（Partial Index）

```sql
-- 只索引未删除的用户（大量已删除用户不占索引空间）
CREATE INDEX idx_users_status ON user_service.users(status) WHERE deleted_at IS NULL;

-- 只索引 pending 状态的订单（已完成订单不需要超时扫描）
CREATE INDEX idx_orders_expires ON order_service.orders(expires_at) WHERE status = 'pending';
```

**为什么用部分索引？** 索引越小越快。如果 90% 的用户是活跃的，那全量索引有 90% 的空间浪费在已删除用户上。

---

## 6. 库存并发控制（核心难点）

### 6.1 三层防护

```
                              ┌──────────────────────┐
  下单请求 ─────────────────→ │  Redis Lua 预扣      │ ← 第 1 层：原子检查+扣减
                              │  （毫秒级，高吞吐）   │
                              └──────────┬───────────┘
                                         │ 成功
                              ┌──────────▼───────────┐
  支付回调 ─────────────────→ │  PG 乐观锁确认       │ ← 第 2 层：WHERE version = $x
                              │  （数据库真实扣减）    │
                              └──────────┬───────────┘
                                         │ 所有操作
                              ┌──────────▼───────────┐
                              │  stock_operations 表  │ ← 第 3 层：审计日志
                              │  （操作记录，可追溯）  │
                              └──────────────────────┘
```

### 6.2 Lua 脚本设计

**单 SKU 扣减（stock-deduct.lua）：**
```
GET stock:{skuId}
  → nil?  返回 -1（key 不存在）
  → < qty? 返回 0（库存不足）
  → DECRBY 返回 1（成功）
```

**多 SKU 原子扣减（stock-deduct-multi.lua）：**
```
Phase 1: 检查所有 SKU 库存是否充足
  → 任何一个不足? 直接返回错误索引
Phase 2: 全部检查通过后，逐个 DECRBY
  → 返回 0（全部成功）
```

**为什么用两阶段？** 避免扣了 SKU-A 后发现 SKU-B 不足，还得回滚 SKU-A。先检查全部，确认都够了再一起扣。

### 6.3 乐观锁（PG 端）

```sql
UPDATE product_service.skus
SET stock = stock - $2,
    version = version + 1,
    updated_at = NOW()
WHERE id = $1 AND version = $3 AND stock >= $2;
```

如果 `version` 不匹配（有并发修改），`UPDATE` 影响 0 行，业务层可重试或报错。

---

## 7. 查询设计哲学

### 7.1 sqlc 查询规范

```sql
-- name: GetUserByEmail :one     ← 查询名 + 返回模式（:one/:many/:exec）
SELECT * FROM user_service.users
WHERE email = $1 AND deleted_at IS NULL;
```

sqlc 根据此 SQL 生成类型安全的 Go 函数：
```go
func (q *Queries) GetUserByEmail(ctx context.Context, email string) (UserServiceUser, error)
```

### 7.2 返回模式

| 标记 | 含义 | 生成的 Go 返回类型 |
|------|------|-------------------|
| `:one` | 返回单行 | `(StructType, error)` |
| `:many` | 返回多行 | `([]StructType, error)` |
| `:exec` | 不返回数据 | `error` |

### 7.3 常用 SQL 模式

**COALESCE 部分更新：**
```sql
SET nickname = COALESCE($2, nickname)  -- $2 非 NULL 用 $2，否则保留原值
```

**数组 IN 查询：**
```sql
WHERE id = ANY($1::varchar[])  -- PG 数组语法，替代 IN ($1, $2, ...)
```

**ON CONFLICT 幂等写入：**
```sql
INSERT INTO ... ON CONFLICT DO NOTHING  -- 已存在则跳过
```

---

## 8. 工具链使用

### 8.1 goose 迁移

```bash
# 执行所有迁移
go run ./cmd/migrate -db "postgresql://user:pass@localhost:5432/dbname" -direction up

# 回滚最近一次迁移
go run ./cmd/migrate -db "postgresql://user:pass@localhost:5432/dbname" -direction down
```

迁移文件通过 `go:embed` 嵌入到二进制中，部署时不需要额外携带 `.sql` 文件。

### 8.2 sqlc 代码生成

```bash
cd internal/database
sqlc generate
```

从 `queries/*.sql` + `migrations/*.sql`（表结构推断）自动生成 `gen/` 目录下的类型安全 Go 代码。

### 8.3 squirrel 动态查询

```go
qb := database.Psql.Select("id", "title", "min_price").
    From("product_service.products").
    Where("status = 'active'")

if keyword != "" {
    qb = qb.Where("to_tsvector(...) @@ plainto_tsquery(...)", keyword)
}

sql, args, _ := qb.ToSql()  // 自动使用 $1, $2 占位符
```

---

## 9. 与其他阶段的衔接

### 9.1 Phase 3 为后续阶段提供的能力

| 后续阶段 | 使用 Phase 3 的 |
|----------|----------------|
| Phase 4 中间件 | Redis 客户端（限流、幂等） |
| Phase 5 用户服务 | users/addresses/tokens 查询 + PG 连接池 |
| Phase 6 商品服务 | products/skus/categories 查询 + squirrel 搜索 + Lua 脚本 |
| Phase 7 购物车服务 | Redis 客户端（Hash 结构） |
| Phase 8 订单服务 | orders 查询 + Lua 库存扣减 + PG 乐观锁 |

### 9.2 待完成项

- [ ] 运行 `sqlc generate` 生成 `internal/database/gen/` 下的类型安全代码
- [ ] 集成测试（需要 testcontainers 启停真实 PG/Redis）

---

## 10. 关键 Go 知识点（本阶段涉及）

| 知识点 | 出现位置 | 说明 |
|--------|----------|------|
| `go:embed` | migrations/embed.go, lua/scripts.go | 编译时将文件内容嵌入二进制 |
| 连接池 | postgres.go, redis.go | 预维护连接，避免每次请求创建连接的开销 |
| `context.Context` | 所有数据库操作 | 传递超时、取消信号、请求级数据 |
| `fmt.Errorf("...: %w", err)` | postgres.go, redis.go | 错误 wrapping，保留原始错误链 |
| 构造函数模式 | NewPool(), NewRedis() | 不用全局变量，依赖注入 |
| `defer` | Ping 失败时 Close | 确保资源释放 |
