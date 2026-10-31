package api

import (
	"context"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// Profile returns the profile of the user userID.
func (c *Client) Profile(ctx context.Context, userID int) (*Profile, error) {
	resp, err := c.gen.ProfilesGetWithResponse(ctx, userID)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*Profile)(resp.JSON200))
}

// SearchUsers returns the users whose username starts with prefix.
// With limit 0 the server uses its default.
func (c *Client) SearchUsers(ctx context.Context, prefix string, limit int) ([]UserMatch, error) {
	resp, err := c.gen.UsersSearchWithResponse(ctx, &gen.UsersSearchParams{Prefix: prefix, Limit: optInt(limit)})
	if err != nil {
		return nil, err
	}
	out, err := result(resp.HTTPResponse, resp.Body, resp.JSON200)
	if err != nil {
		return nil, err
	}
	users := make([]UserMatch, len(out.Items))
	for i, u := range out.Items {
		users[i] = UserMatch{Id: u.Id, Staff: u.Staff, Username: u.Username}
	}
	return users, nil
}

// ProfilePosts returns one page of the posts on the profile wall of the user userID.
// With limit 0 the server uses its default.
func (c *Client) ProfilePosts(ctx context.Context, userID int, cursor string, limit int) (*CursorPage[gen.ProfilePost], error) {
	params := &gen.ProfilePostsListParams{Cursor: optString(cursor), Limit: optInt(limit)}
	resp, err := c.gen.ProfilePostsListWithResponse(ctx, userID, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*CursorPage[gen.ProfilePost])(resp.JSON200))
}

// CreateProfilePost adds a post with the BB code body to the profile wall of the user userID.
func (c *Client) CreateProfilePost(ctx context.Context, userID int, body string) (*gen.ProfilePostDetail, error) {
	resp, err := c.gen.ProfilePostsCreateWithResponse(ctx, userID, gen.ProfilePostsCreateJSONRequestBody{Body: body})
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, resp.JSON201)
}

// ProfileComments returns one page of the comments on a profile post.
// With limit 0 the server uses its default.
func (c *Client) ProfileComments(ctx context.Context, profilePostID int, cursor string, limit int) (*CursorPage[gen.ProfileComment], error) {
	params := &gen.ProfileCommentsListParams{Cursor: optString(cursor), Limit: optInt(limit)}
	resp, err := c.gen.ProfileCommentsListWithResponse(ctx, profilePostID, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*CursorPage[gen.ProfileComment])(resp.JSON200))
}

// CreateProfileComment adds a comment with the BB code body to a profile post.
func (c *Client) CreateProfileComment(ctx context.Context, profilePostID int, body string) (*gen.ProfileCommentDetail, error) {
	in := gen.ProfileCommentsCreateJSONRequestBody{Body: body}
	resp, err := c.gen.ProfileCommentsCreateWithResponse(ctx, profilePostID, in)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, resp.JSON201)
}
