package forum

import (
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
)

// maxWarmForums is the number of unread forums that a full prefetch warms.
const maxWarmForums = 5

// warmCmd returns a Cmd that calls fn and gives no message. fn only adds jobs to the prefetch
// queue, so the Cmd does not wait for the network. A nil fn gives a nil Cmd.
func warmCmd(fn func()) tea.Cmd {
	if fn == nil {
		return nil
	}
	return func() tea.Msg {
		fn()
		return nil
	}
}

// nextPostsWarm returns a func that warms the page after the shown page. It returns nil when the
// setting has no prefetch or the shown page is the last page.
func (r *reader) nextPostsWarm() func() {
	if !r.d.Config.LightPrefetch() || r.page >= r.pages {
		return nil
	}
	c, id, p := r.d.API, r.threadID, r.page+1
	return func() { c.WarmPosts(id, api.PageOpts{Page: p}) }
}

// warmNextPage warms the page after the shown page of the thread list.
func (s *threadList) warmNextPage() tea.Cmd {
	if !s.d.Config.LightPrefetch() || s.page >= s.pages {
		return nil
	}
	c, id, prefix, p, limit := s.d.API, s.nodeID, s.prefixID, s.page+1, s.pageLimit()
	return warmCmd(func() { c.WarmThreads(id, prefix, api.PageOpts{Page: p, Limit: limit}) })
}

// warmUnread warms the first thread page of the first unread forums in display order.
func (s *nodes) warmUnread() tea.Cmd {
	if !s.d.Config.FullPrefetch() {
		return nil
	}
	var ids []int
	for _, n := range s.items {
		if s.unread[n.Id] && len(ids) < maxWarmForums {
			ids = append(ids, n.Id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	c := s.d.API
	return warmCmd(func() {
		for _, id := range ids {
			c.WarmThreads(id, 0, api.PageOpts{Page: 1})
		}
	})
}
