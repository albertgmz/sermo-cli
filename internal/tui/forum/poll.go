package forum

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// pollScreen shows the poll of a thread with its results and takes a vote.
type pollScreen struct {
	d        *ui.Deps
	threadID int
	sp       spinner.Model
	loading  bool
	err      error
	poll     *gen.Poll
	answers  []pollAnswer
	chosen   map[int]bool
	list     ui.List
}

// pollAnswer is one row of the list: an answer of a question.
type pollAnswer struct{ q, a int }

type pollMsg struct {
	poll *gen.Poll
	err  error
}

type votedMsg struct {
	poll *gen.Poll
	err  error
}

func newPoll(d *ui.Deps, threadID int) *pollScreen {
	return &pollScreen{d: d, threadID: threadID, sp: ui.NewSpinner(d.Styles), loading: true, chosen: map[int]bool{}}
}

func (s *pollScreen) Init() tea.Cmd       { return s.load() }
func (s *pollScreen) Title() string       { return i18n.T("forum.poll.title") }
func (s *pollScreen) CapturesInput() bool { return false }

func (s *pollScreen) Keys() []key.Binding {
	return []key.Binding{
		ui.ListKeys()[0],
		key.NewBinding(key.WithKeys("space"), key.WithHelp("Space", i18n.T("forum.key.choose"))),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", i18n.T("forum.key.vote"))),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("R", i18n.T("forum.key.retry"))),
	}
}

func (s *pollScreen) load() tea.Cmd {
	s.loading, s.err = true, nil
	c, id := s.d.API, s.threadID
	return func() tea.Msg {
		v, err := c.Poll(bg(), id)
		if err != nil {
			return pollMsg{err: err}
		}
		return pollMsg{poll: v.Poll}
	}
}

func (s *pollScreen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case pollMsg:
		s.loading = false
		s.err = msg.err
		if msg.err != nil {
			return s, ui.ErrorCmd(msg.err)
		}
		s.set(msg.poll)
	case votedMsg:
		if msg.err != nil {
			return s, ui.ErrorCmd(msg.err)
		}
		s.set(msg.poll)
		return s, ui.Toast(i18n.T("forum.toast.voted"), ui.ToastSuccess)
	case tea.KeyPressMsg:
		return s, s.key(msg)
	}
	return s, nil
}

func (s *pollScreen) set(p *gen.Poll) {
	s.poll, s.answers, s.chosen = p, nil, map[int]bool{}
	if p == nil {
		return
	}
	for qi, q := range p.Questions {
		for ai := range q.Answers {
			s.answers = append(s.answers, pollAnswer{qi, ai})
		}
	}
	s.list.SetLen(len(s.answers))
	s.list.SetHeight(len(s.answers))
}

func (s *pollScreen) key(k tea.KeyPressMsg) tea.Cmd {
	if s.err != nil {
		if k.String() == "r" {
			return s.load()
		}
		return nil
	}
	if s.poll == nil || len(s.answers) == 0 {
		return nil
	}
	switch k.String() {
	case "space":
		return s.toggle()
	case "enter":
		return s.vote()
	}
	s.list.Update(k)
	return nil
}

func (s *pollScreen) cursor() (int, int) {
	a := s.answers[s.list.Cursor()]
	return a.q, a.a
}

// toggle marks the answer at the cursor. A single choice question keeps one
// mark. A question with more choices keeps at most maxChoices marks.
func (s *pollScreen) toggle() tea.Cmd {
	qi, ai := s.cursor()
	q := s.poll.Questions[qi]
	id := q.Answers[ai].Id
	if s.chosen[id] {
		delete(s.chosen, id)
		return nil
	}
	if s.poll.MaxChoices <= 1 {
		for _, a := range q.Answers {
			delete(s.chosen, a.Id)
		}
	} else if s.countChosen(qi) >= s.poll.MaxChoices {
		return ui.Toast(i18n.Plural("forum.toast.maxChoices", s.poll.MaxChoices, nil), ui.ToastInfo)
	}
	s.chosen[id] = true
	return nil
}

func (s *pollScreen) countChosen(qi int) int {
	n := 0
	for _, a := range s.poll.Questions[qi].Answers {
		if s.chosen[a.Id] {
			n++
		}
	}
	return n
}

func (s *pollScreen) vote() tea.Cmd {
	if s.d.Me == nil {
		return loginToast()
	}
	if !s.poll.Permissions.CanVote {
		return toast("forum.toast.cannotVote")
	}
	qi, ai := s.cursor()
	q := s.poll.Questions[qi]
	var ids []int
	for _, a := range q.Answers {
		if s.chosen[a.Id] {
			ids = append(ids, a.Id)
		}
	}
	if len(ids) == 0 {
		ids = []int{q.Answers[ai].Id}
	}
	c, pollID, qid := s.d.API, s.poll.Id, q.Id
	return func() tea.Msg {
		p, err := c.Vote(bg(), pollID, qid, ids)
		return votedMsg{poll: p, err: err}
	}
}

func (s *pollScreen) View(width, height int) string {
	st, t := s.d.Styles, s.d.Theme
	switch {
	case s.loading:
		return ui.Loading(st, s.sp, "", width, height)
	case s.err != nil:
		return errorView(s.d, s.err, width, height)
	case s.poll == nil:
		return ui.Empty(st, i18n.T("forum.toast.noPoll"), width, height)
	}
	lines := []string{ui.Header(st, t, s.Title(), width), s.status()}
	row := 0
	for qi, q := range s.poll.Questions {
		lines = append(lines, "", st.Subtitle.Render(q.Text))
		total := 0
		for _, a := range q.Answers {
			if a.Votes != nil {
				total += *a.Votes
			}
		}
		for ai := range q.Answers {
			lines = append(lines, s.answerLine(qi, ai, row, total, width))
			row++
		}
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func (s *pollScreen) status() string {
	st := s.d.Styles
	parts := []string{}
	if s.poll.VoterCount != nil {
		parts = append(parts, i18n.Plural("forum.poll.voters", *s.poll.VoterCount, nil))
	}
	if s.poll.Closed {
		parts = append(parts, i18n.T("forum.poll.closed"))
	}
	if !s.poll.ResultsShown {
		parts = append(parts, i18n.T("forum.poll.hidden"))
	}
	return st.Muted.Render(strings.Join(parts, " · "))
}

// answerLine draws the mark, the text and the result bar of an answer.
func (s *pollScreen) answerLine(qi, ai, row, total, width int) string {
	st, t := s.d.Styles, s.d.Theme
	a := s.poll.Questions[qi].Answers[ai]
	cursor := "  "
	if row == s.list.Cursor() {
		cursor = st.Accent.Render("▶ ")
	}
	mark := "[ ] "
	if s.chosen[a.Id] || (a.Chosen && len(s.chosen) == 0) {
		mark = st.Accent.Render("[x] ")
	}
	label := ansi.Truncate(a.Text, max(width/3, 10), "…")
	line := cursor + mark + st.Text.Render(label)
	if a.Votes == nil {
		return line
	}
	pct := 0
	if total > 0 {
		pct = *a.Votes * 100 / total
	}
	barW := max(width-ansi.StringWidth(line)-12, 4)
	n := pct * barW / 100
	bar := t.Highlight.Text(strings.Repeat("█", n), 0) + st.Muted.Render(strings.Repeat("░", barW-n))
	return line + "  " + bar + st.Text.Render(" "+strconv.Itoa(pct)+"%")
}
