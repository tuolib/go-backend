# Phase 6 架构纲领 — 用户服务

> **目标：** 完整的用户注册/登录/JWT/资料/地址管理
> **状态：** 已完成

---

## 1. 本阶段在整体架构中的位置

```
Phase 0: 项目脚手架（go.mod, 目录结构, Makefile, Docker Compose）     ✅
Phase 1: 通用基础设施（apperr, response, handler/wrap, config, auth, id） ✅
Phase 2: 服务入口（cmd/monolith, gateway, user, product, cart, order）    ✅
Phase 3: 数据库层（PG 连接池, Redis 客户端, 迁移, sqlc, Lua 脚本）     ✅
Phase 4: 共享中间件（requestid, logger, cors, auth, ratelimit 等）     ✅
Phase 5: API Gateway（registry, proxy, health, server 组装）           ✅
Phase 6: 用户服务（注册/登录/JWT/资料/地址）                           ✅ ← 当前
Phase 7+: 商品/购物车/订单服务...                                       ⬜
```

Phase 6 是第一个**业务服务**。之前的阶段都在搭基础设施，这里开始处理真实的用户请求。它同时也是三层架构（Handler → Service → Repository）的范本——后续服务都会复用这个模式。

---

## 2. 三层架构

```
┌─────────────────────────────────────────────────────┐
│                   HTTP 请求                          │
└──────────────────────┬──────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────┐
│  Handler 层                                          │
│  职责：解析 JSON body → 校验字段 → 调用 Service → 返回 JSON │
│  依赖：dto, service, validator, middleware            │
│  文件：handler/auth.go, handler/user.go, handler/address.go │
└──────────────────────┬──────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────┐
│  Service 层                                          │
│  职责：业务逻辑编排（密码哈希、JWT 签发、校验规则）        │
│  依赖：repository 接口, auth, id                      │
│  文件：service/auth.go, service/user.go, service/address.go │
└──────────────────────┬──────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────┐
│  Repository 层                                       │
│  职责：数据库操作（sqlc 查询封装 + 错误转换）            │
│  依赖：gen.Queries（sqlc 生成）, apperr                │
│  文件：repository/user.go, repository/address.go, repository/token.go │
└──────────────────────┬──────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────┐
│  PostgreSQL / Redis                                  │
└─────────────────────────────────────────────────────┘
```

### 为什么分三层？

| 关注点 | 不分层 | 三层架构 |
|--------|--------|---------|
| Handler 里写 SQL | 改数据库 → 改 handler | 改数据库 → 只改 repository |
| 业务逻辑散布 | 重复代码 | service 集中管理 |
| 测试 | 需要启动 HTTP + DB | 可以 mock repository 单测 service |
| 换框架 | 全部重写 | 只换 handler 层 |

---

## 3. 认证流程

### 3.1 注册

```
客户端 POST /api/v1/auth/register { email, password, nickname }
  │
  ▼ Handler: decodeAndValidate → RegisterInput
  ▼ Service:
  │  1. EmailExists(email) → 检查邮箱唯一
  │  2. Argon2id.HashPassword(password) → 哈希密码
  │  3. GenerateID() → 21字符 nanoid
  │  4. repo.Create(user) → 插入数据库
  │  5. issueTokens(user) → 签发双 Token
  │     ├── SignAccessToken(userId, email, jti)  → 15分钟
  │     ├── SignRefreshToken(userId, email, jti) → 7天
  │     └── SHA-256(refreshToken) → 存数据库
  │
  ▼ 响应: { user, accessToken, refreshToken, expiresAt }
```

### 3.2 登录

```
客户端 POST /api/v1/auth/login { email, password }
  │
  ▼ Service:
  │  1. GetByEmail(email) → 查用户
  │     └── 未找到 → "invalid email or password"（防枚举）
  │  2. Argon2id.VerifyPassword(password, hash) → 验证密码
  │     └── 不匹配 → "invalid email or password"（同一消息！）
  │  3. UpdateLastLogin(userId) → 更新登录时间
  │  4. issueTokens(user) → 签发双 Token
```

### 3.3 刷新 Token

```
客户端 POST /api/v1/auth/refresh { refreshToken }
  │
  ▼ Service:
  │  1. SHA-256(refreshToken) → tokenHash
  │  2. tokenRepo.GetByHash(tokenHash) → 查数据库
  │     └── 未找到/已吊销 → 401
  │  3. jwt.VerifyRefreshToken(refreshToken) → 验证签名
  │  4. tokenRepo.Revoke(tokenHash) → 吊销旧 token（旋转策略）
  │  5. GetByID(userId) → 确认用户仍存在
  │  6. issueTokens(user) → 签发全新的双 Token
```

### 3.4 登出

```
客户端 POST /api/v1/auth/logout { refreshToken? }
  │
  ▼ Auth 中间件: 验证 access token → 注入 userId, jti
  ▼ Service:
  │  1. 如果传了 refreshToken:
  │     └── SHA-256(refreshToken) → tokenRepo.Revoke → 吊销
  │  2. Redis SET blacklist:jti:<jti> "1" EX 15min → 加入黑名单
```

### 3.5 双 Token 安全设计

```
                Access Token                    Refresh Token
  ─────────────────────────────────  ─────────────────────────────
  有效期        15 分钟                          7 天
  用途          访问 API 资源                    换取新 access token
  存储位置      客户端内存                       客户端 secure storage
  服务端存储    无（无状态）                      SHA-256 哈希存 PG
  吊销方式      JTI 加入 Redis 黑名单            PG 标记 revoked_at
  签名密钥      accessSecret                    refreshSecret（独立！）
  刷新时        不变                             旧的吊销，签发新的（旋转）
```

---

## 4. 路由设计

### 4.1 路由分组

```
chi.Router
  ├── POST /health                          ← 健康检查（无认证）
  │
  ├── /api/v1/                              ← 外部接口前缀
  │   ├── POST /auth/register               ← 公开
  │   ├── POST /auth/login                  ← 公开
  │   ├── POST /auth/refresh                ← 公开
  │   │
  │   └── [Auth 中间件]                     ← 需要认证
  │       ├── POST /auth/logout
  │       ├── POST /user/profile
  │       ├── POST /user/update
  │       ├── POST /user/address/list
  │       ├── POST /user/address/create
  │       ├── POST /user/address/update
  │       └── POST /user/address/delete
  │
  └── /internal/                            ← 内部接口前缀
      └── [InternalOnly 中间件]             ← 需要密钥
          ├── POST /user/detail
          ├── POST /user/batch
          └── POST /user/address/detail
```

### 4.2 三层防护区分内部接口

| 层级 | 机制 | 作用 |
|------|------|------|
| 路由前缀 | `/internal/` vs `/api/v1/` | 语义区分 |
| 中间件 | `InternalOnly` 校验 `X-Internal-Secret` | 拒绝无密钥的请求 |
| 网络层 | Docker 内部网络 + Gateway 不代理 `/internal/` | 外部根本访问不到 |

---

## 5. 依赖注入链

```
cmd/user/main.go
  │
  ├── config.Load() + config.LoadUser(k)
  │
  ├── database.NewPool()        → *pgxpool.Pool
  ├── initRedis()               → *redis.Client (可能 nil)
  ├── auth.NewJWTManager()      → *JWTManager
  ├── auth.NewArgon2Hasher()    → *Argon2Hasher
  │
  ├── Repository 层
  │   ├── repository.NewUserRepository(pool)     → UserRepository
  │   ├── repository.NewAddressRepository(pool)  → AddressRepository
  │   └── repository.NewTokenRepository(pool)    → TokenRepository
  │
  ├── Service 层
  │   ├── service.NewAuthService(userRepo, tokenRepo, jwt, hasher, redis)
  │   ├── service.NewUserService(userRepo)
  │   └── service.NewAddressService(addrRepo)
  │
  └── Handler 层
      ├── handler.NewAuthHandler(authService)
      ├── handler.NewUserHandler(userService)
      └── handler.NewAddressHandler(addrService)
```

---

## 6. 文件清单

```
internal/user/
├── dto/
│   └── dto.go              — 请求输入 + 响应输出结构体
├── repository/
│   ├── user.go             — 用户仓储（接口 + sqlc 实现 + pgtype 转换辅助）
│   ├── address.go          — 地址仓储
│   └── token.go            — Token 仓储
├── service/
│   ├── auth.go             — 注册/登录/刷新/登出 + issueTokens + toUserResp
│   ├── user.go             — 获取/更新资料 + 内部批量查询
│   └── address.go          — 地址 CRUD + 默认地址切换 + toAddressResp
└── handler/
    ├── auth.go             — Auth HTTP handlers + decodeAndValidate 辅助
    ├── user.go             — User + Internal HTTP handlers
    └── address.go          — Address + Internal HTTP handlers

cmd/user/
└── main.go                 — 组装入口：配置 → 连接 → DI → 路由 → 启动
```

---

## 7. 数据库表

```
user_service.users
├── id           VARCHAR(21)   PK     nanoid
├── email        VARCHAR(255)  UNIQUE 邮箱
├── password     VARCHAR(255)         Argon2id 哈希
├── nickname     VARCHAR(50)   NULL   昵称
├── avatar_url   VARCHAR(500)  NULL   头像 URL
├── phone        VARCHAR(20)   NULL   手机号
├── status       VARCHAR(20)          active/suspended/deleted
├── last_login   TIMESTAMPTZ   NULL   最后登录时间
├── created_at   TIMESTAMPTZ          创建时间
├── updated_at   TIMESTAMPTZ          更新时间
└── deleted_at   TIMESTAMPTZ   NULL   软删除时间

user_service.user_addresses
├── id           VARCHAR(21)   PK
├── user_id      VARCHAR(21)   FK → users.id
├── label        VARCHAR(50)   NULL   标签（家/公司）
├── recipient    VARCHAR(100)         收件人
├── phone        VARCHAR(20)          收件人电话
├── province     VARCHAR(50)          省
├── city         VARCHAR(50)          市
├── district     VARCHAR(50)          区
├── address      VARCHAR(200)         详细地址
├── postal_code  VARCHAR(10)   NULL   邮编
├── is_default   BOOLEAN              是否默认
├── created_at   TIMESTAMPTZ
└── updated_at   TIMESTAMPTZ

user_service.refresh_tokens
├── id           VARCHAR(21)   PK
├── user_id      VARCHAR(21)   FK → users.id
├── token_hash   VARCHAR(255)  UNIQUE SHA-256 哈希
├── expires_at   TIMESTAMPTZ          过期时间
├── revoked_at   TIMESTAMPTZ   NULL   吊销时间
└── created_at   TIMESTAMPTZ
```

---

## 8. 与 TS 版对比

| 特性 | Go 版 | TS 版 |
|------|-------|-------|
| 三层架构 | Handler → Service → Repository | Controller → Service → Repository |
| 依赖注入 | 构造函数手动注入 | NestJS 装饰器 `@Injectable()` |
| 校验 | `validator` struct tag | `zod` schema |
| 可选字段 | `*string`（指针） | `string \| undefined` |
| Nullable 映射 | `pgtype.Text` ↔ `*string` | `string \| null` |
| 密码哈希 | `argon2id` 库 | `argon2` 库 |
| 错误处理 | `return err` 逐层返回 | `throw` 异常 |
| Token 存储 | SHA-256 哈希存 PG | SHA-256 哈希存 PG |
| 黑名单 | Redis SET + TTL | Redis SET + TTL |
| 路由分组 | `r.Route` + `r.Group` | NestJS `@Module` + `@Controller` |

---

## 9. 与其他阶段的衔接

### 9.1 Phase 6 使用了哪些前序阶段的能力

| 来源阶段 | 使用的能力 |
|----------|-----------|
| Phase 1 | `apperr`（错误码）、`response`（JSON 响应）、`handler.Wrap`（错误统一处理） |
| Phase 1 | `auth.JWTManager`（JWT 签发/验证）、`auth.Argon2Hasher`（密码哈希）、`auth.HashToken`（SHA-256） |
| Phase 1 | `id.GenerateID`（nanoid 主键）、`config.LoadUser`（配置加载） |
| Phase 3 | `database.NewPool`（PG 连接池）、`database.NewRedis`（Redis 客户端） |
| Phase 3 | `gen.Queries`（sqlc 生成的类型安全查询） |
| Phase 4 | `middleware.RequestID/Logger/RateLimit`（全局中间件）、`middleware.Auth`（认证中间件）、`middleware.InternalOnly`（内部鉴权） |

### 9.2 Phase 6 为后续阶段提供的能力

| 后续阶段 | 使用的能力 |
|----------|-----------|
| Phase 7 商品服务 | 三层架构模板（Handler → Service → Repository） |
| Phase 9 订单服务 | `POST /internal/user/detail` 获取用户信息 |
| Phase 9 订单服务 | `POST /internal/user/address/detail` 获取收货地址 |
| Phase 9 订单服务 | `POST /internal/user/batch` 批量获取用户 |

---

## 10. 关键 Go 知识点（本阶段涉及）

| 知识点 | 出现位置 | 说明 |
|--------|----------|------|
| 三层架构 | 整体结构 | Handler → Service → Repository，每层职责单一 |
| 接口隐式实现 | repository/*.go | 不需要 `implements` 声明，方法签名匹配即满足接口 |
| 构造函数注入 | 所有 `New*` 函数 | 依赖通过参数传入，不用全局变量 |
| `*string` 指针 | dto.go | 区分"没传"(nil) 和"传了空字符串"(&"") |
| `pgtype.Text` | repository/*.go | pgx 对 SQL nullable 的表示，需要和 `*string` 互转 |
| `errors.Is/As` | repository, service | 错误类型判断和提取 |
| `json.NewDecoder` | handler/auth.go | 流式 JSON 解析，比 `json.Unmarshal` 更高效 |
| `validator` tag | dto.go | `validate:"required,email"` 声明式校验 |
| `r.Route/r.Group` | cmd/user/main.go | chi 路由前缀分组和中间件分组 |
| 包别名 | cmd/user/main.go | `userhandler` 解决同名包冲突 |
| `make` 预分配 | service/user.go | `make([]T, 0, cap)` 避免切片扩容开销 |
