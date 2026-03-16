package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// contextKey 自定义的 context key 类型，避免和其他包的 key 冲突。
// contextKey is a custom context key type to avoid collisions with keys from other packages.
type contextKey string

const traceIDKey contextKey = "traceId"

// Client 用于微服务间 HTTP 通信，自动注入 traceId 和内部认证头。
// Client handles inter-service HTTP communication, automatically injecting traceId and internal auth headers.
//
// 为什么封装而不是直接用 http.Client？三个原因：
// Why wrap instead of using http.Client directly? Three reasons:
//
// 1. 自动传递 traceId，实现全链路追踪
//    Auto-propagate traceId for end-to-end request tracing
// 2. 自动注入 X-Internal-Secret，内部服务间鉴权
//    Auto-inject X-Internal-Secret for service-to-service authentication
// 3. 统一超时和错误处理，避免每个调用点重复代码
//    Centralized timeout and error handling, avoiding boilerplate at each call site
type Client struct {
	httpClient     *http.Client // 复用同一个 Client 实例，内部维护了连接池，不要每次请求都创建新的 / Reuse a single Client instance — it maintains a connection pool internally; don't create a new one per request
	internalSecret string       // 内部服务鉴权密钥 / Internal service auth secret
}

// New 创建 Client，传入内部服务间通信的共享密钥。
// New creates a Client with the given internal secret for service-to-service auth.
func New(internalSecret string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // 全局超时：防止上游服务无响应时阻塞过久 / Global timeout: prevents blocking too long when upstream is unresponsive
		},
		internalSecret: internalSecret,
	}
}

// Post 发送 POST 请求（JSON body），自动注入 traceId 和内部认证头。
// Post sends a POST request with JSON body, auto-injecting traceId and internal auth headers.
//
// body: 请求体（会被 json.Marshal 序列化），nil 表示无请求体。
// body: request payload (will be json.Marshal'd), nil means no body.
//
// result: 响应会被 json.Decode 到这个指针，nil 表示不关心响应体。
// result: response will be json.Decoded into this pointer, nil means ignore response body.
func (c *Client) Post(ctx context.Context, url string, body any, result any) error {
	// 序列化请求体 / Marshal request body
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			// fmt.Errorf("...%w", err) 用 %w 包装原始错误，让上层可以用 errors.Is/As 解包。
			// fmt.Errorf with %w wraps the original error, so callers can unwrap it with errors.Is/As.
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data) // bytes.NewReader 比 bytes.NewBuffer 更高效（只读场景）/ bytes.NewReader is more efficient than bytes.NewBuffer for read-only use
	}

	// NewRequestWithContext 创建请求并绑定 context，超时和取消信号通过 context 传播。
	// NewRequestWithContext creates a request bound to the context; timeout and cancellation signals propagate through it.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 从 context 中提取 traceId，传播到下游服务，实现全链路追踪。
	// Extract traceId from context and propagate to downstream services for end-to-end tracing.
	if traceID, ok := ctx.Value(traceIDKey).(string); ok && traceID != "" {
		req.Header.Set("X-Trace-Id", traceID)
	}

	// 注入内部鉴权密钥，下游服务通过校验此 header 确认请求来自可信的内部服务。
	// Inject internal auth secret; downstream services verify this header to confirm the request comes from a trusted internal service.
	if c.internalSecret != "" {
		req.Header.Set("X-Internal-Secret", c.internalSecret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	// 必须关闭响应体，否则 TCP 连接无法回到连接池，导致连接泄露。
	// Must close the response body, otherwise the TCP connection can't return to the pool — causing a connection leak.
	defer resp.Body.Close()

	// 4xx/5xx 视为上游错误，读取响应体用于错误日志。
	// 4xx/5xx are treated as upstream errors; read the body for error logging.
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upstream error %d: %s", resp.StatusCode, string(respBody))
	}

	// 将响应体反序列化到 result 指针。
	// Deserialize response body into the result pointer.
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
