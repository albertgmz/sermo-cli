package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// signOutServer answers sign-out with status and records the Authorization
// headers that it got.
func signOutServer(t *testing.T, status int, got *[]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/me/device-login/sign-out" {
			http.NotFound(w, r)
			return
		}
		*got = append(*got, r.Header.Get("Authorization"))
		writeJSON(w, status, map[string]any{"ok": status == http.StatusOK})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func saveToken(t *testing.T, server, token string) {
	t.Helper()
	if err := auth.NewStore().Save(server, auth.Token{AccessToken: token}); err != nil {
		t.Fatal(err)
	}
}

func assertNoToken(t *testing.T, server string) {
	t.Helper()
	if _, err := auth.NewStore().Load(server); !errors.Is(err, auth.ErrNoToken) {
		t.Fatalf("token still stored: %v", err)
	}
}

func TestLogoutSignsOutAndDeletesToken(t *testing.T) {
	setup(t)
	var got []string
	srv := signOutServer(t, http.StatusOK, &got)
	saveToken(t, srv.URL, "tok-1")
	r := run(t, testEnv(), "--server", srv.URL, "logout")
	if r.runErr != nil {
		t.Fatalf("err = %v", r.runErr)
	}
	if len(got) != 1 || got[0] != "Bearer tok-1" {
		t.Fatalf("sign-out Authorization = %v", got)
	}
	assertNoToken(t, srv.URL)
	if !strings.Contains(r.out, i18n.T("cli.logout.done")) {
		t.Fatalf("output has no done text:\n%s", r.out)
	}
}

func TestLogoutUnsupportedDeletesLocallyWithHint(t *testing.T) {
	setup(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	saveToken(t, srv.URL, "tok-1")
	r := run(t, testEnv(), "--server", srv.URL, "logout")
	if r.runErr != nil {
		t.Fatalf("err = %v", r.runErr)
	}
	assertNoToken(t, srv.URL)
	if !strings.Contains(r.out, srv.URL+"/account/security") {
		t.Fatalf("output has no security page:\n%s", r.out)
	}
}

func TestLogoutWithoutStoredToken(t *testing.T) {
	setup(t)
	var got []string
	srv := signOutServer(t, http.StatusOK, &got)
	r := run(t, testEnv(), "--server", srv.URL, "logout")
	if r.runErr != nil {
		t.Fatalf("err = %v", r.runErr)
	}
	if len(got) != 0 {
		t.Fatalf("sign-out called without a token: %v", got)
	}
	if !strings.Contains(r.out, i18n.Tf("cli.logout.none", map[string]any{"Server": srv.URL})) {
		t.Fatalf("output has no none text:\n%s", r.out)
	}
}

func TestLogoutNeverSendsAPIKey(t *testing.T) {
	setup(t)
	t.Setenv("SERMO_API_KEY", "key-1")
	var got []string
	srv := signOutServer(t, http.StatusOK, &got)
	saveToken(t, srv.URL, "tok-1")
	r := run(t, testEnv(), "--server", srv.URL, "logout")
	if r.runErr != nil {
		t.Fatalf("err = %v", r.runErr)
	}
	if len(got) != 1 || got[0] != "Bearer tok-1" {
		t.Fatalf("sign-out Authorization = %v", got)
	}
	if !strings.Contains(r.out, i18n.T("cli.logout.apiKey")) {
		t.Fatalf("output has no API key note:\n%s", r.out)
	}
}

func TestLogoutAlreadyEnded(t *testing.T) {
	setup(t)
	var got []string
	srv := signOutServer(t, http.StatusUnauthorized, &got)
	saveToken(t, srv.URL, "tok-1")
	r := run(t, testEnv(), "--server", srv.URL, "logout")
	if r.runErr != nil {
		t.Fatalf("err = %v", r.runErr)
	}
	assertNoToken(t, srv.URL)
	if !strings.Contains(r.out, i18n.T("cli.logout.done")) {
		t.Fatalf("output has no done text:\n%s", r.out)
	}
}

func TestLogoutNotDeviceLoginDeletesLocallyWithHint(t *testing.T) {
	setup(t)
	var got []string
	srv := signOutServer(t, http.StatusForbidden, &got)
	saveToken(t, srv.URL, "tok-1")
	r := run(t, testEnv(), "--server", srv.URL, "logout")
	if r.runErr != nil {
		t.Fatalf("err = %v", r.runErr)
	}
	if len(got) != 1 {
		t.Fatalf("sign-out calls = %v, want 1", got)
	}
	assertNoToken(t, srv.URL)
	if !strings.Contains(r.out, srv.URL+"/account/security") {
		t.Fatalf("output has no security page:\n%s", r.out)
	}
}
