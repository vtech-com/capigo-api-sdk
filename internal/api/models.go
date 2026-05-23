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

// Me represents the authenticated user from GET /me.
type Me struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	Email       string  `json:"email"`
	AvatarURL   *string `json:"avatar_url"`
}
