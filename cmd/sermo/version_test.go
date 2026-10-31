package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/update"
)

func specServer(t *testing.T, version string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/openapi.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"openapi":"3.1.0","info":{"title":"Sermo API","version":"` + version + `"}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestVersionWarnsWhenServerIsNewer(t *testing.T) {
	setup(t)
	srv := specServer(t, "0.2.0")
	r := run(t, testEnv(), "--server", srv.URL, "version")
	if r.runErr != nil {
		t.Fatalf("err = %v", r.runErr)
	}
	for _, want := range []string{
		srv.URL,
		i18n.Tf("cli.version.sermo", map[string]any{"Version": "0.2.0"}),
		i18n.Tf("cli.version.spec", map[string]any{"Version": api.SpecVersion}),
		i18n.T("cli.version.newerServer"),
	} {
		if !strings.Contains(r.out, want) {
			t.Errorf("output has no %q:\n%s", want, r.out)
		}
	}
}

func TestVersionNoWarningWhenServerMatchesSpec(t *testing.T) {
	setup(t)
	srv := specServer(t, api.SpecVersion)
	r := run(t, testEnv(), "--server", srv.URL, "version")
	if strings.Contains(r.out, i18n.T("cli.version.newerServer")) {
		t.Fatalf("unexpected warning:\n%s", r.out)
	}
}

func TestVersionUnknownServerVersion(t *testing.T) {
	setup(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	r := run(t, testEnv(), "--server", srv.URL, "version")
	if r.runErr != nil {
		t.Fatalf("err = %v", r.runErr)
	}
	want := i18n.Tf("cli.version.unknown", map[string]any{"Reason": i18n.T("errors.notFound")})
	if !strings.Contains(r.out, want) {
		t.Fatalf("output has no %q:\n%s", want, r.out)
	}
}

func TestVersionShowsNewerRelease(t *testing.T) {
	setup(t)
	srv := specServer(t, api.SpecVersion)
	e := testEnv()
	e.checkUpdate = func(context.Context, string) (update.Result, error) {
		return update.Result{Current: "1.0.0", Latest: "1.1.0"}, nil
	}
	r := run(t, e, "--server", srv.URL, "version")
	want := i18n.Tf("cli.version.newRelease", map[string]any{"Version": "1.1.0"})
	if !strings.Contains(r.out, want) {
		t.Fatalf("output has no %q:\n%s", want, r.out)
	}
}

func TestServerFlagBeatsEnvironment(t *testing.T) {
	setup(t)
	fromEnv := specServer(t, "0.1.0")
	fromFlag := specServer(t, "0.1.1")
	t.Setenv("SERMO_SERVER", fromEnv.URL)
	r := run(t, testEnv(), "--server", fromFlag.URL, "version")
	if !strings.Contains(r.out, fromFlag.URL) || strings.Contains(r.out, fromEnv.URL) {
		t.Fatalf("flag server not used:\n%s", r.out)
	}
}

func TestEnvironmentServerUsedWithoutFlag(t *testing.T) {
	setup(t)
	fromEnv := specServer(t, "0.1.0")
	t.Setenv("SERMO_SERVER", fromEnv.URL+"/")
	r := run(t, testEnv(), "version")
	want := i18n.Tf("cli.version.sermo", map[string]any{"Version": "0.1.0"})
	if !strings.Contains(r.out, fromEnv.URL) || !strings.Contains(r.out, want) {
		t.Fatalf("environment server not used:\n%s", r.out)
	}
}

func TestLangFlagSelectsEnglish(t *testing.T) {
	setup(t)
	srv := specServer(t, api.SpecVersion)
	r := run(t, testEnv(), "--lang", "en", "--server", srv.URL, "version")
	if !strings.Contains(r.out, "Server: "+srv.URL) {
		t.Fatalf("output is not English:\n%s", r.out)
	}
}

func TestInvalidServerIsRefused(t *testing.T) {
	setup(t)
	r := run(t, testEnv(), "--server", "ftp://example.com", "version")
	if r.runErr == nil {
		t.Fatalf("no error for an ftp server, output:\n%s", r.out)
	}
}
