package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var productCmd = &cobra.Command{
	Use:   "products",
	Short: "Manage PCMS products",
	Long: `Manage products in the Capigo Product Catalog Management System (PCMS).

All product commands require a tenant to be resolved (via --tenant, CAPIGO_TENANT,
or config default_tenant). Use --help on any subcommand for flag details.`,
}

// --------------------------------------------------------------------------
// products list
// --------------------------------------------------------------------------

var (
	productListUpdatedSince string
	productListQuery        string
	productListPage         int
	productListLimit        int
	productListAll          bool
	productListIDs          string
)

var productsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List products (supports delta sync via --updated-since)",
	Long: `List products from the PCMS catalog.

Supports delta sync: pass --updated-since with an ISO 8601 timestamp returned
in the X-Server-Time header from a previous call. The server timestamp is
always printed to stderr when available regardless of output mode.

Use --ids to fetch specific products by UUID (comma-separated, max 50).
--ids and --updated-since may be combined. --ids and --all are mutually exclusive.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		if productListIDs != "" && productListAll {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "--ids and --all are mutually exclusive; use --ids to fetch specific products or --all to paginate the full catalog",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		if productListQuery != "" && len(productListQuery) < 2 {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "--query must be at least 2 characters",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant, isGlobal := resolveTenant(profile)

		// /pcms/* requires a tenant — reject early per §5.3 rule #2.
		_ = api.PCMSRequiresTenant
		if isGlobal {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "products commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		if productListAll {
			if productListPage > 0 {
				fmt.Fprintf(os.Stderr, "Warning: --page is ignored when --all is set; fetching all pages.\n")
			}
			return productsListAll(ctx, client, tenant)
		}

		params := url.Values{}
		if productListQuery != "" {
			params.Set("q", productListQuery)
		}
		if productListUpdatedSince != "" {
			params.Set("updated_since", productListUpdatedSince)
		}
		if productListIDs != "" {
			params.Set("ids", productListIDs)
		}
		if productListPage > 0 {
			params.Set("page", strconv.Itoa(productListPage))
		}
		if productListLimit > 0 {
			params.Set("limit", strconv.Itoa(productListLimit))
		}

		path := "/pcms/products"
		if len(params) > 0 {
			path += "?" + params.Encode()
		}

		resp, err := client.Do(ctx, "GET", path, nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Product]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		// M2: always emit X-Server-Time to stderr regardless of output mode.
		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s (use as --updated-since for next delta sync)\n", resp.ServerTime)
		}

		if outputMode == "json" {
			// C1 + M1: render full api.Product with meta envelope in JSON mode.
			return renderProductListJSON(os.Stdout, envelope.Data, envelope.Meta)
		}

		items := make([]output.Product, len(envelope.Data))
		for i, p := range envelope.Data {
			items[i] = toOutputProduct(p)
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "product",
		}); err != nil {
			return handleErr(err)
		}

		if outputMode == "table" {
			if envelope.Meta.HasMore {
				fmt.Fprintf(os.Stderr, "Showing %d of %d. Use --page / --limit to paginate, or --all.\n",
					len(envelope.Data), envelope.Meta.Total)
			}
		}

		return nil
	},
}

// productsListAll auto-paginates until has_more is false.
func productsListAll(ctx context.Context, client *api.Client, tenant *string) error {
	page := 1
	var allProducts []api.Product
	var lastMeta api.Meta
	var lastServerTime string

	for {
		params := url.Values{}
		if productListQuery != "" {
			params.Set("q", productListQuery)
		}
		if productListUpdatedSince != "" {
			params.Set("updated_since", productListUpdatedSince)
		}
		params.Set("page", strconv.Itoa(page))
		if productListLimit > 0 {
			params.Set("limit", strconv.Itoa(productListLimit))
		}

		path := "/pcms/products?" + params.Encode()
		resp, err := client.Do(ctx, "GET", path, nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Product]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if resp.ServerTime != "" {
			lastServerTime = resp.ServerTime
		}

		allProducts = append(allProducts, envelope.Data...)
		lastMeta = envelope.Meta

		if !envelope.Meta.HasMore {
			break
		}
		page++
	}

	// M2: always emit X-Server-Time to stderr.
	if lastServerTime != "" {
		fmt.Fprintf(os.Stderr, "Server time: %s (use as --updated-since for next delta sync)\n", lastServerTime)
	}

	if outputMode == "json" {
		// C1 + M1: render full api.Product with synthetic meta for --all.
		syntheticMeta := api.Meta{
			Page:    1,
			Limit:   lastMeta.Limit,
			Total:   len(allProducts),
			HasMore: false,
		}
		return renderProductListJSON(os.Stdout, allProducts, syntheticMeta)
	}

	items := make([]output.Product, len(allProducts))
	for i, p := range allProducts {
		items[i] = toOutputProduct(p)
	}

	if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
		GlobalMode:   false,
		ResourceKind: "product",
	}); err != nil {
		return handleErr(err)
	}

	return nil
}

// --------------------------------------------------------------------------
// products create
// --------------------------------------------------------------------------

var (
	productCreateName          string
	productCreateDescription   string
	productCreateStatus        string
	productCreateCurrency      string
	productCreateSKU           string
	productCreateBarcode       string
	productCreatePrice         float64
	productCreateBrandID       string
	productCreateCategoryID    string
	productCreateProductTypeID string
	productCreateUnitID        string
	productCreateFromJSON      string
)

var productsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new product",
	Long: `Create a new product in PCMS.

Simple mode (single variant): provide --name and optional individual flags.

  capigo products create --name "Blue T-Shirt" --sku "SKU-001" --price 299000

Variant mode (options + variants): provide --from-json <file> with the full
request body as JSON (use - to read from stdin).

  capigo products create --from-json product.json

Simple mode JSON (no options):

  {
    "name": "Blue T-Shirt",
    "sku": "SKU-001",
    "price": 299000,
    "status": "DRAFT"
  }

Variant mode JSON (options + variants required together):

  {
    "name": "T-Shirt",
    "options": [
      {"name": "Color", "values": ["Blue", "Red"]},
      {"name": "Size",  "values": ["S", "M", "L"]}
    ],
    "variants": [
      {"name": "Blue / S", "sku": "SKU-BS", "option1": "Blue", "option2": "S", "price": 299000},
      {"name": "Blue / M", "sku": "SKU-BM", "option1": "Blue", "option2": "M", "price": 299000},
      {"name": "Red / S",  "sku": "SKU-RS", "option1": "Red",  "option2": "S", "price": 279000}
    ]
  }

Rule: if options is provided, variants is required. The backend does not
auto-generate the Cartesian matrix from options alone.

When --from-json is provided, all other flags are ignored.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant, isGlobal := resolveTenant(profile)

		// /pcms/* requires a tenant.
		if isGlobal {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "products commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		var body any
		if productCreateFromJSON != "" {
			raw, err := readJSONInput(productCreateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if productCreateName == "" {
				e := &api.APIError{Code: "VALIDATION_ERROR", Message: "--name is required", HTTPStatus: 400}
				output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
				os.Exit(api.ExitCodeFor(e))
			}

			req := api.CreateProductRequest{
				Name: productCreateName,
			}
			if productCreateDescription != "" {
				req.Description = &productCreateDescription
			}
			if productCreateStatus != "" {
				req.Status = &productCreateStatus
			}
			if productCreateCurrency != "" {
				req.Currency = &productCreateCurrency
			}
			if productCreateSKU != "" {
				req.SKU = &productCreateSKU
			}
			if productCreateBarcode != "" {
				req.Barcode = &productCreateBarcode
			}
			if cmd.Flags().Changed("price") {
				req.Price = &productCreatePrice
			}
			if productCreateBrandID != "" {
				req.BrandID = &productCreateBrandID
			}
			if productCreateCategoryID != "" {
				req.CategoryID = &productCreateCategoryID
			}
			if productCreateProductTypeID != "" {
				req.ProductTypeID = &productCreateProductTypeID
			}
			if productCreateUnitID != "" {
				req.UnitID = &productCreateUnitID
			}
			body = req
		}

		resp, err := client.Do(ctx, "POST", "/pcms/products", body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Product `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		// M2: emit X-Server-Time to stderr.
		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
		}

		if outputMode == "json" {
			// C1: render full api.Product in JSON mode.
			return renderProductJSON(os.Stdout, envelope.Data)
		}

		if err := output.Render(os.Stdout, outputMode, toOutputProduct(envelope.Data), output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "product",
		}); err != nil {
			return handleErr(err)
		}

		return nil
	},
}

// --------------------------------------------------------------------------
// products update
// --------------------------------------------------------------------------

var (
	productUpdateName          string
	productUpdateDescription   string
	productUpdateStatus        string
	productUpdateCurrency      string
	productUpdateBrandID       string
	productUpdateCategoryID    string
	productUpdateProductTypeID string
	productUpdateUnitID        string
	productUpdateAliases       []string
	productUpdateFromJSON      string
)

var productsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a product's core details",
	Long: `Update the core details of an existing product.

All fields are optional; at least one must be provided. Fields not specified
are left unchanged on the server.

Use --from-json to supply the full update body as JSON (file path or - for stdin).
When --from-json is set, all individual field flags are ignored.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		productUpdateID := args[0]

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant, isGlobal := resolveTenant(profile)

		if isGlobal {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "products commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		// M3: --from-json support for update.
		var body any
		if productUpdateFromJSON != "" {
			// Check that no individual field flags were also set.
			individualFlags := []string{"name", "description", "status", "currency",
				"brand-id", "category-id", "product-type-id", "unit-id", "aliases"}
			for _, f := range individualFlags {
				if cmd.Flags().Changed(f) {
					e := &api.APIError{
						Code:       "VALIDATION_ERROR",
						Message:    fmt.Sprintf("--from-json and --%s are mutually exclusive; use one or the other", f),
						HTTPStatus: 400,
					}
					output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
					os.Exit(api.ExitCodeFor(e))
				}
			}
			raw, err := readJSONInput(productUpdateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			req := api.UpdateProductRequest{}
			fieldCount := 0

			if productUpdateName != "" {
				req.Name = &productUpdateName
				fieldCount++
			}
			if productUpdateDescription != "" {
				req.Description = &productUpdateDescription
				fieldCount++
			}
			if productUpdateStatus != "" {
				req.Status = &productUpdateStatus
				fieldCount++
			}
			if productUpdateCurrency != "" {
				req.Currency = &productUpdateCurrency
				fieldCount++
			}
			if productUpdateBrandID != "" {
				req.BrandID = &productUpdateBrandID
				fieldCount++
			}
			if productUpdateCategoryID != "" {
				req.CategoryID = &productUpdateCategoryID
				fieldCount++
			}
			if productUpdateProductTypeID != "" {
				req.ProductTypeID = &productUpdateProductTypeID
				fieldCount++
			}
			if productUpdateUnitID != "" {
				req.UnitID = &productUpdateUnitID
				fieldCount++
			}
			if len(productUpdateAliases) > 0 {
				req.Aliases = productUpdateAliases
				fieldCount++
			}

			if fieldCount == 0 {
				e := &api.APIError{
					Code:       "VALIDATION_ERROR",
					Message:    "at least one field must be provided for update",
					HTTPStatus: 400,
				}
				output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
				os.Exit(api.ExitCodeFor(e))
			}
			body = req
		}

		resp, err := client.Do(ctx, "PUT", "/pcms/products/"+productUpdateID, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Product `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		// M2: emit X-Server-Time to stderr.
		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
		}

		if outputMode == "json" {
			// C1: render full api.Product in JSON mode.
			return renderProductJSON(os.Stdout, envelope.Data)
		}

		if err := output.Render(os.Stdout, outputMode, toOutputProduct(envelope.Data), output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "product",
		}); err != nil {
			return handleErr(err)
		}

		return nil
	},
}

// --------------------------------------------------------------------------
// products variants
// --------------------------------------------------------------------------

var (
	productVariantsProductID string
	productVariantsFromJSON  string
)

var productsVariantsCmd = &cobra.Command{
	Use:   "variants",
	Short: "Upsert product variants (create or update)",
	Long: `Create or update variants of a product in a single call.

The --from-json flag accepts a path to a JSON file containing an array of
variant objects, or - to read from stdin. Items with variant_id are updated;
items without variant_id are created (name is required for new variants).

Maximum 50 items per call.

Example JSON input:
  [
    {"variant_id": "uuid", "price": 99000},
    {"name": "Blue / L", "sku": "SKU-BL", "option1": "Blue", "option2": "L"}
  ]`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		if productVariantsProductID == "" {
			e := &api.APIError{Code: "VALIDATION_ERROR", Message: "--product-id is required", HTTPStatus: 400}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}
		if productVariantsFromJSON == "" {
			e := &api.APIError{Code: "VALIDATION_ERROR", Message: "--from-json is required", HTTPStatus: 400}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant, isGlobal := resolveTenant(profile)

		if isGlobal {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "products commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		raw, err := readJSONInput(productVariantsFromJSON)
		if err != nil {
			return handleErr(fmt.Errorf("read --from-json: %w", err))
		}

		// Validate it is a JSON array before sending.
		var items []api.UpsertVariantItem
		if err := json.Unmarshal(raw, &items); err != nil {
			return handleErr(fmt.Errorf("--from-json must be a JSON array of variant objects: %w", err))
		}

		resp, err := client.Do(ctx, "PUT",
			"/pcms/products/"+productVariantsProductID+"/variants",
			items, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Product `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		// M2: emit X-Server-Time to stderr.
		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
		}

		if outputMode == "json" {
			// C1: render full api.Product in JSON mode.
			return renderProductJSON(os.Stdout, envelope.Data)
		}

		if err := output.Render(os.Stdout, outputMode, toOutputProduct(envelope.Data), output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "product",
		}); err != nil {
			return handleErr(err)
		}

		return nil
	},
}

// --------------------------------------------------------------------------
// init — register flags and subcommands
// --------------------------------------------------------------------------

func init() {
	// products list flags
	productsListCmd.Flags().StringVarP(&productListQuery, "query", "q", "", "free-text search (min 2 chars): matches product name, variant name, SKU, and barcode")
	productsListCmd.Flags().StringVar(&productListUpdatedSince, "updated-since", "", "ISO 8601 timestamp for delta sync (from previous X-Server-Time header)")
	productsListCmd.Flags().IntVar(&productListPage, "page", 0, "page number (default 1)")
	productsListCmd.Flags().IntVar(&productListLimit, "limit", 0, "items per page (1–100, default 20)")
	productsListCmd.Flags().BoolVar(&productListAll, "all", false, "fetch all pages automatically (loads all pages into memory before output; for large catalogs prefer paginating manually)")
	productsListCmd.Flags().StringVar(&productListIDs, "ids", "", "comma-separated list of product UUIDs to fetch (max 50); mutually exclusive with --all")

	// products create flags
	productsCreateCmd.Flags().StringVar(&productCreateName, "name", "", "product name (required unless --from-json is used)")
	productsCreateCmd.Flags().StringVar(&productCreateDescription, "description", "", "product description")
	productsCreateCmd.Flags().StringVar(&productCreateStatus, "status", "", "product status: DRAFT, ACTIVE, or ARCHIVED (default DRAFT)")
	productsCreateCmd.Flags().StringVar(&productCreateCurrency, "currency", "", "currency code (default VND)")
	productsCreateCmd.Flags().StringVar(&productCreateSKU, "sku", "", "default variant SKU")
	productsCreateCmd.Flags().StringVar(&productCreateBarcode, "barcode", "", "default variant barcode")
	productsCreateCmd.Flags().Float64Var(&productCreatePrice, "price", 0, "default variant price (use --price=0 to explicitly set zero)")
	productsCreateCmd.Flags().StringVar(&productCreateBrandID, "brand-id", "", "brand UUID")
	productsCreateCmd.Flags().StringVar(&productCreateCategoryID, "category-id", "", "category UUID")
	productsCreateCmd.Flags().StringVar(&productCreateProductTypeID, "product-type-id", "", "product type UUID")
	productsCreateCmd.Flags().StringVar(&productCreateUnitID, "unit-id", "", "unit UUID")
	productsCreateCmd.Flags().StringVar(&productCreateFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin)")

	// products update flags
	productsUpdateCmd.Flags().StringVar(&productUpdateName, "name", "", "new product name")
	productsUpdateCmd.Flags().StringVar(&productUpdateDescription, "description", "", "new product description")
	productsUpdateCmd.Flags().StringVar(&productUpdateStatus, "status", "", "new status: DRAFT, ACTIVE, or ARCHIVED")
	productsUpdateCmd.Flags().StringVar(&productUpdateCurrency, "currency", "", "new currency code")
	productsUpdateCmd.Flags().StringVar(&productUpdateBrandID, "brand-id", "", "new brand UUID")
	productsUpdateCmd.Flags().StringVar(&productUpdateCategoryID, "category-id", "", "new category UUID")
	productsUpdateCmd.Flags().StringVar(&productUpdateProductTypeID, "product-type-id", "", "new product type UUID")
	productsUpdateCmd.Flags().StringVar(&productUpdateUnitID, "unit-id", "", "new unit UUID")
	productsUpdateCmd.Flags().StringArrayVar(&productUpdateAliases, "aliases", nil, "product aliases (repeatable: --aliases foo --aliases bar)")
	productsUpdateCmd.Flags().StringVar(&productUpdateFromJSON, "from-json", "", "path to JSON file with update body (use - for stdin); mutually exclusive with individual field flags")

	// products variants flags
	productsVariantsCmd.Flags().StringVar(&productVariantsProductID, "product-id", "", "product UUID (required)")
	productsVariantsCmd.Flags().StringVar(&productVariantsFromJSON, "from-json", "", "path to JSON array file (use - for stdin) (required)")

	productCmd.AddCommand(productsListCmd, productsCreateCmd, productsUpdateCmd, productsVariantsCmd)
	rootCmd.AddCommand(productCmd)
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

// toOutputProduct converts an api.Product to the display model.
// SKU and Price are derived from the first non-nil variant fields.
// VariantCount holds the total number of variants.
func toOutputProduct(p api.Product) output.Product {
	sku := ""
	price := ""
	if len(p.Variants) > 0 {
		v := p.Variants[0]
		if v.SKU != nil {
			sku = *v.SKU
		}
		if v.Price != nil {
			price = strconv.FormatFloat(*v.Price, 'f', -1, 64)
		}
	}
	return output.Product{
		ID:           p.ID,
		Name:         p.Name,
		Status:       p.Status,
		SKU:          sku,
		Price:        price,
		VariantCount: len(p.Variants),
	}
}

// renderProductJSON marshals a single api.Product as a JSON object to w.
func renderProductJSON(w io.Writer, p api.Product) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(p)
}

// renderProductListJSON marshals a list of api.Product with pagination meta
// into a JSON envelope: {"data":[...],"meta":{...}}.
// When the list is empty the envelope still includes meta so callers can
// distinguish an empty catalog from an error.
func renderProductListJSON(w io.Writer, products []api.Product, meta api.Meta) error {
	if products == nil {
		products = []api.Product{}
	}
	envelope := struct {
		Data []api.Product `json:"data"`
		Meta api.Meta      `json:"meta"`
	}{
		Data: products,
		Meta: meta,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

// readJSONInput reads raw bytes from a file path or from stdin when path is "-".
func readJSONInput(path string) ([]byte, error) {
	if path == "-" {
		return readStdin()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	return data, nil
}

// readStdin reads all bytes from os.Stdin.
func readStdin() ([]byte, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	return data, nil
}
