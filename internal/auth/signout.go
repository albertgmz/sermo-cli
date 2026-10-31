package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// ErrSignOutUnsupported tells that the server does not have the sign-out endpoint.
var ErrSignOutUnsupported = errors.New("auth: server does not support sign-out")

// SignOut revokes token on server. A nil hc uses http.DefaultClient.
// A 401 reply is a success, because the server does not accept the token any more.
func SignOut(ctx context.Context, hc *http.Client, server, userAgent, token string) error {
	resp, err := postJSON(ctx, hc, server+"/api/auth/sign-out", userAgent, token, struct{}{})
	if err != nil {
		return fmt.Errorf("auth: sign-out request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode <= 299, resp.StatusCode == http.StatusUnauthorized:
		return nil
	case resp.StatusCode == http.StatusNotFound:
		return ErrSignOutUnsupported
	default:
		return fmt.Errorf("auth: sign-out request: HTTP %d", resp.StatusCode)
	}
}
