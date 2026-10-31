package api

import (
	"context"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// Spoiler returns the HTML of the spoiler at position in an item. The body HTML of the item holds
// only a placeholder with that position.
func (c *Client) Spoiler(ctx context.Context, ct ContentType, contentID, position int) (string, error) {
	params := &gen.SpoilersGetParams{
		ContentType: gen.SpoilersGetParamsContentType(ct),
		ContentId:   contentID,
		Position:    position,
	}
	resp, err := c.gen.SpoilersGetWithResponse(ctx, params)
	if err != nil {
		return "", err
	}
	out, err := result(resp.HTTPResponse, resp.Body, resp.JSON200)
	if err != nil {
		return "", err
	}
	return out.Html, nil
}
