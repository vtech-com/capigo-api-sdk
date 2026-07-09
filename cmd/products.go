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
	Long: `Products in the Capigo Product Catalog Management System (PCMS).

Every products command requires a tenant. A product with options carries its
variants alongside it; write the product's own fields with products
create/update, and write variants with products variants.

USAGE
  capigo products <command> --tenant <code> [<args>]`,
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
	// than the stored value reads to a caller like "no such product". The note
	// under --query below only helps a caller who reads it; the trap needs a
	// zero-row hint on stdout before this page can be called a mitigation.
	Long: `List products from the PCMS catalog.

PURPOSE
  Find products, and read them in bulk. Look one up by a human key — name,
  alias, sku, barcode — with --query, then act on the id it returns. To read a
  single product whose id you already have, use products get.

USAGE
  capigo products list --tenant <code> [-q <term>]
                       [--ids <uuid,...> | --all]
                       [--updated-since <timestamp>]
                       [--page <n>] [--limit <n>]

FLAGS
  --tenant <code>
      Tenant to read from. Required. Falls back to CAPIGO_TENANT, then to
      default_tenant in the config file. Exits 5 if none resolves.

        capigo products list --tenant acme

  -q, --query <term>
      Substring search over name, aliases, tags, variant name, sku and barcode.
      2 to 500 characters. The stored value must contain the term, so a term
      longer than what is stored matches nothing: the alias VVD013 is not found
      by searching SLM-DS-VVD013. Search the shortest distinctive fragment.

        capigo products list --tenant acme -q VVD013
        capigo products list --tenant acme -q "áo thun"

  --ids <uuid,...>
      Fetch products by id, at most 50, comma-separated. If any id does not
      come back — deleted, or in another tenant — the products that did are
      still printed, under an error key naming the ids that did not, and the
      command exits 4. Cannot be combined with --all.

        capigo products list --tenant acme --ids 7c1f2e88-...,9ab2c744-...

  --updated-since <timestamp>
      Return only products changed at or after an ISO 8601 timestamp. Pass back
      the server time from the previous call's meta.server_time to fetch a
      delta.

        capigo products list --tenant acme --updated-since 2026-07-01T00:00:00Z

  --all
      Fetch every page. All pages are held in memory before anything prints, so
      page manually over a large catalogue. If a page fails mid-sweep, the rows
      already fetched are still printed, under an error key, and the command
      exits non-zero.

        capigo products list --tenant acme --all

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page parameter
      and lets the server choose.

  --limit <n>
      Rows per page, 1 to 100. Defaults to 20.

        capigo products list --tenant acme --page 2 --limit 100

OUTPUT
  The products are at .data[]:

      {
        "data": [
          {
            "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10",
            "name": "Áo thun basic",
            "slug": "ao-thun-basic",
            "description": "Cotton 100%, form rộng",
            "status": "ACTIVE",
            "currency": "VND",
            "aliases": ["VVD013"],
            "tags": ["hè"],
            "is_deleted": false,
            "brand": { "id": "...", "name": "Coolmate" },
            "category": { "id": "...", "name": "Áo" },
            "product_type": { "id": "...", "name": "Apparel" },
            "unit": { "id": "...", "name": "Cái" },
            "options": [],
            "variants": [ { "id": "...", "sku": "AT-001-S", ... } ],
            "created_at": "2026-06-01T08:00:00Z",
            "updated_at": "2026-06-01T08:00:00Z"
          }
        ],
        "meta": {
          "tenant": "acme", "tenant_source": "flag",
          "page": 1, "limit": 20, "total": 137, "has_more": true,
          "server_time": "2026-07-09T04:12:33Z"
        }
      }

  Read meta.total rather than counting .data[]: a page never holds more than
  --limit, so a full count needs meta, not arithmetic.

  The API's own list meta has only page/limit/total/has_more. The CLI adds
  meta.server_time: the server clock at the time of the call, which no header
  the caller can see would otherwise give them. Feed it to --updated-since on
  the next call.

  When --ids asked for an id the server did not return (exit 4), or an --all
  sweep aborted (exit non-zero), the rows that were fetched are still
  printed — in the same document, beneath an error key:

      { "error": {...}, "data": [...], "meta": {...} }

  An error key means the answer is incomplete. Without it, .data is everything
  the request asked for. See capigo help output.

  meta.tenant is the tenant this call actually ran against, and
  meta.tenant_source says whether that came from the flag, from CAPIGO_TENANT,
  or from the config file. See capigo help tenancy.

  A soft-deleted product is still listed. is_deleted: true is the only signal
  — status alone does not reveal it. See capigo help soft-delete.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		if productListIDs != "" && productListAll {
			failValidation("--ids and --all are mutually exclusive; use --ids to fetch specific products or --all to paginate the full catalog")
		}

		if productListIDs != "" {
			idParts := strings.Split(productListIDs, ",")
			if len(idParts) > 50 {
				failValidation("--ids accepts at most 50 UUIDs (got %d); split into multiple requests", len(idParts))
			}
		}

		validatePCMSLimit(productListLimit)

		if productListQuery != "" && utf8.RuneCountInString(productListQuery) < 2 {
			failValidation("--query must be at least 2 characters")
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
		requireTenant(tenant, "products commands")

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

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := listMeta(tenant, productListTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime

		// You asked for specific ids and did not get all of them. Printed under
		// a clean envelope those rows are indistinguishable from a complete
		// answer — the server reports total: 1, has_more: false, which is what
		// success looks like. So the rows stand, and an error key stands over
		// them.
		if missing, _ := missingProductIDs(productListIDs, idsOf(envelope.Data)); len(missing) > 0 {
			return failWithData(&api.APIError{
				Code:       "NOT_FOUND",
				Message:    fmt.Sprintf("%d of the requested ids were not returned: %s", len(missing), strings.Join(missing, ", ")),
				HTTPStatus: 404,
			}, rawList(envelope.Data), meta)
		}

		return output.Write(os.Stdout, rawList(envelope.Data), meta)
	},
}

// productsListAll auto-paginates until has_more is false. A mid-pagination
// failure does not discard what was already fetched: the partial result is
// still written and the command still exits with the underlying error's code,
// so empty stdout never masquerades as an empty catalogue.
//
// stdout carries one JSON document either way. When the sweep aborts, that
// document carries an error key above the rows, so a caller holding it knows it
// holds a prefix of the catalogue without having to consult the exit code.
func productsListAll(ctx context.Context, client *api.Client, tenant *string) error {
	page := 1
	var allProducts []json.RawMessage // each row exactly as the API sent it
	var lastMeta api.Meta             // zero until the first page decodes; /pcms/products always sends meta
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
			var envelope api.RawEnvelope
			var rows []json.RawMessage
			if uerr := json.Unmarshal(resp.Body, &envelope); uerr != nil {
				err = fmt.Errorf("decode response: %w", uerr)
			} else if uerr := json.Unmarshal(rawList(envelope.Data), &rows); uerr != nil {
				err = fmt.Errorf("decode response: data is not an array: %w", uerr)
			} else {
				if resp.ServerTime != "" {
					lastServerTime = resp.ServerTime
				}
				allProducts = append(allProducts, rows...)
				if envelope.Meta == nil {
					return handleErr(fmt.Errorf("decode response: page %d carried no meta; --all cannot know whether more pages remain", page))
				}
				lastMeta = *envelope.Meta
				if !lastMeta.HasMore {
					break
				}
				page++
				continue
			}
		}
		// A failed page: with nothing fetched yet this is a plain error;
		// with rows already in hand, write the partial set first.
		if len(allProducts) == 0 {
			return handleErr(err)
		}
		fetchErr = err
		break
	}

	complete := fetchErr == nil
	total := len(allProducts)
	if !complete && lastMeta.Total > total {
		total = lastMeta.Total
	}

	meta := listMeta(tenant, productListTenant, &api.Meta{
		Page:    1,
		Limit:   lastMeta.Limit,
		Total:   total,
		HasMore: !complete,
	})
	meta.ServerTime = lastServerTime

	if fetchErr != nil {
		return failWithData(fetchErr, allProducts, meta)
	}

	return output.Write(os.Stdout, allProducts, meta)
}

// --------------------------------------------------------------------------
// products get
// --------------------------------------------------------------------------

var productGetTenant string

var productsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a product by id",
	Long: `Get one product by UUID.

PURPOSE
  Read a single product in full: its variants, options, brand, category, product
  type and unit. This command addresses a product by UUID only. To find that
  UUID from a name, alias, product code, sku or barcode, use
  products list --query first.

USAGE
  capigo products get <id> --tenant <code>

FLAGS
  <id>
      Product UUID. Positional, required.

  --tenant <code>
      Tenant the product belongs to. Required. Exits 4 if the product is not
      in it.

        capigo products get 8f2a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

OUTPUT
  The product is at .data — an object, where a list puts an array:

      {
        "data": {
          "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10",
          "name": "Áo thun basic", "slug": "ao-thun-basic",
          "description": null, "status": "ACTIVE", "currency": "VND",
          "aliases": ["VVD013"], "tags": ["hè"], "is_deleted": false,
          "created_at": "...", "updated_at": "...",
          "brand": {...}, "category": {...}, "product_type": {...},
          "unit": {...}, "options": [...], "variants": [...]
        },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  options[] carries the product's option names in order. That order is what
  option1, option2 and option3 refer to when writing variants with
  products variants.

  A single-item read carries no pagination meta; there is nothing to page.

  A soft-deleted product is returned like any other. Its status may still read
  ACTIVE; is_deleted is the field that tells you. See capigo help soft-delete.

  Exit 4 when no such product exists in the resolved tenant.`,
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
		requireTenant(tenant, "products commands")

		resp, err := client.Do(ctx, "GET", "/pcms/products/"+args[0], nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawItem(envelope.Data), itemMeta(tenant, productGetTenant))
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
  variant, built from --sku, --barcode and --price. To add options and
  variants, use --from-json.

USAGE
  capigo products create --tenant <code> --name <name>
                         [--sku <sku>] [--barcode <code>] [--price <n>]
                         [--currency <code>] [--description <text>]
                         [--status <status>]
                         [--brand-id <uuid>] [--category-id <uuid>]
                         [--product-type-id <uuid>] [--unit-id <uuid>]
                         [--aliases <text>]... [--tags <text>]...
  capigo products create --tenant <code> --from-json <path|->

FLAGS
  --tenant <code>
      Tenant to create the product in. Required. Falls back to CAPIGO_TENANT,
      then to default_tenant in the config file. Exits 5 if none resolves.

        capigo products create --tenant acme --name "Blue T-Shirt" \
          --sku SKU-001 --price 299000

  --name <name>
      Product name. Required in flag mode, unless --from-json is used.

  --sku <sku>, --barcode <code>, --price <n>
      Fields of the auto-created default variant. Ignored if the product ends
      up with options (only possible via --from-json).

  --currency <code>
      Default VND.

  --description <text>

  --status <status>
      DRAFT, ACTIVE, or ARCHIVED. Default DRAFT.

  --brand-id <uuid>, --category-id <uuid>, --product-type-id <uuid>,
  --unit-id <uuid>

  --aliases <text>
      Alternative names and product codes. Repeatable: --aliases foo --aliases
      bar. Matched by the search in products list. Not enforced unique — the
      server accepts a duplicate alias without error.

  --tags <text>
      Free-form labels. Repeatable. Matched by the search in products list.

  --from-json <path|->
      Send the whole request body as JSON, where - reads stdin. Individual
      field flags are ignored when this is set. (products update differs —
      there, passing both exits 5.)

      Simple body:

          { "name": "Blue T-Shirt", "sku": "SKU-001", "price": 299000,
            "status": "DRAFT", "aliases": ["BT-001"], "tags": ["summer"] }

      Body with options and variants. If options is present, variants is
      REQUIRED: the backend does not generate the cartesian matrix from
      options alone. option1..option3 are positional and line up with
      options[] in order:

          { "name": "T-Shirt",
            "options": [ {"name": "Color", "values": ["Blue", "Red"]},
                         {"name": "Size",  "values": ["S", "M"]} ],
            "variants": [
              {"name": "Blue / S", "sku": "SKU-BS",
               "option1": "Blue", "option2": "S", "price": 299000},
              {"name": "Red / M", "sku": "SKU-RM",
               "option1": "Red", "option2": "M", "price": 279000}
            ] }

        echo '{"name":"Pin 13 Pro Max","aliases":["AP-BA-13PM"],
               "tags":["pin"]}' \
          | capigo products create --tenant acme --from-json -

OUTPUT
  The created product is at .data — same shape as products get:

      {
        "data": { "id": "8f2a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "name": "Blue T-Shirt", "status": "DRAFT", ... },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the product was written to. Read it: a write that
  landed in the wrong tenant looks exactly like a write that succeeded.

  A variant sku is unique per tenant: a duplicate exits 8. A barcode is NOT
  enforced unique — the server accepts a duplicate without error.`,
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
		requireTenant(tenant, "products commands")

		var body any
		if productCreateFromJSON != "" {
			raw, err := readJSONInput(productCreateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if productCreateName == "" {
				failValidation("--name is required")
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

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, productCreateTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
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
	Short: "Update a product's core details",
	Long: `Update a product. Fields you do not send are left unchanged.

PURPOSE
  Change a product's metadata: name, description, status, currency, brand,
  category, product type, unit, aliases, tags. Archiving a product is a status
  change: --status ARCHIVED.

  Variants are not product metadata. They are written with products variants.

USAGE
  capigo products update <id> --tenant <code>
                         [--name <name>] [--description <text>]
                         [--status <status>] [--currency <code>]
                         [--brand-id <uuid>] [--category-id <uuid>]
                         [--product-type-id <uuid>] [--unit-id <uuid>]
                         [--aliases <text>]... [--tags <text>]...
  capigo products update <id> --tenant <code> --from-json <path|->

FLAGS
  <id>
      Product UUID. Positional, required.

  --tenant <code>
      Tenant the product belongs to. Required. Falls back to CAPIGO_TENANT,
      then to default_tenant in the config file. Exits 5 if none resolves.

  --name <name>, --description <text>, --currency <code>

  --status <status>
      DRAFT, ACTIVE, or ARCHIVED.

        capigo products update 8f2a1c07-... --tenant acme --status ARCHIVED

  --brand-id <uuid>, --category-id <uuid>, --product-type-id <uuid>,
  --unit-id <uuid>

  --aliases <text>
      Repeatable, and REPLACES the whole array rather than appending to it.
      To add one alias, send every alias you want to keep. Changing or
      dropping an alias breaks whatever refers to the product by the old one.

        capigo products update 8f2a1c07-... --tenant acme \
          --aliases AP-BA-13PM --aliases PIN13PM

  --tags <text>
      Repeatable, and REPLACES the whole array rather than appending to it.

  At least one of the flags above is required in flag mode; sending none
  exits 5.

  --from-json <path|->
      Send the whole update body as JSON, where - reads stdin. MUTUALLY
      EXCLUSIVE with the individual field flags: passing both exits 5.
      (products create differs — there, --from-json silently wins.)

        echo '{"status":"ACTIVE","tags":["summer"]}' \
          | capigo products update 8f2a1c07-... --tenant acme --from-json -

OUTPUT
  The updated product is at .data — same shape as products get:

      {
        "data": { "id": "8f2a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "status": "ACTIVE", "tags": ["summer"], ... },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the write landed in. A write into the wrong
  tenant looks exactly like a write that succeeded, so read it. See
  capigo help tenancy.`,
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
		requireTenant(tenant, "products commands")

		var body any
		if productUpdateFromJSON != "" {
			// Check that no individual field flags were also set.
			individualFlags := []string{"name", "description", "status", "currency",
				"brand-id", "category-id", "product-type-id", "unit-id", "aliases", "tags"}
			for _, f := range individualFlags {
				if cmd.Flags().Changed(f) {
					failValidation("--from-json and --%s are mutually exclusive; use one or the other", f)
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
				failValidation("at least one field must be provided for update")
			}
			body = req
		}

		resp, err := client.Do(ctx, "PUT", "/pcms/products/"+productUpdateID, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, productUpdateTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
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
  both. Variants not included in the payload are left untouched — this
  command does not delete variants. It does not change product metadata; use
  products update for that.

USAGE
  capigo products variants --tenant <code> --product-id <id>
                           --from-json <path|->

FLAGS
  --tenant <code>
      Tenant the product belongs to. Required. Falls back to CAPIGO_TENANT,
      then to default_tenant in the config file. Exits 5 if none resolves.

  --product-id <uuid>
      The product to write variants onto. Required.

  --from-json <path|->
      Required. A JSON ARRAY of variant objects, where - reads stdin.

        Field              Type     Notes
        variant_id         string   present -> UPDATE; absent -> CREATE
        name               string   required when creating; rejects null
        sku                string   variant code. Unique per tenant (server)
        barcode            string   numeric barcode. Not enforced unique
        price              number
        option1..option3   string   positional. The position is NOT inferred
                                    from the label — read the product's
                                    options[] with products get <id> and
                                    match by index.
        manufacturer_code  string
        legacy_code        string
        extra_data         object   arbitrary key-value metadata

      On an UPDATE, a field you OMIT is left unchanged, and a field you send
      as NULL is cleared. Unknown fields are forwarded to the API untouched.

      This call is not atomic. Creates are committed before updates are
      applied, and there is no rollback. If it fails partway, some variants
      may already have been written. Read the product back with
      products get <id> before retrying: resending the same array can create
      duplicates of what already landed.

        # Read the option order before writing option1..option3
        capigo products get 8f2a1c07-... --tenant acme | jq '.data.options'

        # One update, one create in the same call
        echo '[ { "variant_id": "6f1c1a02-...", "price": 250000 },
                 { "name": "Red / M", "sku": "AP-BA-13PM-R-M",
                   "option1": "Red", "option2": "M" } ]' \
          | capigo products variants --tenant acme --product-id 8f2a1c07-... \
              --from-json -

OUTPUT
  The full updated PRODUCT is at .data — not the list of variants you sent. It
  is the same shape as products get:

      {
        "data": { "id": "8f2a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "variants": [...], ... },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  Read .data.variants[] for the variant_id of what you just wrote:

      capigo products variants --tenant acme --product-id 8f2a1c07-... \
        --from-json ./variants.json \
        | jq '.data.variants[] | {id, sku, price}'

  meta.tenant is the tenant the write landed in. See capigo help tenancy.

  A duplicate sku exits 8. A duplicate barcode is accepted by the server
  without error.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		if productVariantsProductID == "" {
			failValidation("--product-id is required")
		}
		if productVariantsFromJSON == "" {
			failValidation("--from-json is required")
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
		requireTenant(tenant, "products commands")

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

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, productVariantsTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
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
	productsListCmd.Flags().StringVar(&productListUpdatedSince, "updated-since", "", "ISO 8601 timestamp for delta sync (from previous meta.server_time)")
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

	productCmd.AddCommand(productsListCmd, productsGetCmd, productsCreateCmd, productsUpdateCmd, productsVariantsCmd)
	rootCmd.AddCommand(productCmd)
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

// missingProductIDs reports which of the comma-separated requested IDs are
// absent from the returned rows, plus how many IDs were requested. Matching
// is case-insensitive since UUIDs compare equal regardless of case.
func missingProductIDs(idsFlag string, got []string) (missing []string, requested int) {
	if idsFlag == "" {
		return nil, 0
	}
	have := make(map[string]bool, len(got))
	for _, id := range got {
		have[strings.ToLower(id)] = true
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
