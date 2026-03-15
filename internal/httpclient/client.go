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

type contextKey string

const traceIDKey contextKey = "traceId"

// Client 用于微服务间 HTTP 通信，自动注入 traceId 和内部认证头。
// 为什么封装而不是直接用 http.Client？三个原因：
// 1. 自动传递 traceId，实现全链路追踪
// 2. 自动注入 X-Internal-Secret，内部服务间鉴权
// 3. 统一超时和错误处理，避免每个调用点重复代码
type Client struct {
	httpClient     *http.Client // 复用同一个 Client 实例，内部维护了连接池，不要每次请求都创建新的
	internalSecret string
}

// New creates a Client with the given internal secret for service-to-service auth.
func New(internalSecret string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		internalSecret: internalSecret,
	}
}

// Post sends a POST request with JSON body to the given URL.
// It injects X-Trace-Id and X-Internal-Secret headers from context.
func (c *Client) Post(ctx context.Context, url string, body any, result any) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Propagate trace ID from context
	if traceID, ok := ctx.Value(traceIDKey).(string); ok && traceID != "" {
		req.Header.Set("X-Trace-Id", traceID)
	}

	// Inject internal auth secret
	if c.internalSecret != "" {
		req.Header.Set("X-Internal-Secret", c.internalSecret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close() // 必须关闭响应体，否则 TCP 连接无法回到连接池，导致连接泄露

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upstream error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
