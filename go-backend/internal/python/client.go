package python

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// TODO: Implement the Python Service HTTP client.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}

	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type Error struct {
	StatusCode int
	Body       string
}

func (e *Error) Error() string {
	return fmt.Sprintf("python service error: HTTP %d - %s", e.StatusCode, e.Body)
}

func (c *Client) doJSON(ctx context.Context, method, path string, input any, output any) error {
	var reqBody io.Reader

	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return err
	}

	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Error{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	if output == nil || len(respBody) == 0 {
		return nil
	}

	return json.Unmarshal(respBody, output)
}

func (c *Client) Health(ctx context.Context) (HealthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var out HealthResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/health", nil, &out)
	return out, err
}

func (c *Client) Capabilities(ctx context.Context) (CapabilitiesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var out CapabilitiesResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/health/capabilities", nil, &out)
	return out, err
}

func (c *Client) CountTokens(ctx context.Context, req TokenCountRequest) (TokenCountResponse, error) {
	var out TokenCountResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/tokenizer/count", req, &out)
	return out, err
}
