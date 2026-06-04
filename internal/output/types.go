package output

// Tenant is the display model for a Capigo tenant.
type Tenant struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Member is the display model for a Capigo workspace member.
type Member struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	TenantCode string `json:"tenant_code,omitempty"`
}

// Board is the display model for a Capigo mission board.
type Board struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	TenantCode string `json:"tenant_code,omitempty"`
}

// BoardDetail is the display model for a single board with its lists.
type BoardDetail struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ListCount int    `json:"list_count"`
}

// Task is the display model for a Capigo mission task.
type Task struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Assignee   string `json:"assignee,omitempty"`
	TenantCode string `json:"tenant_code,omitempty"`
}

// Product is the display model for a Capigo PCMS product.
// SKU and Price are derived from the first variant for table display.
// VariantCount holds the total number of variants for the Variants column.
type Product struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	SKU          string `json:"sku,omitempty"`
	Price        string `json:"price,omitempty"`
	VariantCount int    `json:"variant_count"`
	TenantCode   string `json:"tenant_code,omitempty"`
}

// Brand is the display model for a Capigo PCMS brand.
type Brand struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	LogoURL *string `json:"logo_url"`
}

// Category is the display model for a Capigo PCMS category.
type Category struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
}

// ProductType is the display model for a Capigo PCMS product type.
type ProductType struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// Unit is the display model for a Capigo PCMS unit.
type Unit struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
}

// VariantRecord is the display model for a Capigo PCMS variant record.
type VariantRecord struct {
	ID        string `json:"id"`
	Barcode   string `json:"barcode"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	ProductID string `json:"product_id"`
}
