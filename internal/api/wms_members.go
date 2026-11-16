package api

import (
	"context"
	"net/url"
)

// WarehouseMemberCreateRequest is the body of POST
// /wms/warehouses/{code}/members. Scopes is never nil: the API requires the
// array, and an empty one stores scope.view.
type WarehouseMemberCreateRequest struct {
	UserID string   `json:"user_id"`
	Scopes []string `json:"scopes"`
}

// WarehouseMemberScopesRequest is the body of the scopes add and remove routes.
type WarehouseMemberScopesRequest struct {
	Scopes []string `json:"scopes"`
}

func warehouseMembersPath(warehouse string) string {
	return "/wms/warehouses/" + url.PathEscape(warehouse) + "/members"
}

// ListWarehouseMembers reads one page of a warehouse's memberships.
func (c *Client) ListWarehouseMembers(ctx context.Context, warehouse string, page, limit int, tenant *string) (*Response, error) {
	path := warehouseMembersPath(warehouse)
	if params := buildParams(nil, page, limit); len(params) > 0 {
		path += "?" + params.Encode()
	}
	return c.Do(ctx, "GET", path, nil, tenant)
}

// AddWarehouseMember gives a tenant member access to a warehouse.
func (c *Client) AddWarehouseMember(ctx context.Context, warehouse string, body WarehouseMemberCreateRequest, tenant *string) (*Response, error) {
	return c.Do(ctx, "POST", warehouseMembersPath(warehouse), body, tenant)
}

// ChangeWarehouseMemberScopes adds or removes scopes on one membership. change
// is "add" or "remove"; memberID is the warehouse membership id, not a user id.
func (c *Client) ChangeWarehouseMemberScopes(ctx context.Context, warehouse, memberID, change string, body WarehouseMemberScopesRequest, tenant *string) (*Response, error) {
	return c.Do(ctx, "POST", warehouseMembersPath(warehouse)+"/"+url.PathEscape(memberID)+"/scopes/"+change, body, tenant)
}

// RemoveWarehouseMember ends one membership. memberID is the warehouse
// membership id, not a user id.
func (c *Client) RemoveWarehouseMember(ctx context.Context, warehouse, memberID string, tenant *string) (*Response, error) {
	return c.Do(ctx, "DELETE", warehouseMembersPath(warehouse)+"/"+url.PathEscape(memberID), nil, tenant)
}
