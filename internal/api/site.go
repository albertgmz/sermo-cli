package api

import (
	"context"
	"strconv"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// Me returns the signed-in user and the permissions. For a guest, Me.User is nil.
func (c *Client) Me(ctx context.Context) (*Me, error) {
	resp, err := c.gen.AuthMeWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*Me)(resp.JSON200))
}

// Counters returns the unread counts for notifications and conversations.
func (c *Client) Counters(ctx context.Context) (*Counters, error) {
	resp, err := c.gen.AuthCountersWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*Counters)(resp.JSON200))
}

// Site returns the site settings. The value is cached.
func (c *Client) Site(ctx context.Context) (*gen.Site, error) {
	return cached(&c.cache, keySite, func() (*gen.Site, error) {
		resp, err := c.gen.SiteGetWithResponse(ctx)
		if err != nil {
			return nil, err
		}
		return result(resp.HTTPResponse, resp.Body, resp.JSON200)
	})
}

// Palette returns the colors of one kind in display order. The value is cached for each kind.
func (c *Client) Palette(ctx context.Context, kind gen.PaletteColorsListParamsKind) ([]gen.PaletteColor, error) {
	return cached(&c.cache, keyPalette+string(kind), func() ([]gen.PaletteColor, error) {
		resp, err := c.gen.PaletteColorsListWithResponse(ctx, &gen.PaletteColorsListParams{Kind: kind})
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

// ReactionTypes returns all reaction types in display order. The value is cached.
func (c *Client) ReactionTypes(ctx context.Context) ([]gen.ReactionType, error) {
	return cached(&c.cache, keyReactionTypes, func() ([]gen.ReactionType, error) {
		resp, err := c.gen.ReactionTypesListWithResponse(ctx, &gen.ReactionTypesListParams{})
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

// ThreadPrefixes returns the prefixes that the node nodeID offers. The value is cached for each node.
func (c *Client) ThreadPrefixes(ctx context.Context, nodeID int) ([]gen.ThreadPrefix, error) {
	return cached(&c.cache, keyPrefixes+strconv.Itoa(nodeID), func() ([]gen.ThreadPrefix, error) {
		resp, err := c.gen.ThreadPrefixesListWithResponse(ctx, &gen.ThreadPrefixesListParams{NodeId: &nodeID})
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

// Groups returns the user groups.
func (c *Client) Groups(ctx context.Context) ([]gen.Group, error) {
	resp, err := c.gen.GroupsListWithResponse(ctx, &gen.GroupsListParams{})
	if err != nil {
		return nil, err
	}
	body, err := result(resp.HTTPResponse, resp.Body, resp.JSON200)
	if err != nil {
		return nil, err
	}
	return body.Items, nil
}
