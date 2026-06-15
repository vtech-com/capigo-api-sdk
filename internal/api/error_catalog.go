package api

// ErrorInfo is a human- and agent-actionable interpretation of a Capigo error
// code. The server already returns a descriptive `message` for most codes (shown
// as the `Server:` line); this catalog adds only what that message lacks:
//
//   - Next: the concrete action to take (never part of the server message).
//   - CapabilityNote: the "a failed write is not a missing capability" brake.
//   - Meaning: an interpretation, populated ONLY for codes whose server message
//     is a generic fallback (E9426/E9427 — see product-write-errors.ts). For
//     every other code the server message is descriptive enough, so Meaning is
//     left empty to avoid maintaining a second, drift-prone copy of it.
type ErrorInfo struct {
	Meaning        string
	Next           string
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
	// ----- Generic-fallback codes: the server message is uninformative
	// ("Failed to update product variants"), so Meaning fills the void. -----
	"E9426": {
		Meaning:        "The server rejected creating a product variant for an unclassified reason (not a known SKU/option conflict — those have their own codes).",
		Next:           "Re-check the new variant's fields (name, sku, option1/2/3). If nothing is obviously wrong, the cause is server-side only (DB constraint) — surface the Response below to a human; do not guess.",
		CapabilityNote: true,
	},
	"E9427": {
		Meaning:        "The server failed to update a product variant for an unclassified reason.",
		Next:           "Re-check the variant fields; if it persists, surface the Response below to a human.",
		CapabilityNote: true,
	},

	// ----- Codes with a descriptive server message: Meaning omitted, the
	// `Server:` line carries it. We add only Next + the capability brake. -----
	"E9445": {
		Next:           "Change the sku on the new variant, or update the existing one by passing its variant_id.",
		CapabilityNote: true,
	},
	"E9446": {
		Next:           "Change option1/option2/option3 so the combination is unique.",
		CapabilityNote: true,
	},
	"E9447": {
		Next:           "Keep at least one variant on a variant-mode product.",
		CapabilityNote: true,
	},
	"E9463": {
		Next:           "compare_at_price must be greater than price (or null); fix the value and retry.",
		CapabilityNote: true,
	},
	"E9443": {
		Next:           "Fix the option name and retry.",
		CapabilityNote: true,
	},
	"E9444": {
		Next:           "Fix the option values and retry.",
		CapabilityNote: true,
	},
	"E9425": {
		Next:           "Re-check the product/variant ID and the tenant.",
		CapabilityNote: false,
	},
	"E9417": {
		Next:           "Re-check the product ID and tenant.",
		CapabilityNote: false,
	},
	"E9418": {
		Next:           "Re-check the payload; if it persists, surface the Response below to a human.",
		CapabilityNote: true,
	},
	"E9419": {
		Next:           "Re-check the payload; if it persists, surface the Response below to a human.",
		CapabilityNote: true,
	},
	"VALIDATION_ERROR": {
		Next:           "Read the server message above, fix the offending field, and retry.",
		CapabilityNote: true,
	},
	"E0102": {
		Next:           "Owner or admin role is required; surface this to a human.",
		CapabilityNote: false,
	},
	"E9103": {
		Next:           "Verify the tenant code and your access to it.",
		CapabilityNote: false,
	},
	"E0004": {
		Next:           "Run: capigo auth login --key csk_...",
		CapabilityNote: false,
	},
}
