package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// ClientID is the OAuth client ID that the server knows for this program.
const ClientID = "sermo-cli"

const (
	deviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"
	defaultInterval = 5 * time.Second
	slowDownStep    = 5 * time.Second
)

var (
	// ErrUnsupported tells that the server does not have the device login endpoints.
	ErrUnsupported = errors.New("auth: server does not support device login")
	// ErrExpired tells that the device code expired before the user approved it.
	ErrExpired = errors.New("auth: device code expired")
	// ErrDenied tells that the user refused the device login.
	ErrDenied = errors.New("auth: device login denied")
	// ErrInvalidGrant tells that the server does not accept the device code any more.
	// The code is unknown, the client used it before, or it belongs to a different client.
	ErrInvalidGrant = errors.New("auth: device code is not valid")
	// ErrBanned tells that the account is banned.
	ErrBanned = errors.New("auth: account is banned")
)

// RateLimitError tells that the server refused a request because of its rate limit.
// Wait is the time to wait before the next request.
type RateLimitError struct {
	Wait time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("auth: rate limited, wait %s", e.Wait)
}

// DeviceCode is the reply to a device code request.
type DeviceCode struct {
	DeviceCode, UserCode, VerificationURI, VerificationURIComplete string
	ExpiresIn, Interval                                            time.Duration
}

// Token is an access token. A zero ExpiresAt means that the token has no fixed lifetime.
type Token struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt,omitzero"`
}

func (t Token) expired(now time.Time) bool {
	return !t.ExpiresAt.IsZero() && now.After(t.ExpiresAt)
}

// Device runs the device authorization flow against Server.
// A nil HTTP uses http.DefaultClient. A nil Sleep uses a timer that stops when the context ends.
type Device struct {
	Server    string
	HTTP      *http.Client
	UserAgent string
	Sleep     func(context.Context, time.Duration) error
}

// RequestCode asks the server for a new device code and user code.
func (d *Device) RequestCode(ctx context.Context) (DeviceCode, error) {
	resp, err := postJSON(ctx, d.HTTP, d.Server+"/api/auth/device/code", d.UserAgent, "",
		map[string]string{"client_id": ClientID})
	if err != nil {
		return DeviceCode{}, fmt.Errorf("auth: device code request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return DeviceCode{}, ErrUnsupported
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return DeviceCode{}, &RateLimitError{Wait: retryAfter(resp.Header, defaultInterval+slowDownStep)}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return DeviceCode{}, fmt.Errorf("auth: device code request: HTTP %d", resp.StatusCode)
	}
	var body struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int64  `json:"expires_in"`
		Interval                int64  `json:"interval"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return DeviceCode{}, fmt.Errorf("auth: decode device code: %w", err)
	}
	dc := DeviceCode{
		DeviceCode:              body.DeviceCode,
		UserCode:                body.UserCode,
		VerificationURI:         body.VerificationURI,
		VerificationURIComplete: body.VerificationURIComplete,
		ExpiresIn:               time.Duration(body.ExpiresIn) * time.Second,
		Interval:                time.Duration(body.Interval) * time.Second,
	}
	if dc.Interval <= 0 {
		dc.Interval = defaultInterval
	}
	return dc, nil
}

// PollToken waits for the user to approve dc and returns the access token.
// It returns ErrExpired when the server tells so or when dc.ExpiresIn has passed.
// When the server refuses a poll because of its rate limit, PollToken waits and polls again.
func (d *Device) PollToken(ctx context.Context, dc DeviceCode) (Token, error) {
	p := pollState{interval: dc.Interval}
	if p.interval <= 0 {
		p.interval = defaultInterval
	}
	p.next = p.interval
	var deadline time.Time
	if dc.ExpiresIn > 0 {
		deadline = time.Now().Add(dc.ExpiresIn)
	}
	for {
		if err := d.wait(ctx, p.next, deadline); err != nil {
			return Token{}, err
		}
		r, err := d.requestToken(ctx, dc.DeviceCode)
		if err != nil {
			return Token{}, err
		}
		done, err := p.handle(r)
		if err != nil {
			return Token{}, err
		}
		if done {
			return r.token(time.Now()), nil
		}
	}
}

// pollState keeps the poll interval and the time to wait before the next poll.
type pollState struct {
	interval, next time.Duration
}

// handle reads one token response. It returns true when r holds the token.
func (p *pollState) handle(r tokenResponse) (bool, error) {
	p.next = p.interval
	switch {
	case r.limited:
		p.next = r.retryAfter
		if p.next <= 0 {
			p.next = p.interval + slowDownStep
		}
		return false, nil
	case r.Error == "slow_down":
		p.interval += slowDownStep
		p.next = p.interval
		return false, nil
	case r.Error == "" && r.Code == "":
		return true, nil
	}
	return false, pollError(r)
}

// pollError returns nil when the client must poll again.
func pollError(r tokenResponse) error {
	if r.Code == "ACCOUNT_BANNED" {
		return ErrBanned
	}
	switch r.Error {
	case "authorization_pending":
		return nil
	case "expired_token":
		return ErrExpired
	case "access_denied":
		return ErrDenied
	case "invalid_grant":
		return fmt.Errorf("%w: %s: %s", ErrInvalidGrant, r.Error, r.ErrorDescription)
	default:
		return fmt.Errorf("auth: device token: %s%s: %s", r.Error, r.Code, r.ErrorDescription)
	}
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int64  `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	// Code is the error code of a Better Auth error body, for example ACCOUNT_BANNED.
	Code string `json:"code"`
	// limited is true when the server refused the poll because of its rate limit.
	// retryAfter is the wait that the server gave, or zero.
	limited    bool
	retryAfter time.Duration
}

func (r tokenResponse) token(now time.Time) Token {
	t := Token{AccessToken: r.AccessToken}
	if r.ExpiresIn > 0 {
		t.ExpiresAt = now.Add(time.Duration(r.ExpiresIn) * time.Second)
	}
	return t
}

func (d *Device) requestToken(ctx context.Context, deviceCode string) (tokenResponse, error) {
	var r tokenResponse
	resp, err := postJSON(ctx, d.HTTP, d.Server+"/api/auth/device/token", d.UserAgent, "", map[string]string{
		"grant_type":  deviceGrantType,
		"device_code": deviceCode,
		"client_id":   ClientID,
	})
	if err != nil {
		return r, fmt.Errorf("auth: device token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests {
		r.limited, r.retryAfter = true, retryAfter(resp.Header, 0)
		return r, nil
	}
	if resp.StatusCode >= 500 {
		return r, fmt.Errorf("auth: device token request: HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return r, fmt.Errorf("auth: decode device token: %w", err)
	}
	if r.Error == "" && r.Code == "" && r.AccessToken == "" {
		return r, fmt.Errorf("auth: device token: HTTP %d without access token", resp.StatusCode)
	}
	return r, nil
}

// retryAfter returns the seconds in the X-Retry-After header, else in the Retry-After header.
// It returns fallback when neither header holds a positive number of seconds.
func retryAfter(h http.Header, fallback time.Duration) time.Duration {
	for _, name := range []string{"X-Retry-After", "Retry-After"} {
		if n, err := strconv.Atoi(h.Get(name)); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return fallback
}

// wait sleeps for interval. It returns ErrExpired when a non-zero deadline has passed.
func (d *Device) wait(ctx context.Context, interval time.Duration, deadline time.Time) error {
	if err := d.sleep(ctx, interval); err != nil {
		return err
	}
	if !deadline.IsZero() && time.Now().After(deadline) {
		return ErrExpired
	}
	return nil
}

func (d *Device) sleep(ctx context.Context, dur time.Duration) error {
	if d.Sleep != nil {
		return d.Sleep(ctx, dur)
	}
	t := time.NewTimer(dur)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// postJSON sends body as JSON with the standard headers. A non-empty bearer adds an Authorization header.
func postJSON(ctx context.Context, hc *http.Client, url, userAgent, bearer string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	return hc.Do(req)
}
