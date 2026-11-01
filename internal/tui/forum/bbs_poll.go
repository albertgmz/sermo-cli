package forum

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// Prompt is the label of the BBS prompt.
func (s *pollScreen) Prompt() string { return i18n.T("bbs.poll.prompt") }

// Commands are the BBS commands of the poll: a number marks an answer and
// Enter sends the vote.
func (s *pollScreen) Commands() []ui.Command {
	return []ui.Command{
		{Name: "#", Label: i18n.T("bbs.poll.mark"), Run: s.mark},
		{Name: "", Label: i18n.T("bbs.poll.vote"), Run: func(string) tea.Cmd {
			if s.poll == nil || len(s.answers) == 0 {
				return nil
			}
			return s.vote()
		}},
		{Name: "V", Label: i18n.T("bbs.global.back")},
	}
}

// mark marks the answer with the number arg, like Space on the answer.
func (s *pollScreen) mark(arg string) tea.Cmd {
	if s.poll == nil {
		return nil
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > len(s.answers) {
		return toast("bbs.poll.noAnswer")
	}
	s.list.Select(n - 1)
	return s.toggle()
}

// bbsView shows the questions with numbered answers and bars of #.
func (s *pollScreen) bbsView(width, height int) string {
	st := s.d.Styles
	barW := max(width/4, 10)
	t := ui.NewTable(width, ui.Column{Title: i18n.T("bbs.help.number"), Width: 2, Align: ui.AlignRight},
		ui.Column{Title: i18n.T("bbs.poll.answer")}, ui.Column{Title: i18n.T("bbs.poll.result"), Width: barW + 5})
	t.Full(st.Subtitle.Render(s.Title()))
	if status := s.status(); status != "" {
		t.Full(status)
	}
	t.Header()
	row := 0
	for qi, q := range s.poll.Questions {
		t.Section(q.Text)
		row = s.answerRows(t, qi, row, barW)
	}
	lines := strings.Split(t.Render(st), "\n")
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

// answerRows adds the rows of the answers of question qi to t. The first row has
// the number row+1. It returns the number of the next row.
func (s *pollScreen) answerRows(t *ui.Table, qi, row, barW int) int {
	q := s.poll.Questions[qi]
	total := 0
	for _, a := range q.Answers {
		if a.Votes != nil {
			total += *a.Votes
		}
	}
	for _, a := range q.Answers {
		mark := "[ ] "
		if s.chosen[a.Id] || (a.Chosen && len(s.chosen) == 0) {
			mark = "[x] "
		}
		result := ""
		if a.Votes != nil {
			result = asciiBar(*a.Votes, total, barW)
		}
		t.Row(ui.Num(row+1, len(s.answers)), mark+a.Text, result).Zone(s.d, s.list.RowZone(row))
		if row == s.list.Cursor() {
			t.Select()
		}
		row++
	}
	return row
}

// asciiBar draws a bar of w cells for votes of total and the percent.
func asciiBar(votes, total, w int) string {
	pct := 0
	if total > 0 {
		pct = votes * 100 / total
	}
	n := pct * w / 100
	return strings.Repeat("#", n) + strings.Repeat(".", w-n) + fmt.Sprintf(" %3d%%", pct)
}
