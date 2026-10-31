package api

import (
	"context"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// Conversations returns one page of the conversations of the viewer.
func (c *Client) Conversations(ctx context.Context, p PageOpts) (*Page[gen.ConversationSummary], error) {
	params := &gen.ConversationsListParams{Page: optInt(p.Page), Cursor: optString(p.Cursor), Limit: optInt(p.Limit)}
	resp, err := c.gen.ConversationsListWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*Page[gen.ConversationSummary])(resp.JSON200))
}

// CreateConversation starts a conversation with the recipients and a first message in BB code.
func (c *Client) CreateConversation(ctx context.Context, title, body string, recipientIDs []int) (*NewConversation, error) {
	in := gen.ConversationsCreateJSONRequestBody{Title: title, Body: body, RecipientIds: recipientIDs}
	resp, err := c.gen.ConversationsCreateWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*NewConversation)(resp.JSON201))
}

// Conversation returns one conversation with its participants.
func (c *Client) Conversation(ctx context.Context, conversationID int) (*gen.Conversation, error) {
	resp, err := c.gen.ConversationsGetWithResponse(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, resp.JSON200)
}

// Messages returns one page of the messages of a conversation.
// With newestFirst true, the list starts at the latest message and the cursor goes to older messages.
func (c *Client) Messages(ctx context.Context, conversationID int, newestFirst bool, p PageOpts) (*Page[gen.Message], error) {
	params := &gen.ConversationsListMessagesParams{Page: optInt(p.Page), Cursor: optString(p.Cursor), Limit: optInt(p.Limit)}
	if newestFirst {
		order := gen.Newest
		params.Order = &order
	}
	resp, err := c.gen.ConversationsListMessagesWithResponse(ctx, conversationID, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*Page[gen.Message])(resp.JSON200))
}

// ReplyConversation adds a message with the BB code body to the conversation.
func (c *Client) ReplyConversation(ctx context.Context, conversationID int, body string) (*gen.Message, error) {
	resp, err := c.gen.ConversationsReplyWithResponse(ctx, conversationID, gen.ConversationsReplyJSONRequestBody{Body: body})
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, resp.JSON201)
}

// LocateUnreadMessage returns the page and the cursor of the first unread message.
// The page size is limit. With limit 0 the server uses 20. It does not mark a message as read.
func (c *Client) LocateUnreadMessage(ctx context.Context, conversationID, limit int) (*gen.MessageLocation, error) {
	params := &gen.ConversationsLocateUnreadParams{Limit: optInt(limit)}
	resp, err := c.gen.ConversationsLocateUnreadWithResponse(ctx, conversationID, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, resp.JSON200)
}

// MarkConversationRead marks the conversation as read.
func (c *Client) MarkConversationRead(ctx context.Context, conversationID int) error {
	resp, err := c.gen.ConversationsMarkReadWithResponse(ctx, conversationID, gen.ConversationsMarkReadJSONRequestBody{})
	if err != nil {
		return err
	}
	return check(resp.HTTPResponse, resp.Body)
}

// QuoteMessage returns the author and the BB code source of a message, for a quote.
func (c *Client) QuoteMessage(ctx context.Context, messageID int) (*gen.MessageQuote, error) {
	resp, err := c.gen.ConversationsQuoteMessageWithResponse(ctx, messageID)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, resp.JSON200)
}
