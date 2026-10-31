package api

import (
	"context"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// SearchOpts are the filters of Search. A zero filter is not sent.
type SearchOpts struct {
	Query      string
	TitlesOnly bool
	NodeID     int
	AuthorID   int
	PrefixID   int
	PageOpts
}

// Search returns one page of the posts that hold all the words of o.Query, newest first.
// A page can hold fewer than the limit while NextCursor is not nil.
func (c *Client) Search(ctx context.Context, o SearchOpts) (*Page[gen.SearchResult], error) {
	params := &gen.SearchQueryParams{
		Q:          o.Query,
		TitlesOnly: optBool(o.TitlesOnly),
		NodeId:     optInt(o.NodeID),
		AuthorId:   optInt(o.AuthorID),
		PrefixId:   optInt(o.PrefixID),
		Page:       optInt(o.Page),
		Cursor:     optString(o.Cursor),
		Limit:      optInt(o.Limit),
	}
	resp, err := c.gen.SearchQueryWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*Page[gen.SearchResult])(resp.JSON200))
}
