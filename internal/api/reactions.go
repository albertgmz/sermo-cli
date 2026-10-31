package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// ContentType is the kind of item that a reaction is on.
type ContentType string

// The content types that take a reaction.
const (
	ContentPost           ContentType = "post"
	ContentProfilePost    ContentType = "profile_post"
	ContentProfileComment ContentType = "profile_post_comment"
	ContentMessage        ContentType = "conversation_message"
)

// Reaction is one reaction of one user on an item.
type Reaction struct {
	User           gen.UserSummary `json:"user"`
	ReactionTypeId int             `json:"reactionTypeId"`
	CreatedAt      time.Time       `json:"createdAt"`
}

func reactionPath(ct ContentType, contentID int) string {
	return fmt.Sprintf("/api/v1/reactions/%s/%d", url.PathEscape(string(ct)), contentID)
}

// Reactions returns one page of the users that reacted to an item, and their reaction types.
// With limit 0 the server uses 20.
func (c *Client) Reactions(ctx context.Context, ct ContentType, contentID int, cursor string, limit int) (*CursorPage[Reaction], error) {
	q := url.Values{}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if limit != 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := reactionPath(ct, contentID)
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out CursorPage[Reaction]
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetReaction gives the reaction type reactionTypeID to an item, or changes the reaction of the viewer.
// It returns the reaction counts of the item after the change.
func (c *Client) SetReaction(ctx context.Context, ct ContentType, contentID, reactionTypeID int) (*gen.ReactionSummary, error) {
	in := struct {
		ReactionTypeID int `json:"reactionTypeId"`
	}{reactionTypeID}
	defer c.pages.drop(prefixPosts, prefixMessages)
	var out gen.ReactionSummary
	if err := c.doJSON(ctx, http.MethodPut, reactionPath(ct, contentID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveReaction removes the reaction of the viewer from an item.
// It returns the reaction counts of the item after the change.
func (c *Client) RemoveReaction(ctx context.Context, ct ContentType, contentID int) (*gen.ReactionSummary, error) {
	defer c.pages.drop(prefixPosts, prefixMessages)
	var out gen.ReactionSummary
	if err := c.doJSON(ctx, http.MethodDelete, reactionPath(ct, contentID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
