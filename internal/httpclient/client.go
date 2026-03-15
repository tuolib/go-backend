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

// Client is an HTTP client for inter-service communication.
// It automatically injects traceId and internal auth headers.
type Client struct {
	httpClient     *http.Client
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
	defer resp.Body.Close()

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
