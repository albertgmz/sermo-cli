package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func signOutServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/auth/sign-out" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok-1" {
			t.Errorf("Authorization = %q", got)
		}
		checkHeaders(t, r)
		if b, _ := io.ReadAll(r.Body); string(b) != "{}" {
			t.Errorf("body = %q, want {}", b)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSignOutSuccess(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNoContent, http.StatusUnauthorized} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := signOutServer(t, status)
			if err := SignOut(context.Background(), http.DefaultClient, srv.URL, "test-agent", "tok-1"); err != nil {
				t.Fatalf("SignOut() error = %v", err)
			}
		})
	}
}

func TestSignOutUnsupported(t *testing.T) {
	srv := signOutServer(t, http.StatusNotFound)
	err := SignOut(context.Background(), http.DefaultClient, srv.URL, "test-agent", "tok-1")
	if !errors.Is(err, ErrSignOutUnsupported) {
		t.Fatalf("SignOut() error = %v, want ErrSignOutUnsupported", err)
	}
}

func TestSignOutFailure(t *testing.T) {
	srv := signOutServer(t, http.StatusInternalServerError)
	err := SignOut(context.Background(), http.DefaultClient, srv.URL, "test-agent", "tok-1")
	if err == nil || errors.Is(err, ErrSignOutUnsupported) {
		t.Fatalf("SignOut() error = %v, want a generic error", err)
	}
}
