package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestTransportSendsAuthHeaderWithToken(t *testing.T) {
	var auth, ua, accept string
	c, _ := newTestClient(t, "tok123", func(w http.ResponseWriter, r *http.Request) {
		auth, ua, accept = r.Header.Get("Authorization"), r.Header.Get("User-Agent"), r.Header.Get("Accept")
		writeJSON(w, http.StatusOK, siteJSON)
	})
	if _, err := c.Site(context.Background()); err != nil {
		t.Fatalf("Site: %v", err)
	}
	if auth != "Bearer tok123" {
		t.Errorf("Authorization = %q, want %q", auth, "Bearer tok123")
	}
	if ua != "sermo-cli/test" {
		t.Errorf("User-Agent = %q, want %q", ua, "sermo-cli/test")
	}
	if accept != "application/json" {
		t.Errorf("Accept = %q, want application/json", accept)
	}
}

func TestTransportOmitsAuthHeaderForGuest(t *testing.T) {
	var auth []string
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Values("Authorization")
		writeJSON(w, http.StatusOK, siteJSON)
	})
	if _, err := c.Site(context.Background()); err != nil {
		t.Fatalf("Site: %v", err)
	}
	if len(auth) != 0 {
		t.Errorf("Authorization = %q, want none", auth)
	}
}

func TestTransportRetriesOnceAfterShortWait(t *testing.T) {
	var calls atomic.Int32
	c, s := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "3")
			writeJSON(w, http.StatusTooManyRequests, `{"error":{"code":"rate_limited","message":"slow"}}`)
			return
		}
		writeJSON(w, http.StatusOK, siteJSON)
	})
	site, err := c.Site(context.Background())
	if err != nil {
		t.Fatalf("Site: %v", err)
	}
	if site.Name != "FCuba" {
		t.Errorf("Name = %q", site.Name)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("calls = %d, want 2", n)
	}
	if got := s.got(); len(got) != 1 || got[0] != 3*time.Second {
		t.Errorf("waits = %v, want [3s]", got)
	}
}

func TestTransportRetriesOnlyOnce(t *testing.T) {
	var calls atomic.Int32
	c, s := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusTooManyRequests, `{"error":{"code":"rate_limited","message":"slow"}}`)
	})
	_, err := c.Site(context.Background())
	if !IsRateLimited(err) {
		t.Fatalf("err = %v, want rate limited", err)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("calls = %d, want 2", n)
	}
	if got := s.got(); len(got) != 1 {
		t.Errorf("waits = %v, want 1 wait", got)
	}
}

func TestTransportReturnsLongWaitAsError(t *testing.T) {
	var calls atomic.Int32
	c, s := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, `{"error":{"code":"rate_limited","message":"Too many requests"}}`)
	})
	_, err := c.Site(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *Error", err)
	}
	if !IsRateLimited(err) || apiErr.RetryAfter != time.Minute {
		t.Errorf("err = %+v, want 429 with RetryAfter 1m", apiErr)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("calls = %d, want 1", n)
	}
	if got := s.got(); len(got) != 0 {
		t.Errorf("waits = %v, want none", got)
	}
}

func TestTransportReadsHTTPDateRetryAfter(t *testing.T) {
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", time.Now().Add(2*time.Hour).UTC().Format(http.TimeFormat))
		writeJSON(w, http.StatusTooManyRequests, `{"error":{"code":"rate_limited","message":"m"}}`)
	})
	_, err := c.Site(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *Error", err)
	}
	if apiErr.RetryAfter < 119*time.Minute || apiErr.RetryAfter > 2*time.Hour {
		t.Errorf("RetryAfter = %v, want about 2h", apiErr.RetryAfter)
	}
}

func TestTransportFallsBackToOtherRetryHeaders(t *testing.T) {
	tests := []struct{ header, value string }{
		{"x-retry-after", "4"},
		{"ratelimit-reset", "5"},
	}
	for _, tc := range tests {
		t.Run(tc.header, func(t *testing.T) {
			var calls atomic.Int32
			c, s := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set(tc.header, tc.value)
					writeJSON(w, http.StatusTooManyRequests, `{"error":{"code":"rate_limited","message":"m"}}`)
					return
				}
				writeJSON(w, http.StatusOK, siteJSON)
			})
			if _, err := c.Site(context.Background()); err != nil {
				t.Fatalf("Site: %v", err)
			}
			want, _ := time.ParseDuration(tc.value + "s")
			if got := s.got(); len(got) != 1 || got[0] != want {
				t.Errorf("waits = %v, want [%v]", got, want)
			}
		})
	}
}

func TestTransportReplaysBodyOnRetry(t *testing.T) {
	var bodies []string
	c, _ := newTestClient(t, "tok", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if len(bodies) == 1 {
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusTooManyRequests, `{"error":{"code":"rate_limited","message":"m"}}`)
			return
		}
		writeJSON(w, http.StatusOK, `{"html":"<p>hola</p>"}`)
	})
	html, err := c.Preview(context.Background(), "hola", ReplyTarget(7))
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if html != "<p>hola</p>" {
		t.Errorf("html = %q", html)
	}
	if len(bodies) != 2 || bodies[0] == "" || bodies[0] != bodies[1] {
		t.Errorf("bodies = %q, want the same body twice", bodies)
	}
}

func TestTransportStopsWhenSleepFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		writeJSON(w, http.StatusTooManyRequests, `{"error":{"code":"rate_limited","message":"m"}}`)
	}))
	t.Cleanup(srv.Close)
	c, err := New(Options{Server: srv.URL, Sleep: func(context.Context, time.Duration) error { return context.Canceled }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = c.Site(context.Background())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
