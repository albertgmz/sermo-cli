package api

import (
	"context"
	"sync"
	"time"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// Card holds the author details of a member for the side of a post.
type Card struct {
	// GroupTitle is the user title of the display group, else the title of the display group, else the
	// title of the primary group.
	GroupTitle string
	// CustomTitle is the own title of the member. It is "" when the member has no title or cannot show one.
	CustomTitle   string
	PostCount     int
	JoinedAt      time.Time
	ReactionScore int
	// SignatureHTML is the rendered signature. It is "" when the member has no signature.
	SignatureHTML string
}

// cardCall is one profile request. Callers for the same user wait on done.
type cardCall struct {
	done chan struct{}
	card *Card
	err  error
}

// cards keeps the cards of the session and the requests in flight.
type cards struct {
	mu      sync.Mutex
	done    map[int]*Card
	pending map[int]*cardCall
}

// AuthorCard returns the card of the member userID. The card is cached for the life of c. Calls for the same
// user at the same time share one request. A guest author (userID 0) gives nil and no error. An error is not
// cached, so the next call tries again.
func (c *Client) AuthorCard(ctx context.Context, userID int) (*Card, error) {
	if userID == 0 {
		return nil, nil
	}
	call, leader := c.cards.start(userID)
	if leader {
		call.card, call.err = c.fetchCard(ctx, userID)
		c.cards.finish(userID, call)
	}
	select {
	case <-call.done:
		return call.card, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// start returns the call for userID. It reports true when the caller must make the request.
func (s *cards) start(userID int) (*cardCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if card, ok := s.done[userID]; ok {
		call := &cardCall{done: make(chan struct{}), card: card}
		close(call.done)
		return call, false
	}
	if call, ok := s.pending[userID]; ok {
		return call, false
	}
	if s.pending == nil {
		s.pending, s.done = map[int]*cardCall{}, map[int]*Card{}
	}
	call := &cardCall{done: make(chan struct{})}
	s.pending[userID] = call
	return call, true
}

// finish keeps the card of a successful call and wakes the callers that wait.
func (s *cards) finish(userID int, call *cardCall) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, userID)
	if call.err == nil {
		s.done[userID] = call.card
	}
	close(call.done)
}

func (c *Client) fetchCard(ctx context.Context, userID int) (*Card, error) {
	p, err := c.Profile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &Card{
		GroupTitle:    groupTitle(p),
		CustomTitle:   deref(p.CustomTitle),
		PostCount:     p.PostCount,
		JoinedAt:      p.CreatedAt,
		ReactionScore: p.ReactionScore,
		SignatureHTML: deref(p.SignatureHtml),
	}, nil
}

func groupTitle(p *Profile) string {
	if g := p.DisplayGroup; g != nil {
		if g.UserTitle != "" {
			return g.UserTitle
		}
		if g.Title != "" {
			return g.Title
		}
	}
	return p.GroupTitle
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// OnlinePage is the success body of OnlineList.
type OnlinePage struct {
	Items   []gen.OnlineVisitor `json:"items"`
	Page    int                 `json:"page"`
	PerPage int                 `json:"perPage"`
	Total   int                 `json:"total"`
}

// OnlineList returns the first page of the current visitors of all kinds. Total counts all of them. The value
// is not cached.
func (c *Client) OnlineList(ctx context.Context) (*OnlinePage, error) {
	resp, err := c.gen.OnlineListWithResponse(ctx, &gen.OnlineListParams{})
	if err != nil {
		return nil, err
	}
	return result(resp.HTTPResponse, resp.Body, (*OnlinePage)(resp.JSON200))
}

// SiteStats is the success body of SiteStats.
type SiteStats struct {
	LatestMember *gen.MemberCard `json:"latestMember"`
	Members      int             `json:"members"`
	Posts        int             `json:"posts"`
	Threads      int             `json:"threads"`
}

// SiteStats returns the totals of the site. The value is cached.
func (c *Client) SiteStats(ctx context.Context) (*SiteStats, error) {
	return cached(&c.cache, keyStats, func() (*SiteStats, error) {
		resp, err := c.gen.SiteStatsWithResponse(ctx)
		if err != nil {
			return nil, err
		}
		return result(resp.HTTPResponse, resp.Body, (*SiteStats)(resp.JSON200))
	})
}
