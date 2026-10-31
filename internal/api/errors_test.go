package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestErrorEnvelopeByStatus(t *testing.T) {
	tests := []struct {
		status int
		is     func(error) bool
	}{
		{http.StatusUnauthorized, IsUnauthorized},
		{http.StatusForbidden, IsForbidden},
		{http.StatusNotFound, IsNotFound},
	}
	for _, tc := range tests {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			c, _ := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, tc.status, `{"error":{"code":"some_code","message":"Some message",`+
					`"issues":[{"path":["body",0],"message":"Too short"}]}}`)
			})
			_, err := c.Post(context.Background(), 5)
			checkEnvelope(t, err, tc.status)
			if !tc.is(err) {
				t.Errorf("helper for %d returned false", tc.status)
			}
			if IsRateLimited(err) {
				t.Errorf("IsRateLimited = true for %d", tc.status)
			}
		})
	}
}

func checkEnvelope(t *testing.T, err error, status int) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("err = %v, want *Error", err)
	}
	if e.Status != status || e.Code != "some_code" || e.Message != "Some message" {
		t.Errorf("err = %+v", e)
	}
	if len(e.Issues) != 1 || e.Issues[0].Path != "body.0" || e.Issues[0].Message != "Too short" {
		t.Errorf("Issues = %+v", e.Issues)
	}
}

func TestErrorFallbackWhenBodyIsNotEnvelope(t *testing.T) {
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>bad gateway</html>"))
	})
	_, err := c.Post(context.Background(), 5)
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("err = %v, want *Error", err)
	}
	if e.Status != http.StatusBadGateway || e.Message != "Bad Gateway" || e.Code != "" {
		t.Errorf("err = %+v", e)
	}
	if e.Error() == "" {
		t.Error("Error() is empty")
	}
}

func TestIsHelpersRejectOtherErrors(t *testing.T) {
	err := errors.New("plain")
	if IsUnauthorized(err) || IsForbidden(err) || IsNotFound(err) || IsRateLimited(err) {
		t.Error("helper matched a plain error")
	}
}
