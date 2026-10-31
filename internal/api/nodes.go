package api

import (
	"context"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// Nodes returns the node tree as a flat list in display order. The value is cached.
// MarkNodeRead and MarkAllRead remove it from the cache.
func (c *Client) Nodes(ctx context.Context) ([]gen.Node, error) {
	return cached(&c.cache, keyNodes, func() ([]gen.Node, error) {
		resp, err := c.gen.NodesListWithResponse(ctx, &gen.NodesListParams{})
		if err != nil {
			return nil, err
		}
		body, err := result(resp.HTTPResponse, resp.Body, resp.JSON200)
		if err != nil {
			return nil, err
		}
		return body.Items, nil
	})
}

// Node returns one node with its breadcrumbs and the watch state of the viewer.
func (c *Client) Node(ctx context.Context, nodeID int) (*NodeView, error) {
	resp, err := c.gen.NodesGetWithResponse(ctx, nodeID, &gen.NodesGetParams{})
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*NodeView)(resp.JSON200))
}

// UnreadNodeIDs returns the IDs of the forums that hold an unread thread. The value is not cached.
func (c *Client) UnreadNodeIDs(ctx context.Context) ([]int, error) {
	resp, err := c.gen.NodesUnreadWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	body, err := result(resp.HTTPResponse, resp.Body, resp.JSON200)
	if err != nil {
		return nil, err
	}
	return body.NodeIds, nil
}

// MarkNodeRead marks the node nodeID and the nodes below it as read.
func (c *Client) MarkNodeRead(ctx context.Context, nodeID int) error {
	resp, err := c.gen.NodesMarkReadWithResponse(ctx, nodeID, gen.NodesMarkReadJSONRequestBody{})
	if err != nil {
		return err
	}
	c.cache.drop(keyNodes)
	return check(resp.HTTPResponse, resp.Body)
}

// MarkAllRead marks all the forums that the viewer can see as read.
func (c *Client) MarkAllRead(ctx context.Context) error {
	resp, err := c.gen.NodesMarkAllReadWithResponse(ctx, gen.NodesMarkAllReadJSONRequestBody{})
	if err != nil {
		return err
	}
	c.cache.drop(keyNodes)
	return check(resp.HTTPResponse, resp.Body)
}

// WatchNode starts to watch the node nodeID, or changes the mode and the email setting.
func (c *Client) WatchNode(ctx context.Context, nodeID int, mode gen.NodesWatchJSONBodyMode, email bool) error {
	body := gen.NodesWatchJSONRequestBody{Mode: &mode, Email: &email}
	resp, err := c.gen.NodesWatchWithResponse(ctx, nodeID, body)
	if err != nil {
		return err
	}
	return check(resp.HTTPResponse, resp.Body)
}

// UnwatchNode stops to watch the node nodeID.
func (c *Client) UnwatchNode(ctx context.Context, nodeID int) error {
	resp, err := c.gen.NodesUnwatchWithResponse(ctx, nodeID)
	if err != nil {
		return err
	}
	return check(resp.HTTPResponse, resp.Body)
}
