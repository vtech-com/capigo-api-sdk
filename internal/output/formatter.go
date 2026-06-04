package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

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
		"product": {
			headers: []string{"ID", "Name", "Status", "SKU", "Price", "Variants"},
			rowFn: func(v any) []any {
				p := v.(Product)
				return []any{p.ID, p.Name, p.Status, p.SKU, p.Price, p.VariantCount}
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
		return renderJSON(w, items)
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

func renderJSON(w io.Writer, items []any) error {
	if len(items) == 0 {
		_, err := fmt.Fprintln(w, "[]")
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
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
