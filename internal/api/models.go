package api

// Meta holds pagination metadata returned by list endpoints.
type Meta struct {
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

// Envelope is a generic response wrapper for paginated list endpoints.
type Envelope[T any] struct {
	Data T    `json:"data"`
	Meta Meta `json:"meta"`
}

// Tenant represents a PublicTenantResponse from GET /tenants.
type Tenant struct {
	TenantCode string `json:"tenant_code"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	JoinedAt   string `json:"joined_at"`
}

// Member represents a PublicMemberResponse from GET /members.
type Member struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	Email       string  `json:"email"`
	Role        string  `json:"role"`
	AvatarURL   *string `json:"avatar_url"`
}

// Board represents a PublicBoardResponse from GET /mission/boards.
type Board struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsPublic    bool    `json:"is_public"`
	CreatedAt   string  `json:"created_at"`
}

// BoardList represents a list column inside a board detail.
type BoardList struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

// BoardDetail represents a PublicBoardDetailResponse from GET /mission/boards/{id}.
type BoardDetail struct {
	Board
	Lists []BoardList `json:"lists"`
}

// TaskUser is a lightweight user reference embedded in Task.
type TaskUser struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// Task represents a PublicTaskResponse from GET /mission/tasks and related endpoints.
type Task struct {
	ID           string    `json:"id"`
	Code         string    `json:"code"`
	Title        string    `json:"title"`
	Description  *string   `json:"description"`
	Status       string    `json:"status"`
	Priority     *string   `json:"priority"`
	Assignee     *TaskUser `json:"assignee"`
	Owner        *TaskUser `json:"owner"`
	BoardID      *string   `json:"board_id"`
	BoardListID  *string   `json:"board_list_id"`
	DueDate      *string   `json:"due_date"`
	ParentTaskID *string   `json:"parent_task_id"`
	HasSubtasks  bool      `json:"has_subtasks"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
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

// TaskComment represents a PublicTaskCommentResponse from
// GET /mission/tasks/{id}/comments. It is a read-only projection of a thread
// message: either a human "comment" or a system "activity" event.
type TaskComment struct {
	ID          string              `json:"id"`
	Author      CommentAuthor       `json:"author"`
	Kind        string              `json:"kind"` // comment | activity | card | artifact
	Content     *string             `json:"content"`
	UIData      map[string]any      `json:"ui_data"`
	Attachments []CommentAttachment `json:"attachments"`
	ParentID    *string             `json:"parent_id"`
	CreatedAt   string              `json:"created_at"`
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

// ProductVariantDimensions holds physical dimensions of a variant.
type ProductVariantDimensions struct {
	L *float64 `json:"l"`
	W *float64 `json:"w"`
	H *float64 `json:"h"`
}

// ProductVariant represents a PublicProductVariantResponse.
type ProductVariant struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	SKU            *string                   `json:"sku"`
	Barcode        *string                   `json:"barcode"`
	Price          *float64                  `json:"price"`
	CompareAtPrice *float64                  `json:"compare_at_price"`
	Currency       string                    `json:"currency"`
	Weight         *float64                  `json:"weight"`
	Dimensions     *ProductVariantDimensions `json:"dimensions"`
	Option1        *string                   `json:"option1"`
	Option2        *string                   `json:"option2"`
	Option3        *string                   `json:"option3"`
	VariantType    string                    `json:"variant_type"`
	CreatedAt      string                    `json:"created_at"`
	UpdatedAt      string                    `json:"updated_at"`
}

// ProductRef is a lightweight reference object (brand, category, product_type, unit).
type ProductRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ProductCategory extends ProductRef with optional parent info.
type ProductCategory struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	ParentID *string     `json:"parent_id"`
	Parent   *ProductRef `json:"parent"`
}

// ProductOption describes a variant option axis (e.g. Color, Size).
type ProductOption struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Position int      `json:"position"`
	Values   []string `json:"values"`
}

// Product represents a PublicProductResponse from GET /pcms/products.
type Product struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Slug        string           `json:"slug"`
	Description *string          `json:"description"`
	Status      string           `json:"status"`
	Currency    string           `json:"currency"`
	Aliases     []string         `json:"aliases"`
	IsDeleted   bool             `json:"is_deleted"`
	CreatedAt   string           `json:"created_at"`
	UpdatedAt   string           `json:"updated_at"`
	Brand       *ProductRef      `json:"brand"`
	Category    *ProductCategory `json:"category"`
	ProductType *ProductRef      `json:"product_type"`
	Unit        *ProductRef      `json:"unit"`
	Options     []ProductOption  `json:"options"`
	Variants    []ProductVariant `json:"variants"`
}

// Health is the response from GET /health — a preflight that confirms
// connectivity and that the API key is accepted.
type Health struct {
	OK        bool   `json:"ok"`
	Timestamp string `json:"timestamp"`
}

// Me represents the authenticated user from GET /me.
type Me struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	Email       string  `json:"email"`
	AvatarURL   *string `json:"avatar_url"`
}

// CreateProductOptionItem is one option axis in POST /pcms/products.
// Name is the axis label (e.g. "Color"); Values are the allowed values (e.g. ["Red","Blue"]).
type CreateProductOptionItem struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// CreateProductVariantItem is one variant entry in POST /pcms/products.
// Option1/2/3 reference the values of the corresponding option axes.
type CreateProductVariantItem struct {
	Name    string   `json:"name"`
	SKU     *string  `json:"sku,omitempty"`
	Barcode *string  `json:"barcode,omitempty"`
	Price   *float64 `json:"price,omitempty"`
	Option1 *string  `json:"option1,omitempty"`
	Option2 *string  `json:"option2,omitempty"`
	Option3 *string  `json:"option3,omitempty"`
}

// CreateProductRequest is the body for POST /pcms/products.
// Only Name is required. When Options are provided, Variants must also be provided;
// the backend does not auto-generate the Cartesian matrix.
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
}

// UpsertVariantItem is one element of the PUT /pcms/products/{id}/variants array.
// VariantID present -> UPDATE the existing variant.
// VariantID absent  -> CREATE a new variant (Name is required in that case).
type UpsertVariantItem struct {
	VariantID *string  `json:"variant_id,omitempty"`
	Name      *string  `json:"name,omitempty"`
	SKU       *string  `json:"sku,omitempty"`
	Barcode   *string  `json:"barcode,omitempty"`
	Price     *float64 `json:"price,omitempty"`
	Option1   *string  `json:"option1,omitempty"`
	Option2   *string  `json:"option2,omitempty"`
	Option3   *string  `json:"option3,omitempty"`
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

// Brand represents a PublicBrandResponse from GET /pcms/brands.
type Brand struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	LogoURL *string `json:"logo_url"`
}

// Category represents a PublicCategoryResponse from GET /pcms/categories.
type Category struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
}

// ProductType represents a PublicProductTypeResponse from GET /pcms/product-types.
type ProductType struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// Unit represents a PublicUnitResponse from GET /pcms/units.
type Unit struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
}

// VariantRecord represents a flat variant entry from GET /pcms/variants.
// Named VariantRecord to avoid collision with the existing ProductVariant type.
type VariantRecord struct {
	ID        string  `json:"id"`
	Barcode   *string `json:"barcode"`
	SKU       *string `json:"sku"`
	Name      string  `json:"name"`
	ProductID string  `json:"product_id"`
}

// ListBrandsResponse is an Envelope for paginated brand lists.
type ListBrandsResponse = Envelope[[]Brand]

// ListCategoriesResponse is an Envelope for paginated category lists.
type ListCategoriesResponse = Envelope[[]Category]

// ListProductTypesResponse is an Envelope for paginated product-type lists.
type ListProductTypesResponse = Envelope[[]ProductType]

// ListUnitsResponse is an Envelope for paginated unit lists.
type ListUnitsResponse = Envelope[[]Unit]

// ListVariantRecordsResponse is an Envelope for paginated variant-record lists.
type ListVariantRecordsResponse = Envelope[[]VariantRecord]
