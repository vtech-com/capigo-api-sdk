// Package output writes what a command has to say, as JSON, to stdout.
//
// There is one output shape and no way to ask for another. Every successful
// command emits the same envelope:
//
//	{ "data": …, "meta": { … } }
//
// data is an array for a list and an object for a single item; meta always
// names the tenant the call resolved to. A caller reads .data and .meta and is
// done — there is no second shape to branch on, and nothing true of the call
// that lives only in prose on a stream nobody parses.
//
// This replaces three renderers (table, quiet, json) whose differences were
// the source of most of this CLI's documentation bugs: totals that appeared in
// one mode and not another, a deletion marker only the table carried, a
// server timestamp that changed streams depending on the flag.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// Meta is the envelope's meta object. Every field is optional except the
// tenant fields, which every tenant-scoped command sets.
//
// Pointers, not bare ints: a list of zero rows must emit "total": 0, which an
// omitempty int would silently drop.
type Meta struct {
	// Tenant is the tenant code the call resolved to, and TenantSource says
	// where that value came from: "flag", "env" (CAPIGO_TENANT) or "config"
	// (default_tenant). A cross-tenant read leaves both empty.
	Tenant       string `json:"tenant,omitempty"`
	TenantSource string `json:"tenant_source,omitempty"`

	// Pagination, mirrored from the API's own meta. Lists only.
	Page    *int  `json:"page,omitempty"`
	Limit   *int  `json:"limit,omitempty"`
	Total   *int  `json:"total,omitempty"`
	HasMore *bool `json:"has_more,omitempty"`

	// ServerTime is the X-Server-Time header. Feed it back as --updated-since.
	ServerTime string `json:"server_time,omitempty"`

	// MissingIDs are the ids a --ids request asked for and did not get back.
	// The key is absent when none are missing — including when --ids was never
	// passed. A caller that asked for specific ids knows it did, so an absent
	// key there means "all of them came back".
	MissingIDs []string `json:"missing_ids,omitempty"`

	// Complete is false when an --all sweep aborted part-way, so the rows in
	// data are a prefix of the truth rather than all of it.
	Complete *bool `json:"complete,omitempty"`
}

// envelope is the one shape this CLI emits on success.
type envelope struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

// Write emits the envelope to w. A nil or nil-slice data becomes [], so an
// empty result is never indistinguishable from a missing one.
func Write(w io.Writer, data any, meta Meta) error {
	rv := reflect.ValueOf(data)
	if rv.Kind() == reflect.Slice && rv.IsNil() {
		data = reflect.MakeSlice(rv.Type(), 0, 0).Interface()
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope{Data: data, Meta: meta})
}

// Ptr is a convenience for the pointer fields of Meta.
func Ptr[T any](v T) *T { return &v }

const capabilityBrake = "a failed write does NOT mean this operation is unsupported."

// ErrorDetail carries everything needed to render a self-diagnosing error: the
// raw API fields plus the catalog interpretation (meaning, next step, and the
// capability brake). Meaning/Next/RawBody may be empty when no catalog entry
// matched.
type ErrorDetail struct {
	Code           string
	Message        string
	RequestID      string
	HTTPStatus     int
	Meaning        string
	Next           string
	CapabilityNote bool
	RawBody        string
}

// RenderError writes a failure as JSON on stdout and a one-line summary on
// stderr. stdout stays machine-readable even when the command failed: a caller
// that parses stdout unconditionally gets an object with an "error" key rather
// than a parse error on top of an API error.
func RenderError(stdout, stderr io.Writer, d ErrorDetail) {
	// Only the keys that carry something. A `"next": ""` forces a caller to
	// tell an absent step apart from an empty one, which is a distinction
	// without a difference and a branch nobody writes correctly.
	e := map[string]any{"code": d.Code, "message": d.Message}
	putIf := func(k string, v string) {
		if v != "" {
			e[k] = v
		}
	}
	putIf("request_id", d.RequestID)
	putIf("meaning", d.Meaning)
	putIf("next", d.Next)
	putIf("raw", d.RawBody)
	if d.HTTPStatus > 0 {
		e["http_status"] = d.HTTPStatus
	}
	// The brake is the sentence, not a boolean: `"capability_note": true` tells
	// a reader nothing, while the sentence is the thing that stops an agent
	// concluding "the API cannot do this" from a request it got wrong.
	if d.CapabilityNote {
		e["capability_note"] = capabilityBrake
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"error": e})
	_, _ = fmt.Fprintf(stderr, "Error: %s (code=%s, request_id=%s)\n", d.Message, d.Code, d.RequestID)
}
