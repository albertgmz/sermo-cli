package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/auth"
)

func TestLoginErrorsShowFriendlyTextAndRetry(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"invalid grant", fmt.Errorf("%w: invalid_grant: x", auth.ErrInvalidGrant), "El código ya no sirve. Empieza otra vez."},
		{"banned", auth.ErrBanned, "Tu cuenta está suspendida, así que no puedes entrar."},
		{"rate limit", &auth.RateLimitError{Wait: 12500 * time.Millisecond}, "Espera 13 s"},
	}
	for _, c := range cases {
		f := newFakeSermo(t)
		d := testDeps(t, f, "")
		newModel(context.Background(), d, Options{})
		l := newLogin(context.Background(), d, "", noSleep)
		l.start()
		l.Update(loginFailedMsg{from: l, err: c.err})
		out := ansi.Strip(l.View(76, 20))
		if !strings.Contains(out, c.want) || !strings.Contains(out, "[Enter] Intentar otra vez") {
			t.Errorf("%s: view misses %q or the retry hint:\n%s", c.name, c.want, out)
		}
		if strings.Contains(out, "dispositivo") {
			t.Errorf("%s: the login text must not talk about the device session:\n%s", c.name, out)
		}
	}
}
