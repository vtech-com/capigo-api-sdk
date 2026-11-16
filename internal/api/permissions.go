package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// PermissionModules and PermissionScopeTypes are the enums GET /permissions
// declares for module and scope_type; the list filters accept nothing else.
var (
	PermissionModules    = []string{"catalog", "po", "wms"}
	PermissionScopeTypes = []string{"tenant", "warehouse"}
)

// ListPermissions reads the static permission catalog. It names no tenant: the
// list is the same for every caller and the route ignores X-Tenant-Code.
func (c *Client) ListPermissions(ctx context.Context) (*Response, error) {
	return c.Do(ctx, "GET", "/permissions", nil, nil)
}

// FilterPermissions keeps the catalog entries whose module and scope_type match.
// The route takes no query parameters, so the filter runs here. Each kept entry
// keeps every field it was sent; an empty filter keeps everything.
func FilterPermissions(data json.RawMessage, module, scopeType string) (json.RawMessage, error) {
	var entries []json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("decode permissions: %w", err)
	}
	kept := make([]json.RawMessage, 0, len(entries))
	for _, entry := range entries {
		var fields struct {
			Module    string `json:"module"`
			ScopeType string `json:"scope_type"`
		}
		if err := json.Unmarshal(entry, &fields); err != nil {
			return nil, fmt.Errorf("decode permission: %w", err)
		}
		if module != "" && fields.Module != module {
			continue
		}
		if scopeType != "" && fields.ScopeType != scopeType {
			continue
		}
		kept = append(kept, entry)
	}
	return json.Marshal(kept)
}

// MemberPermissionsRequest is the body of POST /members/{id}/permissions/add
// and /remove: the keys to change, leaving every other key as it is.
type MemberPermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

// ChangeMemberPermissions adds or removes tenant permission keys on one member.
// change is "add" or "remove"; userID is the member's user id.
func (c *Client) ChangeMemberPermissions(ctx context.Context, userID, change string, body MemberPermissionsRequest, tenant *string) (*Response, error) {
	return c.Do(ctx, "POST", "/members/"+url.PathEscape(userID)+"/permissions/"+change, body, tenant)
}
