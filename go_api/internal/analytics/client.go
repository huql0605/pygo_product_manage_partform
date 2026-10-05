package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/huql0605/go_api/internal/common/requestid"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	// 把上游请求的 RequestID 透传给 Python，实现跨语言日志串联。
	if requestID := requestid.FromContext(ctx); requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call python : %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("python returned %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

func (c *Client) Sales(ctx context.Context, days int) (map[string]any, error) {
	q := url.Values{}
	q.Set("days", fmt.Sprintf("%d", days))

	var out map[string]any
	if err := c.get(ctx, "/analytics/sales", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) TopProducts(ctx context.Context, limit int) ([]map[string]any, error) {
	q := url.Values{}
	q.Set("limit", fmt.Sprintf("%d", limit))

	var out []map[string]any
	if err := c.get(ctx, "/analytics/top-products", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Brands(ctx context.Context, limit int) ([]map[string]any, error) {
	q := url.Values{}
	q.Set("limit", fmt.Sprintf("%d", limit))
	var out []map[string]any
	if err := c.get(ctx, "/analytics/brands", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) MonthlyTrend(ctx context.Context, productor string) ([]map[string]any, error) {
	q := url.Values{}
	if productor != "" {
		q.Set("productor", productor)
	}
	var out []map[string]any
	if err := c.get(ctx, "/analytics/monthly-trend", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}
