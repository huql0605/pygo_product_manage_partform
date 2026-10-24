package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

var ErrUpstreamNotFound = errors.New("upstream 404")

type Filter struct {
	Category   string
	Productor  string
	StartMonth string
	EndMonth   string
}

func (f Filter) apply(q url.Values) {
	if f.Category != "" {
		q.Set("category", f.Category)
	}
	if f.Productor != "" {
		q.Set("productor", f.Productor)
	}
	if f.StartMonth != "" {
		q.Set("start_month", f.StartMonth)
	}
	if f.EndMonth != "" {
		q.Set("end_month", f.EndMonth)
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

	if resp.StatusCode == http.StatusNotFound {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("%w: %s", ErrUpstreamNotFound, strings.TrimSpace(string(body)))
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("python returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

func (c *Client) Sales(ctx context.Context, f Filter) (map[string]any, error) {
	q := url.Values{}
	f.apply(q)
	var out map[string]any
	if err := c.get(ctx, "/analytics/sales", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) TopProducts(ctx context.Context, limit int, f Filter) ([]map[string]any, error) {
	q := url.Values{}
	f.apply(q)
	q.Set("limit", strconv.Itoa(limit))

	var out []map[string]any
	if err := c.get(ctx, "/analytics/top-products", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Brands(ctx context.Context, limit int, f Filter) ([]map[string]any, error) {
	q := url.Values{}
	f.apply(q)
	q.Set("limit", strconv.Itoa(limit))

	var out []map[string]any
	if err := c.get(ctx, "/analytics/brands", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) MonthlyTrend(ctx context.Context, f Filter) ([]map[string]any, error) {
	q := url.Values{}
	f.apply(q)

	var out []map[string]any
	if err := c.get(ctx, "/analytics/monthly-trend", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Attributes 按任意已登记属性做分组聚合。走 sales 明细而不是 dws_sales，
// 属于探索口径，不与确认口径混用。
func (c *Client) Attributes(ctx context.Context, key string, limit int, f Filter) (map[string]any, error) {
	q := url.Values{}
	f.apply(q)
	q.Set("key", key)
	q.Set("limit", strconv.Itoa(limit))

	var out map[string]any
	if err := c.get(ctx, "/analytics/attributes", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}
