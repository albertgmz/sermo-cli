package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/buildinfo"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// releaseTimeout limits the search for a new client release.
const releaseTimeout = 5 * time.Second

// version prints the client, server and API versions.
func (c *cli) version(cmd *cobra.Command, _ []string) error {
	out, ctx := cmd.OutOrStdout(), cmd.Context()
	_, _ = fmt.Fprintln(out, i18n.Tf("cli.version.client", map[string]any{"Version": buildinfo.Version}))
	if buildinfo.Commit != "" {
		_, _ = fmt.Fprintln(out, i18n.Tf("cli.version.build", map[string]any{"Commit": buildinfo.Commit, "Date": buildinfo.Date}))
	}
	_, _ = fmt.Fprintln(out, i18n.Tf("cli.version.server", map[string]any{"Server": c.server}))
	c.printServerVersion(ctx, out)
	_, _ = fmt.Fprintln(out, i18n.Tf("cli.version.spec", map[string]any{"Version": api.SpecVersion}))
	c.printRelease(ctx, out)
	return nil
}

func (c *cli) printServerVersion(ctx context.Context, out io.Writer) {
	v, err := c.serverVersion(ctx)
	if err != nil {
		v = i18n.Tf("cli.version.unknown", map[string]any{"Reason": ui.ErrorText(err)})
	}
	_, _ = fmt.Fprintln(out, i18n.Tf("cli.version.sermo", map[string]any{"Version": v}))
	if err == nil && newer(v, api.SpecVersion) {
		_, _ = fmt.Fprintln(out, i18n.T("cli.version.newerServer"))
	}
}

func (c *cli) serverVersion(ctx context.Context) (string, error) {
	client, err := c.guestClient()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	return client.ServerVersion(ctx)
}

// printRelease tells about a newer client release. It prints nothing when
// the search fails.
func (c *cli) printRelease(ctx context.Context, out io.Writer) {
	ctx, cancel := context.WithTimeout(ctx, releaseTimeout)
	defer cancel()
	res, err := c.env.checkUpdate(ctx, buildinfo.Version)
	if err != nil || !res.Newer() {
		return
	}
	_, _ = fmt.Fprintln(out, i18n.Tf("cli.version.newRelease", map[string]any{"Version": res.Latest}))
}

// newer reports whether version a is newer than version b. It returns
// false when a version cannot be parsed.
func newer(a, b string) bool {
	va, err := semver.NewVersion(a)
	if err != nil {
		return false
	}
	vb, err := semver.NewVersion(b)
	if err != nil {
		return false
	}
	return va.GreaterThan(vb)
}
