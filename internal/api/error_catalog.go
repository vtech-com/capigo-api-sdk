package api

// ErrorInfo is a human- and agent-actionable interpretation of a Capigo error
// code. It turns an opaque EXXXX/code string into a meaning, a concrete next
// step, and a flag for whether the "a failed write is not a missing capability"
// brake applies.
type ErrorInfo struct {
	// Meaning explains what the code actually signifies.
	Meaning string
	// Next is the concrete action the caller should take.
	Next string
	// CapabilityNote is true when the failure could be misread as "the API does
	// not support this operation" — i.e. write-side validation/conflict/business
	// errors. When true, callers print the capability brake.
	CapabilityNote bool
}

// LookupError returns the catalog entry for a code, if known.
//
// This is a temporary, hand-maintained map sourced from the backend's
// error-codes.ts / product-write-errors.ts. Once the OpenAPI document publishes
// a code catalog, this map should be replaced by (or generated from) that spec
// so it cannot drift. Do NOT seed entries from the stale comments in errors.go.
func LookupError(code string) (ErrorInfo, bool) {
	info, ok := errorCatalog[code]
	return info, ok
}

var errorCatalog = map[string]ErrorInfo{
	// ----- Product variant write errors -----
	"E9426": {
		Meaning:        "The server rejected creating a product variant.",
		Next:           "Check the new variant's fields (name, sku, option1/2/3) in your payload, fix the offending value, and retry.",
		CapabilityNote: true,
	},
	"E9427": {
		Meaning:        "The server failed to update a product variant.",
		Next:           "Re-check the variant fields; if it persists, surface the response below to a human.",
		CapabilityNote: true,
	},
	"E9445": {
		Meaning:        "A variant with this SKU already exists in the tenant.",
		Next:           "Change the sku on the new variant, or update the existing one by passing its variant_id.",
		CapabilityNote: true,
	},
	"E9446": {
		Meaning:        "A variant with this option combination already exists on the product.",
		Next:           "Change option1/option2/option3 so the combination is unique.",
		CapabilityNote: true,
	},
	"E9447": {
		Meaning:        "Variant limit exceeded (max 50 items per request).",
		Next:           "Split the variants into batches of 50 or fewer.",
		CapabilityNote: true,
	},
	"E9463": {
		Meaning:        "Invalid compare-at-price for the variant.",
		Next:           "compare_at_price must be greater than price; fix the value and retry.",
		CapabilityNote: true,
	},
	"E9425": {
		Meaning:        "The product or variant was not found, or does not belong to this tenant.",
		Next:           "Re-check the product/variant ID and the tenant.",
		CapabilityNote: false,
	},

	// ----- Product write errors -----
	"E9417": {
		Meaning:        "Product not found.",
		Next:           "Re-check the product ID and tenant.",
		CapabilityNote: false,
	},
	"E9418": {
		Meaning:        "The server failed to create the product.",
		Next:           "Re-check the payload; if it persists, surface the response below to a human.",
		CapabilityNote: true,
	},
	"E9419": {
		Meaning:        "The server failed to update the product.",
		Next:           "Re-check the payload; if it persists, surface the response below to a human.",
		CapabilityNote: true,
	},
	"E9443": {
		Meaning:        "Invalid product option name.",
		Next:           "Fix the option name and retry.",
		CapabilityNote: true,
	},
	"E9444": {
		Meaning:        "Invalid product option values.",
		Next:           "Fix the option values and retry.",
		CapabilityNote: true,
	},

	// ----- Generic / shared -----
	"VALIDATION_ERROR": {
		Meaning:        "The request payload failed validation.",
		Next:           "Read the server message above, fix the offending field, and retry.",
		CapabilityNote: true,
	},
	"E0102": {
		Meaning:        "You lack permission to perform this operation in this tenant.",
		Next:           "Owner or admin role is required; surface this to a human.",
		CapabilityNote: false,
	},
	"E9103": {
		Meaning:        "Access to this tenant is denied.",
		Next:           "Verify the tenant code and your access to it.",
		CapabilityNote: false,
	},
	"E0004": {
		Meaning:        "Authentication is required (missing or invalid API key).",
		Next:           "Run: capigo auth login --key csk_...",
		CapabilityNote: false,
	},
}
