package ui

import (
	"context"
	"errors"
	"math"
	"net"
	"net/http"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// ErrorText returns a friendly text for err in the current language.
// For a validation error (400, 409, 422) it gives the text of the server.
// It returns "" for a nil err and for a canceled context.
func ErrorText(err error) string {
	if err == nil || errors.Is(err, context.Canceled) {
		return ""
	}
	var e *api.Error
	if errors.As(err, &e) {
		return apiErrorText(e)
	}
	if errors.Is(err, auth.ErrUnsupported) {
		return i18n.T("errors.deviceUnsupported")
	}
	var ne net.Error
	if errors.As(err, &ne) || errors.Is(err, context.DeadlineExceeded) {
		return i18n.T("errors.network")
	}
	return err.Error()
}

func apiErrorText(e *api.Error) string {
	switch e.Status {
	case http.StatusUnauthorized:
		return i18n.T("errors.unauthorized")
	case http.StatusForbidden:
		return forbiddenText(e)
	case http.StatusNotFound:
		return i18n.T("errors.notFound")
	case http.StatusTooManyRequests:
		secs := max(1, int(math.Ceil(e.RetryAfter.Seconds())))
		return i18n.Tf("errors.rateLimited", map[string]any{"Seconds": secs})
	case http.StatusBadRequest, http.StatusConflict, http.StatusUnprocessableEntity:
		return validationText(e)
	}
	return i18n.T("errors.server")
}

// forbiddenText tells the member to accept the new terms of the site when
// the server refuses a write for this reason.
func forbiddenText(e *api.Error) string {
	for _, is := range e.Issues {
		if is.Path == "acceptance" {
			return i18n.T("errors.acceptTerms")
		}
	}
	return i18n.T("errors.forbidden")
}

// validationText prefers the first field issue because it is more exact
// than the general message.
func validationText(e *api.Error) string {
	if len(e.Issues) > 0 && e.Issues[0].Message != "" {
		return e.Issues[0].Message
	}
	if e.Message != "" {
		return e.Message
	}
	return i18n.T("errors.server")
}
