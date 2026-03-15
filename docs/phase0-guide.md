# Phase 0 学习笔记：项目骨架 + 工具链

> 搭房子之前，先把地基和框架立好。Phase 0 不写业务代码，只搭建所有后续代码的基础。

---

## 第一步：`go mod init` — 给项目办身份证

```bash
go mod init go-backend
```

Go 项目必须有一个**模块名**，就像每个 npm 项目需要 `package.json` 一样。`go.mod` 这个文件告诉 Go：

- 我叫 `go-backend`
- 我用 Go 1.22+
- 我依赖了哪些第三方库

之后你 `import "go-backend/internal/response"` 才能找到对应代码。

---

## 第二步：创建目录结构 — Go 项目的"规矩"

```
cmd/           ← 每个可运行程序的入口（main 函数在这里）
internal/      ← 核心业务代码（Go 的特殊约定：别人无法导入这个目录）
```

**为什么是 `cmd/` 和 `internal/`？**

这是 Go 社区的标准布局，不是我们发明的。`internal/` 是 Go 语言层面的保护机制——编译器会阻止外部项目导入你的 `internal` 包。这就像把核心代码锁在保险箱里。

```
cmd/
├── monolith/main.go    ← 一个进程跑全部服务（开发用）
├── gateway/main.go     ← API 网关（生产用）
├── user/main.go        ← 用户服务
├── product/main.go     ← 商品服务
├── cart/main.go        ← 购物车服务
├── order/main.go       ← 订单服务
└── migrate/main.go     ← 数据库迁移工具
```

**为什么 7 个 main.go？**

Go 的规则：一个 `main` 包 = 一个可执行文件。我们将来既可以全部打包成一个程序跑（monolith），也可以每个服务独立部署（微服务）。但在 Phase 0，它们都只有一个功能：告诉你"我活着"。

---

## 第三步：三个核心包 — 整个项目的"基础设施"

### 1. `internal/apperr/` — 统一错误语言

```go
// 任何地方出错了，都用这个格式
type AppError struct {
    Code       int    // 错误码，比如 1003 = 没找到
    Message    string // "user not found: xxx"
    StatusCode int    // HTTP 状态码，比如 404
}
```

**为什么不直接用 Go 的 `errors.New("出错了")`？**

因为前端需要知道：这是什么错？该显示什么？HTTP 状态码是多少？裸的 `error` 只有一段文字，不够结构化。`AppError` 把错误变成了一个"标准格式"，全项目统一。

### 2. `internal/response/` — 统一响应格式

```go
// 所有 API 返回都长这样：
{
    "code": 0,           // 0 = 成功
    "success": true,
    "data": { ... },     // 实际数据
    "message": "ok",
    "traceId": "xxx"     // 追踪 ID，排查问题用
}
```

**为什么要统一？** 前端只需要学一种格式，就能处理所有接口的响应。不会出现有的接口返回 `{ result: ... }`，有的返回 `{ data: ... }` 的混乱局面。

### 3. `internal/handler/wrap.go` — 最精妙的一个设计

先看普通写法和我们的写法对比：

```go
// ❌ 普通写法：每个接口都要重复错误处理
func Profile(w http.ResponseWriter, r *http.Request) {
    user, err := service.GetProfile(r.Context(), userID)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(500)
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return   // 别忘了 return！忘了就出 bug
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(200)
    json.NewEncoder(w).Encode(user)
}

// ✅ 我们的写法：handler 只管业务逻辑，错误处理交给 Wrap
func (h *UserHandler) Profile(w http.ResponseWriter, r *http.Request) error {
    user, err := h.service.GetProfile(r.Context(), userID)
    if err != nil {
        return err  // 直接返回，完事
    }
    return response.Success(w, r, user)
}
```

**`Wrap` 做了什么？**

```go
func Wrap(h AppHandler) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if err := h(w, r); err != nil {
            response.HandleError(w, r, err)  // 自动转成 JSON 错误响应
        }
    }
}
```

它就像一个"包装纸"——把我们自定义的 `func(...) error` 包成 Go 标准库要求的 `func(w, r)` 格式，同时在外层统一捕获错误。

**好处**：你写 50 个接口，每个都不用操心错误怎么变成 JSON，`return err` 就行。

---

## 第四步：每个服务的 main.go — 最小可运行结构

以用户服务为例，拆解每一段：

```go
func main() {
    // 1️⃣ 读配置（现在只有端口号）
    port := envOr("USER_SERVICE_PORT", "3001")

    // 2️⃣ 创建路由器（chi 是个轻量路由库）
    r := chi.NewRouter()
    r.Post("/health", handler.Wrap(healthHandler))

    // 3️⃣ 创建 HTTP 服务器
    srv := &http.Server{
        Addr:    ":" + port,
        Handler: r,
    }

    // 4️⃣ 监听系统信号（Ctrl+C 或 kill）
    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    // 5️⃣ 在后台启动服务器
    go func() {
        srv.ListenAndServe()
    }()

    // 6️⃣ 等着，直到收到停止信号
    <-ctx.Done()

    // 7️⃣ 优雅关停：等正在处理的请求完成，再退出
    srv.Shutdown(timeoutCtx)
}
```

**为什么不直接 `srv.ListenAndServe()` 就完事？**

因为如果你 Ctrl+C 强杀进程，正在处理中的请求会被截断。优雅关停会说："新请求别进来了，但已经在处理的，让它们做完。"这在生产环境非常重要。

---

## 第五步：验收 — 确认地基稳固

```bash
go build ./...     # 全部代码能编译 ✅
make build-all     # 生成 7 个二进制 ✅
curl POST /health  # 每个服务都能响应 ✅
```

---

## 总结：Phase 0 建立了什么？

| 我们搭好的 | 后续阶段会用来做什么 |
|-----------|-------------------|
| `go.mod` | 所有依赖管理的基础 |
| `cmd/*/main.go` | 往里面加路由、中间件、依赖注入 |
| `apperr` | 所有业务错误都用它 |
| `response` | 所有接口响应都用它 |
| `handler.Wrap` | 所有 handler 都用它包装 |
| `Makefile` | 开发、测试、部署的命令入口 |

**Phase 0 一行业务代码都没写**，但是所有后续代码都会建立在这些"地基"上。

---

## Go 核心概念速查（Phase 0 涉及的）

### package 与 import

```go
package main              // 声明这个文件属于哪个包
import "go-backend/internal/response"  // 导入其他包
```

- `package main` 是特殊的——它表示"这是一个可执行程序的入口"
- 其他包名（如 `package response`）只是代码库，被别人导入使用

### struct — Go 的"类"

```go
type AppError struct {
    Code    int
    Message string
}
```

Go 没有 class，用 struct（结构体）代替。可以给 struct 绑定方法：

```go
func (e *AppError) Error() string {
    return e.Message
}
```

### interface — Go 的"契约"

```go
type error interface {
    Error() string
}
```

只要你的类型有 `Error() string` 方法，它就自动满足 `error` 接口。不需要写 `implements`。这叫**隐式实现**，是 Go 最优雅的设计之一。

### goroutine — Go 的"轻量线程"

```go
go func() {
    srv.ListenAndServe()  // 在后台运行
}()
```

`go` 关键字启动一个 goroutine，它会和 main 函数并行执行。比操作系统线程轻很多（一个程序可以轻松跑百万个 goroutine）。

### channel — goroutine 之间的"传话筒"

```go
<-ctx.Done()  // 等待信号，阻塞在这里
```

`<-` 是从 channel 接收数据。`ctx.Done()` 返回一个 channel，当收到 Ctrl+C 信号时，这个 channel 会被关闭，`<-` 就不再阻塞，程序继续往下走（执行关停逻辑）。

### defer — "最后别忘了做这件事"

```go
defer stop()  // 函数结束前一定会执行 stop()
```

不管函数是正常返回还是出错，`defer` 的代码都会执行。常用于关闭文件、释放资源、解锁等清理操作。

---

## 下一阶段预告：Phase 1

数据库连接、配置加载、Redis 连接——让程序能"连上"存储。就像地基打好之后，接水管和电线。
