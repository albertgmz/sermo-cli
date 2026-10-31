package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
)

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
func (d *Device) PollToken(ctx context.Context, dc DeviceCode) (Token, error) {
	interval := dc.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	var deadline time.Time
	if dc.ExpiresIn > 0 {
		deadline = time.Now().Add(dc.ExpiresIn)
	}
	for {
		if err := d.wait(ctx, interval, deadline); err != nil {
			return Token{}, err
		}
		r, err := d.requestToken(ctx, dc.DeviceCode)
		if err != nil {
			return Token{}, err
		}
		if r.Error == "" {
			return r.token(time.Now()), nil
		}
		if r.Error == "slow_down" {
			interval += slowDownStep
			continue
		}
		if err := pollError(r); err != nil {
			return Token{}, err
		}
	}
}

// pollError returns nil when the client must poll again.
func pollError(r tokenResponse) error {
	switch r.Error {
	case "authorization_pending":
		return nil
	case "expired_token":
		return ErrExpired
	case "access_denied":
		return ErrDenied
	default:
		return fmt.Errorf("auth: device token: %s: %s", r.Error, r.ErrorDescription)
	}
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int64  `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
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
	if resp.StatusCode >= 500 {
		return r, fmt.Errorf("auth: device token request: HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return r, fmt.Errorf("auth: decode device token: %w", err)
	}
	if r.Error == "" && r.AccessToken == "" {
		return r, fmt.Errorf("auth: device token: HTTP %d without access token", resp.StatusCode)
	}
	return r, nil
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
