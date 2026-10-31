package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Error is a response from Sermo with a status that is not 2xx.
type Error struct {
	Status     int
	Code       string
	Message    string
	Issues     []Issue
	RetryAfter time.Duration
}

// Issue is one problem in a request field. Path joins the field path with dots, for example "body.0".
type Issue struct {
	Path    string
	Message string
}

// Error returns the message with the status and the code.
func (e *Error) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("sermo: %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("sermo: %d %s: %s", e.Status, e.Code, e.Message)
}

// IsUnauthorized reports whether err is a 401 from Sermo.
func IsUnauthorized(err error) bool { return hasStatus(err, http.StatusUnauthorized) }

// IsForbidden reports whether err is a 403 from Sermo.
func IsForbidden(err error) bool { return hasStatus(err, http.StatusForbidden) }

// IsNotFound reports whether err is a 404 from Sermo.
func IsNotFound(err error) bool { return hasStatus(err, http.StatusNotFound) }

// IsRateLimited reports whether err is a 429 from Sermo.
func IsRateLimited(err error) bool { return hasStatus(err, http.StatusTooManyRequests) }

func hasStatus(err error, status int) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == status
}

// envelope is the error body of Sermo.
type envelope struct {
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Issues  []struct {
			Path    []any  `json:"path"`
			Message string `json:"message"`
		} `json:"issues"`
	} `json:"error"`
}

// check returns nil for a 2xx response. Otherwise it returns an *Error made from the body.
func check(resp *http.Response, body []byte) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	e := &Error{Status: resp.StatusCode}
	if resp.StatusCode == http.StatusTooManyRequests {
		e.RetryAfter, _ = retryAfter(resp.Header, time.Now())
	}
	var env envelope
	if json.Unmarshal(body, &env) != nil || env.Error == nil || env.Error.Message == "" {
		e.Message = fallbackMessage(resp.StatusCode)
		return e
	}
	e.Code, e.Message = env.Error.Code, env.Error.Message
	for _, is := range env.Error.Issues {
		e.Issues = append(e.Issues, Issue{Path: joinPath(is.Path), Message: is.Message})
	}
	return e
}

func fallbackMessage(status int) string {
	if text := http.StatusText(status); text != "" {
		return text
	}
	return fmt.Sprintf("HTTP %d", status)
}

func joinPath(path []any) string {
	parts := make([]string, len(path))
	for i, p := range path {
		parts[i] = fmt.Sprint(p)
	}
	return strings.Join(parts, ".")
}

// result returns v when the response is 2xx and v holds the decoded body.
func result[T any](resp *http.Response, body []byte, v *T) (*T, error) {
	if err := check(resp, body); err != nil {
		return nil, err
	}
	if v == nil {
		return nil, fmt.Errorf("sermo: %d: unexpected response body", resp.StatusCode)
	}
	return v, nil
}
