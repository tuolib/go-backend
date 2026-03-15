# Phase 0 逐行代码解析

> 本文档针对 Go 新手，对 Phase 0 生成的每一个文件、每一行代码进行详细解释。
> 如果你完全没学过 Go，建议从头到尾读一遍，代码中的每个符号都会解释。

---

## 目录

1. [internal/handler/wrap.go — Handler 包装器](#1-internalhandlerwrapgo)
2. [internal/apperr/codes.go — 错误码常量](#2-internalapperrcodesgo)
3. [internal/apperr/errors.go — 错误类型定义](#3-internalapperrerrorsgo)
4. [internal/response/response.go — 统一响应](#4-internalresponseresponsego)
5. [cmd/user/main.go — 用户服务入口（代表所有微服务）](#5-cmdusermain go)
6. [cmd/monolith/main.go — 单体入口](#6-cmdmonolithmain go)
7. [cmd/migrate/main.go — 迁移工具](#7-cmdmigratemain go)

---

## 1. internal/handler/wrap.go

这是整个项目最核心的设计模式，只有 22 行，但非常精妙。

```go
package handler                                                    // ①
```
**① `package handler`**
声明这个文件属于 `handler` 包。Go 中同一个目录下所有 `.go` 文件必须属于同一个包。包名就像一个"命名空间"，别人 import 这个包后用 `handler.Xxx` 来调用。

```go
import (                                                           // ②
    "net/http"                                                     // ③

    "go-backend/internal/response"                                 // ④
)
```
**② `import (...)`**
导入其他包。圆括号可以一次导入多个。

**③ `"net/http"`**
Go 标准库的 HTTP 包。提供了 `http.ResponseWriter`、`http.Request`、`http.HandlerFunc` 等类型。这是 Go 写 Web 服务的基础。

**④ `"go-backend/internal/response"`**
我们自己项目的包。`go-backend` 是模块名（go.mod 里定义的），`internal/response` 是目录路径。

```go
type AppHandler func(w http.ResponseWriter, r *http.Request) error // ⑤
```
**⑤ `type AppHandler func(...) error`**
定义一个新类型 `AppHandler`。这个类型是一个**函数类型**（Go 中函数也是一种类型，可以当变量传递）。

拆解函数签名：
- `w http.ResponseWriter` — "写回器"，用来向客户端写入 HTTP 响应（状态码、响应体等）
- `r *http.Request` — "请求"，包含客户端发来的所有信息（URL、Header、Body 等）。`*` 表示这是一个**指针**（后面会解释）
- `error` — 返回值是 Go 内置的错误接口。如果没出错返回 `nil`，出错返回一个 error 对象

**为什么要自定义这个类型？**
Go 标准库的 `http.HandlerFunc` 签名是 `func(w, r)` 没有返回值。我们加了个 `error` 返回值，让 handler 可以"把错误往上抛"，而不用每个 handler 自己处理错误。

```go
func Wrap(h AppHandler) http.HandlerFunc {                         // ⑥
```
**⑥ `func Wrap(h AppHandler) http.HandlerFunc`**
定义一个叫 `Wrap` 的函数：
- 参数 `h AppHandler` — 接收一个我们自定义的 handler 函数
- 返回值 `http.HandlerFunc` — 返回一个 Go 标准库认识的 handler 函数

**大写字母开头 = 导出（公开）**。其他包可以调用 `handler.Wrap(...)`。小写开头的函数只能在包内部使用。

```go
    return func(w http.ResponseWriter, r *http.Request) {          // ⑦
```
**⑦ 匿名函数（闭包）**
`return func(...)` 返回一个没有名字的函数。这个函数"记住了"外层的变量 `h`（这叫**闭包**）。当这个返回的函数被调用时，它还能访问 `h`。

```go
        if err := h(w, r); err != nil {                            // ⑧
```
**⑧ `if err := h(w, r); err != nil`**
Go 特有的"短声明 + 判断"写法，等价于：
```go
err := h(w, r)      // 调用实际的 handler，得到返回值
if err != nil {      // 如果返回了错误
```
`:=` 是 Go 的**短变量声明**，自动推导类型（相当于 `var err error = h(w, r)`）。
`nil` 是 Go 的"空值"，类似其他语言的 `null`。

```go
            response.HandleError(w, r, err)                        // ⑨
```
**⑨ 统一错误处理**
如果 handler 返回了错误，交给 `response.HandleError` 把错误转成 JSON 响应发给客户端。

**整体效果：** 所有 handler 只需 `return err`，不用关心怎么转 JSON。

---

## 2. internal/apperr/codes.go

```go
package apperr                                                     // ①
```
**① `package apperr`**
包名叫 `apperr`（application error 的缩写）。Go 包名习惯用**小写、不带下划线**的短名字。

```go
const (                                                            // ②
    ErrCodeBadRequest   = 1000                                     // ③
    ErrCodeUnauthorized = 1001
    ErrCodeForbidden    = 1002
    ErrCodeNotFound     = 1003
    ErrCodeConflict     = 1004
    ErrCodeInternal     = 1500
    ErrCodeRateLimit    = 1006
)
```
**② `const (...)`**
声明一组**常量**。常量在编译时确定，运行时不可修改。用圆括号批量声明。

**③ `ErrCodeBadRequest = 1000`**
定义一个常量，值是整数 1000。Go 会自动推导类型为 `int`。
- 大写开头（`ErrCode...`）→ 其他包可以用 `apperr.ErrCodeBadRequest` 访问
- 命名规范：`ErrCode` 前缀表示"错误码"，一看就知道用途

**为什么用数字而不是字符串？**
数字比较快、不会拼错、前端也方便用 switch 判断。不同业务域用不同数字段（1xxx 用户、2xxx 商品……），避免冲突。

---

## 3. internal/apperr/errors.go

```go
package apperr

import (
    "fmt"                                                          // ①
    "net/http"                                                     // ②
)
```
**① `"fmt"`** — 格式化包，提供 `Sprintf`（格式化字符串）等功能。
**② `"net/http"`** — 这里用它提供的 HTTP 状态码常量，比如 `http.StatusNotFound = 404`。

```go
type AppError struct {                                             // ③
    Code       int    `json:"code"`                                // ④
    Message    string `json:"message"`                             // ⑤
    StatusCode int    `json:"-"`                                   // ⑥
}
```
**③ `type AppError struct { ... }`**
定义一个**结构体**（struct）。结构体是 Go 中组织数据的主要方式，类似其他语言的 class，但没有继承。

**④ `Code int \`json:"code"\``**
- `Code` — 字段名（大写 = 导出，外部可访问）
- `int` — 类型是整数
- `` `json:"code"` `` — 这是**结构体标签（struct tag）**，用反引号包裹。它告诉 JSON 编码器：当把这个 struct 转成 JSON 时，这个字段叫 `"code"` 而不是 `"Code"`

**⑤ `Message string \`json:"message"\``**
`string` 类型的字段。JSON 中会变成 `"message": "xxx"`。

**⑥ `StatusCode int \`json:"-"\``**
`` `json:"-"` `` 中的 `-` 是特殊标记：**转 JSON 时忽略这个字段**。因为 HTTP 状态码已经在响应头里了，不需要重复出现在 JSON body 中。

```go
func (e *AppError) Error() string {                                // ⑦
    return e.Message
}
```
**⑦ 方法定义**
这行的语法需要拆解：
- `func` — 定义函数
- `(e *AppError)` — 这个部分叫**接收器（receiver）**，意思是"这个函数属于 `*AppError` 类型"。`e` 是变量名，`*AppError` 是 AppError 的指针类型
- `Error()` — 方法名
- `string` — 返回值类型

**为什么叫 `Error()`？**
因为 Go 内置的 `error` 接口定义就是：
```go
type error interface {
    Error() string
}
```
只要你的类型有 `Error() string` 方法，它就**自动实现了 `error` 接口**。这意味着 `*AppError` 可以被当作普通的 `error` 来用（赋值、返回、传参都行）。Go 叫这个**隐式实现**——不需要写 `implements`。

```go
func New(code int, statusCode int, message string) *AppError {     // ⑧
    return &AppError{                                              // ⑨
        Code:       code,
        StatusCode: statusCode,
        Message:    message,
    }
}
```
**⑧ 工厂函数**
Go 没有构造函数（constructor），习惯用 `New...()` 函数来创建对象。

**⑨ `&AppError{...}`**
- `AppError{...}` — 创建一个 AppError 值，`{字段名: 值}` 语法初始化
- `&` — 取地址，返回指针（`*AppError`）。为什么返回指针？因为指针共享同一块内存，传递高效；而且只有指针类型才有上面的 `Error()` 方法

```go
func NewNotFound(resource, identifier string) *AppError {          // ⑩
    return &AppError{
        Code:       ErrCodeNotFound,                               // ⑪
        StatusCode: http.StatusNotFound,                           // ⑫
        Message:    fmt.Sprintf("%s not found: %s", resource, identifier), // ⑬
    }
}
```
**⑩ `resource, identifier string`**
两个参数类型相同时可以合并写。等价于 `resource string, identifier string`。

**⑪ `ErrCodeNotFound`** — 引用 codes.go 中定义的常量 1003。

**⑫ `http.StatusNotFound`** — Go 标准库定义的常量，值是 404。比写数字 404 更清晰、不容易出错。

**⑬ `fmt.Sprintf`** — 格式化字符串，`%s` 是字符串占位符。结果类似 `"user not found: test@example.com"`。

后面的 `NewBadRequest`、`NewUnauthorized`、`NewInternal` 都是同样的模式，只是状态码和错误码不同，不再重复解释。

---

## 4. internal/response/response.go

```go
package response

import (
    "encoding/json"                                                // ①
    "errors"                                                       // ②
    "log/slog"                                                     // ③
    "net/http"

    "go-backend/internal/apperr"
)
```
**① `"encoding/json"`** — Go 标准库的 JSON 编解码包。把 struct 变成 JSON 字符串，或反过来。
**② `"errors"`** — Go 标准库的错误处理工具包，提供 `errors.As()`（后面解释）。
**③ `"log/slog"`** — Go 1.21 新增的**结构化日志**标准库。输出的日志是 key=value 格式，方便机器解析。

```go
type SuccessResp struct {
    Code    int    `json:"code"`
    Success bool   `json:"success"`                                // ④
    Data    any    `json:"data"`                                   // ⑤
    Message string `json:"message"`
    TraceID string `json:"traceId"`
}
```
**④ `Success bool`** — `bool` 类型，只有 `true` 或 `false`。

**⑤ `Data any`** — `any` 是 Go 1.18 新增的类型别名，等同于 `interface{}`（空接口）。意思是"这个字段可以放**任何类型**的数据"——字符串、数字、数组、另一个 struct 都行。JSON 编码器会根据实际类型自动转换。

```go
type ErrorResp struct {
    Code    int    `json:"code"`
    Success bool   `json:"success"`
    Message string `json:"message"`
    Data    any    `json:"data"`
    Meta    *ErrorMeta `json:"meta,omitempty"`                     // ⑥
    TraceID string     `json:"traceId"`
}
```
**⑥ `*ErrorMeta` 和 `omitempty`**
- `*ErrorMeta` — 指针类型。指针可以是 `nil`（表示"没有 Meta"）
- `json:"meta,omitempty"` — `omitempty` 告诉 JSON 编码器：**如果这个字段是零值（nil），就不输出到 JSON 中**。这样成功响应里就不会出现多余的 `"meta": null`

```go
func Success(w http.ResponseWriter, r *http.Request, data any) error {
    resp := SuccessResp{                                           // ⑦
        Code:    0,
        Success: true,
        Data:    data,
        Message: "ok",
        TraceID: traceIDFrom(r),
    }
    return writeJSON(w, http.StatusOK, resp)                       // ⑧
}
```
**⑦ `resp := SuccessResp{...}`**
创建一个 SuccessResp 实例并初始化。`:=` 自动推导出 `resp` 的类型是 `SuccessResp`。

**⑧ `return writeJSON(w, http.StatusOK, resp)`**
调用下面定义的 `writeJSON` 函数，传入响应写回器、状态码 200、响应体。`writeJSON` 也返回 `error`，这里直接把它 `return` 出去。

```go
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
    var appErr *apperr.AppError                                    // ⑨
    if errors.As(err, &appErr) {                                   // ⑩
```
**⑨ `var appErr *apperr.AppError`**
用 `var` 声明一个变量。`var` 和 `:=` 的区别：
- `var x Type` — 显式指定类型，初始值是零值（指针的零值是 nil）
- `x := value` — 必须立刻赋值，类型自动推导

**⑩ `errors.As(err, &appErr)`**
这是 Go 的**错误类型断言**。意思是："传入的 `err` 是不是 `*AppError` 类型？如果是，把它赋值给 `appErr`"。
- `&appErr` — 取 `appErr` 变量的地址传给 `errors.As`，让它往里面写值
- 返回 `true`/`false` 表示匹配成功/失败

**为什么用 `errors.As` 而不是类型转换？** 因为 `errors.As` 能穿透错误链——即使错误被 `fmt.Errorf("...: %w", err)` 包装了多层，也能找到最里面的 `AppError`。

```go
    // Unknown error → 500
    slog.ErrorContext(r.Context(), "unhandled error", "error", err) // ⑪
```
**⑪ 结构化日志**
`slog.ErrorContext` 输出一条 ERROR 级别的日志。参数是 key-value 对：`"error"` 是 key，`err` 是 value。输出类似：
```
2026/03/15 ERROR unhandled error error="something went wrong"
```
`r.Context()` 传入请求上下文，日志可以自动关联 traceId。

```go
func writeJSON(w http.ResponseWriter, status int, v any) error {
    w.Header().Set("Content-Type", "application/json")             // ⑫
    w.WriteHeader(status)                                          // ⑬
    return json.NewEncoder(w).Encode(v)                            // ⑭
}
```
**⑫ `w.Header().Set(...)`** — 设置响应头。告诉客户端"返回的是 JSON"。
**⑬ `w.WriteHeader(status)`** — 写入 HTTP 状态码（200、404、500 等）。
**⑭ `json.NewEncoder(w).Encode(v)`** — 创建一个 JSON 编码器，直接往 `w`（响应流）里写入 `v`（任意类型的数据）编码后的 JSON。`Encode` 会自动处理所有类型转换。

```go
type contextKey string                                             // ⑮
const traceIDKey contextKey = "traceId"                            // ⑯
```
**⑮ 自定义类型防碰撞**
`type contextKey string` 创建了一个**新类型**，底层是 string 但和 string 是不同类型。为什么不直接用 string？因为 Go 的 context 是全局的"背包"，如果大家都用 string 做 key，比如两个包都用 `"id"` 做 key，就会冲突。自定义类型保证了键唯一。

**⑯** 用自定义类型声明一个常量作为 context 的键。

```go
func traceIDFrom(r *http.Request) string {
    if id, ok := r.Context().Value(traceIDKey).(string); ok {      // ⑰
        return id
    }
    return ""                                                      // ⑱
}
```
**⑰ `r.Context().Value(key).(string)`**
这行做了三件事：
1. `r.Context()` — 从请求中取出 context（请求级的"背包"）
2. `.Value(traceIDKey)` — 从 context 中用 key 取值，返回 `any` 类型
3. `.(string)` — **类型断言**：把 `any` 转成 `string`。`ok` 表示转换是否成功

**⑱ `return ""`** — 如果 context 中没有 traceId，返回空字符串。

---

## 5. cmd/user/main.go

这是用户服务的入口。其他微服务（product/cart/order/gateway）结构完全相同，只是服务名和端口不同。

```go
package main                                                       // ①
```
**① `package main`**
特殊的包名。`main` 包 + `main()` 函数 = Go 编译器会把这个包编译成一个**可执行文件**。一个目录下只能有一个 `main` 包。

```go
import (
    "context"                                                      // ②
    "log/slog"
    "net/http"
    "os"                                                           // ③
    "os/signal"                                                    // ④
    "syscall"                                                      // ⑤
    "time"                                                         // ⑥

    "github.com/go-chi/chi/v5"                                     // ⑦

    "go-backend/internal/handler"
    "go-backend/internal/response"
)
```
**②** `"context"` — Go 的上下文包。用于在函数间传递请求范围的数据、截止时间、取消信号。几乎所有 I/O 操作都接受 context 参数。
**③** `"os"` — 操作系统功能（环境变量、退出程序等）。
**④** `"os/signal"` — 监听操作系统信号（Ctrl+C 等）。
**⑤** `"syscall"` — 系统调用常量（`SIGINT` = Ctrl+C，`SIGTERM` = kill 命令）。
**⑥** `"time"` — 时间相关（时长、定时器等）。
**⑦** `"github.com/go-chi/chi/v5"` — 第三方路由库。`github.com/...` 表示从 GitHub 下载的依赖。

```go
func main() {
    port := envOr("USER_SERVICE_PORT", "3001")                     // ⑧
```
**⑧ 读取端口配置**
调用下面定义的 `envOr` 函数：先查环境变量 `USER_SERVICE_PORT`，如果没设置就用默认值 `"3001"`。

```go
    r := chi.NewRouter()                                           // ⑨
    r.Post("/health", handler.Wrap(healthHandler))                 // ⑩
```
**⑨** 创建 chi 路由器。路由器的作用：根据请求的 URL 和方法（GET/POST），把请求分发给对应的处理函数。

**⑩ 注册路由**
拆解：
- `r.Post("/health", ...)` — 当客户端发送 `POST /health` 时，执行后面的函数
- `handler.Wrap(healthHandler)` — 用 Wrap 把我们自定义的 `healthHandler`（返回 error）包装成标准 handler
- 串起来就是：`POST /health` → Wrap 包装 → 调用 healthHandler → 如果有错误自动转 JSON

```go
    srv := &http.Server{                                           // ⑪
        Addr:              ":" + port,                             // ⑫
        Handler:           r,                                      // ⑬
        ReadHeaderTimeout: 10 * time.Second,                       // ⑭
    }
```
**⑪ 创建 HTTP 服务器**
`&http.Server{...}` 创建一个 Server 结构体的指针。

**⑫ `Addr: ":" + port`**
监听地址。`":3001"` 表示监听所有网络接口的 3001 端口。字符串用 `+` 拼接。

**⑬ `Handler: r`**
把我们创建的 chi 路由器交给服务器，所有请求都通过路由器分发。

**⑭ `ReadHeaderTimeout: 10 * time.Second`**
安全设置：读取请求头最多等 10 秒。防止慢速攻击（有人故意很慢地发送请求头来占用连接）。`10 * time.Second` 是 Go 的时长写法。

```go
    ctx, stop := signal.NotifyContext(                             // ⑮
        context.Background(),                                      // ⑯
        syscall.SIGINT,                                            // ⑰
        syscall.SIGTERM,                                           // ⑱
    )
    defer stop()                                                   // ⑲
```
**⑮ `signal.NotifyContext`**
创建一个特殊的 context：当收到指定信号时，这个 context 会被"取消"。函数返回两个值：
- `ctx` — 一个 context，可以用来检查是否收到了信号
- `stop` — 一个清理函数，调用后停止监听信号

**Go 支持多返回值**——一个函数可以返回多个结果，调用时用 `:=` 同时接收。

**⑯ `context.Background()`**
创建一个"根 context"——它是所有 context 的祖先，永远不会被取消。是 context 链的起点。

**⑰ `syscall.SIGINT`** — 信号量 2，即 Ctrl+C。
**⑱ `syscall.SIGTERM`** — 信号量 15，即 `kill <pid>` 或 Docker 停止容器。

**⑲ `defer stop()`**
`defer` 关键字：**在函数返回前执行**。不管函数怎么结束（正常 return、panic），defer 的代码都会执行。常用于资源清理。这里确保信号监听器最终被释放。

```go
    go func() {                                                    // ⑳
        slog.Info("user service starting", "port", port)           // ㉑
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed { // ㉒
            slog.Error("server error", "error", err)
            os.Exit(1)                                             // ㉓
        }
    }()                                                            // ㉔
```
**⑳ `go func() { ... }()`**
`go` 关键字启动一个 **goroutine**（Go 的轻量级线程）。`func() { ... }()` 是立刻执行的匿名函数。

**为什么用 goroutine？** 因为 `ListenAndServe()` 是**阻塞的**——它会一直等待请求，不会返回。如果直接调用，后面的代码（监听 Ctrl+C）就永远执行不到了。放到 goroutine 里，主 goroutine 和服务器 goroutine 就能**并行运行**。

**㉑ `slog.Info("...", "port", port)`**
输出 INFO 级别日志，`"port"` 是键，`port` 变量是值。输出类似：
```
2026/03/15 INFO user service starting port=3001
```

**㉒ `err != http.ErrServerClosed`**
`ListenAndServe` 在正常关停时会返回 `ErrServerClosed` 错误。这不是真的错误，是正常行为，所以要排除它。

**㉓ `os.Exit(1)`** — 立刻退出程序，返回码 1 表示"出错了"（0 表示成功）。

**㉔ `}()`** — 匿名函数定义结束，`()` 立刻调用它。

```go
    <-ctx.Done()                                                   // ㉕
```
**㉕ 等待停止信号**
`ctx.Done()` 返回一个 **channel**（Go 的线程间通信管道）。`<-` 是从 channel 接收数据的操作符。

这行代码会**阻塞在这里**，直到 context 被取消（即收到 Ctrl+C 或 SIGTERM）。收到信号后，代码继续往下执行。

```go
    slog.Info("shutting down user service")
    shutdownCtx, cancel := context.WithTimeout(                    // ㉖
        context.Background(), 10*time.Second,
    )
    defer cancel()                                                 // ㉗
    srv.Shutdown(shutdownCtx)                                      // ㉘
```
**㉖ `context.WithTimeout`**
创建一个带超时的 context：10 秒后自动取消。用于限制关停时间——不能等太久。

**㉗ `defer cancel()`** — 函数结束时释放定时器资源。

**㉘ `srv.Shutdown(shutdownCtx)`**
**优雅关停**：停止接受新连接，等待已有请求处理完，最多等 10 秒。如果 10 秒还没完成，强制关闭。

```go
func healthHandler(w http.ResponseWriter, r *http.Request) error {
    return response.Success(w, r, map[string]string{               // ㉙
        "service": "user",
        "status":  "ok",
    })
}
```
**㉙ `map[string]string{...}`**
Go 的 **map**（映射/字典）：键是 string，值也是 string。`{"service": "user", "status": "ok"}` 是初始化语法。

传给 `response.Success` 后，会被 JSON 编码成：
```json
{"service": "user", "status": "ok"}
```

```go
func envOr(key, fallback string) string {                          // ㉚
    if v := os.Getenv(key); v != "" {                              // ㉛
        return v
    }
    return fallback
}
```
**㉚ `key, fallback string`** — 两个参数同类型，合并写法。
**㉛ `os.Getenv(key)`** — 读取系统环境变量。如果不存在返回空字符串 `""`。

---

## 6. cmd/monolith/main.go

单体入口和微服务入口的区别：**它把所有服务的路由挂到一个进程里**。

大部分代码和 user/main.go 相同，只解释不同的部分：

```go
    r.Route("/api/v1/user", func(r chi.Router) {                   // ①
        r.Post("/health", handler.Wrap(serviceHealth("user")))      // ②
    })
```
**① `r.Route("/api/v1/user", func(r chi.Router) { ... })`**
chi 的**路由分组**。在 `/api/v1/user` 前缀下注册子路由。里面的 `r` 是一个新的子路由器（参数名和外层 `r` 相同，这叫**变量遮蔽**——内层的 `r` 覆盖了外层的 `r`，但只在这个函数体内有效）。

**② `handler.Wrap(serviceHealth("user"))`**
`serviceHealth("user")` 返回一个函数（闭包），然后 Wrap 再包装一次。

```go
func serviceHealth(name string) handler.AppHandler {               // ③
    return func(w http.ResponseWriter, r *http.Request) error {    // ④
        return response.Success(w, r, map[string]string{
            "service": name,                                       // ⑤
            "status":  "ok",
        })
    }
}
```
**③ 返回值是 `handler.AppHandler`**
这个函数返回一个函数——"生产函数的函数"，也叫**高阶函数**。

**④ 返回的匿名函数是闭包**
它"记住了"外层传入的 `name` 参数。

**⑤ `"service": name`**
每次调用 `serviceHealth("user")`/`serviceHealth("product")` 会生成不同的 handler，`name` 值分别是 `"user"`、`"product"`。这样不用为每个服务写一个几乎一样的 handler 函数。

---

## 7. cmd/migrate/main.go

最简单的一个入口，Phase 0 只是占位。

```go
    direction := flag.String("direction", "up", "migration direction: up or down") // ①
    flag.Parse()                                                   // ②
```
**① `flag.String(...)`**
Go 标准库的命令行参数解析。三个参数：
- `"direction"` — 参数名（用法：`-direction up`）
- `"up"` — 默认值
- `"migration direction..."` — 帮助文本

返回值是 `*string`（字符串指针），不是 string。

**② `flag.Parse()`**
实际解析命令行参数。必须在访问参数值之前调用。

```go
    slog.Info("migrate tool", "direction", *direction)             // ③
```
**③ `*direction`**
`*` 是**解引用**操作——从指针取出实际值。因为 `flag.String` 返回的是指针 `*string`，我们需要 `*direction` 来获取它指向的字符串值。

```go
    fmt.Fprintf(os.Stderr, "migrate %s: no migrations configured yet\n", *direction) // ④
```
**④ `fmt.Fprintf(os.Stderr, ...)`**
- `fmt.Fprintf` — 往指定输出流写格式化字符串
- `os.Stderr` — 标准错误输出（区别于 `os.Stdout` 标准输出）。错误信息习惯写到 stderr
- `%s` — 字符串占位符
- `\n` — 换行符
