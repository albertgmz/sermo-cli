package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/i18n"
)

func TestRootNeedsTerminal(t *testing.T) {
	setup(t)
	r := run(t, testEnv())
	if !errors.Is(r.runErr, errReported) {
		t.Fatalf("err = %v, want errReported", r.runErr)
	}
	if !strings.Contains(r.err, i18n.T("cli.root.noTerminal")) {
		t.Fatalf("stderr has no terminal text:\n%s", r.err)
	}
}

func TestUpdateRefusesDevBuild(t *testing.T) {
	setup(t)
	r := run(t, testEnv(), "update")
	if !errors.Is(r.runErr, errReported) {
		t.Fatalf("err = %v, want errReported", r.runErr)
	}
	if !strings.Contains(r.err, i18n.T("cli.update.dev")) {
		t.Fatalf("stderr has no dev build text:\n%s", r.err)
	}
}

func TestStartLocale(t *testing.T) {
	setup(t)
	cases := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"--lang", "en", "version"}, "en"},
		{[]string{"version", "--lang=en"}, "en"},
		{[]string{"--lang"}, ""},
	}
	for _, c := range cases {
		if got := startLocale(c.args); got != c.want {
			t.Errorf("startLocale(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}
