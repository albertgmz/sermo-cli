package api

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// maxRetryWait is the longest 429 wait that the transport waits out itself.
const maxRetryWait = 10 * time.Second

// transport adds the Sermo headers and retries one short 429.
type transport struct {
	base      http.RoundTripper
	origin    *url.URL
	token     string
	userAgent string
	sleep     func(context.Context, time.Duration) error
}

// RoundTrip sends req. On a 429 with a short wait it waits and sends req again one time.
// It does not send a prefetch request again.
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	out := t.prepare(req)
	resp, err := t.base.RoundTrip(out)
	if err != nil || resp.StatusCode != http.StatusTooManyRequests || isPrefetch(req.Context()) {
		return resp, err
	}
	wait, ok := retryAfter(resp.Header, time.Now())
	if !ok || wait > maxRetryWait {
		return resp, nil
	}
	again, ok := rewind(out)
	if !ok {
		return resp, nil
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if err := t.sleep(req.Context(), wait); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(again)
}

// prepare returns a copy of req with the Sermo headers. A RoundTripper must not change req.
func (t *transport) prepare(req *http.Request) *http.Request {
	out := req.Clone(req.Context())
	if t.token != "" && sameOrigin(out.URL, t.origin) {
		out.Header.Set("Authorization", "Bearer "+t.token)
	}
	if t.userAgent != "" {
		out.Header.Set("User-Agent", t.userAgent)
	}
	out.Header.Set("Accept", "application/json")
	return out
}

// sameOrigin reports whether u has the scheme and host of origin.
// The transport sends the token only to the configured server, also after a redirect.
func sameOrigin(u, origin *url.URL) bool {
	return strings.EqualFold(u.Scheme, origin.Scheme) && strings.EqualFold(u.Host, origin.Host)
}

// rewind returns a copy of req with a new body. It returns false when the body cannot be read again.
func rewind(req *http.Request) (*http.Request, bool) {
	again := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return again, true
	}
	if req.GetBody == nil {
		return nil, false
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, false
	}
	again.Body = body
	return again, true
}

// retryAfter reads the wait from Retry-After, then x-retry-after, then ratelimit-reset.
func retryAfter(h http.Header, now time.Time) (time.Duration, bool) {
	for _, name := range []string{"Retry-After", "X-Retry-After", "Ratelimit-Reset"} {
		if d, ok := parseWait(h.Get(name), now); ok {
			return d, true
		}
	}
	return 0, false
}

// parseWait reads a number of seconds or an HTTP date.
func parseWait(v string, now time.Time) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.ParseFloat(v, 64); err == nil {
		if secs < 0 {
			return 0, true
		}
		return time.Duration(secs * float64(time.Second)), true
	}
	at, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	return max(at.Sub(now), 0), true
}

// sleepCtx waits for d or until ctx ends.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
