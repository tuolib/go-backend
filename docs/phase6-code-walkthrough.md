# Phase 6 代码逐行详解 — 用户服务

> 本文档对 Phase 6 所有新增/修改代码进行详细解释，帮助 Go 初学者理解用户服务的实现细节。

---

## 目录

1. [DTO 数据传输对象 — dto.go](#1-dto-数据传输对象)
2. [Repository 用户仓储 — repository/user.go](#2-repository-用户仓储)
3. [Repository 地址仓储 — repository/address.go](#3-repository-地址仓储)
4. [Repository Token 仓储 — repository/token.go](#4-repository-token-仓储)
5. [Service 认证逻辑 — service/auth.go](#5-service-认证逻辑)
6. [Service 用户资料 — service/user.go](#6-service-用户资料)
7. [Service 地址管理 — service/address.go](#7-service-地址管理)
8. [Handler Auth — handler/auth.go](#8-handler-auth)
9. [Handler User — handler/user.go](#9-handler-user)
10. [Handler Address — handler/address.go](#10-handler-address)
11. [Main 入口 — cmd/user/main.go](#11-main-入口)

---

## 1. DTO 数据传输对象

**文件：** `internal/user/dto/dto.go`

### 核心概念：请求/响应的「契约」

DTO（Data Transfer Object）定义了 Handler 和 Service 之间传递数据的格式。

```
Client JSON → Handler 解析 → XxxInput → Service → XxxResp → Handler 序列化 → Client JSON
```

### validate tag

```go
type RegisterInput struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8,max=100"`
    Nickname string `json:"nickname" validate:"omitempty,min=1,max=50"`
}
```

一个字段可以有多个 struct tag：
- `json:"email"` — JSON 序列化时的字段名
- `validate:"required,email"` — validator 库的校验规则

等价于 TS 的 Zod：`z.string().email()` → `validate:"required,email"`

### *string 指针处理可选更新

```go
type UpdateUserInput struct {
    Nickname  *string `json:"nickname"`
    AvatarURL *string `json:"avatarUrl"`
}
```

| JSON 输入 | `*string` 值 | 含义 |
|-----------|-------------|------|
| `{}` | `nil` | 没传，不更新 |
| `{"nickname": "Tom"}` | `&"Tom"` | 传了，更新为 "Tom" |
| `{"nickname": ""}` | `&""` | 传了空串，更新为空 |

TS 用 `undefined` vs `""` 区分，Go 没有 undefined，用指针 `nil` 代替。

### 响应不含 Password

```go
type UserResp struct {
    ID    string `json:"id"`
    Email string `json:"email"`
    // 没有 Password 字段 — 永远不返回密码
}
```

数据库 model 有 Password 字段，但 DTO 故意省略。Service 层的 `toUserResp()` 函数负责过滤。

---

## 2. Repository 用户仓储

**文件：** `internal/user/repository/user.go`

### 核心概念：接口 + 实现分离

```go
// 接口定义 — Service 层依赖这个
type UserRepository interface {
    GetByID(ctx context.Context, id string) (gen.UserServiceUser, error)
    GetByEmail(ctx context.Context, email string) (gen.UserServiceUser, error)
    Create(ctx context.Context, arg gen.CreateUserParams) (gen.UserServiceUser, error)
    ...
}

// 实现 — 小写开头，外部不可见
type userRepository struct {
    q *gen.Queries
}

// 构造函数 — 返回接口类型，不返回具体类型
func NewUserRepository(db gen.DBTX) UserRepository {
    return &userRepository{q: gen.New(db)}
}
```

Go 的接口是**隐式实现**的 — `userRepository` 不需要写 `implements UserRepository`，只要实现了接口的所有方法就自动满足。

### ErrNoRows → AppError 转换

```go
func (r *userRepository) GetByID(ctx context.Context, id string) (gen.UserServiceUser, error) {
    user, err := r.q.GetUserByID(ctx, id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return gen.UserServiceUser{}, apperr.New(apperr.ErrCodeUserNotFound, 404, "user not found")
        }
        return gen.UserServiceUser{}, err
    }
    return user, nil
}
```

Repository 的职责之一：把底层数据库错误翻译成业务错误。Service 层只需要处理 `AppError`，不需要知道 pgx 的存在。

### pgtype 转换辅助函数

```go
// 数据库 NULL → Go nil
func TextToStringPtr(t pgtype.Text) *string {
    if !t.Valid { return nil }
    return &t.String
}

// Go nil → 数据库 NULL
func StringPtrToText(s *string) pgtype.Text {
    if s == nil { return pgtype.Text{} }  // Valid=false → NULL
    return pgtype.Text{String: *s, Valid: true}
}
```

pgx 用 `pgtype.Text{Valid: false}` 表示 SQL NULL，而 Go API 层用 `*string` 的 `nil`。这两个函数在中间做桥接。

### gen.DBTX 接口参数

```go
func NewUserRepository(db gen.DBTX) UserRepository {
```

`gen.DBTX` 是 sqlc 生成的接口：

```go
type DBTX interface {
    Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
    Query(context.Context, string, ...interface{}) (pgx.Rows, error)
    QueryRow(context.Context, string, ...interface{}) pgx.Row
}
```

`*pgxpool.Pool`（连接池）和 `pgx.Tx`（事务）都实现了这个接口，所以同一个 repository 既能用连接池也能用事务。

---

## 3. Repository 地址仓储

**文件：** `internal/user/repository/address.go`

模式与 user.go 完全相同。一个注意点：

### 地址归属校验在 SQL 层

```go
func (r *addressRepository) GetByID(ctx context.Context, id, userID string) (...) {
    addr, err := r.q.GetAddressByID(ctx, gen.GetAddressByIDParams{
        ID:     id,
        UserID: userID,  // 同时校验 user_id
    })
}
```

SQL: `WHERE id = $1 AND user_id = $2` — 即使攻击者猜到了别人的地址 ID，因为 user_id 不匹配，查询返回 0 行 → 404。这比先查再判断更安全。

---

## 4. Repository Token 仓储

**文件：** `internal/user/repository/token.go`

### 应用层过期检查

```go
func (r *tokenRepository) GetByHash(ctx context.Context, tokenHash string) (...) {
    token, err := r.q.GetRefreshTokenByHash(ctx, tokenHash)
    // ... ErrNoRows 处理 ...

    // 额外检查过期
    if token.ExpiresAt.Valid && token.ExpiresAt.Time.Before(time.Now()) {
        return ..., apperr.New(apperr.ErrCodeTokenExpired, 401, "refresh token has expired")
    }
}
```

SQL 只过滤了 `revoked_at IS NULL`，过期检查在应用层做。原因：需要返回不同错误码（1104 过期 vs 1105 吊销），前端可以据此做不同处理。

### NewTimestamptz 辅助函数

```go
func NewTimestamptz(t time.Time) pgtype.Timestamptz {
    return pgtype.Timestamptz{Time: t, Valid: true}
}
```

创建 token 时需要设置 `expires_at`，每次写 `pgtype.Timestamptz{Time: t, Valid: true}` 太冗长。

---

## 5. Service 认证逻辑

**文件：** `internal/user/service/auth.go`

### AuthService 是编排者

```go
type AuthService struct {
    userRepo  repository.UserRepository   // 查/建用户
    tokenRepo repository.TokenRepository  // 存/查/吊销 token
    jwt       *auth.JWTManager            // 签发/验证 JWT
    hasher    *auth.Argon2Hasher           // 哈希/验证密码
    redis     *redis.Client               // access token 黑名单
}
```

AuthService 自己不做底层操作，协调 5 个组件完成流程。

### 登录安全 — 防邮箱枚举

```go
user, err := s.userRepo.GetByEmail(ctx, input.Email)
if err != nil {
    var appErr *apperr.AppError
    if errors.As(err, &appErr) && appErr.Code == apperr.ErrCodeUserNotFound {
        // 用户不存在 → 统一返回 "invalid email or password"
        return nil, apperr.New(apperr.ErrCodeInvalidCredentials, 401, "invalid email or password")
    }
    return nil, err
}

match, err := s.hasher.VerifyPassword(input.Password, user.Password)
if !match {
    // 密码错误 → 也返回 "invalid email or password"（同一消息！）
    return nil, apperr.New(apperr.ErrCodeInvalidCredentials, 401, "invalid email or password")
}
```

无论是「邮箱不存在」还是「密码错误」，都返回相同错误消息。防止攻击者通过不同错误消息枚举有效邮箱。

### errors.As — 错误类型断言

```go
var appErr *apperr.AppError
if errors.As(err, &appErr) && appErr.Code == apperr.ErrCodeUserNotFound {
```

- `errors.Is(err, target)` — 判断是不是**同一个**错误
- `errors.As(err, &target)` — 判断错误链中是否有**某个类型**，并提取

等价于 TS 的 `if (err instanceof AppError)`。

### issueTokens — 三方法复用

```go
func (s *AuthService) issueTokens(ctx context.Context, user gen.UserServiceUser) (*dto.AuthResp, error) {
    accessJTI, _ := id.GenerateID()
    refreshJTI, _ := id.GenerateID()

    accessToken, _ := s.jwt.SignAccessToken(user.ID, user.Email, accessJTI)
    refreshToken, _ := s.jwt.SignRefreshToken(user.ID, user.Email, refreshJTI)

    // 存 refresh token 哈希到数据库
    s.tokenRepo.Create(ctx, gen.CreateRefreshTokenParams{
        TokenHash: auth.HashToken(refreshToken),  // SHA-256
        ExpiresAt: repository.NewTimestamptz(now.Add(s.jwt.RefreshExpiresIn())),
    })

    return &dto.AuthResp{
        User:         toUserResp(user),
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ...
    }, nil
}
```

Register、Login、Refresh 三个方法都调用 `issueTokens`，避免重复代码。

### Refresh Token 旋转策略

```go
func (s *AuthService) Refresh(ctx context.Context, input dto.RefreshInput) (*dto.AuthResp, error) {
    // ...
    s.tokenRepo.Revoke(ctx, tokenHash)  // 吊销旧 token
    return s.issueTokens(ctx, user)     // 签发全新 token
}
```

每次刷新都生成新 refresh token，旧的立即作废。如果旧 token 被盗，攻击者使用后合法用户的 token 失效 → 用户发现异常。

### toUserResp — Model → DTO 转换

```go
func toUserResp(u gen.UserServiceUser) *dto.UserResp {
    return &dto.UserResp{
        ID:        u.ID,
        Email:     u.Email,
        // Password 被跳过！
        Nickname:  repository.TextToStringPtr(u.Nickname),
        AvatarURL: repository.TextToStringPtr(u.AvatarUrl),
        LastLogin: repository.TimestamptzToTimePtr(u.LastLogin),
        CreatedAt: u.CreatedAt.Time,  // pgtype.Timestamptz → time.Time
    }
}
```

这个函数是安全防火墙 — 确保密码永远不出现在响应中。同时做类型转换（`pgtype` → Go 标准类型）。

---

## 6. Service 用户资料

**文件：** `internal/user/service/user.go`

### COALESCE 策略的完整链路

```
*string(nil)                → 没传，不更新
    ↓ StringPtrToText
pgtype.Text{Valid:false}    → SQL 传 NULL
    ↓ SQL COALESCE
COALESCE(NULL, nickname)    → 保留原值

*string(&"Tom")             → 传了 "Tom"
    ↓ StringPtrToText
pgtype.Text{String:"Tom", Valid:true}
    ↓ SQL COALESCE
COALESCE("Tom", nickname)   → 更新为 "Tom"
```

### make 预分配切片

```go
results := make([]*dto.UserResp, 0, len(userIDs))
```

`make([]T, length, capacity)`:
- `length=0` — 初始为空
- `capacity=len(userIDs)` — 预分配底层数组

避免 `append` 时反复扩容（分配新数组 + 复制旧数据）。

---

## 7. Service 地址管理

**文件：** `internal/user/service/address.go`

### 默认地址切换

```go
// 同一用户只能有一个默认地址
if input.IsDefault {
    s.addrRepo.ClearDefault(ctx, userID)  // 清除旧默认
}
// 然后创建/更新为新默认
```

### 地址数量上限

```go
const maxAddressCount = 20

count, err := s.addrRepo.CountByUser(ctx, userID)
if count >= maxAddressCount {
    return nil, apperr.New(apperr.ErrCodeAddressLimit, 400, "address count limit reached")
}
```

防止恶意用户创建大量地址占用数据库空间。用 `const` 定义上限，修改时只改一处。

---

## 8. Handler Auth

**文件：** `internal/user/handler/auth.go`

### decodeAndValidate 辅助函数

```go
func decodeAndValidate(r *http.Request, dst any) error {
    if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
        return apperr.NewBadRequest("invalid request body")
    }
    if err := validate.Struct(dst); err != nil {
        return apperr.NewValidation(err.Error())
    }
    return nil
}
```

合并 JSON 解析 + 字段校验两步操作。三个 handler 文件共用。

`json.NewDecoder` vs `json.Unmarshal`:
- `NewDecoder` — 流式读取，直接从 `r.Body`（`io.Reader`）解析
- `Unmarshal` — 需要先 `io.ReadAll` 把整个 body 读到 `[]byte`

### validator 包级实例

```go
var validate = validator.New()
```

`validator.New()` 内部缓存 struct tag 解析结果，所以创建一次复用。`Struct()` 方法并发安全。

### Logout 的特殊处理

```go
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) error {
    var input dto.LogoutInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        input = dto.LogoutInput{}  // 解析失败不报错
    }
    accessJTI := middleware.TokenJTIFrom(r.Context())
    // ...
}
```

Logout 的 `refreshToken` 是可选的。客户端可能不发 body，解析失败时用空值。`accessJTI` 从 auth 中间件注入的 context 中获取。

---

## 9. Handler User

**文件：** `internal/user/handler/user.go`

### 外部接口 vs 内部接口

```go
// 外部接口 — userID 来自 JWT（auth 中间件注入 context）
func (h *UserHandler) Profile(w http.ResponseWriter, r *http.Request) error {
    userID := middleware.UserIDFrom(r.Context())
    // ...
}

// 内部接口 — userID 来自请求 body
func (h *UserHandler) Detail(w http.ResponseWriter, r *http.Request) error {
    var input dto.GetUserDetailInput  // { "id": "xxx" }
    // ...
}
```

同一个 `UserHandler`，方法本身是「中性」的。内部 vs 外部的区分在路由注册时由中间件决定。

---

## 10. Handler Address

**文件：** `internal/user/handler/address.go`

模式与 auth/user handler 完全一致。所有 handler 方法都是同一个模板的变体：

```go
func (h *XxxHandler) Action(w http.ResponseWriter, r *http.Request) error {
    userID := middleware.UserIDFrom(r.Context())           // 可选
    var input dto.XxxInput                                  // 可选
    if err := decodeAndValidate(r, &input); err != nil { return err }  // 可选
    result, err := h.service.Xxx(r.Context(), ...)
    if err != nil { return err }
    return response.Success(w, r, result)                   // 或 nil
}
```

---

## 11. Main 入口

**文件：** `cmd/user/main.go`

### 从骨架到完整组装

旧版（Phase 2 骨架）：
```go
port := envOr("USER_SERVICE_PORT", "3001")
r := chi.NewRouter()
r.Post("/health", handler.Wrap(healthHandler))
```

新版（Phase 6）：
```go
cfg := config.LoadUser(k)
pool, _ := database.NewPool(context.Background(), cfg.Postgres.URL)
redisClient := initRedis(cfg.Redis.URL)
// ... 构建三层 ...
r := chi.NewRouter()
r.Use(middleware.RequestID, middleware.Logger, middleware.RateLimit(...))
r.Route("/api/v1", func(r chi.Router) { ... })
r.Route("/internal", func(r chi.Router) { ... })
```

### 包别名

```go
import (
    "go-backend/internal/handler"                    // handler.Wrap
    userhandler "go-backend/internal/user/handler"   // userhandler.NewAuthHandler
)
```

两个包都叫 `handler`，用别名 `userhandler` 区分。TS 按名字导入不会冲突，Go 按包导入需要别名。

### r.Route + r.Group

```go
r.Route("/api/v1", func(r chi.Router) {        // 路径前缀分组
    r.Post("/auth/register", ...)               // → /api/v1/auth/register

    r.Group(func(r chi.Router) {                // 中间件分组（不加前缀）
        r.Use(middleware.Auth(...))
        r.Post("/auth/logout", ...)             // → /api/v1/auth/logout
    })
})
```

- `r.Route("/prefix", fn)` — 路径前缀分组，组内所有路由自动加前缀
- `r.Group(fn)` — 只分享中间件，不改变路径

### 依赖注入的构造顺序

```go
// 构造是自底向上的（先底层，后上层）
userRepo := repository.NewUserRepository(pool)           // 底层
authService := service.NewAuthService(userRepo, ...)     // 中层
authHandler := userhandler.NewAuthHandler(authService)   // 上层

// 但依赖方向是自顶向下的
// Handler → 依赖 Service → 依赖 Repository → 依赖 Database
```

所有依赖都在 `main()` 里一次性组装完成，没有全局变量，没有 `init()` 函数。
