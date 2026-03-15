# Phase 2 架构总览：共享工具包

> 本文档以架构师视角讲解 Phase 2 的全局设计——为什么需要这些包、它们之间的关系、以及每个设计决策背后的思考。

---

## 一句话概括

Phase 2 是给整个项目造**工具箱**。后续所有业务代码（用户注册、商品查询、下单支付……）都会用到这些工具。

---

## 全局依赖关系图

```
cmd/monolith/main.go  ──使用──→  config/       ← 读取环境变量，构建配置
        │                            │
        │                            ▼
        ├──使用──→  handler/wrap.go   ← 包装所有 HTTP 接口
        │              │
        │              ▼
        ├──使用──→  response/         ← 统一 JSON 响应格式
        │              │
        │              ▼
        │          apperr/            ← 统一错误类型 + 错误码
        │
        ├──使用──→  auth/             ← 用户认证（JWT + 密码哈希）
        │
        ├──使用──→  id/               ← 生成唯一 ID 和订单号
        │
        └──使用──→  httpclient/       ← 服务间互相调用的 HTTP 客户端
```

**依赖方向永远是：上层依赖下层，下层不知道上层的存在。**

---

## 每个包解决什么问题？

### 1. config/ — "每个服务只拿自己需要的配置"

**问题：** 5 个服务 + 1 个网关，每个需要不同的配置（用户服务需要 JWT 密钥，购物车服务不需要；订单服务需要数据库，购物车只要 Redis）。

**设计：**
```
config/
├── config.go      ← 公共基础（加载环境变量的核心函数 + 公共类型）
├── user.go        ← UserServiceConfig（只包含用户服务需要的字段）
├── product.go     ← ProductServiceConfig
├── cart.go        ← CartServiceConfig（没有 Postgres 字段——因为购物车只用 Redis）
├── order.go       ← OrderServiceConfig
└── gateway.go     ← GatewayConfig（有下游服务 URL 字段）
```

**关键决策：**
- **不用全局配置对象**。每个服务有独立的 Config struct，只暴露自己需要的字段。这样启动用户服务时，不会因为缺少购物车的配置而报错。
- **用 koanf 而不是直接 `os.Getenv()`**。koanf 提供类型转换（字符串→时长、字符串→切片），未来还能支持配置文件、远程配置中心，扩展性好。
- **环境变量 → 小写键映射**。`DATABASE_URL` 变成 `database_url`，统一风格。

### 2. apperr/ — "所有错误都有统一格式"

**问题：** 一个电商平台有几十种业务错误（用户不存在、库存不足、订单已过期……）。如果每个地方自己写 `errors.New("xxx")`，前端收到的错误格式五花八门，无法统一处理。

**设计：**
```go
// 一个错误 = 三个维度的信息
type AppError struct {
    Code       int    // 业务错误码（前端用来判断类型、显示对应 UI）
    Message    string // 可读文字（前端可能直接展示给用户）
    StatusCode int    // HTTP 状态码（404/400/500，前端框架用来判断成功/失败）
}
```

**错误码分段设计：**
```
1000-1199  通用错误（参数错误、未授权、频率限制……）
1100-1199  用户服务
2000-2099  商品服务
3000-3099  购物车服务
4000-4099  订单服务
5000-5099  管理后台
9000-9099  网关
```
分段的好处：看到错误码就知道是哪个服务出的问题，排查时一目了然。

**工厂函数设计：**
```go
apperr.NewNotFound("user", email)   // → 404, code=1003, "user not found: xxx"
apperr.NewConflict("user", email)   // → 409, code=1004, "user already exists: xxx"
apperr.NewBadRequest("invalid age") // → 400, code=1000, "invalid age"
```
调用方不需要记住状态码和错误码，函数名就是语义。

### 3. response/ — "所有 API 响应长一个样"

**问题：** 如果每个接口自己拼 JSON 响应，格式不统一：有的写 `{"result": ...}`，有的写 `{"data": ...}`，前端要针对每个接口写不同的解析逻辑。

**设计：统一信封格式**
```json
// 成功
{"code": 0, "success": true, "data": {实际数据}, "message": "ok", "traceId": "xxx"}

// 失败
{"code": 1003, "success": false, "message": "user not found", "data": null, "meta": {...}, "traceId": "xxx"}

// 分页
{"code": 0, "success": true, "data": {"items": [...], "pagination": {page, total, ...}}, ...}
```

**三个核心函数：**
- `response.Success(w, r, data)` — 包装成功响应
- `response.Paginated(w, r, items, total, page, pageSize)` — 包装分页响应
- `response.HandleError(w, r, err)` — 把 error 转成错误 JSON 响应

**关键决策：HandleError 的两层处理**
```
收到 error
  ├── 是 AppError？→ 用它的 Code/StatusCode/Message 构建响应
  └── 是其他 error？→ 记录日志 + 返回通用 500 错误
```
这保证了：即使代码中有未预料到的错误（数据库连接断了、空指针等），客户端也会收到格式正确的 JSON，而不是一堆 HTML 错误页。

### 4. auth/ — "用户身份的三层安全"

**三个文件对应三个安全问题：**

| 文件 | 解决的问题 | 比喻 |
|------|-----------|------|
| jwt.go | "你是谁？" — 身份认证 | 身份证 |
| argon2.go | "密码怎么存？" — 密码安全存储 | 保险箱 |
| hash.go | "token 怎么存？" — token 安全存储 | 指纹 |

**JWT 双令牌机制：**
```
用户登录
  → 服务器签发两个 token：
    ├── Access Token（15分钟有效）— 每次请求都带，用于鉴权
    └── Refresh Token（7天有效）— 存在 HttpOnly Cookie，用于刷新

Access Token 过期了？
  → 客户端用 Refresh Token 换一对新的
  → 用户无需重新输入密码（静默续期）

Refresh Token 也过期了？
  → 用户必须重新登录
```

**为什么不用一个 token？**
如果一个 token 有效期很长（比如 7 天），被偷了就能用 7 天。双令牌方案中，被偷的 Access Token 最多用 15 分钟；Refresh Token 存在 HttpOnly Cookie 中，JavaScript 偷不到。

**密码哈希为什么用 Argon2id 而不是 MD5/SHA256？**
- MD5/SHA256 太快了——黑客可以每秒试几十亿个密码
- Argon2id 故意设计得很慢（~50ms 一次），还会占用大量内存，让暴力破解变得极其昂贵
- 每次哈希都加随机盐，同样的密码 "123456" 每次生成的哈希都不同，彩虹表攻击失效

**Refresh Token 为什么要 SHA256 再存库？**
数据库可能被泄露（SQL注入、备份泄露等）。如果存原始 token，黑客直接拿去用。存哈希后，黑客拿到的是不可逆的指纹，无法还原出原始 token。

### 5. id/ — "全球唯一的 21 个字符"

**问题：** 数据库主键用什么？

| 方案 | 缺点 |
|------|------|
| 自增整数（1, 2, 3...） | 暴露业务量（ID=10000 说明有 1 万用户）；分库时容易冲突 |
| UUID（550e8400-e29b-...） | 太长（36 字符），URL 不友好 |
| **nanoid（V1StGXR8_Z...）** | 21 字符、URL 安全、碰撞概率极低 |

**订单号为什么单独生成？**
订单号不只是 ID——它是给用户看的、客服要用的、物流要核对的。格式 `YYYYMMDDHHmmss + 8位随机数` 让你一眼就能看出订单是什么时候下的。

### 6. httpclient/ — "微服务之间怎么说话"

**问题：** 创建订单时，订单服务需要查商品价格（商品服务）、查收货地址（用户服务）、扣库存（商品服务）。在微服务模式下，这些服务跑在不同的进程/容器里，怎么互相调用？

**设计：封装一个带"身份证"的 HTTP 客户端**
```
订单服务 ──POST /internal/product/sku/batch──→ 商品服务
           Headers:
             X-Trace-Id: abc123     ← 追踪ID，串联整个调用链
             X-Internal-Secret: xxx ← 内部密钥，证明"我是自家服务"
             Content-Type: application/json
```

**为什么需要 X-Internal-Secret？**
`/internal/*` 接口只允许服务间调用，不能被外部用户访问。这个密钥就是"暗号"——只有知道暗号的服务才能调用内部接口。

**为什么传递 Trace ID？**
一个用户请求可能触发多个服务的调用链。Trace ID 贯穿整个链路：
```
用户 → Gateway(traceId=abc) → 订单服务(traceId=abc) → 商品服务(traceId=abc)
                                                     → 用户服务(traceId=abc)
```
出问题时，用 traceId=abc 一搜日志，整个调用链一目了然。

---

## 这些包在后续阶段怎么用？

| Phase | 用到的共享包 |
|-------|-------------|
| Phase 3（数据库迁移） | config/ |
| Phase 4（用户注册登录） | config/ + apperr/ + response/ + handler/ + auth/ + id/ |
| Phase 5（商品管理） | config/ + apperr/ + response/ + handler/ + id/ |
| Phase 6（购物车） | config/ + apperr/ + response/ + handler/ + httpclient/ |
| Phase 7（订单+库存） | **全部都用到** |

Phase 2 不生产任何用户可见的功能，但它是所有功能的基石。好的基础设施让后续开发又快又安全。

---

## 设计原则总结

1. **依赖注入，不用全局变量** — 每个工具都是 `New...()` 创建，依赖通过参数传入
2. **接口隐式实现** — `AppError` 自动满足 `error` 接口，不需要声明
3. **关注点分离** — 错误定义（apperr）、错误展示（response）、错误产生（业务代码）三者分离
4. **安全优先** — 密码用 Argon2id、token 存哈希、服务间调用有密钥
5. **约定大于配置** — JSON 字段名、错误码分段、函数命名都有统一规范
