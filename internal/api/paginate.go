package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

const maxPages = 1000

// FetchAll paginates through all pages of path, accumulating results until
// meta.has_more is false or maxPages is reached. params holds extra query
// parameters; "page" is managed internally and must not be set by the caller.
func FetchAll[T any](ctx context.Context, c *Client, path string, params map[string]string) ([]T, error) {
	var all []T

	for page := 1; page <= maxPages; page++ {
		query := "?page=" + strconv.Itoa(page)
		for k, v := range params {
			if k == "page" {
				continue
			}
			query += "&" + k + "=" + v
		}

		resp, err := c.Do(ctx, "GET", path+query, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("fetch page %d: %w", page, err)
		}

		var envelope Envelope[[]T]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return nil, fmt.Errorf("decode page %d: %w", page, err)
		}

		all = append(all, envelope.Data...)

		if !envelope.Meta.HasMore {
			break
		}
	}

	return all, nil
}
