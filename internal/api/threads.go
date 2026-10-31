package api

import (
	"context"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// Threads returns one page of the threads in the node nodeID, newest activity first.
// Use prefixID 0 for all prefixes. Sticky threads come only on the first page.
func (c *Client) Threads(ctx context.Context, nodeID, prefixID int, p PageOpts) (*ThreadList, error) {
	params := &gen.ThreadsListParams{
		PrefixId: optInt(prefixID),
		Page:     optInt(p.Page),
		Cursor:   optString(p.Cursor),
		Limit:    optInt(p.Limit),
	}
	resp, err := c.gen.ThreadsListWithResponse(ctx, nodeID, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*ThreadList)(resp.JSON200))
}

// Thread returns one thread with its node, permissions, tags and watch state. The server counts a view.
func (c *Client) Thread(ctx context.Context, threadID int) (*ThreadView, error) {
	resp, err := c.gen.ThreadsGetWithResponse(ctx, threadID, &gen.ThreadsGetParams{})
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*ThreadView)(resp.JSON200))
}

// CreateThread starts a thread in the node nodeID.
func (c *Client) CreateThread(ctx context.Context, nodeID int, body gen.ThreadsCreateJSONRequestBody) (*NewThread, error) {
	resp, err := c.gen.ThreadsCreateWithResponse(ctx, nodeID, body)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*NewThread)(resp.JSON201))
}

// LocateUnread returns the first unread post of the thread, or its newest post when all are read.
func (c *Client) LocateUnread(ctx context.Context, threadID int) (*UnreadPost, error) {
	resp, err := c.gen.ThreadsLocateUnreadWithResponse(ctx, threadID)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*UnreadPost)(resp.JSON200))
}

// MarkThreadRead marks the thread as read up to the post at position. It returns the stored read position.
func (c *Client) MarkThreadRead(ctx context.Context, threadID, position int) (int, error) {
	resp, err := c.gen.ThreadsMarkReadWithResponse(ctx, threadID, gen.ThreadsMarkReadJSONRequestBody{Position: position})
	if err != nil {
		return 0, err
	}
	body, err := result(resp.HTTPResponse, resp.Body, resp.JSON200)
	if err != nil {
		return 0, err
	}
	return body.ReadPosition, nil
}

// WatchThread starts to watch the thread. With email true, the server also sends email.
func (c *Client) WatchThread(ctx context.Context, threadID int, email bool) error {
	resp, err := c.gen.ThreadsWatchWithResponse(ctx, threadID, gen.ThreadsWatchJSONRequestBody{Email: &email})
	if err != nil {
		return err
	}
	return check(resp.HTTPResponse, resp.Body)
}

// UnwatchThread stops to watch the thread.
func (c *Client) UnwatchThread(ctx context.Context, threadID int) error {
	resp, err := c.gen.ThreadsUnwatchWithResponse(ctx, threadID)
	if err != nil {
		return err
	}
	return check(resp.HTTPResponse, resp.Body)
}
