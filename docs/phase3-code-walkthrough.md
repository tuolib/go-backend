# Phase 3 代码逐行详解 — 数据库层

> 本文档对 Phase 3 所有新增代码进行详细解释，帮助 Go 初学者理解每个文件的作用和实现细节。

---

## 目录

1. [PostgreSQL 连接池 — postgres.go](#1-postgresql-连接池)
2. [Redis 客户端 — redis.go](#2-redis-客户端)
3. [SQL 构建器 — squirrel.go](#3-sql-构建器)
4. [迁移文件嵌入 — embed.go](#4-迁移文件嵌入)
5. [迁移工具 — cmd/migrate/main.go](#5-迁移工具)
6. [sqlc 配置 — sqlc.yaml](#6-sqlc-配置)
7. [迁移文件详解（6 个 SQL 文件）](#7-迁移文件详解)
8. [查询文件详解（12 个 SQL 文件）](#8-查询文件详解)
9. [Lua 脚本详解（4 个脚本 + Go 封装）](#9-lua-脚本详解)

---

## 1. PostgreSQL 连接池

**文件：** `internal/database/postgres.go`

### 核心概念：什么是连接池？

想象一个餐厅：
- **没有连接池** = 每个客人来都要新建一张桌子，走了就拆掉。频繁搬桌子很慢。
- **有连接池** = 预先摆好 20 张桌子，客人来了就坐，走了桌子留着给下一个人用。

数据库连接的创建开销很大（TCP 握手 + TLS + 认证），连接池预先维护一组连接，请求时借出，用完归还。

### 代码解析

```go
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
```
- `ctx context.Context`：Go 的"上下文"，可以传递超时、取消信号。所有可能耗时的操作都应该接受 ctx。
- `databaseURL string`：连接字符串，如 `postgresql://user:pass@localhost:5432/dbname`
- 返回 `(*pgxpool.Pool, error)`：Go 惯用模式——成功返回值+nil，失败返回 nil+错误

```go
config, err := pgxpool.ParseConfig(databaseURL)
if err != nil {
    return nil, fmt.Errorf("parse database url: %w", err)
}
```
- `ParseConfig`：将连接字符串解析为结构化配置对象
- `fmt.Errorf("...: %w", err)`：**错误 wrapping**——用 `%w` 包装原始错误，保留错误链。调用方可以用 `errors.Is(err, target)` 判断底层错误类型
- `if err != nil`：Go 没有 try/catch，每个可能出错的操作都返回 error，调用方必须检查

```go
config.MaxConns = 20                       // 最大 20 个数据库连接
config.MinConns = 5                        // 最少保持 5 个空闲连接
config.MaxConnLifetime = 30 * time.Minute  // 每个连接最多活 30 分钟
config.MaxConnIdleTime = 5 * time.Minute   // 空闲超过 5 分钟就回收
config.HealthCheckPeriod = 30 * time.Second // 每 30 秒检查一次连接健康
```

参数调优逻辑：
- `MaxConns=20`：防止连接数打满数据库（PostgreSQL 默认最大 100 连接）
- `MinConns=5`：冷启动时不用等创建连接
- `MaxConnLifetime=30m`：定期轮换连接，防止连接老化（DNS 变更、内存泄漏等）
- `MaxConnIdleTime=5m`：低流量时释放多余连接，节省数据库资源

```go
pool, err := pgxpool.NewWithConfig(ctx, config)
```
- 创建连接池并立即尝试建立至少一个连接。如果数据库不可达，这里就会报错。

```go
if err := pool.Ping(ctx); err != nil {
    pool.Close()  // Ping 失败，关闭池释放资源
    return nil, fmt.Errorf("ping database: %w", err)
}
```
- `Ping`：发送一个简单查询验证数据库可达
- **快速失败（Fail Fast）**：启动时就发现数据库连不上，比运行时第一个请求失败好得多
- `pool.Close()`：Ping 失败时必须关闭池，否则会泄漏后台 goroutine

### HealthCheck 函数

```go
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()
    return pool.Ping(ctx)
}
```
- `context.WithTimeout`：创建一个 3 秒后自动取消的 context，防止健康检查卡死
- `defer cancel()`：**必须调用 cancel**，否则会泄漏 context 相关的 goroutine
- 这个函数暴露给 `/health` 端点，让 Docker / Kubernetes 检测服务是否健康

---

## 2. Redis 客户端

**文件：** `internal/database/redis.go`

### 核心概念：Redis 在本项目中的角色

Redis 是内存数据库，读写速度比 PostgreSQL 快 10-100 倍。本项目中用于：
- **库存预扣**：高并发下单时，先在 Redis 扣减（毫秒级），再异步写 PG
- **购物车**：Hash 结构存储，天然支持字段级读写
- **缓存**：商品详情、分类树等热数据缓存
- **分布式锁**：防止并发冲突
- **限流**：API 请求频率控制

### 代码解析

```go
func NewRedis(redisURL string) (*redis.Client, error) {
    opts, err := redis.ParseURL(redisURL)
```
- `ParseURL` 解析 Redis URL 格式：`redis://:password@host:6379/0`（0 是数据库编号）

```go
opts.PoolSize = 20          // 最大 20 个连接
opts.MinIdleConns = 5       // 最小 5 个空闲连接
opts.MaxRetries = 3         // 自动重试 3 次
opts.DialTimeout = 5 * time.Second
opts.ReadTimeout = 3 * time.Second
opts.WriteTimeout = 3 * time.Second
```
- `MaxRetries=3`：网络抖动时自动重试，避免一次失败就报错
- 超时设置：防止 Redis 不响应时请求一直挂着

```go
client := redis.NewClient(opts)
```
- go-redis 内部也维护连接池，和 pgxpool 类似，不需要手动管理连接生命周期

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := client.Ping(ctx).Err(); err != nil {
    client.Close()
    return nil, fmt.Errorf("ping redis: %w", err)
}
```
- `context.Background()`：创建一个空的根 context（因为 NewRedis 不接收 ctx 参数）
- `client.Ping(ctx).Err()`：go-redis 的链式调用风格——`Ping` 返回 `*StatusCmd`，`.Err()` 提取错误
- 同样是快速失败 + 失败时清理资源

---

## 3. SQL 构建器

**文件：** `internal/database/squirrel.go`

```go
var Psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
```

### 为什么需要 squirrel？

sqlc 只能处理**编译时已知**的固定 SQL。但商品搜索需要根据用户输入**动态组合** WHERE 条件：

```
用户搜索请求：
  关键词: "iPhone"      → WHERE to_tsvector(...) @@ plainto_tsquery('iPhone')
  分类: "手机"          → WHERE id IN (SELECT ... WHERE category_id = ?)
  价格: 3000-8000       → WHERE min_price >= 3000 AND max_price <= 8000
  排序: 按销量           → ORDER BY total_sales DESC
```

每个条件都是可选的，组合数爆炸（2^4 = 16 种），不可能为每种组合写一个 sqlc 查询。

### `sq.Dollar` 是什么？

- MySQL 用 `?` 做占位符：`WHERE id = ?`
- PostgreSQL 用 `$1, $2, $3`：`WHERE id = $1 AND name = $2`
- `sq.Dollar` 告诉 squirrel 生成 PostgreSQL 风格的占位符

### 使用示例（后续阶段会用到）

```go
qb := database.Psql.Select("id", "title", "min_price").
    From("product_service.products").
    Where("status = 'active'")

if keyword != "" {
    qb = qb.Where("to_tsvector('simple', ...) @@ plainto_tsquery('simple', ?)", keyword)
}
if priceMin > 0 {
    qb = qb.Where("min_price >= ?", priceMin)
}

sql, args, _ := qb.ToSql()
// sql = "SELECT id, title, min_price FROM product_service.products WHERE status = 'active' AND min_price >= $1"
// args = [3000]
```

squirrel 自动将 `?` 转换为 `$1, $2`，并且参数化查询防 SQL 注入。

---

## 4. 迁移文件嵌入

**文件：** `internal/database/migrations/embed.go`

```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

### go:embed 是什么？

Go 1.16 引入的编译时文件嵌入指令。`//go:embed *.sql` 的含义：
1. 编译时，Go 编译器找到当前目录下所有 `.sql` 文件
2. 将文件内容**嵌入到编译后的二进制**中
3. 运行时通过 `FS` 变量访问这些文件，就像读取真实文件系统一样

### 为什么这很重要？

**没有 go:embed 时：**
```
部署包/
  ├── migrate (二进制)
  ├── 001_create_schemas.sql     ← 必须一起部署
  ├── 002_user_service_tables.sql
  ├── ...
```

**有 go:embed 时：**
```
部署包/
  └── migrate (二进制，内含所有 SQL 文件)
```

只需要一个二进制文件，不需要额外携带 .sql 文件。部署更简单，不会出现"忘了拷贝 SQL 文件"的事故。

### `embed.FS` 类型

`embed.FS` 实现了标准库的 `fs.FS` 接口，这意味着：
- goose 可以像读取真实文件系统一样读取嵌入的 SQL 文件
- 不需要特殊的适配代码

---

## 5. 迁移工具

**文件：** `cmd/migrate/main.go`

### goose 是什么？

数据库迁移工具，管理数据库 schema 的版本。每个 `.sql` 文件是一个"迁移"：
- `-- +goose Up`：往前迁移（创建表、加列等）
- `-- +goose Down`：回滚迁移（删表、删列等）

goose 在数据库中维护一张 `goose_db_version` 表，记录当前已应用到第几个迁移。

### 代码解析

```go
direction := flag.String("direction", "up", "migration direction: up or down")
dbURL := flag.String("db", "", "database URL (or set DATABASE_URL env)")
flag.Parse()
```
- `flag` 是 Go 标准库的命令行参数解析器
- `flag.String` 返回 `*string`（指针），解析后用 `*direction` 读取值
- 使用：`go run ./cmd/migrate -db "postgresql://..." -direction up`

```go
connStr := *dbURL
if connStr == "" {
    connStr = os.Getenv("DATABASE_URL")
}
```
- 优先使用命令行参数，其次使用环境变量。这是工具类程序的常见模式。

```go
connConfig, err := pgx.ParseConfig(connStr)
dsn := stdlib.RegisterConnConfig(connConfig)
db, err := sql.Open("pgx", dsn)
```
- **为什么这么绕？** goose 需要标准 `*sql.DB` 接口（Go 标准库的数据库接口），但我们想用 pgx 驱动（性能更好）
- `pgx.ParseConfig`：pgx 解析连接字符串
- `stdlib.RegisterConnConfig`：把 pgx 配置注册到标准库驱动中，返回一个 DSN 字符串
- `sql.Open("pgx", dsn)`：通过标准库接口打开连接，底层实际使用 pgx 驱动

```go
goose.SetBaseFS(migrationsFS)
goose.SetDialect("postgres")
```
- `SetBaseFS`：告诉 goose 从嵌入的文件系统读取迁移文件
- `SetDialect`：告诉 goose 使用 PostgreSQL 语法

```go
switch *direction {
case "up":
    goose.UpContext(ctx, db, ".")   // "." 表示从 BaseFS 根目录读取
case "down":
    goose.DownContext(ctx, db, ".") // 每次只回滚一个迁移
}
```
- `UpContext`：应用所有未执行的迁移
- `DownContext`：回滚最近一次迁移（安全起见，每次只回滚一个）

---

## 6. sqlc 配置

**文件：** `internal/database/sqlc.yaml`

```yaml
version: "2"
sql:
  - engine: "postgresql"        # 数据库类型
    queries: "queries/"         # SQL 查询文件在哪
    schema: "migrations/"       # 表结构定义在哪（sqlc 从迁移文件推断）
    gen:
      go:
        package: "gen"          # 生成的 Go 包名
        out: "gen"              # 输出目录
        sql_package: "pgx/v5"   # 使用 pgx v5 驱动
        emit_json_tags: true    # 生成 JSON tag（API 响应用）
        emit_empty_slices: true # 空结果返回 [] 不返回 nil
```

### sqlc 工作流程

```
queries/*.sql (手写 SQL)  ─┐
                            ├─→  sqlc generate  ─→  gen/*.go (类型安全 Go 代码)
migrations/*.sql (表结构)  ─┘
```

sqlc 做了什么：
1. 读取迁移文件，推断出所有表的列名和类型
2. 读取查询文件，解析每个 SQL 的输入参数和返回列
3. 生成对应的 Go struct（代表表行）和函数（执行查询）

### `emit_empty_slices: true` 的作用

```go
// false (默认)：
// 查询结果为空时返回 nil
// JSON 序列化为: "data": null

// true：
// 查询结果为空时返回 []
// JSON 序列化为: "data": []
```

前端通常期望收到空数组 `[]` 而不是 `null`，避免 `Cannot read property 'length' of null` 错误。

---

## 7. 迁移文件详解

### 001_create_schemas.sql — Schema 隔离

```sql
CREATE SCHEMA IF NOT EXISTS user_service;
CREATE SCHEMA IF NOT EXISTS product_service;
CREATE SCHEMA IF NOT EXISTS order_service;
```

PostgreSQL schema 相当于"命名空间"。同一个数据库中，`user_service.users` 和 `product_service.products` 完全隔离。

**好处：**
- 单数据库，共享连接池
- 逻辑隔离，每个服务只操作自己的 schema
- 未来可以按 schema 拆分到独立数据库

### 002_user_service_tables.sql — 用户相关表

**users 表关键字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | VARCHAR(21) | nanoid 生成，不用自增 ID（更安全，分布式友好） |
| `email` | VARCHAR(255) UNIQUE | 登录唯一标识 |
| `password` | VARCHAR(255) | Argon2id 哈希后的密码，不是明文 |
| `status` | VARCHAR(20) DEFAULT 'active' | 用户状态：active/suspended/deleted |
| `deleted_at` | TIMESTAMPTZ | 软删除标记，NULL=未删除，非NULL=已删除 |

**为什么用 nanoid 而不是自增 ID？**
- 自增 ID 可预测（1, 2, 3...），攻击者可以遍历所有用户
- nanoid 是 21 字符随机字符串（如 `V1StGXR8_Z5jdHi6B-myT`），不可预测
- 分布式环境下不需要中心化 ID 生成器

**refresh_tokens 表：**

为什么存 `token_hash` 而不是原始 token？
- 如果数据库被黑客拿到，原始 token 可以直接冒充用户登录
- 存 SHA-256 哈希后，即使数据库泄露，黑客也无法还原出原始 token

### 003_product_service_tables.sql — 商品相关表

**categories 自引用（多级分类）：**

```sql
parent_id VARCHAR(21) REFERENCES product_service.categories(id)
```

```
电子产品 (parent_id = NULL)
  ├── 手机 (parent_id = "电子产品.id")
  │   ├── 苹果 (parent_id = "手机.id")
  │   └── 安卓 (parent_id = "手机.id")
  └── 电脑 (parent_id = "电子产品.id")
```

**product_categories 多对多关联：**

```sql
PRIMARY KEY (product_id, category_id)  -- 联合主键
```

一个商品可以属于多个分类（如 iPhone 属于"手机"和"苹果"分类），通过中间表实现多对多关系。

**skus 表——什么是 SKU？**

SKU = Stock Keeping Unit（库存保有单位），代表一个可购买的具体规格组合：

```
商品: iPhone 15
  ├── SKU-1: 红色 + 128GB  → 价格 5999, 库存 100
  ├── SKU-2: 红色 + 256GB  → 价格 6999, 库存 50
  ├── SKU-3: 蓝色 + 128GB  → 价格 5999, 库存 80
  └── SKU-4: 蓝色 + 256GB  → 价格 6999, 库存 30
```

```sql
attributes JSONB  -- {"color":"红","size":"128GB"}
version INTEGER NOT NULL DEFAULT 0  -- 乐观锁版本号
```

`version` 字段用于乐观锁：每次更新库存时 `WHERE version = $old_version`，如果版本号不匹配说明有并发修改。

### 004_order_service_tables.sql — 订单相关表

**order_items 快照设计：**

```sql
product_title VARCHAR(200) NOT NULL,  -- 商品标题快照
sku_attrs     JSONB NOT NULL,         -- SKU 规格快照
unit_price    DECIMAL(12,2) NOT NULL, -- 下单时单价
```

**为什么要快照？** 假设用户下单时 iPhone 卖 5999，第二天涨价到 6999。如果订单引用商品表的实时数据，用户的订单金额会莫名变化。快照保证订单记录反映下单那一刻的真实情况。

**order_addresses 为什么不 FK 到 user_addresses？**

同样的快照逻辑：用户下单后可能修改/删除收货地址，但订单的收货信息必须固定。

**stock_operations 审计表：**

```sql
type VARCHAR(20) NOT NULL  -- reserve / confirm / release / adjust
```

记录每次库存变动，用于：
- **对账**：Redis 库存与 PG 库存不一致时，可以追溯每笔操作
- **审计**：谁在什么时候扣了多少库存

### 005_create_indexes.sql — 索引策略

**普通索引（加速查找）：**
```sql
CREATE INDEX idx_user_addresses_user ON user_service.user_addresses(user_id);
```
没有索引时，查找某用户的地址需要扫描全表（O(n)）。有索引后是 B-Tree 查找（O(log n)）。

**部分索引（Partial Index）——只索引有用的行：**
```sql
CREATE INDEX idx_users_status ON user_service.users(status) WHERE deleted_at IS NULL;
```
只索引未删除的用户。如果 100 万用户中有 90 万已删除，这个索引只包含 10 万行，比全量索引小 10 倍。

**GIN 索引（倒排索引）——加速 JSONB 和全文搜索：**
```sql
CREATE INDEX idx_products_attrs ON product_service.products USING GIN(attributes);
```
GIN (Generalized Inverted Index) 将 JSONB 中的每个 key-value 对建立倒排索引，支持 `@>`, `?`, `?|` 等 JSONB 查询操作符。

**复合索引（多列联合）：**
```sql
CREATE INDEX idx_orders_user_status ON order_service.orders(user_id, status);
```
适用于 `WHERE user_id = ? AND status = ?` 查询。列顺序很重要——最左前缀原则。

### 006_fulltext_search.sql — 全文搜索

```sql
CREATE INDEX idx_products_fulltext ON product_service.products
    USING GIN(to_tsvector('simple', title || ' ' || coalesce(description, '') || ' ' || coalesce(brand, '')));
```

**`to_tsvector('simple', ...)`：**
- `tsvector` = Text Search Vector，将文本拆分为词条
- `'simple'` 配置不做词干提取（适合中英文混合）
- `'english'` 配置会做词干提取（running → run），但不适合中文

**`coalesce(description, '')`：**
- `NULL || 'text'` 在 PostgreSQL 中结果是 `NULL`
- 如果 description 为 NULL，整个表达式变成 NULL，索引就废了
- `coalesce` 将 NULL 转为空字符串，避免这个问题

**查询时配套使用：**
```sql
WHERE to_tsvector('simple', title || ' ' || coalesce(description, '') || ' ' || coalesce(brand, ''))
  @@ plainto_tsquery('simple', 'iPhone')
```
- `@@` 是全文搜索匹配操作符
- `plainto_tsquery` 将搜索关键词转为查询条件

---

## 8. 查询文件详解

### 查询命名规范

```sql
-- name: GetUserByEmail :one
```
- `-- name:`：sqlc 识别的魔法注释
- `GetUserByEmail`：生成的 Go 函数名
- `:one`：返回单行（对应 Go 的单个 struct）
- `:many`：返回多行（对应 Go 的 slice）
- `:exec`：不返回数据（对应 Go 的 error）

### users.sql — 用户 CRUD（6 个查询）

| 查询名 | 类型 | 用途 |
|--------|------|------|
| GetUserByID | :one | 按 ID 查用户（排除已软删除） |
| GetUserByEmail | :one | 登录时按邮箱查用户 |
| CreateUser | :one | 注册新用户，返回完整记录 |
| UpdateUser | :one | 更新资料（COALESCE 部分更新） |
| UpdateLastLogin | :exec | 记录最后登录时间 |
| SoftDeleteUser | :exec | 软删除（设 deleted_at） |

**COALESCE 部分更新模式：**
```sql
SET nickname = COALESCE($2, nickname)
```
如果用户只想改头像不改昵称，传入 `nickname = nil`，COALESCE 会保留原来的昵称值。这样一个 SQL 就能处理"改任意字段"的场景。

### addresses.sql — 地址管理（8 个查询）

**设置默认地址的两步操作：**
```sql
-- Step 1: 清除旧的默认地址
-- name: ClearDefaultAddress :exec
UPDATE user_service.user_addresses SET is_default = false WHERE user_id = $1 AND is_default = true;

-- Step 2: 设置新的默认地址
-- name: SetDefaultAddress :exec
UPDATE user_service.user_addresses SET is_default = true WHERE id = $1 AND user_id = $2;
```

为什么分两步？确保同一用户只有一个默认地址。业务层在事务中调用这两个操作。

**地址数量限制：**
```sql
-- name: CountAddressesByUser :one
SELECT COUNT(*) FROM user_service.user_addresses WHERE user_id = $1;
```
业务层检查数量是否超过限制（如最多 10 个地址），超过就返回 `ErrCodeAddressLimit` 错误。

### tokens.sql — Token 管理（5 个查询）

| 查询名 | 用途 |
|--------|------|
| CreateRefreshToken | 登录时存储 token 哈希 |
| GetRefreshTokenByHash | 刷新 token 时验证 |
| RevokeRefreshToken | 登出时吊销单个 token |
| RevokeAllUserTokens | 强制登出所有设备 |
| DeleteExpiredTokens | 定时清理过期记录 |

### categories.sql — 分类管理（7 个查询）

**活跃分类查询：**
```sql
-- name: ListActiveCategories :many
SELECT * FROM product_service.categories WHERE is_active = true ORDER BY sort_order ASC, name ASC;
```
前台展示用，只返回启用的分类。后台管理用 `ListCategories` 返回全部。

### products.sql — 商品管理（10 个查询）

**价格范围更新（从 SKU 聚合）：**
```sql
-- name: UpdateProductPriceRange :exec
UPDATE product_service.products SET min_price = $2, max_price = $3, updated_at = NOW() WHERE id = $1;
```
当 SKU 的价格变化时，需要重新计算商品的最低/最高价格。这是**冗余字段维护**——用额外的写入换取列表页查询的性能（不需要 JOIN skus 表）。

**商品-分类关联（幂等写入）：**
```sql
-- name: AddProductCategory :exec
INSERT INTO product_service.product_categories (product_id, category_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;
```
`ON CONFLICT DO NOTHING`：如果关联已存在就跳过，不报错。这是幂等操作——多次调用结果相同。

### skus.sql — SKU 管理（7 个查询）

**批量查询（数组 IN 语法）：**
```sql
-- name: BatchGetSKUs :many
SELECT * FROM product_service.skus WHERE id = ANY($1::varchar[]);
```
- `$1::varchar[]`：将参数强制转为 varchar 数组类型
- `ANY(...)`：等价于 `IN (...)`，但支持参数化（sqlc 可以正确处理）
- 下单时用：一次查询多个 SKU 的价格和库存信息

**乐观锁库存确认：**
```sql
-- name: ConfirmStockDeduction :exec
UPDATE product_service.skus
SET stock = stock - $2, version = version + 1, updated_at = NOW()
WHERE id = $1 AND version = $3 AND stock >= $2;
```
三重保护：
1. `version = $3`：乐观锁——版本号不匹配则不更新
2. `stock >= $2`：二次校验库存充足
3. `version = version + 1`：更新版本号，下次并发操作会被拦截

### orders.sql — 订单管理（8 个查询）

**幂等创建订单：**
```sql
-- name: GetOrderByIdempotencyKey :one
SELECT * FROM order_service.orders WHERE idempotency_key = $1;
```
创建订单前先查幂等键，如果已存在则直接返回已有订单，防止网络重试导致重复下单。

**超时订单扫描：**
```sql
-- name: ListExpiredPendingOrders :many
SELECT * FROM order_service.orders
WHERE status = 'pending' AND expires_at < NOW()
ORDER BY expires_at ASC LIMIT 100;
```
定时任务每分钟调用，找出超时未支付的订单，自动取消并释放库存。`LIMIT 100` 防止一次取太多导致处理超时。

### payments.sql — 支付记录（5 个查询）

**支付状态更新的并发安全：**
```sql
-- name: UpdatePaymentSuccess :exec
UPDATE order_service.payment_records
SET status = 'success', transaction_id = $2, raw_notify = $3, updated_at = NOW()
WHERE id = $1 AND status = 'pending';
```
`AND status = 'pending'`：只有 pending 状态才能转为 success。如果支付回调重复到达，第二次 UPDATE 影响 0 行，不会重复处理。

### stock_operations.sql — 库存审计（3 个查询）

记录每次库存操作，类型包括：
- `reserve`：下单时预扣
- `confirm`：支付成功后确认
- `release`：取消订单时释放
- `adjust`：人工调整库存

---

## 9. Lua 脚本详解

### 为什么用 Lua 脚本？

Redis 是单线程的，但多个客户端同时操作同一个 key 时仍然可能出问题：

```
客户端 A: GET stock:sku1 → 100
客户端 B: GET stock:sku1 → 100
客户端 A: SET stock:sku1 99  (扣 1)
客户端 B: SET stock:sku1 99  (也扣 1，但实际应该是 98！)
```

Lua 脚本在 Redis 服务端**原子执行**——脚本执行期间不会被其他命令打断。

### stock-deduct.lua — 单 SKU 扣减

```lua
local stock = tonumber(redis.call('GET', KEYS[1]))  -- 读取当前库存
if stock == nil then return -1 end                    -- key 不存在
if stock < tonumber(ARGV[1]) then return 0 end        -- 库存不足
redis.call('DECRBY', KEYS[1], ARGV[1])               -- 原子扣减
return 1                                               -- 成功
```

**调用方式：**
```
EVALSHA <sha> 1 stock:sku123 5
// KEYS[1] = "stock:sku123"
// ARGV[1] = "5"（要扣 5 个）
```

### stock-deduct-multi.lua — 多 SKU 原子扣减

这是最复杂的脚本，用于下单时一次性扣减所有商品的库存。

**两阶段设计：**

```
Phase 1 (检查)：              Phase 2 (扣减)：
  SKU-A 有 100, 要扣 2 ✓       DECRBY stock:skuA 2
  SKU-B 有 50, 要扣 1  ✓       DECRBY stock:skuB 1
  SKU-C 有 30, 要扣 3  ✓       DECRBY stock:skuC 3

如果 Phase 1 任何一个检查失败，直接返回错误，不执行 Phase 2。
```

**为什么不边检查边扣？**
如果 SKU-A 扣成功了但 SKU-B 不足，就得回滚 SKU-A。在 Lua 中回滚逻辑复杂且容易出错。两阶段设计更清晰：先确认都够，再一起扣。

**返回值设计：**
- `0` = 全部成功
- `正数 i` = 第 i 个 SKU 库存不足（1-based）
- `负数 -i` = 第 i 个 SKU key 不存在

业务层可以根据返回值精确定位是哪个 SKU 有问题。

### stock-release.lua — 单 SKU 释放

```lua
local stock = tonumber(redis.call('GET', KEYS[1]))
if stock == nil then return -1 end    -- key 不存在
redis.call('INCRBY', KEYS[1], ARGV[1])  -- 增加库存
return 1
```

使用场景：用户取消订单、订单超时自动取消时，归还预扣的库存。

### stock-release-multi.lua — 多 SKU 原子释放

与 deduct-multi 类似的两阶段设计，但更简单（释放不需要检查"库存是否足够"）。

### scripts.go — Go 封装层

```go
//go:embed stock-deduct.lua
var stockDeductScript string
```
将 Lua 脚本内容在编译时嵌入到 Go 字符串变量中。

**EVALSHA vs EVAL：**
```
EVAL:     每次发送完整 Lua 脚本内容 → 网络带宽浪费
EVALSHA:  只发送脚本的 SHA1 哈希值 → 节省带宽
```

`LoadStockScripts` 在服务启动时调用 `SCRIPT LOAD` 将脚本加载到 Redis，获取 SHA1：
```go
scripts.deductSHA, err = rdb.ScriptLoad(ctx, stockDeductScript).Result()
```

之后每次调用只传 SHA：
```go
rdb.EvalSha(ctx, s.deductSHA, []string{"stock:" + skuID}, quantity)
```

**StockItem struct：**
```go
type StockItem struct {
    SkuID    string
    Quantity int
}
```
用于 `DeductMulti` 和 `ReleaseMulti`，表示"扣减/释放哪个 SKU 多少数量"。

---

## 10. 新引入的 Go 知识点总结

### 10.1 go:embed（编译时嵌入）

```go
//go:embed *.sql           // 嵌入多个文件到 embed.FS
var FS embed.FS

//go:embed stock-deduct.lua  // 嵌入单个文件到 string
var script string
```
- `embed.FS`：嵌入多个文件，通过文件系统接口访问
- `string`：嵌入单个文件的内容为字符串

### 10.2 context.WithTimeout

```go
ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
defer cancel()  // 必须调用！
```
创建一个会在指定时间后自动取消的 context。传递给数据库操作后，如果操作超时，会自动中断。

### 10.3 错误 wrapping

```go
return nil, fmt.Errorf("create connection pool: %w", err)
```
- `%w`：wrap 错误，保留原始错误链
- 调用方可以用 `errors.Is(err, pgx.ErrNoRows)` 判断底层错误

### 10.4 defer 资源清理

```go
pool, err := pgxpool.NewWithConfig(ctx, config)
// ...
if err := pool.Ping(ctx); err != nil {
    pool.Close()  // 连接池创建成功但 ping 失败，必须手动关闭
    return nil, err
}
```

正常流程中，调用方负责调用 `pool.Close()`（通常用 `defer pool.Close()`）。但在创建过程中出错时，需要在函数内部清理。

### 10.5 指针参数（flag 包）

```go
direction := flag.String("direction", "up", "...")  // 返回 *string
flag.Parse()
fmt.Println(*direction)  // 用 * 解引用读取值
```

`flag.String` 返回指针是因为 `Parse()` 之后才会写入值，返回指针让 `Parse` 可以修改变量。

### 10.6 `_ "embed"` blank import

```go
import _ "embed"
```
空导入（blank import）：不直接使用包的导出符号，但需要包的 `init()` 函数执行（注册 `//go:embed` 指令的处理器）。

---

## 11. 与 TypeScript 版本的对照

| 方面 | TypeScript 版 | Go 版 |
|------|--------------|-------|
| 迁移工具 | Drizzle 自带迁移 | goose（独立工具） |
| 查询生成 | Drizzle 运行时构建 | sqlc 编译时生成 + squirrel 运行时构建 |
| 数据库驱动 | pg / postgres.js | pgx v5 + pgxpool |
| 连接池 | 驱动内置 | pgxpool（显式配置） |
| SQL 嵌入 | 字符串模板 | go:embed 编译时嵌入 |
| Redis 客户端 | ioredis | go-redis v9 |
| Lua 脚本 | fs.readFileSync + eval | go:embed + EVALSHA |
| 类型安全 | TypeScript 类型 | sqlc 生成的 Go struct |
