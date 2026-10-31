package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/albertgmz/sermo-cli/internal/buildinfo"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/update"
)

// update replaces the binary with the latest release.
func (c *cli) update(cmd *cobra.Command, _ []string) error {
	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	_, _ = fmt.Fprintln(out, i18n.T("cli.update.checking"))
	res, err := update.Apply(cmd.Context(), buildinfo.Version)
	switch {
	case errors.Is(err, update.ErrDevBuild):
		_, _ = fmt.Fprintln(errOut, i18n.T("cli.update.dev"))
		return errReported
	case err != nil:
		_, _ = fmt.Fprintln(errOut, i18n.Tf("cli.update.failed", map[string]any{"Error": err}))
		return errReported
	case res.Updated:
		_, _ = fmt.Fprintln(out, i18n.Tf("cli.update.done", map[string]any{"Version": res.Latest}))
	default:
		_, _ = fmt.Fprintln(out, i18n.Tf("cli.update.current", map[string]any{"Version": res.Current}))
	}
	return nil
}
