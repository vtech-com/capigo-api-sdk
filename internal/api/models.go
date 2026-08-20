package api

import "encoding/json"

// RawEnvelope keeps the API's `data` exactly as it arrived.
//
// The CLI is a transport, not a second definition of the platform's schemas.
// Decoding a response into a Go struct and marshalling it again silently drops
// every field the struct does not declare, and invents every field it declares
// that the API did not send. Both happened: `tasks get` discarded `parent` and
// emitted a `parent_task_id` no response ever carried, and `variants list`
// truncated nineteen fields to five, so `manufacturer_code` was visible through
// `variants get` and absent through `variants list`.
//
// Nothing announced either. A caller cannot see a field that was never printed.
//
// So commands decode into this, print Data verbatim, and reach for a typed view
// only where they need to *read* something — an id, a page cursor — never to
// decide what the caller may see.
type RawEnvelope struct {
	Data json.RawMessage `json:"data"`
	Meta json.RawMessage `json:"meta"`
}

// CommentAuthor is the resolved author of a task comment / activity entry.
// Name follows the server's resolution chain (member display_name → agent name →
// "System"); the email is never exposed.
type CommentAuthor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "user" | "agent"
}

// CommentAttachment is flat attachment metadata on a task comment.
type CommentAttachment struct {
	ID        string `json:"id"`
	FileName  string `json:"file_name"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
}

// AttachmentDownload is the response of both attachment download endpoints:
// GET /mission/tasks/{id}/attachments/{attachmentId}/download and
// GET /mission/tasks/{id}/comments/attachments/{attachmentId}/download.
// URL is a short-lived (5 minute) signed URL — fetch it immediately; do not
// persist or hand it off, and never assume you can retry against a URL from
// a previous command invocation.
type AttachmentDownload struct {
	URL       string `json:"url"`
	FileName  string `json:"file_name"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	ExpiresAt string `json:"expires_at"`
}

// CreateTaskRequest is the body for POST /mission/tasks.
// tenant_code is required and is sent as a body field (not a header).
type CreateTaskRequest struct {
	TenantCode  string   `json:"tenant_code"`
	Title       string   `json:"title"`
	Description *string  `json:"description,omitempty"`
	AssigneeID  *string  `json:"assignee_id,omitempty"`
	BoardID     *string  `json:"board_id,omitempty"`
	BoardListID *string  `json:"board_list_id,omitempty"`
	FollowerIDs []string `json:"follower_ids,omitempty"`
	Priority    *string  `json:"priority,omitempty"`
	Status      *string  `json:"status,omitempty"`
	DueDate     *string  `json:"due_date,omitempty"`
}

// SubtaskItem is one entry in a batch subtask create (max 25 per request).
// Only Title is required. DueDate is a calendar date (YYYY-MM-DD), not a datetime.
type SubtaskItem struct {
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	AssigneeID  *string `json:"assignee_id,omitempty"`
	DueDate     *string `json:"due_date,omitempty"`
	Priority    *string `json:"priority,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// CreateSubtasksRequest is the body for POST /mission/tasks/{id}/subtasks.
// tenant_code is a required body field. Validation is all-or-nothing: if any
// subtask is invalid, nothing is created.
type CreateSubtasksRequest struct {
	TenantCode string        `json:"tenant_code"`
	Subtasks   []SubtaskItem `json:"subtasks"`
}

// CreateTaskWithSubtasksTask is the parent-task portion of a with-subtasks
// create. It mirrors CreateTaskRequest minus tenant_code (which lives at the
// envelope level), plus assigned_agent_key (assign the parent to an AI agent;
// cannot be combined with assignee_id).
type CreateTaskWithSubtasksTask struct {
	Title            string   `json:"title"`
	Description      *string  `json:"description,omitempty"`
	AssigneeID       *string  `json:"assignee_id,omitempty"`
	AssignedAgentKey *string  `json:"assigned_agent_key,omitempty"`
	DueDate          *string  `json:"due_date,omitempty"`
	BoardID          *string  `json:"board_id,omitempty"`
	BoardListID      *string  `json:"board_list_id,omitempty"`
	FollowerIDs      []string `json:"follower_ids,omitempty"`
	Priority         *string  `json:"priority,omitempty"`
	Status           *string  `json:"status,omitempty"`
}

// CreateTaskWithSubtasksRequest is the body for POST /mission/tasks/with-subtasks.
// Creates a parent task and its subtasks atomically (all-or-nothing).
type CreateTaskWithSubtasksRequest struct {
	TenantCode string                     `json:"tenant_code"`
	Task       CreateTaskWithSubtasksTask `json:"task"`
	Subtasks   []SubtaskItem              `json:"subtasks"`
}

// ProductVariantDimensions holds physical dimensions of a variant.
type ProductVariantDimensions struct {
	L *float64 `json:"l"`
	W *float64 `json:"w"`
	H *float64 `json:"h"`
}

// ProductVariant represents a PublicProductVariantResponse.
type ProductVariant struct {
	ID               string                    `json:"id"`
	Name             string                    `json:"name"`
	SKU              *string                   `json:"sku"`
	Barcode          *string                   `json:"barcode"`
	Price            *float64                  `json:"price"`
	CompareAtPrice   *float64                  `json:"compare_at_price"`
	Currency         string                    `json:"currency"`
	Weight           *float64                  `json:"weight"`
	Dimensions       *ProductVariantDimensions `json:"dimensions"`
	Option1          *string                   `json:"option1"`
	Option2          *string                   `json:"option2"`
	Option3          *string                   `json:"option3"`
	VariantType      string                    `json:"variant_type"`
	ManufacturerCode *string                   `json:"manufacturer_code"`
	LegacyCode       *string                   `json:"legacy_code"`
	ExtraData        map[string]any            `json:"extra_data"`
	CreatedAt        string                    `json:"created_at"`
	UpdatedAt        string                    `json:"updated_at"`
}

// CreateProductOptionItem is one option axis in POST /pcms/products.
// Name is the axis label (e.g. "Color"); Values are the allowed values (e.g. ["Red","Blue"]).
type CreateProductOptionItem struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// CreateProductVariantItem is one variant entry in POST /pcms/products.
// Option1/2 reference the values of the corresponding option axes. There is no
// Option3: the API caps a product at two option axes and rejects a variant that
// carries any key not listed here.
type CreateProductVariantItem struct {
	Name    string   `json:"name"`
	SKU     *string  `json:"sku,omitempty"`
	Barcode *string  `json:"barcode,omitempty"`
	Price   *float64 `json:"price,omitempty"`
	Option1 *string  `json:"option1,omitempty"`
	Option2 *string  `json:"option2,omitempty"`
}

// CreateProductRequest is the body for POST /pcms/products.
// Name and UnitID are required. When Options are provided, Variants must
// also be provided; the backend does not auto-generate the Cartesian matrix.
type CreateProductRequest struct {
	Name          string                     `json:"name"`
	Description   *string                    `json:"description,omitempty"`
	BrandID       *string                    `json:"brand_id,omitempty"`
	CategoryID    *string                    `json:"category_id,omitempty"`
	ProductTypeID *string                    `json:"product_type_id,omitempty"`
	UnitID        *string                    `json:"unit_id,omitempty"`
	Status        *string                    `json:"status,omitempty"`
	Currency      *string                    `json:"currency,omitempty"`
	SKU           *string                    `json:"sku,omitempty"`
	Barcode       *string                    `json:"barcode,omitempty"`
	Price         *float64                   `json:"price,omitempty"`
	Aliases       []string                   `json:"aliases,omitempty"`
	Tags          []string                   `json:"tags,omitempty"`
	Options       []CreateProductOptionItem  `json:"options,omitempty"`
	Variants      []CreateProductVariantItem `json:"variants,omitempty"`
}

// UpdateProductRequest is the body for PUT /pcms/products/{id}.
// All fields are optional pointers; only non-nil fields are serialized.
// At least one field must be provided.
type UpdateProductRequest struct {
	Name          *string  `json:"name,omitempty"`
	Description   *string  `json:"description,omitempty"`
	BrandID       *string  `json:"brand_id,omitempty"`
	CategoryID    *string  `json:"category_id,omitempty"`
	ProductTypeID *string  `json:"product_type_id,omitempty"`
	UnitID        *string  `json:"unit_id,omitempty"`
	Status        *string  `json:"status,omitempty"`
	Currency      *string  `json:"currency,omitempty"`
	Aliases       []string `json:"aliases,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

// UpsertVariantItem is one element of the PUT /pcms/products/{id}/variants array.
// VariantID present -> UPDATE the existing variant.
// VariantID absent  -> CREATE a new variant (Name is required in that case).
// These are all the keys the API accepts on a variant: it rejects the request
// when an item carries any other key, Option3 included.
type UpsertVariantItem struct {
	VariantID        *string        `json:"variant_id,omitempty"`
	Name             *string        `json:"name,omitempty"`
	SKU              *string        `json:"sku,omitempty"`
	Barcode          *string        `json:"barcode,omitempty"`
	Price            *float64       `json:"price,omitempty"`
	Option1          *string        `json:"option1,omitempty"`
	Option2          *string        `json:"option2,omitempty"`
	ManufacturerCode *string        `json:"manufacturer_code,omitempty"`
	LegacyCode       *string        `json:"legacy_code,omitempty"`
	Status           *string        `json:"status,omitempty"`
	ExtraData        map[string]any `json:"extra_data,omitempty"`
}

// CreateBrandRequest is the body for POST /pcms/brands.
type CreateBrandRequest struct {
	Name    string  `json:"name"`
	LogoURL *string `json:"logo_url,omitempty"`
}

// UpdateBrandRequest is the body for PATCH /pcms/brands/{id}.
// All fields are optional; at least one must be provided.
type UpdateBrandRequest struct {
	Name    *string `json:"name,omitempty"`
	LogoURL *string `json:"logo_url,omitempty"`
}

// ReplaceBrandRequest is the body for PUT /pcms/brands/{id}.
// All fields are required; the server enforces this with Zod.
// LogoURL must always be sent (use nil to set null).
type ReplaceBrandRequest struct {
	Name    string  `json:"name"`
	LogoURL *string `json:"logo_url"` // no omitempty — null must be sent
}

// CreateCategoryRequest is the body for POST /pcms/categories.
type CreateCategoryRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id,omitempty"`
}

// UpdateCategoryRequest is the body for PATCH /pcms/categories/{id}.
// All fields are optional; at least one must be provided.
// Passing parent_id: null promotes the category to root.
type UpdateCategoryRequest struct {
	Name     *string `json:"name,omitempty"`
	ParentID *string `json:"parent_id,omitempty"`
}

// ReplaceCategoryRequest is the body for PUT /pcms/categories/{id}.
// All fields are required; the server enforces this with Zod.
// ParentID must always be sent (use nil/null to promote to root).
type ReplaceCategoryRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"` // no omitempty — null must be sent
}

// CreateProductTypeRequest is the body for POST /pcms/product-types.
type CreateProductTypeRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// UpdateProductTypeRequest is the body for PATCH /pcms/product-types/{id}.
// All fields are optional; at least one must be provided.
type UpdateProductTypeRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ReplaceProductTypeRequest is the body for PUT /pcms/product-types/{id}.
// All fields are required; the server enforces this with Zod.
// Description must always be sent (use nil to set null).
type ReplaceProductTypeRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"` // no omitempty — null must be sent
}

// CreateUnitRequest is the body for POST /pcms/units.
// Both Name and Abbreviation are required.
type CreateUnitRequest struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
}

// UpdateUnitRequest is the body for PATCH /pcms/units/{id}.
// All fields are optional; at least one must be provided.
type UpdateUnitRequest struct {
	Name         *string `json:"name,omitempty"`
	Abbreviation *string `json:"abbreviation,omitempty"`
}

// ReplaceUnitRequest is the body for PUT /pcms/units/{id}.
// All fields are required; the server enforces this with Zod.
type ReplaceUnitRequest struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
}
