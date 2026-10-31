package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/buildinfo"
	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// securityPath is the page of the Sermo web app that lists the sessions of
// the member.
const securityPath = "/account/security"

// logout revokes the stored device token and deletes it. It never sends
// the API key.
func (c *cli) logout(cmd *cobra.Command, _ []string) error {
	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	if os.Getenv("SERMO_API_KEY") != "" {
		_, _ = fmt.Fprintln(out, i18n.T("cli.logout.apiKey"))
	}
	store := auth.NewStore()
	tok, err := store.Load(c.server)
	if errors.Is(err, auth.ErrNoToken) {
		_, _ = fmt.Fprintln(out, i18n.Tf("cli.logout.none", map[string]any{"Server": c.server}))
		return nil
	}
	if err != nil {
		return logoutFailed(errOut, err)
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), requestTimeout)
	defer cancel()
	signOutErr := auth.SignOut(ctx, nil, c.server, buildinfo.UserAgent(), tok.AccessToken)
	if err := store.Delete(c.server); err != nil {
		return logoutFailed(errOut, err)
	}
	if signOutErr != nil {
		_, _ = fmt.Fprintln(out, i18n.Tf("cli.logout.local", map[string]any{"URL": c.server + securityPath}))
		return nil
	}
	_, _ = fmt.Fprintln(out, i18n.T("cli.logout.done"))
	return nil
}

func logoutFailed(w io.Writer, err error) error {
	_, _ = fmt.Fprintln(w, i18n.Tf("cli.logout.failed", map[string]any{"Error": err}))
	return errReported
}
