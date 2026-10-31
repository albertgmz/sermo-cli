package api

import (
	"context"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// Notifications returns one page of the notifications of the viewer, most recently updated first.
func (c *Client) Notifications(ctx context.Context, unreadOnly bool, p PageOpts) (*NotificationList, error) {
	params := &gen.NotificationsListParams{
		UnreadOnly: optBool(unreadOnly),
		Page:       optInt(p.Page),
		Cursor:     optString(p.Cursor),
		Limit:      optInt(p.Limit),
	}
	resp, err := c.gen.NotificationsListWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*NotificationList)(resp.JSON200))
}

// MarkNotificationsRead marks the notifications ids as read. With no ids, it marks all of them.
// It returns the number of unread notifications that stay.
func (c *Client) MarkNotificationsRead(ctx context.Context, ids ...int) (int, error) {
	body := gen.NotificationsMarkReadJSONRequestBody{Ids: &ids}
	if len(ids) == 0 {
		all := true
		body = gen.NotificationsMarkReadJSONRequestBody{All: &all}
	}
	resp, err := c.gen.NotificationsMarkReadWithResponse(ctx, body)
	if err != nil {
		return 0, err
	}
	out, err := result(resp.HTTPResponse, resp.Body, resp.JSON200)
	if err != nil {
		return 0, err
	}
	return out.UnreadCount, nil
}

// MarkNotificationsViewed tells the server that the viewer saw the notification list.
func (c *Client) MarkNotificationsViewed(ctx context.Context) error {
	resp, err := c.gen.NotificationsMarkViewedWithResponse(ctx, gen.NotificationsMarkViewedJSONRequestBody{})
	if err != nil {
		return err
	}
	return check(resp.HTTPResponse, resp.Body)
}
