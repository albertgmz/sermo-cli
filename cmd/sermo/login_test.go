package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/i18n"
)

const userCode = "ABCD-EFGH"

// deviceServer serves the device endpoints and /api/v1/me. The token
// endpoint answers with the error codes in sequence and then with a token.
type deviceServer struct {
	mu      sync.Mutex
	pending []string
	srv     *httptest.Server
}

func newDeviceServer(t *testing.T, errorCodes ...string) *deviceServer {
	t.Helper()
	d := &deviceServer{pending: errorCodes}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/device/code", func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		writeJSON(w, http.StatusOK, map[string]any{
			"device_code": "dev-1", "user_code": userCode,
			"verification_uri":          base + "/device",
			"verification_uri_complete": base + "/device?user_code=" + userCode,
			"expires_in":                600, "interval": 1,
		})
	})
	mux.HandleFunc("POST /api/auth/device/token", d.token)
	mux.HandleFunc("GET /api/v1/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok-1" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"code": "UNAUTHORIZED", "message": "no"}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": map[string]any{"id": 7, "username": "Ana"}})
	})
	d.srv = httptest.NewServer(mux)
	t.Cleanup(d.srv.Close)
	return d
}

func (d *deviceServer) token(w http.ResponseWriter, _ *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pending) > 0 {
		code := d.pending[0]
		d.pending = d.pending[1:]
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": code, "error_description": code})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": "tok-1", "token_type": "Bearer"})
}

func TestLoginSavesTokenAndGreets(t *testing.T) {
	setup(t)
	d := newDeviceServer(t, "authorization_pending")
	e := testEnv()
	var opened string
	e.openURL = func(u string) error { opened = u; return nil }
	r := run(t, e, "--server", d.srv.URL, "login")
	if r.runErr != nil {
		t.Fatalf("err = %v\nstderr: %s", r.runErr, r.err)
	}
	complete := d.srv.URL + "/device?user_code=" + userCode
	for _, want := range []string{userCode, complete, i18n.Tf("cli.login.welcome", map[string]any{"Name": "Ana", "Server": d.srv.URL})} {
		if !strings.Contains(r.out, want) {
			t.Errorf("output has no %q:\n%s", want, r.out)
		}
	}
	if opened != complete {
		t.Errorf("opened %q, want %q", opened, complete)
	}
	tok, err := auth.NewStore().Load(d.srv.URL)
	if err != nil || tok.AccessToken != "tok-1" {
		t.Fatalf("stored token = %+v, %v", tok, err)
	}
}

func TestLoginContinuesWhenBrowserFails(t *testing.T) {
	setup(t)
	d := newDeviceServer(t)
	e := testEnv()
	e.openURL = func(string) error { return errors.New("no browser") }
	r := run(t, e, "--server", d.srv.URL, "login")
	if r.runErr != nil {
		t.Fatalf("err = %v", r.runErr)
	}
	if !strings.Contains(r.out, i18n.T("cli.login.browserFailed")) {
		t.Fatalf("output has no browser hint:\n%s", r.out)
	}
}

func TestLoginUnsupportedMentionsAPIKey(t *testing.T) {
	setup(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	r := run(t, testEnv(), "--server", srv.URL, "login")
	if !errors.Is(r.runErr, errReported) {
		t.Fatalf("err = %v, want errReported", r.runErr)
	}
	if !strings.Contains(r.err, "SERMO_API_KEY") {
		t.Fatalf("stderr has no SERMO_API_KEY:\n%s", r.err)
	}
}

func TestLoginFailures(t *testing.T) {
	cases := []struct{ code, msgID string }{
		{"expired_token", "cli.login.expired"},
		{"access_denied", "cli.login.denied"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			setup(t)
			d := newDeviceServer(t, c.code)
			r := run(t, testEnv(), "--server", d.srv.URL, "login")
			if !errors.Is(r.runErr, errReported) {
				t.Fatalf("err = %v, want errReported", r.runErr)
			}
			if !strings.Contains(r.err, i18n.T(c.msgID)) {
				t.Fatalf("stderr has no %q:\n%s", i18n.T(c.msgID), r.err)
			}
			if _, err := auth.NewStore().Load(d.srv.URL); !errors.Is(err, auth.ErrNoToken) {
				t.Fatalf("token stored after failure: %v", err)
			}
		})
	}
}

func TestLoginCancel(t *testing.T) {
	setup(t)
	d := newDeviceServer(t, "authorization_pending")
	e := testEnv()
	e.sleep = func(context.Context, time.Duration) error { return context.Canceled }
	r := run(t, e, "--server", d.srv.URL, "login")
	if !errors.Is(r.runErr, errReported) {
		t.Fatalf("err = %v, want errReported", r.runErr)
	}
	if !strings.Contains(r.err, i18n.T("cli.login.canceled")) {
		t.Fatalf("stderr has no cancel text:\n%s", r.err)
	}
}

func TestLoginNotesAPIKey(t *testing.T) {
	setup(t)
	t.Setenv("SERMO_API_KEY", "key-1")
	d := newDeviceServer(t)
	r := run(t, testEnv(), "--server", d.srv.URL, "login")
	if r.runErr != nil {
		t.Fatalf("err = %v", r.runErr)
	}
	if !strings.Contains(r.out, i18n.T("cli.login.apiKey")) {
		t.Fatalf("output has no API key note:\n%s", r.out)
	}
}

func TestLoginInvalidGrant(t *testing.T) {
	setup(t)
	d := newDeviceServer(t, "invalid_grant")
	r := run(t, testEnv(), "--server", d.srv.URL, "login")
	if !errors.Is(r.runErr, errReported) {
		t.Fatalf("err = %v, want errReported", r.runErr)
	}
	if !strings.Contains(r.err, i18n.T("cli.login.invalidGrant")) {
		t.Fatalf("stderr has no invalid grant text:\n%s", r.err)
	}
}

// codeServer answers the device code request with code and the token poll with poll.
func codeServer(t *testing.T, code, poll http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/device/code", code)
	mux.HandleFunc("POST /api/auth/device/token", poll)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestLoginBanned(t *testing.T) {
	setup(t)
	srv := codeServer(t,
		func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"device_code": "dev-1", "user_code": userCode, "verification_uri": "http://x/device",
				"expires_in": 600, "interval": 1,
			})
		},
		func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusForbidden, map[string]any{"message": "This account is banned.", "code": "ACCOUNT_BANNED"})
		})
	r := run(t, testEnv(), "--server", srv.URL, "login")
	if !errors.Is(r.runErr, errReported) {
		t.Fatalf("err = %v, want errReported", r.runErr)
	}
	if !strings.Contains(r.err, i18n.T("cli.login.banned")) {
		t.Fatalf("stderr has no banned text:\n%s", r.err)
	}
}

func TestLoginRateLimitedTellsWait(t *testing.T) {
	setup(t)
	srv := codeServer(t,
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Retry-After", "42")
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"message": "Too many requests."})
		},
		func(w http.ResponseWriter, _ *http.Request) {
			t.Error("token poll after a refused code request")
		})
	r := run(t, testEnv(), "--server", srv.URL, "login")
	if !errors.Is(r.runErr, errReported) {
		t.Fatalf("err = %v, want errReported", r.runErr)
	}
	want := i18n.Tf("cli.login.rateLimited", map[string]any{"Seconds": 42})
	if !strings.Contains(r.err, want) || !strings.Contains(want, "42") {
		t.Fatalf("stderr has no %q:\n%s", want, r.err)
	}
}
