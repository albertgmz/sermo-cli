package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// ErrSignOutUnsupported tells that the server does not have the sign-out endpoint.
var ErrSignOutUnsupported = errors.New("auth: server does not support sign-out")

// SignOut ends the device login of token on server. It ends only this login. A nil hc uses http.DefaultClient.
// A 401 reply is a success, because the login ended before.
// A 403 reply tells that token is not a device login. SignOut then returns an error.
func SignOut(ctx context.Context, hc *http.Client, server, userAgent, token string) error {
	resp, err := postJSON(ctx, hc, server+"/api/v1/me/device-login/sign-out", userAgent, token, struct{}{})
	if err != nil {
		return fmt.Errorf("auth: sign-out request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode <= 299, resp.StatusCode == http.StatusUnauthorized:
		return nil
	case resp.StatusCode == http.StatusNotFound:
		return ErrSignOutUnsupported
	case resp.StatusCode == http.StatusForbidden:
		return errors.New("auth: sign-out request: the token is not a device login")
	default:
		return fmt.Errorf("auth: sign-out request: HTTP %d", resp.StatusCode)
	}
}
