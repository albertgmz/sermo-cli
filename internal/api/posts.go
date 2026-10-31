package api

import (
	"context"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// Posts returns one page of the posts of a thread in position order.
// A page from WarmPosts comes from the page cache.
func (c *Client) Posts(ctx context.Context, threadID int, p PageOpts) (*PostList, error) {
	return fromCache(c, ctx, postsKey(threadID, p), func(ctx context.Context) (*PostList, error) {
		return c.fetchPosts(ctx, threadID, p)
	})
}

func (c *Client) fetchPosts(ctx context.Context, threadID int, p PageOpts) (*PostList, error) {
	params := &gen.PostsListParams{Page: optInt(p.Page), Cursor: optString(p.Cursor), Limit: optInt(p.Limit)}
	resp, err := c.gen.PostsListWithResponse(ctx, threadID, params)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*PostList)(resp.JSON200))
}

// Post returns one post with its BB code source.
func (c *Client) Post(ctx context.Context, postID int) (*gen.PostDetail, error) {
	resp, err := c.gen.PostsGetWithResponse(ctx, postID)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, resp.JSON200)
}

// Reply adds a post with the BB code body to the thread.
func (c *Client) Reply(ctx context.Context, threadID int, body string) (*gen.PostDetail, error) {
	defer c.pages.drop(threadPages(threadID), prefixThreads)
	resp, err := c.gen.PostsCreateWithResponse(ctx, threadID, gen.PostsCreateJSONRequestBody{Body: body})
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, resp.JSON201)
}

// EditPost replaces the BB code body of a post.
func (c *Client) EditPost(ctx context.Context, postID int, body string) (*gen.PostDetail, error) {
	defer c.pages.drop(prefixPosts)
	resp, err := c.gen.PostsUpdateWithResponse(ctx, postID, gen.PostsUpdateJSONRequestBody{Body: body})
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, resp.JSON200)
}

// Preview returns the HTML of a BB code body. Make target with NewThreadTarget, ReplyTarget or MessageTarget.
func (c *Client) Preview(ctx context.Context, body string, target gen.PreviewTarget) (string, error) {
	resp, err := c.gen.PostsPreviewWithResponse(ctx, gen.PostsPreviewJSONRequestBody{Body: body, Target: target})
	if err != nil {
		return "", err
	}
	out, err := result(resp.HTTPResponse, resp.Body, resp.JSON200)
	if err != nil {
		return "", err
	}
	return out.Html, nil
}

// Poll returns the poll of a thread. PollView.Poll is nil when the thread has no poll.
func (c *Client) Poll(ctx context.Context, threadID int) (*PollView, error) {
	resp, err := c.gen.PollsGetWithResponse(ctx, threadID)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*PollView)(resp.JSON200))
}

// Vote sends the answers for one question of a poll and returns the poll after the vote.
func (c *Client) Vote(ctx context.Context, pollID, questionID int, answerIDs []int) (*gen.Poll, error) {
	defer c.pages.drop(prefixPosts)
	body := gen.PollsVoteJSONRequestBody{QuestionId: questionID, AnswerIds: answerIDs}
	resp, err := c.gen.PollsVoteWithResponse(ctx, pollID, body)
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, resp.JSON200)
}
