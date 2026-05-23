package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vtech-com/capigo-api-sdk/internal/version"
)

// Response wraps the raw HTTP response.
type Response struct {
	StatusCode int
	Body       []byte
	RequestID  string
}

// Client is an HTTP client for the Capigo Public API.
type Client struct {
	http    *http.Client
	baseURL string
	apiKey  string
}

// NewClient creates a Client. baseURL must use HTTPS unless host is localhost or 127.0.0.1.
func NewClient(baseURL, apiKey string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	host := u.Hostname()
	isLocal := host == "localhost" || host == "127.0.0.1"
	if u.Scheme != "https" && !isLocal {
		return nil, fmt.Errorf("base URL must use HTTPS (got %q)", baseURL)
	}
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
	}, nil
}

// RedactedAuthorization returns the Authorization header value with the key redacted.
func RedactedAuthorization(apiKey string) string {
	if len(apiKey) <= 8 {
		return "Bearer [REDACTED]"
	}
	return "Bearer " + apiKey[:4] + "...[REDACTED]"
}

// Do executes an HTTP request against path. body is JSON-encoded when non-nil.
// tenant is the X-Tenant-Code value; when nil the header is omitted entirely.
func (c *Client) Do(ctx context.Context, method, path string, body any, tenant *string) (*Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("X-Request-Id", uuid.New().String())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tenant != nil {
		req.Header.Set("X-Tenant-Code", *tenant)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, fmt.Errorf("close response body: %w", err)
	}

	r := &Response{
		StatusCode: resp.StatusCode,
		Body:       rawBody,
		RequestID:  resp.Header.Get("X-Request-Id"),
	}

	if resp.StatusCode >= 400 {
		apiErr := parseAPIError(rawBody, resp.StatusCode, r.RequestID)
		return r, apiErr
	}

	return r, nil
}

// parseAPIError attempts to extract the structured error from the response body.
func parseAPIError(body []byte, status int, requestID string) *APIError {
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Code != "" {
		rid := envelope.Error.RequestID
		if rid == "" {
			rid = requestID
		}
		return &APIError{
			Code:       envelope.Error.Code,
			Message:    envelope.Error.Message,
			RequestID:  rid,
			HTTPStatus: status,
		}
	}
	return &APIError{
		Code:       http.StatusText(status),
		Message:    string(body),
		RequestID:  requestID,
		HTTPStatus: status,
	}
}
