package api

import (
	"context"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// Bookmarks returns one page of the bookmarks of the viewer.
func (c *Client) Bookmarks(ctx context.Context, p PageOpts) (*Page[gen.Bookmark], error) {
	params := &gen.BookmarksListParams{Page: optInt(p.Page), Cursor: optString(p.Cursor), Limit: optInt(p.Limit)}
	resp, err := c.gen.BookmarksListWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*Page[gen.Bookmark])(resp.JSON200))
}

// SetBookmark adds a bookmark when on is true, and removes it when on is false.
func (c *Client) SetBookmark(ctx context.Context, contentType gen.BookmarksSetJSONBodyContentType, contentID int, on bool) error {
	defer c.pages.drop(prefixPosts)
	body := gen.BookmarksSetJSONRequestBody{ContentType: contentType, ContentId: contentID, Bookmarked: on}
	resp, err := c.gen.BookmarksSetWithResponse(ctx, body)
	if err != nil {
		return err
	}
	return check(resp.HTTPResponse, resp.Body)
}

// BookmarkStatus returns the ids that the viewer has a bookmark for.
func (c *Client) BookmarkStatus(ctx context.Context, contentType gen.BookmarksStatusParamsContentType, ids []int) ([]int, error) {
	resp, err := c.gen.BookmarksStatusWithResponse(ctx, &gen.BookmarksStatusParams{ContentType: contentType, Ids: ids})
	if err != nil {
		return nil, err
	}
	out, err := result(resp.HTTPResponse, resp.Body, resp.JSON200)
	if err != nil {
		return nil, err
	}
	return out.Bookmarked, nil
}

// WatchedThreads returns one page of the threads that the viewer watches.
// With limit 0 the server uses its default.
func (c *Client) WatchedThreads(ctx context.Context, cursor string, limit int) (*CursorPage[gen.ThreadWatch], error) {
	params := &gen.WatchesListThreadsParams{Cursor: optString(cursor), Limit: optInt(limit)}
	resp, err := c.gen.WatchesListThreadsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*CursorPage[gen.ThreadWatch])(resp.JSON200))
}

// WatchedNodes returns one page of the nodes that the viewer watches.
// With limit 0 the server uses its default.
func (c *Client) WatchedNodes(ctx context.Context, cursor string, limit int) (*CursorPage[gen.NodeWatch], error) {
	params := &gen.WatchesListNodesParams{Cursor: optString(cursor), Limit: optInt(limit)}
	resp, err := c.gen.WatchesListNodesWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*CursorPage[gen.NodeWatch])(resp.JSON200))
}
