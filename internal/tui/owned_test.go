package tui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// slowLoadedMsg is the result of the load of a slowScreen.
type slowLoadedMsg struct{}

// slowScreen loads when release closes. A recorder screen records the
// messages that it gets.
type slowScreen struct {
	release chan struct{}
	loaded  bool
	got     []tea.Msg
}

func newSlowScreen() *slowScreen { return &slowScreen{release: make(chan struct{})} }

func (s *slowScreen) Init() tea.Cmd {
	release := s.release
	return func() tea.Msg {
		<-release
		return slowLoadedMsg{}
	}
}

func (s *slowScreen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	if _, ok := msg.(slowLoadedMsg); ok {
		s.loaded = true
	}
	s.got = append(s.got, msg)
	return s, nil
}

func (s *slowScreen) View(int, int) string {
	if s.loaded {
		return "LOADED"
	}
	return "WAITING"
}

func (s *slowScreen) Title() string       { return "slow" }
func (s *slowScreen) Keys() []key.Binding { return nil }
func (s *slowScreen) CapturesInput() bool { return false }

func (s *slowScreen) gotLoad() bool { return hasMsg[slowLoadedMsg](s.got) }

func TestResultReachesItsScreenUnderAnotherScreen(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, ""), StartHome)
	drive(m, m.Init(), wait)
	slow, top := newSlowScreen(), newSlowScreen()
	_, load := m.Update(ui.PushMsg{Screen: slow})
	m.Update(ui.PushMsg{Screen: top})
	close(slow.release)
	drive(m, load, wait)
	if top.gotLoad() {
		t.Fatal("the top screen got the result of the screen under it")
	}
	if !slow.loaded {
		t.Fatal("the result did not reach the screen that asked for it")
	}
	pressKey(m, "esc")
	if out := screenText(m); !strings.Contains(out, "LOADED") {
		t.Fatalf("after pop the screen must show the loaded state:\n%s", out)
	}
}

func TestResultOfPoppedScreenIsDropped(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, ""), StartHome)
	drive(m, m.Init(), wait)
	under, slow := newSlowScreen(), newSlowScreen()
	m.Update(ui.PushMsg{Screen: under})
	_, load := m.Update(ui.PushMsg{Screen: slow})
	pressKey(m, "esc")
	close(slow.release)
	drive(m, load, wait)
	if slow.loaded || under.gotLoad() {
		t.Fatal("a result of a popped screen must be dropped")
	}
	if m.top() != under {
		t.Fatal("the stack changed")
	}
}

// gatedForum serves a forum whose node call waits until release closes.
func gatedForum(t *testing.T, release <-chan struct{}) *fakeSermo {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join("forum", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	node, threads := read("node.json"), read("threads.json")
	serve := func(body []byte) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/site", serve([]byte(`{"name":"Foros Cuba"}`)))
	mux.HandleFunc("GET /api/v1/nodes", serve([]byte(`{"items":[]}`)))
	mux.HandleFunc("GET /api/v1/thread-prefixes", serve([]byte(`{"items":[]}`)))
	mux.HandleFunc("GET /api/v1/nodes/44/threads", serve(threads))
	var once sync.Once
	mux.HandleFunc("GET /api/v1/nodes/44", func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { <-release })
		serve(node)(w, r)
	})
	f := &fakeSermo{Server: httptest.NewServer(mux)}
	t.Cleanup(f.Close)
	return f
}

func TestForumLoadsWhileHelpShows(t *testing.T) {
	release := make(chan struct{})
	f := gatedForum(t, release)
	m := newTestModel(t, testDeps(t, f, ""), StartHome)
	drive(m, m.Init(), wait)
	_, load := m.Update(ui.OpenNodeMsg{NodeID: 44})
	pressKey(m, "?")
	if _, ok := m.top().(*helpScreen); !ok {
		t.Fatalf("? must open help, top is %T", m.top())
	}
	close(release)
	drive(m, load, wait)
	pressKey(m, "esc")
	if out := screenText(m); !strings.Contains(out, "Ay bbs pq dejaron morir esto?") {
		t.Fatalf("after help the forum must show its threads:\n%s", out)
	}
}

func TestPopTellsTheNewTopScreen(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, ""), StartHome)
	under, over := newSlowScreen(), newSlowScreen()
	m.Update(ui.PushMsg{Screen: under})
	m.Update(ui.PushMsg{Screen: over})
	if hasMsg[ui.ResumedMsg](under.got) {
		t.Fatal("a push must not resume the screen under it")
	}
	pressKey(m, "esc")
	if !hasMsg[ui.ResumedMsg](under.got) || hasMsg[ui.ResumedMsg](over.got) {
		t.Fatal("after a pop only the new top screen must get ResumedMsg")
	}
}
