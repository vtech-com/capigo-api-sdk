package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// WriteJSONList marshals a list response as {"data":[...],"meta":{...}} to w
// with 2-space indentation. When data is nil or an empty slice the envelope
// still emits "data":[] so callers can reliably distinguish an empty result
// from an error.
//
// data must be a slice value (or nil); meta may be any JSON-serialisable value.
func WriteJSONList(w io.Writer, data, meta any) error {
	// Normalise nil and empty slices so JSON always emits "data":[].
	rv := reflect.ValueOf(data)
	if data == nil || (rv.Kind() == reflect.Slice && rv.IsNil()) {
		data = emptySliceOf(data)
	}

	envelope := struct {
		Data any `json:"data"`
		Meta any `json:"meta"`
	}{Data: data, Meta: meta}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

// emptySliceOf returns an empty non-nil slice of the same element type as v,
// or []any{} when v is nil / not a slice.
func emptySliceOf(v any) any {
	if v == nil {
		return []any{}
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return []any{}
	}
	return reflect.MakeSlice(rv.Type(), 0, 0).Interface()
}

// WriteJSONObject marshals a single item as a bare JSON object (no wrapper)
// to w with 2-space indentation. Use for get / create / update / replace
// commands where the response is a single resource.
func WriteJSONObject(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// RenderOpts controls rendering behaviour.
type RenderOpts struct {
	// GlobalMode inserts a Tenant column as the first column in table output.
	GlobalMode bool
	// ResourceKind selects the registered renderer (e.g. "task", "board").
	ResourceKind string
}

// renderer describes how to display a single resource.
type renderer struct {
	// headers are the column header strings (without Tenant, which is injected dynamically).
	headers []string
	// rowFn extracts column values for a single item; order must match headers.
	rowFn func(v any) []any
	// idFn returns the primary key for quiet mode.
	idFn func(v any) string
}

var resourceRenderers map[string]renderer

func init() {
	resourceRenderers = map[string]renderer{
		"tenant": {
			headers: []string{"Code", "Name"},
			rowFn: func(v any) []any {
				t := v.(Tenant)
				return []any{t.Code, t.Name}
			},
			idFn: func(v any) string { return v.(Tenant).Code },
		},
		"member": {
			headers: []string{"ID", "Name", "Email", "Role"},
			rowFn: func(v any) []any {
				m := v.(Member)
				return []any{m.ID, m.Name, m.Email, m.Role}
			},
			idFn: func(v any) string { return v.(Member).ID },
		},
		"board": {
			headers: []string{"ID", "Title", "Public", "Description"},
			rowFn: func(v any) []any {
				b := v.(Board)
				pub := "no"
				if b.IsPublic {
					pub = "yes"
				}
				return []any{b.ID, b.Title, pub, b.Description}
			},
			idFn: func(v any) string { return v.(Board).ID },
		},
		"board_detail": {
			headers: []string{"ID", "Title", "Lists"},
			rowFn: func(v any) []any {
				b := v.(BoardDetail)
				return []any{b.ID, b.Title, b.ListCount}
			},
			idFn: func(v any) string { return v.(BoardDetail).ID },
		},
		"task": {
			// Code (e.g. "TASK-123") is the primary human/agent-facing reference;
			// it appears first so it is immediately visible in table output.
			headers: []string{"Code", "ID", "Title", "Status", "Assignee"},
			rowFn: func(v any) []any {
				t := v.(Task)
				return []any{t.Code, t.ID, t.Title, t.Status, t.Assignee}
			},
			idFn: func(v any) string { return v.(Task).ID },
		},
		"task_comment": {
			// A timeline is read top-to-bottom, so Created leads. ID is a UUID with
			// little human value and would crowd the wide Content column, so it is
			// kept for quiet mode (idFn) but omitted from the table.
			headers: []string{"Created", "Author", "Kind", "Content", "Files"},
			rowFn: func(v any) []any {
				c := v.(TaskComment)
				return []any{c.Created, c.Author, c.Kind, c.Content, c.Attachments}
			},
			idFn: func(v any) string { return v.(TaskComment).ID },
		},
		"product": {
			headers: []string{"ID", "Name", "Status", "SKU", "Price", "Variants", "Aliases"},
			rowFn: func(v any) []any {
				p := v.(Product)
				return []any{p.ID, p.Name, p.Status, p.SKU, p.Price, p.VariantCount, p.Aliases}
			},
			idFn: func(v any) string { return v.(Product).ID },
		},
		"brand": {
			headers: []string{"ID", "Name"},
			rowFn: func(v any) []any {
				b := v.(Brand)
				return []any{b.ID, b.Name}
			},
			idFn: func(v any) string { return v.(Brand).ID },
		},
		"category": {
			headers: []string{"ID", "Name", "ParentID"},
			rowFn: func(v any) []any {
				c := v.(Category)
				parentID := ""
				if c.ParentID != nil {
					parentID = *c.ParentID
				}
				return []any{c.ID, c.Name, parentID}
			},
			idFn: func(v any) string { return v.(Category).ID },
		},
		"product_type": {
			headers: []string{"ID", "Name"},
			rowFn: func(v any) []any {
				pt := v.(ProductType)
				return []any{pt.ID, pt.Name}
			},
			idFn: func(v any) string { return v.(ProductType).ID },
		},
		"unit": {
			headers: []string{"ID", "Name", "Abbreviation"},
			rowFn: func(v any) []any {
				u := v.(Unit)
				return []any{u.ID, u.Name, u.Abbreviation}
			},
			idFn: func(v any) string { return v.(Unit).ID },
		},
		"variant_record": {
			headers: []string{"ID", "Barcode", "SKU", "Name", "ProductID"},
			rowFn: func(v any) []any {
				vr := v.(VariantRecord)
				return []any{vr.ID, vr.Barcode, vr.SKU, vr.Name, vr.ProductID}
			},
			idFn: func(v any) string { return v.(VariantRecord).ID },
		},
		"variant": {
			headers: []string{"ID", "Name", "SKU", "Barcode", "Price", "Type"},
			rowFn: func(v any) []any {
				vr := v.(Variant)
				return []any{vr.ID, vr.Name, vr.SKU, vr.Barcode, vr.Price, vr.VariantType}
			},
			idFn: func(v any) string { return v.(Variant).ID },
		},
	}
}

// Render writes data to w in the requested mode.
// data may be a single struct or a slice of structs that match opts.ResourceKind.
func Render(w io.Writer, mode string, data any, opts RenderOpts) error {
	r, ok := resourceRenderers[opts.ResourceKind]
	if !ok {
		return fmt.Errorf("unknown resource kind: %q", opts.ResourceKind)
	}

	items := toSlice(data)

	switch mode {
	case "json":
		// Render is the human-facing (display-model) path. The machine-facing
		// JSON contract is intentionally NOT served here: lists must emit
		// {"data":[...],"meta":{...}} and single items a bare object. Commands
		// handle json mode themselves via WriteJSONList / WriteJSONObject before
		// reaching Render. Rejecting json here stops a future command from
		// silently emitting the wrong (display-model array) shape.
		return fmt.Errorf("output.Render does not serve json mode; use output.WriteJSONList (lists) or output.WriteJSONObject (single items)")
	case "quiet":
		return renderQuiet(w, items, r)
	case "table", "":
		return renderTable(w, items, r, opts.GlobalMode)
	default:
		return fmt.Errorf("unknown output format %q: supported formats are table, json, quiet", mode)
	}
}

// RenderError writes a formatted error to w.
// When mode is "json" the output is a JSON object; otherwise plain text.
func RenderError(w io.Writer, mode, code, message, requestID string) {
	if mode == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{
			"error": map[string]string{
				"code":       code,
				"message":    message,
				"request_id": requestID,
			},
		}); err != nil {
			return
		}
		return
	}
	_, _ = fmt.Fprintf(w, "Error: %s (code=%s, request_id=%s)\n", message, code, requestID)
}

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

const capabilityBrake = "a failed write does NOT mean this operation is unsupported."

// RenderErrorRich writes a self-diagnosing error. The full diagnostic block goes
// to stdout (where an AI agent reads), and a concise one-line summary goes to
// stderr (for humans and scripts). Exit-code handling stays with the caller.
//
//   - table mode: human-readable block on stdout + one line on stderr.
//   - json mode:  enriched JSON error object on stdout + one line on stderr.
//   - quiet mode: nothing on stdout; one line on stderr (unchanged behaviour).
func RenderErrorRich(stdout, stderr io.Writer, mode string, d ErrorDetail) {
	stderrLine := func() {
		_, _ = fmt.Fprintf(stderr, "Error: %s (code=%s, request_id=%s)\n", d.Message, d.Code, d.RequestID)
	}

	switch mode {
	case "quiet":
		stderrLine()
		return

	case "json":
		obj := map[string]any{
			"error": map[string]any{
				"code":            d.Code,
				"message":         d.Message,
				"request_id":      d.RequestID,
				"meaning":         d.Meaning,
				"next":            d.Next,
				"capability_note": d.CapabilityNote,
				"raw":             d.RawBody,
			},
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(obj)
		stderrLine()
		return

	default: // table / text
		_, _ = fmt.Fprintf(stdout, "✗ Failed · %s", d.Code)
		if d.HTTPStatus > 0 {
			_, _ = fmt.Fprintf(stdout, " · HTTP %d", d.HTTPStatus)
		}
		_, _ = fmt.Fprintln(stdout)
		if d.Message != "" {
			_, _ = fmt.Fprintf(stdout, "  Server:   %s\n", d.Message)
		}
		if d.Meaning != "" {
			_, _ = fmt.Fprintf(stdout, "  Means:    %s\n", d.Meaning)
		}
		if d.CapabilityNote {
			_, _ = fmt.Fprintf(stdout, "  Note:     %s\n", capabilityBrake)
		}
		if d.Next != "" {
			_, _ = fmt.Fprintf(stdout, "  Next:     %s\n", d.Next)
		}
		if d.RawBody != "" {
			_, _ = fmt.Fprintf(stdout, "  Response: %s\n", d.RawBody)
		}
		if d.RequestID != "" {
			_, _ = fmt.Fprintf(stdout, "  request_id=%s\n", d.RequestID)
		}
		stderrLine()
		return
	}
}

// toSlice normalises data into []any regardless of whether it arrives as a
// single struct or a slice of structs.
func toSlice(data any) []any {
	if data == nil {
		return nil
	}
	rv := reflect.ValueOf(data)
	if rv.Kind() == reflect.Slice {
		out := make([]any, rv.Len())
		for i := range rv.Len() {
			out[i] = rv.Index(i).Interface()
		}
		return out
	}
	return []any{data}
}

func renderQuiet(w io.Writer, items []any, r renderer) error {
	for _, item := range items {
		_, err := fmt.Fprintln(w, r.idFn(item))
		if err != nil {
			return err
		}
	}
	return nil
}

func renderTable(w io.Writer, items []any, r renderer, globalMode bool) error {
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.Style().Color = table.ColorOptions{}
	t.Style().Format.Header = text.FormatDefault

	// Build header row.
	var hdr table.Row
	if globalMode {
		hdr = append(hdr, "Tenant")
	}
	for _, h := range r.headers {
		hdr = append(hdr, h)
	}
	t.AppendHeader(hdr)

	for _, item := range items {
		cols := r.rowFn(item)

		var row table.Row
		if globalMode {
			row = append(row, tenantCodeOf(item))
		}
		for _, c := range cols {
			row = append(row, c)
		}
		t.AppendRow(row)
	}

	t.Render()
	return nil
}

// tenantCodeOf extracts TenantCode from items that carry it; returns empty
// string for types that do not.
func tenantCodeOf(v any) string {
	switch x := v.(type) {
	case Task:
		return x.TenantCode
	case Board:
		return x.TenantCode
	case Member:
		return x.TenantCode
	case Tenant:
		return x.Code
	case Product:
		return x.TenantCode
	default:
		return ""
	}
}
