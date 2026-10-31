package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/banner"
	"github.com/albertgmz/sermo-cli/internal/buildinfo"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/theme"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// requestTimeout limits one short request to the server.
const requestTimeout = 10 * time.Second

// login runs the device login in plain text, so that it works over ssh.
func (c *cli) login(cmd *cobra.Command, _ []string) error {
	out, errOut, ctx := cmd.OutOrStdout(), cmd.ErrOrStderr(), cmd.Context()
	if os.Getenv("SERMO_API_KEY") != "" {
		_, _ = fmt.Fprintln(out, i18n.T("cli.login.apiKey"))
	}
	dev := &auth.Device{Server: c.server, UserAgent: buildinfo.UserAgent(), Sleep: c.env.sleep}
	_, _ = fmt.Fprintln(out, i18n.Tf("cli.login.requesting", map[string]any{"Server": c.server}))
	dc, err := dev.RequestCode(ctx)
	if err != nil {
		return loginFailed(errOut, err)
	}
	c.showCode(out, dc)
	stop := startWait(out, i18n.T("cli.login.waiting"))
	tok, err := dev.PollToken(ctx, dc)
	stop()
	if err != nil {
		return loginFailed(errOut, err)
	}
	c.finishLogin(ctx, out, errOut, tok)
	return nil
}

// showCode prints the address and the user code, and opens the browser.
func (c *cli) showCode(out io.Writer, dc auth.DeviceCode) {
	addr := dc.VerificationURIComplete
	if addr == "" {
		addr = dc.VerificationURI
	}
	_, _ = fmt.Fprintf(out, "\n%s\n\n    %s\n\n%s\n\n", i18n.T("cli.login.step1"), addr, i18n.T("cli.login.step2"))
	color := colorprofile.NewWriter(out, os.Environ())
	gradient := theme.ByID(c.cfg.Theme).Title
	for _, line := range banner.Lines(dc.UserCode, termWidth(out)) {
		_, _ = fmt.Fprintln(color, gradient.Text(line, 0))
	}
	_, _ = fmt.Fprintf(out, "\n    %s\n\n", dc.UserCode)
	if err := c.env.openURL(addr); err != nil {
		_, _ = fmt.Fprintln(out, i18n.T("cli.login.browserFailed"))
		return
	}
	_, _ = fmt.Fprintln(out, i18n.T("cli.login.browserOpened"))
}

// finishLogin saves the token and greets the member by name.
func (c *cli) finishLogin(ctx context.Context, out, errOut io.Writer, tok auth.Token) {
	if err := auth.NewStore().Save(c.server, tok); err != nil {
		_, _ = fmt.Fprintln(errOut, i18n.Tf("cli.login.saveFailed", map[string]any{"Error": err}))
	}
	data := map[string]any{"Server": c.server}
	if name := c.memberName(ctx, tok.AccessToken); name != "" {
		data["Name"] = name
		_, _ = fmt.Fprintln(out, i18n.Tf("cli.login.welcome", data))
		return
	}
	_, _ = fmt.Fprintln(out, i18n.Tf("cli.login.done", data))
}

// memberName returns the username for token, or "" when the server does
// not tell it.
func (c *cli) memberName(ctx context.Context, token string) string {
	client, err := api.New(api.Options{Server: c.server, Token: token, UserAgent: buildinfo.UserAgent()})
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	me, err := client.Me(ctx)
	if err != nil || me.User == nil {
		return ""
	}
	return me.User.Username
}

func loginFailed(w io.Writer, err error) error {
	_, _ = fmt.Fprintln(w, loginErrorText(err))
	return errReported
}

func loginErrorText(err error) string {
	switch {
	case errors.Is(err, auth.ErrUnsupported):
		return i18n.T("cli.login.unsupported")
	case errors.Is(err, auth.ErrExpired):
		return i18n.T("cli.login.expired")
	case errors.Is(err, auth.ErrDenied):
		return i18n.T("cli.login.denied")
	case errors.Is(err, context.Canceled):
		return i18n.T("cli.login.canceled")
	}
	return i18n.Tf("cli.login.failed", map[string]any{"Error": ui.ErrorText(err)})
}
