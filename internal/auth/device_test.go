package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type tokenReply struct {
	status int
	body   string
}

// fakeServer serves the device endpoints. It answers token polls with the replies in sequence.
type fakeServer struct {
	t       *testing.T
	mu      sync.Mutex
	replies []tokenReply
	polls   int
	srv     *httptest.Server
}

func newFakeServer(t *testing.T, replies ...tokenReply) *fakeServer {
	t.Helper()
	f := &fakeServer{t: t, replies: replies}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/device/code", f.code)
	mux.HandleFunc("POST /api/auth/device/token", f.token)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func checkHeaders(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := r.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
	if got := r.Header.Get("User-Agent"); got != "test-agent" {
		t.Errorf("User-Agent = %q", got)
	}
}

func (f *fakeServer) code(w http.ResponseWriter, r *http.Request) {
	checkHeaders(f.t, r)
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["client_id"] != ClientID {
		f.t.Errorf("code body = %v, err = %v", body, err)
	}
	_, _ = w.Write([]byte(`{"device_code":"dev-1","user_code":"ABCD-EFGH","verification_uri":"https://x/device",` +
		`"verification_uri_complete":"https://x/device?user_code=ABCD-EFGH","expires_in":1800,"interval":7}`))
}

func (f *fakeServer) token(w http.ResponseWriter, r *http.Request) {
	checkHeaders(f.t, r)
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		f.t.Errorf("token body decode: %v", err)
	}
	if body["grant_type"] != "urn:ietf:params:oauth:grant-type:device_code" || body["device_code"] != "dev-1" ||
		body["client_id"] != ClientID {
		f.t.Errorf("token body = %v", body)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.polls >= len(f.replies) {
		f.t.Errorf("unexpected poll %d", f.polls+1)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	rep := f.replies[f.polls]
	f.polls++
	w.WriteHeader(rep.status)
	_, _ = w.Write([]byte(rep.body))
}

func (f *fakeServer) pollCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.polls
}

type sleepRecorder struct{ slept []time.Duration }

func (s *sleepRecorder) sleep(ctx context.Context, d time.Duration) error {
	s.slept = append(s.slept, d)
	return ctx.Err()
}

func newDevice(server string, rec *sleepRecorder) *Device {
	return &Device{Server: server, HTTP: http.DefaultClient, UserAgent: "test-agent", Sleep: rec.sleep}
}

const (
	pending  = `{"error":"authorization_pending","error_description":"wait"}`
	slowDown = `{"error":"slow_down","error_description":"slow"}`
)

func testCode() DeviceCode {
	return DeviceCode{DeviceCode: "dev-1", Interval: 5 * time.Second, ExpiresIn: time.Hour}
}

func TestRequestCode(t *testing.T) {
	f := newFakeServer(t)
	dc, err := newDevice(f.srv.URL, &sleepRecorder{}).RequestCode(context.Background())
	if err != nil {
		t.Fatalf("RequestCode() error = %v", err)
	}
	want := DeviceCode{
		DeviceCode: "dev-1", UserCode: "ABCD-EFGH", VerificationURI: "https://x/device",
		VerificationURIComplete: "https://x/device?user_code=ABCD-EFGH",
		ExpiresIn:               1800 * time.Second, Interval: 7 * time.Second,
	}
	if dc != want {
		t.Fatalf("RequestCode() = %+v, want %+v", dc, want)
	}
}

func TestRequestCodeDefaultInterval(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"device_code":"d","user_code":"u","verification_uri":"v","expires_in":60}`))
	}))
	t.Cleanup(srv.Close)
	dc, err := newDevice(srv.URL, &sleepRecorder{}).RequestCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dc.Interval != 5*time.Second {
		t.Fatalf("Interval = %v, want 5s", dc.Interval)
	}
}

func TestRequestCodeErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{"404 unsupported", http.StatusNotFound, `{}`, ErrUnsupported},
		{"5xx", http.StatusBadGateway, `bad gateway`, nil},
		{"non-JSON", http.StatusOK, `<html>`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)
			_, err := newDevice(srv.URL, &sleepRecorder{}).RequestCode(context.Background())
			if err == nil {
				t.Fatal("RequestCode() error = nil")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("RequestCode() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && errors.Is(err, ErrUnsupported) {
				t.Fatalf("RequestCode() error = %v, must not be ErrUnsupported", err)
			}
		})
	}
}

func TestPollTokenPendingThenSuccess(t *testing.T) {
	f := newFakeServer(t,
		tokenReply{http.StatusBadRequest, pending},
		tokenReply{http.StatusBadRequest, pending},
		tokenReply{http.StatusOK, `{"access_token":"tok-1","token_type":"Bearer","expires_in":3600}`},
	)
	rec := &sleepRecorder{}
	before := time.Now()
	tok, err := newDevice(f.srv.URL, rec).PollToken(context.Background(), testCode())
	if err != nil {
		t.Fatalf("PollToken() error = %v", err)
	}
	if tok.AccessToken != "tok-1" {
		t.Fatalf("AccessToken = %q", tok.AccessToken)
	}
	if tok.ExpiresAt.Before(before.Add(time.Hour)) || tok.ExpiresAt.After(time.Now().Add(time.Hour)) {
		t.Fatalf("ExpiresAt = %v, want about 1h from now", tok.ExpiresAt)
	}
	assertSlept(t, rec, 5*time.Second, 5*time.Second, 5*time.Second)
}

func TestPollTokenNoExpiry(t *testing.T) {
	f := newFakeServer(t, tokenReply{http.StatusOK, `{"access_token":"tok-1","token_type":"Bearer"}`})
	tok, err := newDevice(f.srv.URL, &sleepRecorder{}).PollToken(context.Background(), testCode())
	if err != nil {
		t.Fatal(err)
	}
	if !tok.ExpiresAt.IsZero() {
		t.Fatalf("ExpiresAt = %v, want zero", tok.ExpiresAt)
	}
}

func TestPollTokenSlowDownRaisesInterval(t *testing.T) {
	f := newFakeServer(t,
		tokenReply{http.StatusBadRequest, slowDown},
		tokenReply{http.StatusBadRequest, slowDown},
		tokenReply{http.StatusBadRequest, pending},
		tokenReply{http.StatusOK, `{"access_token":"tok-1","token_type":"Bearer"}`},
	)
	rec := &sleepRecorder{}
	if _, err := newDevice(f.srv.URL, rec).PollToken(context.Background(), testCode()); err != nil {
		t.Fatal(err)
	}
	assertSlept(t, rec, 5*time.Second, 10*time.Second, 15*time.Second, 15*time.Second)
}

func TestPollTokenDefaultInterval(t *testing.T) {
	f := newFakeServer(t, tokenReply{http.StatusOK, `{"access_token":"tok-1"}`})
	rec := &sleepRecorder{}
	dc := testCode()
	dc.Interval = 0
	if _, err := newDevice(f.srv.URL, rec).PollToken(context.Background(), dc); err != nil {
		t.Fatal(err)
	}
	assertSlept(t, rec, 5*time.Second)
}

func TestPollTokenTerminalErrors(t *testing.T) {
	tests := []struct {
		name    string
		reply   tokenReply
		wantErr error
		wantMsg []string
	}{
		{"expired", tokenReply{http.StatusBadRequest, `{"error":"expired_token"}`}, ErrExpired, nil},
		{"denied", tokenReply{http.StatusBadRequest, `{"error":"access_denied"}`}, ErrDenied, nil},
		{"unknown code", tokenReply{http.StatusBadRequest, `{"error":"invalid_grant","error_description":"bad code"}`},
			nil, []string{"invalid_grant", "bad code"}},
		{"5xx", tokenReply{http.StatusInternalServerError, `oops`}, nil, []string{"500"}},
		{"non-JSON", tokenReply{http.StatusOK, `<html>`}, nil, nil},
		{"no token", tokenReply{http.StatusOK, `{}`}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeServer(t, tt.reply)
			_, err := newDevice(f.srv.URL, &sleepRecorder{}).PollToken(context.Background(), testCode())
			if err == nil {
				t.Fatal("PollToken() error = nil")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("PollToken() error = %v, want %v", err, tt.wantErr)
			}
			for _, s := range tt.wantMsg {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("error %q does not contain %q", err, s)
				}
			}
		})
	}
}

func TestPollTokenLocalDeadline(t *testing.T) {
	f := newFakeServer(t, tokenReply{http.StatusBadRequest, pending}, tokenReply{http.StatusBadRequest, pending})
	dc := testCode()
	dc.ExpiresIn = 50 * time.Millisecond
	sleep := func(ctx context.Context, _ time.Duration) error {
		time.Sleep(30 * time.Millisecond)
		return ctx.Err()
	}
	d := &Device{Server: f.srv.URL, HTTP: http.DefaultClient, UserAgent: "test-agent", Sleep: sleep}
	_, err := d.PollToken(context.Background(), dc)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("PollToken() error = %v, want ErrExpired", err)
	}
	if n := f.pollCount(); n != 1 {
		t.Fatalf("polls = %d, want 1", n)
	}
}

func TestPollTokenContextCancel(t *testing.T) {
	f := newFakeServer(t, tokenReply{http.StatusBadRequest, pending})
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	sleep := func(ctx context.Context, _ time.Duration) error {
		calls++
		if calls == 2 {
			cancel()
		}
		return ctx.Err()
	}
	d := &Device{Server: f.srv.URL, HTTP: http.DefaultClient, UserAgent: "test-agent", Sleep: sleep}
	_, err := d.PollToken(ctx, testCode())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PollToken() error = %v, want context.Canceled", err)
	}
	if n := f.pollCount(); n != 1 {
		t.Fatalf("polls = %d, want 1", n)
	}
}

func TestPollTokenDefaultSleepHonorsContext(t *testing.T) {
	f := newFakeServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	d := &Device{Server: f.srv.URL, UserAgent: "test-agent"}
	start := time.Now()
	_, err := d.PollToken(ctx, testCode())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("PollToken() error = %v, want DeadlineExceeded", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("default sleep did not stop on context cancel")
	}
}

func assertSlept(t *testing.T, rec *sleepRecorder, want ...time.Duration) {
	t.Helper()
	if len(rec.slept) != len(want) {
		t.Fatalf("slept %v, want %v", rec.slept, want)
	}
	for i := range want {
		if rec.slept[i] != want[i] {
			t.Fatalf("slept %v, want %v", rec.slept, want)
		}
	}
}

// seqServer answers each request with the next handler in handlers.
func seqServer(t *testing.T, handlers ...http.HandlerFunc) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		i := n
		n++
		mu.Unlock()
		if i >= len(handlers) {
			t.Errorf("unexpected request %d", i+1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		handlers[i](w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func reply(status int, body string, header ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		for i := 0; i+1 < len(header); i += 2 {
			w.Header().Set(header[i], header[i+1])
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

const (
	tooMany  = `{"message":"Too many requests. Please try again later."}`
	tokenOK  = `{"access_token":"tok-1","token_type":"Bearer","expires_in":7776000}`
	bannedJS = `{"message":"This account is banned.","code":"ACCOUNT_BANNED"}`
)

func TestPollTokenInvalidGrant(t *testing.T) {
	srv := seqServer(t, reply(http.StatusBadRequest, `{"error":"invalid_grant","error_description":"Invalid device code"}`))
	_, err := newDevice(srv.URL, &sleepRecorder{}).PollToken(context.Background(), testCode())
	if !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("PollToken() error = %v, want ErrInvalidGrant", err)
	}
}

func TestPollTokenBanned(t *testing.T) {
	srv := seqServer(t, reply(http.StatusForbidden, bannedJS))
	_, err := newDevice(srv.URL, &sleepRecorder{}).PollToken(context.Background(), testCode())
	if !errors.Is(err, ErrBanned) {
		t.Fatalf("PollToken() error = %v, want ErrBanned", err)
	}
}

func TestPollTokenRateLimitedWaits(t *testing.T) {
	tests := []struct {
		name   string
		header []string
		want   time.Duration
	}{
		{"X-Retry-After", []string{"X-Retry-After", "12", "Retry-After", "99"}, 12 * time.Second},
		{"Retry-After", []string{"Retry-After", "8"}, 8 * time.Second},
		{"no header", nil, 10 * time.Second},
		{"bad header", []string{"X-Retry-After", "soon"}, 10 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := seqServer(t,
				reply(http.StatusTooManyRequests, tooMany, tt.header...),
				reply(http.StatusBadRequest, pending),
				reply(http.StatusOK, tokenOK),
			)
			rec := &sleepRecorder{}
			before := time.Now()
			tok, err := newDevice(srv.URL, rec).PollToken(context.Background(), testCode())
			if err != nil {
				t.Fatalf("PollToken() error = %v", err)
			}
			assertSlept(t, rec, 5*time.Second, tt.want, 5*time.Second)
			if want := before.Add(90 * 24 * time.Hour); tok.ExpiresAt.Before(want) {
				t.Fatalf("ExpiresAt = %v, want 90 days from now", tok.ExpiresAt)
			}
		})
	}
}

func TestPollTokenRateLimitedPastDeadline(t *testing.T) {
	srv := seqServer(t, reply(http.StatusTooManyRequests, tooMany, "X-Retry-After", "1"))
	dc := testCode()
	dc.ExpiresIn = 50 * time.Millisecond
	sleep := func(ctx context.Context, _ time.Duration) error {
		time.Sleep(30 * time.Millisecond)
		return ctx.Err()
	}
	d := &Device{Server: srv.URL, HTTP: http.DefaultClient, UserAgent: "test-agent", Sleep: sleep}
	if _, err := d.PollToken(context.Background(), dc); !errors.Is(err, ErrExpired) {
		t.Fatalf("PollToken() error = %v, want ErrExpired", err)
	}
}

func TestRequestCodeRateLimited(t *testing.T) {
	tests := []struct {
		name   string
		header []string
		want   time.Duration
	}{
		{"X-Retry-After", []string{"X-Retry-After", "30"}, 30 * time.Second},
		{"no header", nil, 10 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := seqServer(t, reply(http.StatusTooManyRequests, tooMany, tt.header...))
			_, err := newDevice(srv.URL, &sleepRecorder{}).RequestCode(context.Background())
			var rl *RateLimitError
			if !errors.As(err, &rl) {
				t.Fatalf("RequestCode() error = %v, want *RateLimitError", err)
			}
			if rl.Wait != tt.want {
				t.Fatalf("Wait = %v, want %v", rl.Wait, tt.want)
			}
		})
	}
}
