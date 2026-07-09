package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var productCmd = &cobra.Command{
	Use:   "products",
	Short: "Manage PCMS products",
	Long: `Manage products in the Capigo Product Catalog Management System (PCMS).

Every products command requires a tenant.
  capigo help tenancy`,
}

// --------------------------------------------------------------------------
// products list
// --------------------------------------------------------------------------

var (
	productListTenant       string
	productListUpdatedSince string
	productListQuery        string
	productListPage         int
	productListLimit        int
	productListAll          bool
	productListIDs          string
)

var productsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List products (search, paginate, delta-sync, fetch by id)",
	// push: TODO — a --query that returns zero rows because the term is longer
	// than the stored value reads to a caller like "no such product". The CAVEATS
	// note below only helps a caller who reads it; the trap needs a zero-row hint
	// on stdout before this page can be called a mitigation.
	Long: `List products from the PCMS catalog.

PURPOSE
  Find products and read them in bulk. Look a product up by a human key — name,
  alias, product code, sku, barcode — with --query, then act on the returned id.
  A single-item read is: capigo products get <uuid>

INPUT
  (flags only; no request body)
  --tenant <code>        required
  -q, --query <term>     free-text search, 2-500 chars. Matches product name,
                         aliases, tags, variant name, sku, and barcode.
  --ids <uuid,uuid>      fetch specific products, max 50. Mutually exclusive
                         with --all; may be combined with --updated-since.
  --updated-since <ts>   ISO 8601 timestamp for delta sync. Feed back the
                         server time reported by a previous call.
  --all                  auto-paginate. Loads every page into memory before
                         printing; for a large catalogue prefer paging manually.
  --page <n>             page number (0 = server default)
  --limit <n>            items per page, 1-100 (default 20)

OUTPUT
  -o json emits the list envelope. Beyond the meta fields carried by every
  list, this command reports:

      meta.server_time   feed to --updated-since on the next call
      meta.complete      only with --all. false means pagination aborted and
                         the result is incomplete. The command also exits
                         non-zero, and the rows already fetched are printed.
      meta.missing_ids   only with --ids. Requested UUIDs the server did not
                         return — deleted, or in another tenant. The exit code
                         is still 0.

  In table mode the same id reconciliation appears as a line:

      Requested 5 ids · 3 found · missing: <uuid>, <uuid>

  Table columns include Aliases and Tags.

  The envelope, meta.total, list footers, and which stream carries the server
  timestamp: capigo help output

CAVEATS
  --query is a substring match: the stored value must contain your term. A term
  longer than the stored value finds nothing — a stored alias "VVD013" is not
  matched by the query "SLM-DS-VVD013". Search the shortest distinctive
  fragment, or use --all and filter locally.

  Soft-deleted products still appear here: capigo help soft-delete

EXAMPLES
  # Find a product by a code fragment and take its id
  capigo products list --tenant acme -q VVD013 -o json | jq -r '.data[0].id'

  # How many products are there? Read meta.total; do not pull the catalogue
  capigo products list --tenant acme --limit 1 -o json | jq '.meta.total'

  # Full export, and check that it completed
  capigo products list --tenant acme --all -o json | jq '.meta.complete'

  # Everything changed since the last run
  capigo products list --tenant acme --updated-since 2026-07-01T00:00:00Z -o json

SEE ALSO
  products get <id>       one product in full
  products variants       create or update variants on a product
  variants list           query variants by barcode prefix
  capigo help output      output modes and the JSON contract
  capigo help tenancy     how --tenant resolves
  capigo help exit-codes  what a non-zero exit means`,
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

		if productListIDs != "" {
			idParts := strings.Split(productListIDs, ",")
			if len(idParts) > 50 {
				e := &api.APIError{
					Code:       "VALIDATION_ERROR",
					Message:    fmt.Sprintf("--ids accepts at most 50 UUIDs (got %d); split into multiple requests", len(idParts)),
					HTTPStatus: 400,
				}
				output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
				os.Exit(api.ExitCodeFor(e))
			}
		}

		validatePCMSLimit(productListLimit)

		if productListQuery != "" && utf8.RuneCountInString(productListQuery) < 2 {
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

		tenant := resolveTenant(productListTenant, profile)

		// /pcms/* requires a tenant — reject early per §5.3 rule #2.
		if tenant == nil {
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

		// Delta-sync cursor: stdout in table mode, stderr otherwise.
		emitServerTime(resp.ServerTime, serverTimeHint)

		// --ids: surface any requested IDs the server did not return — a
		// clean exit 0 with fewer rows than requested must not read as
		// "the missing products are fine".
		missing, requested := missingProductIDs(productListIDs, envelope.Data)

		if outputMode == "json" {
			// C1 + M1: render full api.Product with meta envelope in JSON mode.
			return output.WriteJSONList(os.Stdout, envelope.Data, listMetaExtras{
				Meta:       envelope.Meta,
				ServerTime: resp.ServerTime,
				MissingIDs: missing,
			})
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
			output.WriteListSummary(os.Stdout, output.ListSummary{
				Tenant:     derefTenant(tenant),
				TenantNote: tenantNote(tenant, productListTenant),
				Shown:      len(envelope.Data),
				Page:       envelope.Meta.Page,
				Limit:      envelope.Meta.Limit,
				Total:      envelope.Meta.Total,
				HasMore:    envelope.Meta.HasMore,
				HintAll:    true,
			})
			if len(missing) > 0 {
				_, _ = fmt.Fprintf(os.Stdout, "Requested %d ids · %d found · missing: %s\n",
					requested, len(envelope.Data), strings.Join(missing, ", "))
			}
		}

		return nil
	},
}

// productsListAll auto-paginates until has_more is false. A mid-pagination
// failure does not discard what was already fetched: the partial result is
// still rendered, marked INCOMPLETE (table footer) / "complete": false (JSON
// meta), and the command still exits with the underlying error's code —
// empty stdout must never masquerade as an empty catalogue.
func productsListAll(ctx context.Context, client *api.Client, tenant *string) error {
	page := 1
	var allProducts []api.Product
	var lastMeta api.Meta
	var lastServerTime string
	var fetchErr error

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
		if err == nil {
			var envelope api.Envelope[[]api.Product]
			if uerr := json.Unmarshal(resp.Body, &envelope); uerr != nil {
				err = fmt.Errorf("decode response: %w", uerr)
			} else {
				if resp.ServerTime != "" {
					lastServerTime = resp.ServerTime
				}
				allProducts = append(allProducts, envelope.Data...)
				lastMeta = envelope.Meta
				if !envelope.Meta.HasMore {
					break
				}
				page++
				continue
			}
		}
		// A failed page: with nothing fetched yet this is a plain error;
		// with rows already in hand, render the partial set first.
		if len(allProducts) == 0 {
			return handleErr(err)
		}
		fetchErr = err
		break
	}

	emitServerTime(lastServerTime, serverTimeHint)

	complete := fetchErr == nil
	total := len(allProducts)
	if !complete && lastMeta.Total > total {
		total = lastMeta.Total
	}

	if outputMode == "json" {
		// C1 + M1: render full api.Product with synthetic meta for --all;
		// complete reports whether pagination reached the last page.
		if err := output.WriteJSONList(os.Stdout, allProducts, allMeta{
			Meta: api.Meta{
				Page:    1,
				Limit:   lastMeta.Limit,
				Total:   total,
				HasMore: !complete,
			},
			ServerTime: lastServerTime,
			Complete:   complete,
		}); err != nil {
			return handleErr(err)
		}
		if fetchErr != nil {
			return handleErr(fetchErr)
		}
		return nil
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

	if outputMode == "table" {
		s := output.ListSummary{
			Tenant:     derefTenant(tenant),
			TenantNote: tenantNote(tenant, productListTenant),
			Shown:      len(allProducts),
			Page:       1,
			Limit:      len(allProducts),
			Total:      total,
			HasMore:    !complete,
		}
		if !complete {
			s.Incomplete = fmt.Sprintf("aborted at page %d — results are PARTIAL", page)
		}
		output.WriteListSummary(os.Stdout, s)
	}

	if fetchErr != nil {
		return handleErr(fetchErr)
	}
	return nil
}

// --------------------------------------------------------------------------
// products get
// --------------------------------------------------------------------------

var productGetTenant string

var productsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a product by ID",
	Long: `Get one product by UUID.

PURPOSE
  Read a single product in full: its variants, options, brand, category, product
  type and unit. This command addresses a product by UUID only. To find that
  UUID from a name, alias, product code, sku or barcode, use
  products list --query first.

INPUT
  <id>              product UUID (positional, required)
  --tenant <code>   required

OUTPUT
  -o json emits the bare product object — the same shape as an item in
  products list .data[]:

      { id, name, slug, description, status, currency, aliases[], tags[],
        is_deleted, created_at, updated_at, brand, category, product_type,
        unit, options[], variants[] }

  options[] carries the product's option names in order. That order is what
  option1, option2 and option3 refer to when writing variants.

  Exit 4 when no such product exists in the resolved tenant.

  Output modes and the JSON contract: capigo help output

CAVEATS
  A soft-deleted product is returned like any other. Its status may still read
  ACTIVE; is_deleted is the field that tells you.
      capigo help soft-delete

EXAMPLES
  capigo products get 8f2a-... --tenant acme

  # The option order to use when writing variants
  capigo products get 8f2a-... --tenant acme -o json | jq '.options'

SEE ALSO
  products list           find a product by name, alias, sku or barcode
  products variants       create or update variants on this product
  capigo help exit-codes  what a non-zero exit means`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(productGetTenant, profile)

		// /pcms/* requires a tenant.
		if tenant == nil {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "products commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.Do(ctx, "GET", "/pcms/products/"+args[0], nil, tenant)
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
		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
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
// products create
// --------------------------------------------------------------------------

var (
	productCreateTenant        string
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
	productCreateAliases       []string
	productCreateTags          []string
	productCreateFromJSON      string
)

var productsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new product",
	// push: TODO — a duplicate alias or barcode is accepted silently. The CAVEATS
	// note below reaches only a caller who reads it; a collision notice on stdout
	// at write time is what would make this page a mitigation rather than a warning.
	Long: `Create a product, optionally with its options and variants in one call.

PURPOSE
  Add a product to the catalogue. A product with no options carries one default
  variant, built from --sku, --barcode and --price.

INPUT
  --tenant <code>          required

  Simple mode — flags:
    --name                 required, unless --from-json is used
    --sku --barcode --price --currency --description
    --status               DRAFT, ACTIVE, or ARCHIVED (default DRAFT)
    --brand-id --category-id --product-type-id --unit-id
    --aliases              repeatable; alternative names and product codes
    --tags                 repeatable; free-form labels

  Both aliases and tags are matched by products list --query.

  Body mode — --from-json <path|->, where - reads stdin. When --from-json is
  given the individual field flags are ignored. (products update differs —
  there, passing both exits 5.)

  Simple body:

      { "name": "Blue T-Shirt", "sku": "SKU-001", "price": 299000,
        "status": "DRAFT", "aliases": ["BT-001"], "tags": ["summer"] }

  Body with options and variants. If options is present, variants is REQUIRED:
  the backend does not generate the cartesian matrix from options alone.
  option1..option3 are positional and line up with options[] in order:

      { "name": "T-Shirt",
        "options": [ {"name": "Color", "values": ["Blue", "Red"]},
                     {"name": "Size",  "values": ["S", "M"]} ],
        "variants": [
          {"name": "Blue / S", "sku": "SKU-BS", "option1": "Blue", "option2": "S", "price": 299000},
          {"name": "Red / M",  "sku": "SKU-RM", "option1": "Red",  "option2": "M", "price": 279000}
        ] }

OUTPUT
  -o json emits the bare created product — the same shape as products get.
  quiet prints its id.

  Output modes and the JSON contract: capigo help output

CAVEATS
  A variant sku is unique per tenant: a duplicate exits 8. An alias and a
  barcode are NOT enforced unique — the server accepts duplicates of either
  without error.

EXAMPLES
  capigo products create --tenant acme --name "Blue T-Shirt" --sku SKU-001 --price 299000

  echo '{"name":"Pin 13 Pro Max","aliases":["AP-BA-13PM"],"tags":["pin"]}' \
    | capigo products create --tenant acme --from-json -

SEE ALSO
  products update <id>    change a product after it exists
  products variants       add or change variants on it
  capigo help exit-codes  what exit 8 means`,
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

		tenant := resolveTenant(productCreateTenant, profile)
		defer echoTenant(tenant, productCreateTenant)

		// /pcms/* requires a tenant.
		if tenant == nil {
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
			if len(productCreateAliases) > 0 {
				req.Aliases = productCreateAliases
			}
			if len(productCreateTags) > 0 {
				req.Tags = productCreateTags
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
		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			// C1: render full api.Product in JSON mode.
			return output.WriteJSONObject(os.Stdout, envelope.Data)
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
	productUpdateTenant        string
	productUpdateName          string
	productUpdateDescription   string
	productUpdateStatus        string
	productUpdateCurrency      string
	productUpdateBrandID       string
	productUpdateCategoryID    string
	productUpdateProductTypeID string
	productUpdateUnitID        string
	productUpdateAliases       []string
	productUpdateTags          []string
	productUpdateFromJSON      string
)

var productsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a product's core details (partial)",
	Long: `Update a product. Fields you do not send are left unchanged.

PURPOSE
  Change a product's metadata: name, description, status, currency, brand,
  category, product type, unit, aliases, tags. Archiving a product is a status
  change: --status ARCHIVED.

  Variants are not product metadata. They are written with products variants.

INPUT
  <id>                     product UUID (positional, required)
  --tenant <code>          required

  Any of --name --description --status --currency --brand-id --category-id
  --product-type-id --unit-id --aliases --tags. At least one is required;
  sending none exits 5.

  --aliases and --tags are repeatable, and each REPLACES the whole array
  rather than appending to it. To add one alias, send every alias you want to
  keep.

  --status is DRAFT, ACTIVE, or ARCHIVED.

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5. (products create differs — there, --from-json silently wins.)

OUTPUT
  -o json emits the bare updated product — the same shape as products get.
  quiet prints its id.

  Output modes and the JSON contract: capigo help output

CAVEATS
  Changing an sku, barcode or alias that already exists breaks whatever refers
  to it by that value.

EXAMPLES
  # Archive a product
  capigo products update 8f2a-... --tenant acme --status ARCHIVED

  # Replace the alias list (both aliases survive; a single --aliases would drop the other)
  capigo products update 8f2a-... --tenant acme --aliases AP-BA-13PM --aliases PIN13PM

  echo '{"status":"ACTIVE","tags":["summer"]}' \
    | capigo products update 8f2a-... --tenant acme --from-json -

SEE ALSO
  products get <id>       read the current values before overwriting an array
  products variants       create or update variants
  capigo help exit-codes  what exit 5 means`,
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

		tenant := resolveTenant(productUpdateTenant, profile)
		defer echoTenant(tenant, productUpdateTenant)

		if tenant == nil {
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
				"brand-id", "category-id", "product-type-id", "unit-id", "aliases", "tags"}
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
			if len(productUpdateTags) > 0 {
				req.Tags = productUpdateTags
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
		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			// C1: render full api.Product in JSON mode.
			return output.WriteJSONObject(os.Stdout, envelope.Data)
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
	productVariantsTenant    string
	productVariantsProductID string
	productVariantsFromJSON  string
)

var productsVariantsCmd = &cobra.Command{
	Use:   "variants",
	Short: "Upsert product variants (create or update)",
	// push: TODO — three traps below reach only a caller who reads this page.
	// Each needs a line on stdout at the moment it bites before the CAVEATS
	// section can be called a mitigation rather than a warning:
	//   1. the call is not atomic; a failed write may have applied some items
	//   2. an explicit null clears a field
	//   3. a duplicate barcode is accepted silently
	Long: `Create or update variants of a product in a single call (upsert).

PURPOSE
  Write variants onto an existing product. An item carrying variant_id updates
  that variant; an item without one creates a variant. A single call may mix
  both.

INPUT
  --tenant <code>        required
  --product-id <uuid>    required
  --from-json <path|->   required. A JSON ARRAY of variant objects, where -
                         reads stdin.

    Field              Type     Notes
    variant_id         string   present -> UPDATE that variant; absent -> CREATE
    name               string   required when creating; rejects null
    sku                string   variant code. Unique per tenant (server-enforced)
    barcode            string   numeric barcode. Not enforced unique
    price              number
    option1..option3   string   positional. The position is NOT inferred from
                                the label — read the product's options[] with
                                products get <id> and match by index.
    manufacturer_code  string
    legacy_code        string
    extra_data         object   arbitrary key-value metadata

  On an UPDATE, a field you OMIT is left unchanged, and a field you send as
  NULL is cleared. Unknown fields are forwarded to the API untouched.

  Example body — one update, one create:

      [ { "variant_id": "6f1c-...", "price": 250000 },
        { "name": "Red / M", "sku": "AP-BA-13PM-R-M",
          "option1": "Red", "option2": "M" } ]

OUTPUT
  The full updated PRODUCT — not the list of variants you sent.

  -o json emits the bare product object, the same shape as products get.
  table prints a product summary and does NOT surface variant_id; if you will
  need to reference a variant you just wrote, use -o json.
  quiet prints the product id.

  Output modes and the JSON contract: capigo help output

CAVEATS
  This call is not atomic. Creates are committed before updates are applied,
  and there is no rollback. If it fails partway, some variants may already have
  been written. Read the product back with products get <id> before retrying:
  resending the same array can create duplicates of what already landed.

  A duplicate barcode is accepted by the server without error. A duplicate sku
  exits 8.

EXAMPLES
  # Read the option order before writing option1..option3
  capigo products get 8f2a-... --tenant acme -o json | jq '.options'

  # Add one variant
  echo '[{"name":"Red / M","sku":"AP-BA-13PM-R-M","option1":"Red","option2":"M"}]' \
    | capigo products variants --tenant acme --product-id 8f2a-... --from-json -

  # Change only the price; every other field on that variant is untouched
  echo '[{"variant_id":"6f1c-...","price":250000}]' \
    | capigo products variants --tenant acme --product-id 8f2a-... --from-json -

  # Read back the variant ids that were written
  capigo products variants --tenant acme --product-id 8f2a-... \
    --from-json ./variants.json -o json | jq '.variants[] | {id, sku, price}'

SEE ALSO
  products get <id>       the product, its options[] and its variants
  variants get <id>       one variant in full
  variants list           query variants by barcode prefix
  products update <id>    change product metadata, not variants
  capigo help exit-codes  what exit 8 means`,
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

		tenant := resolveTenant(productVariantsTenant, profile)
		defer echoTenant(tenant, productVariantsTenant)

		if tenant == nil {
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

		// Validate it is a JSON array, then send the raw bytes untouched (same
		// raw-passthrough pattern as brands/categories/units --from-json).
		// Decoding into api.UpsertVariantItem and re-marshaling it here would
		// silently drop any field the struct doesn't know about (e.g.
		// manufacturer_code/legacy_code/extra_data) before the request ever
		// reaches the API.
		var probe []json.RawMessage
		if err := json.Unmarshal(raw, &probe); err != nil {
			return handleErr(fmt.Errorf("--from-json must be a JSON array of variant objects: %w", err))
		}

		resp, err := client.Do(ctx, "PUT",
			"/pcms/products/"+productVariantsProductID+"/variants",
			json.RawMessage(raw), tenant)
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
		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			// C1: render full api.Product in JSON mode.
			return output.WriteJSONObject(os.Stdout, envelope.Data)
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
	// products get flags
	productsGetCmd.Flags().StringVar(&productGetTenant, "tenant", "", "tenant code (required)")

	// products list flags
	productsListCmd.Flags().StringVar(&productListTenant, "tenant", "", "tenant code (required)")
	productsListCmd.Flags().StringVarP(&productListQuery, "query", "q", "", "free-text search (2–500 chars): matches product name, aliases, tags, variant name, SKU, and barcode")
	productsListCmd.Flags().StringVar(&productListUpdatedSince, "updated-since", "", "ISO 8601 timestamp for delta sync (from previous X-Server-Time header)")
	productsListCmd.Flags().IntVar(&productListPage, "page", 0, "page number (0 = server default)")
	productsListCmd.Flags().IntVar(&productListLimit, "limit", 0, "items per page (1–100, default 20)")
	productsListCmd.Flags().BoolVar(&productListAll, "all", false, "fetch all pages automatically (loads all pages into memory before output; for large catalogs prefer paginating manually)")
	productsListCmd.Flags().StringVar(&productListIDs, "ids", "", "comma-separated list of product UUIDs to fetch (max 50); mutually exclusive with --all")

	// products create flags
	productsCreateCmd.Flags().StringVar(&productCreateTenant, "tenant", "", "tenant code (required)")
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
	productsCreateCmd.Flags().StringArrayVar(&productCreateAliases, "aliases", nil, "product aliases / alternative search names (repeatable: --aliases foo --aliases bar)")
	productsCreateCmd.Flags().StringArrayVar(&productCreateTags, "tags", nil, "free-form product tags (repeatable: --tags foo --tags bar)")
	productsCreateCmd.Flags().StringVar(&productCreateFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin)")

	// products update flags
	productsUpdateCmd.Flags().StringVar(&productUpdateTenant, "tenant", "", "tenant code (required)")
	productsUpdateCmd.Flags().StringVar(&productUpdateName, "name", "", "new product name")
	productsUpdateCmd.Flags().StringVar(&productUpdateDescription, "description", "", "new product description")
	productsUpdateCmd.Flags().StringVar(&productUpdateStatus, "status", "", "new status: DRAFT, ACTIVE, or ARCHIVED")
	productsUpdateCmd.Flags().StringVar(&productUpdateCurrency, "currency", "", "new currency code")
	productsUpdateCmd.Flags().StringVar(&productUpdateBrandID, "brand-id", "", "new brand UUID")
	productsUpdateCmd.Flags().StringVar(&productUpdateCategoryID, "category-id", "", "new category UUID")
	productsUpdateCmd.Flags().StringVar(&productUpdateProductTypeID, "product-type-id", "", "new product type UUID")
	productsUpdateCmd.Flags().StringVar(&productUpdateUnitID, "unit-id", "", "new unit UUID")
	productsUpdateCmd.Flags().StringArrayVar(&productUpdateAliases, "aliases", nil, "product aliases (repeatable: --aliases foo --aliases bar)")
	productsUpdateCmd.Flags().StringArrayVar(&productUpdateTags, "tags", nil, "free-form product tags (repeatable: --tags foo --tags bar)")
	productsUpdateCmd.Flags().StringVar(&productUpdateFromJSON, "from-json", "", "path to JSON file with update body (use - for stdin); mutually exclusive with individual field flags")

	// products variants flags
	productsVariantsCmd.Flags().StringVar(&productVariantsTenant, "tenant", "", "tenant code (required)")
	productsVariantsCmd.Flags().StringVar(&productVariantsProductID, "product-id", "", "product UUID (required)")
	productsVariantsCmd.Flags().StringVar(&productVariantsFromJSON, "from-json", "", "path to JSON array file (use - for stdin) (required)")

	productCmd.AddCommand(productsGetCmd, productsListCmd, productsCreateCmd, productsUpdateCmd, productsVariantsCmd)
	rootCmd.AddCommand(productCmd)
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

// listMetaExtras decorates the server's pagination meta on the JSON path with
// the delta-sync cursor and, when --ids was used, any requested IDs the
// server did not return.
type listMetaExtras struct {
	api.Meta
	ServerTime string   `json:"server_time,omitempty"`
	MissingIDs []string `json:"missing_ids,omitempty"`
}

// allMeta is the synthetic meta for --all. Complete reports whether the
// pagination loop reached the last page; false means the result is PARTIAL.
type allMeta struct {
	api.Meta
	ServerTime string `json:"server_time,omitempty"`
	Complete   bool   `json:"complete"`
}

// missingProductIDs reports which of the comma-separated requested IDs are
// absent from the returned rows, plus how many IDs were requested. Matching
// is case-insensitive since UUIDs compare equal regardless of case.
func missingProductIDs(idsFlag string, got []api.Product) (missing []string, requested int) {
	if idsFlag == "" {
		return nil, 0
	}
	have := make(map[string]bool, len(got))
	for _, p := range got {
		have[strings.ToLower(p.ID)] = true
	}
	for _, raw := range strings.Split(idsFlag, ",") {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		requested++
		if !have[strings.ToLower(id)] {
			missing = append(missing, id)
		}
	}
	return missing, requested
}

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
	// Soft-deleted products must never look live: surface the tombstone in
	// the Status cell, the one column every reader scans.
	status := p.Status
	if p.IsDeleted {
		status += " (DELETED)"
	}
	return output.Product{
		ID:           p.ID,
		Name:         p.Name,
		Status:       status,
		SKU:          sku,
		Price:        price,
		VariantCount: len(p.Variants),
		Aliases:      strings.Join(p.Aliases, ", "),
		Tags:         strings.Join(p.Tags, ", "),
	}
}
