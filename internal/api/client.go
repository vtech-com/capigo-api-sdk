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
		ServerTime: resp.Header.Get("X-Server-Time"),
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

// CreateBrand creates a new brand for the given tenant.
func (c *Client) CreateBrand(ctx context.Context, tenant *string, body CreateBrandRequest) (*Response, error) {
	return c.Do(ctx, "POST", "/pcms/brands", body, tenant)
}

// UpdateBrand updates an existing brand by ID.
func (c *Client) UpdateBrand(ctx context.Context, tenant *string, id string, body UpdateBrandRequest) (*Response, error) {
	return c.Do(ctx, "PUT", "/pcms/brands/"+id, body, tenant)
}

// CreateCategory creates a new category for the given tenant.
func (c *Client) CreateCategory(ctx context.Context, tenant *string, body CreateCategoryRequest) (*Response, error) {
	return c.Do(ctx, "POST", "/pcms/categories", body, tenant)
}

// UpdateCategory updates an existing category by ID.
func (c *Client) UpdateCategory(ctx context.Context, tenant *string, id string, body UpdateCategoryRequest) (*Response, error) {
	return c.Do(ctx, "PUT", "/pcms/categories/"+id, body, tenant)
}

// CreateProductType creates a new product type for the given tenant.
func (c *Client) CreateProductType(ctx context.Context, tenant *string, body CreateProductTypeRequest) (*Response, error) {
	return c.Do(ctx, "POST", "/pcms/product-types", body, tenant)
}

// UpdateProductType updates an existing product type by ID.
func (c *Client) UpdateProductType(ctx context.Context, tenant *string, id string, body UpdateProductTypeRequest) (*Response, error) {
	return c.Do(ctx, "PUT", "/pcms/product-types/"+id, body, tenant)
}

// CreateUnit creates a new unit for the given tenant.
func (c *Client) CreateUnit(ctx context.Context, tenant *string, body CreateUnitRequest) (*Response, error) {
	return c.Do(ctx, "POST", "/pcms/units", body, tenant)
}

// UpdateUnit updates an existing unit by ID.
func (c *Client) UpdateUnit(ctx context.Context, tenant *string, id string, body UpdateUnitRequest) (*Response, error) {
	return c.Do(ctx, "PUT", "/pcms/units/"+id, body, tenant)
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
