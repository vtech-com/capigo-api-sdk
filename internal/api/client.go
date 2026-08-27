package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	// ServerTime holds the X-Server-Time response header value (ISO 8601).
	// Populated on GET /pcms/products responses; empty when absent.
	// Use as updated_since for the next delta sync call.
	ServerTime string
}

// Client is an HTTP client for the Capigo Public API.
type Client struct {
	http    *http.Client
	baseURL string
	apiKey  string
	// verboseW, when non-nil, receives a redacted trace of every request and
	// response. Enabled via EnableVerbose (the --verbose flag).
	verboseW io.Writer
}

// EnableVerbose turns on request/response tracing to w. The Authorization
// header is always redacted. Pass nil to disable.
func (c *Client) EnableVerbose(w io.Writer) {
	c.verboseW = w
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

	if c.verboseW != nil {
		_, _ = fmt.Fprintf(c.verboseW, "> %s %s\n", method, c.baseURL+path)
		_, _ = fmt.Fprintf(c.verboseW, "> Authorization: %s\n", RedactedAuthorization(c.apiKey))
		if tenant != nil {
			_, _ = fmt.Fprintf(c.verboseW, "> X-Tenant-Code: %s\n", *tenant)
		}
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
		ServerTime: resp.Header.Get("X-Server-Time"),
	}

	if c.verboseW != nil {
		_, _ = fmt.Fprintf(c.verboseW, "< HTTP %d\n", resp.StatusCode)
		if len(rawBody) > 0 {
			_, _ = fmt.Fprintf(c.verboseW, "< %s\n", string(rawBody))
		}
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
			RawBody:    body,
		}
	}
	return &APIError{
		Code:       http.StatusText(status),
		Message:    string(body),
		RequestID:  requestID,
		HTTPStatus: status,
		RawBody:    body,
	}
}

// ListBrands fetches a page of brands for the given tenant.
// q is an optional name-contains filter; pass empty string to skip.
func (c *Client) ListBrands(ctx context.Context, tenant *string, q string, page, limit int) (*Response, error) {
	params := buildParams(map[string]string{"q": q}, page, limit)
	path := "/pcms/brands"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return c.Do(ctx, "GET", path, nil, tenant)
}

// ListCategories fetches a page of categories for the given tenant.
// q is an optional name-contains filter; pass empty string to skip.
func (c *Client) ListCategories(ctx context.Context, tenant *string, q string, page, limit int) (*Response, error) {
	params := buildParams(map[string]string{"q": q}, page, limit)
	path := "/pcms/categories"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return c.Do(ctx, "GET", path, nil, tenant)
}

// ListProductTypes fetches a page of product types for the given tenant.
// q is an optional name-contains filter; pass empty string to skip.
func (c *Client) ListProductTypes(ctx context.Context, tenant *string, q string, page, limit int) (*Response, error) {
	params := buildParams(map[string]string{"q": q}, page, limit)
	path := "/pcms/product-types"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return c.Do(ctx, "GET", path, nil, tenant)
}

// ListUnits fetches a page of units for the given tenant.
// q is an optional name-contains filter; pass empty string to skip.
func (c *Client) ListUnits(ctx context.Context, tenant *string, q string, page, limit int) (*Response, error) {
	params := buildParams(map[string]string{"q": q}, page, limit)
	path := "/pcms/units"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return c.Do(ctx, "GET", path, nil, tenant)
}

// ListVariants fetches a page of variant records for the given tenant.
// barcodePrefix and sort are optional; pass empty strings to skip.
func (c *Client) ListVariants(ctx context.Context, tenant *string, barcodePrefix, sort string, page, limit int) (*Response, error) {
	params := buildParams(map[string]string{
		"barcode_prefix": barcodePrefix,
		"sort":           sort,
	}, page, limit)
	path := "/pcms/variants"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return c.Do(ctx, "GET", path, nil, tenant)
}

// CreateBoard creates a board for the given tenant. body is the JSON request body.
func (c *Client) CreateBoard(ctx context.Context, body any, tenant *string) (*Response, error) {
	return c.Do(ctx, "POST", "/mission/boards", body, tenant)
}

// UpdateBoard updates a board by id.
func (c *Client) UpdateBoard(ctx context.Context, boardID string, body any, tenant *string) (*Response, error) {
	return c.Do(ctx, "PATCH", "/mission/boards/"+boardID, body, tenant)
}

// CreateBoardList creates a list under a board.
func (c *Client) CreateBoardList(ctx context.Context, boardID string, body any, tenant *string) (*Response, error) {
	return c.Do(ctx, "POST", "/mission/boards/"+boardID+"/lists", body, tenant)
}

// UpdateBoardList updates a list by id.
func (c *Client) UpdateBoardList(ctx context.Context, boardID, listID string, body any, tenant *string) (*Response, error) {
	return c.Do(ctx, "PATCH", "/mission/boards/"+boardID+"/lists/"+listID, body, tenant)
}

// buildParams constructs a url.Values from the given string map (skipping empty values)
// and appends page/limit when non-zero.
func buildParams(extras map[string]string, page, limit int) url.Values {
	params := url.Values{}
	for k, v := range extras {
		if v != "" {
			params.Set(k, v)
		}
	}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	return params
}
