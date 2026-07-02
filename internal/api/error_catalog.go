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
		Next:           "Not authenticated — ask the user to re-authenticate: capigo auth login --key <their csk_… key>.",
		CapabilityNote: false,
	},

	// ----- Public API auth codes (apps/platform: auth-guard.ts / proxy.ts). These
	// are what THIS CLI sees on an auth failure — distinct from the internal web
	// app's E0004 family. The server message is descriptive, so Meaning stays
	// empty; Next carries the action plus the brake against the most common
	// agent misread: treating a rejected key as a transient, retryable hiccup.
	// CapabilityNote stays false — an auth failure is never a missing feature.
	//
	// The key distinction encoded here: only AUTH_INTERNAL_ERROR (a 500 from the
	// auth service) is genuinely transient and worth a retry. A rejected/malformed
	// key or a tenant mismatch is deterministic — the CLI sends a fixed key every
	// call, so the same key will be rejected the same way; retrying changes nothing.
	"AUTH_INVALID_KEY": {
		Next: "The API key was rejected — this is auth (exit 2), not a rate limit, so retrying the same key will NOT help. Ask the user to re-authenticate: capigo auth login --key <their csk_… key>. If a freshly provided key still returns 401, the cause is server-side, not the key — surface it; do not loop.",
	},
	"AUTH_INVALID_KEY_PREFIX": {
		Next: "The API key is malformed (it must look like csk_…). Ask the user for the complete key and re-run: capigo auth login --key <their csk_… key>. Retrying the same value will not help.",
	},
	"AUTH_MISSING_HEADER": {
		Next: "No API key was sent — the CLI is not logged in. Ask the user to run: capigo auth login --key <their csk_… key> (or set CAPIGO_API_KEY). Do not retry until a key is configured.",
	},
	"AUTH_INVALID_FORMAT": {
		Next: "The Authorization header was malformed. Re-authenticate with a clean key: capigo auth login --key <their csk_… key>. Retrying the same value will not help.",
	},
	"AUTH_TENANT_MISMATCH": {
		Next: "This API key is scoped to a different tenant (exit 3) — retrying or re-authenticating will not change that. Use a --tenant the key can access, or ask the user for a key scoped to this tenant.",
	},
	"AUTH_INTERNAL_ERROR": {
		Next: "The auth service itself errored (server-side, HTTP 500). Unlike a rejected key, this one IS transient — retry once after a short backoff. If it persists, surface it; it is not your key.",
	},

	// ----- Local, CLI-detected condition (not a Capigo API error code): the
	// storage host rejected the byte fetch for a just-minted signed URL. -----
	"ATTACHMENT_URL_EXPIRED": {
		Meaning: "The signed download URL (5-minute TTL) was rejected by storage when the CLI tried to fetch the bytes, immediately after requesting it.",
		Next:    "Re-run the same `tasks attachments download` / `tasks comments attachments download` command — it mints a fresh URL on every call. Do not reuse a URL or metadata from a previous invocation.",
	},
}
