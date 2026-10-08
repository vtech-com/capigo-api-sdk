package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var productsMediaCmd = &cobra.Command{
	Use:   "media",
	Short: "Add, change and delete a product's images and videos",
	Long: `A product's gallery: images and videos, their order, default and variant links.

--tenant is required on every command here, and the key's member needs the catalog
content permission (owner, catalog admin or catalog contributor).
  capigo help tenancy

USAGE
  capigo products media <command> <product-id> --tenant <code> [<args>]`,
}

func mediaPath(productID, mediaID string) string {
	p := "/pcms/products/" + url.PathEscape(productID) + "/media"
	if mediaID != "" {
		p += "/" + url.PathEscape(mediaID)
	}
	return p
}

// products media add flags
var (
	mediaAddTenant         string
	mediaAddFile           string
	mediaAddSourceURL      string
	mediaAddFilename       string
	mediaAddVariantIDs     []string
	mediaAddDefault        bool
	mediaAddIdempotencyKey string
)

var productsMediaAddCmd = &cobra.Command{
	Use:   "add <product-id>",
	Short: "Add an image or video to a product",
	Long: `Add an image or video to a product's gallery, from a file or from a public address.

PURPOSE
  One call does what the PCMS editor does in several steps: it checks for a
  duplicate, stores the bytes in private storage, creates the item, links the
  variants and sets the default. The first item of a product is the product default
  without asking.

  Limits: a product holds at most 50 live items (exit 5, E9454). JPEG, PNG, WebP
  and GIF up to 10 MB (E9452); MP4 up to 50 MB; no empty file (E9460). The type is
  decided from the bytes, never from the file name; anything else is E9453. The
  answer never carries a storage path: the item has a signed link in url and its
  expiry in expires_at.

  The same file name and size as a live item, with the same dimensions, is not
  stored twice: the call answers with the existing item (exit 0, HTTP 200).

USAGE
  capigo products media add <product-id> --tenant <code>
                            (--file <path> | --source-url <https-url>)
                            [--filename <name>] [--variant-id <uuid>]...
                            [--default] [--idempotency-key <key>]

FLAGS
  <product-id>
      Product UUID. Positional, required.

  --tenant <code>
      Tenant the product belongs to. Required.

  --file <path>
      A file on this machine. The file's own name becomes the item's name.

  --source-url <url>
      A public https address on port 443 for the server to fetch, instead of a file.
      The name is the last path segment of the address unless --filename is given.
      A host that does not answer is exit 6 (SOURCE_FETCH_FAILED).

  --filename <name>
      Overrides the name taken from the address (with --source-url) or the file.
      1 to 200 characters, no path separator or control character.

  --variant-id <uuid>   (repeatable, at most 50)
      Link the item to these variants of this product.

  --default
      Make the item the product default.

  --idempotency-key <key>
      Make a retry safe. The same key with the same file answers 200 and stores
      nothing new; a different file with it is exit 8 (E0601).

        capigo products media add 8f2a… --tenant acme --file ./red-front.jpg \\
          --variant-id 6f1c… --default

OUTPUT
  The stored item is at .data:

      { "data": { "id": "…", "position": 1, "is_default": true, "alt_text": null,
                  "url": "https://…signed…", "expires_at": "2026-10-07T04:00:00Z",
                  "variant_ids": ["6f1c…"] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  Exit 5 when neither or both of --file and --source-url are given.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if (mediaAddFile == "") == (mediaAddSourceURL == "") {
			failValidation("products media add: give exactly one of --file and --source-url")
		}
		if len(mediaAddVariantIDs) > 50 {
			failValidation("products media add: at most 50 --variant-id")
		}
		idempotencyKey := requireUsableKey(cmd.Flags().Changed("idempotency-key"), mediaAddIdempotencyKey, "idempotency-key")

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile := activeProfileOrEmpty(cfg)
		tenant := resolveTenant(mediaAddTenant, profile)
		requireTenant(tenant, "products media add")

		var resp *api.Response
		if mediaAddFile != "" {
			data, err := api.ReadUploadFile(mediaAddFile)
			if err != nil {
				return handleErr(err)
			}
			name := strings.TrimSpace(mediaAddFilename)
			if name == "" {
				name = filepath.Base(mediaAddFile)
			}
			resp, err = client.UploadProductMedia(ctx, mediaPath(args[0], ""), name,
				api.DetectContentType(mediaAddFile, data), data, mediaAddVariantIDs, mediaAddDefault, tenant, idempotencyKey)
			if err != nil {
				return handleErr(err)
			}
		} else {
			body := map[string]any{"source_url": strings.TrimSpace(mediaAddSourceURL)}
			if v := strings.TrimSpace(mediaAddFilename); v != "" {
				body["filename"] = v
			}
			if len(mediaAddVariantIDs) > 0 {
				body["variant_ids"] = mediaAddVariantIDs
			}
			if mediaAddDefault {
				body["is_default"] = true
			}
			headers := map[string]string{}
			if idempotencyKey != "" {
				headers["Idempotency-Key"] = idempotencyKey
			}
			resp, err = client.DoWithHeaders(ctx, "POST", mediaPath(args[0], ""), body, tenant, headers)
			if err != nil {
				return handleErr(err)
			}
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, mediaAddTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

// products media update flags
var (
	mediaUpdateTenant       string
	mediaUpdatePosition     int
	mediaUpdateDefault      bool
	mediaUpdateVariantIDs   []string
	mediaUpdateClearLinks   bool
	mediaUpdateAltText      string
	mediaUpdateClearAltText bool
)

var productsMediaUpdateCmd = &cobra.Command{
	Use:   "update <product-id> <media-id>",
	Short: "Change an item's order, default, variant links or alt text",
	Long: `Change one gallery item: its place, whether it is the default, its variant links or its alt text.

PURPOSE
  Send only what you mean to change; at least one flag is required. When several are
  given they are applied in a fixed order: alt text, links, default, position. A
  later failure leaves the earlier steps in place; a reorder applies whole or not at
  all.

USAGE
  capigo products media update <product-id> <media-id> --tenant <code>
                               [--position <n>] [--default]
                               [--variant-id <uuid>]... [--clear-variants]
                               [--alt-text <text> | --clear-alt-text]

FLAGS
  <product-id>, <media-id>
      Product UUID and the item's UUID (from products get, under .media). Positional.

  --tenant <code>
      Tenant the product belongs to. Required.

  --position <n>
      1-based place among the product's live items. The others keep their relative
      order and the default does not change. Below 1 or past the number of items is
      exit 5; it is never clamped.

  --default
      Make this item the product default. The order does not change. There is no way
      to unset a default except by making another item the default.

  --variant-id <uuid>   (repeatable)
      The complete set of this product's variants the item is linked to: missing
      links are added and the others removed. A variant that is not this product's
      is exit 4 and the links do not change. --clear-variants removes every link and
      the item stays.

  --alt-text <text>, --clear-alt-text
      Up to 500 characters, or clear it.

        capigo products media update 8f2a… 3c9d… --tenant acme --position 1 --alt-text "Front view"

OUTPUT
  The item as it now stands is at .data.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		flags := cmd.Flags()

		body := map[string]any{}
		if flags.Changed("position") {
			if mediaUpdatePosition < 1 {
				failValidation("products media update: --position must be 1 or more")
			}
			body["position"] = mediaUpdatePosition
		}
		if mediaUpdateDefault {
			body["is_default"] = true
		}
		if mediaUpdateClearLinks && len(mediaUpdateVariantIDs) > 0 {
			failValidation("products media update: give --variant-id or --clear-variants, not both")
		}
		if len(mediaUpdateVariantIDs) > 0 {
			body["variant_ids"] = mediaUpdateVariantIDs
		} else if mediaUpdateClearLinks {
			body["variant_ids"] = []string{}
		}
		if mediaUpdateClearAltText && flags.Changed("alt-text") {
			failValidation("products media update: give --alt-text or --clear-alt-text, not both")
		}
		if flags.Changed("alt-text") {
			if len([]rune(mediaUpdateAltText)) > 500 {
				failValidation("products media update: --alt-text is over 500 characters")
			}
			body["alt_text"] = mediaUpdateAltText
		} else if mediaUpdateClearAltText {
			body["alt_text"] = nil
		}
		if len(body) == 0 {
			failValidation("products media update: give at least one field to change")
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile := activeProfileOrEmpty(cfg)
		tenant := resolveTenant(mediaUpdateTenant, profile)
		requireTenant(tenant, "products media update")

		resp, err := client.Do(ctx, "PATCH", mediaPath(args[0], args[1]), body, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, mediaUpdateTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

var mediaDeleteTenant string

var productsMediaDeleteCmd = &cobra.Command{
	Use:   "delete <product-id> <media-id>",
	Short: "Delete a gallery item",
	Long: `Delete one image or video from a product's gallery.

PURPOSE
  The item leaves every read at once and every variant link to it is removed. If it
  was the product default, the product has no default afterwards; no other item is
  promoted. The stored file is removed on a best-effort basis, so a failed file
  removal never undoes the delete. There is no restore: add the file again if you
  need it. A product, item or tenant that does not match, or an item already deleted,
  is exit 4.

USAGE
  capigo products media delete <product-id> <media-id> --tenant <code>

FLAGS
  <product-id>, <media-id>
      Product UUID and the item's UUID. Positional, required.

  --tenant <code>
      Tenant the product belongs to. Required.

OUTPUT
  The deletion is confirmed at .data.`,
	Args: cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile := activeProfileOrEmpty(cfg)
		tenant := resolveTenant(mediaDeleteTenant, profile)
		requireTenant(tenant, "products media delete")

		resp, err := client.Do(ctx, "DELETE", mediaPath(args[0], args[1]), nil, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, mediaDeleteTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

func init() {
	productsMediaAddCmd.Flags().StringVar(&mediaAddTenant, "tenant", "", "tenant the product belongs to (required)")
	productsMediaAddCmd.Flags().StringVar(&mediaAddFile, "file", "", "file to upload")
	productsMediaAddCmd.Flags().StringVar(&mediaAddSourceURL, "source-url", "", "public https address for the server to fetch")
	productsMediaAddCmd.Flags().StringVar(&mediaAddFilename, "filename", "", "override the item's name")
	productsMediaAddCmd.Flags().StringArrayVar(&mediaAddVariantIDs, "variant-id", nil, "variant UUID to link; repeatable")
	productsMediaAddCmd.Flags().BoolVar(&mediaAddDefault, "default", false, "make the item the product default")
	productsMediaAddCmd.Flags().StringVar(&mediaAddIdempotencyKey, "idempotency-key", "", "make a retry safe")

	productsMediaUpdateCmd.Flags().StringVar(&mediaUpdateTenant, "tenant", "", "tenant the product belongs to (required)")
	productsMediaUpdateCmd.Flags().IntVar(&mediaUpdatePosition, "position", 0, "1-based place among the live items")
	productsMediaUpdateCmd.Flags().BoolVar(&mediaUpdateDefault, "default", false, "make the item the product default")
	productsMediaUpdateCmd.Flags().StringArrayVar(&mediaUpdateVariantIDs, "variant-id", nil, "variant UUID; repeat for the complete set")
	productsMediaUpdateCmd.Flags().BoolVar(&mediaUpdateClearLinks, "clear-variants", false, "remove every variant link")
	productsMediaUpdateCmd.Flags().StringVar(&mediaUpdateAltText, "alt-text", "", "alt text, up to 500 characters")
	productsMediaUpdateCmd.Flags().BoolVar(&mediaUpdateClearAltText, "clear-alt-text", false, "clear the alt text")

	productsMediaDeleteCmd.Flags().StringVar(&mediaDeleteTenant, "tenant", "", "tenant the product belongs to (required)")

	productsMediaCmd.AddCommand(productsMediaAddCmd, productsMediaUpdateCmd, productsMediaDeleteCmd)
}
