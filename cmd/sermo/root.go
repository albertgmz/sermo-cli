package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/buildinfo"
	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
	"github.com/albertgmz/sermo-cli/internal/update"
)

// env holds the parts of the system that tests replace.
type env struct {
	// sleep waits between the polls of the device login. Nil uses a timer.
	sleep       func(context.Context, time.Duration) error
	openURL     func(string) error
	checkUpdate func(context.Context, string) (update.Result, error)
}

// cli holds the flags and the state that all commands share.
type cli struct {
	env    *env
	server string
	lang   string
	noAnim bool
	cfg    config.Config
}

func newRootCmd(e *env) *cobra.Command {
	c := &cli{env: e}
	root := &cobra.Command{
		Use:               "sermo",
		Short:             i18n.T("cli.root.short"),
		Args:              cobra.NoArgs,
		SilenceUsage:      true,
		SilenceErrors:     true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		PersistentPreRunE: func(*cobra.Command, []string) error { return c.prepare() },
		RunE:              c.runTUI,
	}
	f := root.PersistentFlags()
	f.StringVar(&c.server, "server", "", i18n.T("cli.flag.server"))
	f.BoolVar(&c.noAnim, "no-anim", false, i18n.T("cli.flag.noAnim"))
	f.StringVar(&c.lang, "lang", "", i18n.T("cli.flag.lang"))
	root.AddCommand(
		&cobra.Command{Use: "login", Short: i18n.T("cli.login.short"), Args: cobra.NoArgs, RunE: c.login},
		&cobra.Command{Use: "logout", Short: i18n.T("cli.logout.short"), Args: cobra.NoArgs, RunE: c.logout},
		&cobra.Command{Use: "version", Short: i18n.T("cli.version.short"), Args: cobra.NoArgs, RunE: c.version},
		&cobra.Command{Use: "update", Short: i18n.T("cli.update.short"), Args: cobra.NoArgs, RunE: c.update},
	)
	return root
}

// prepare reads the config file, sets the language and resolves the server.
func (c *cli) prepare() error {
	cfg, err := config.Load()
	if err != nil {
		return errors.New(i18n.Tf("cli.config.failed", map[string]any{"Error": err}))
	}
	c.cfg = cfg
	i18n.SetLocale(cfg.Locale)
	if c.lang != "" {
		i18n.SetLocale(c.lang)
	}
	server, err := config.ResolveServer(c.server, cfg)
	if err != nil {
		return errors.New(i18n.Tf("cli.server.invalid", map[string]any{"Error": err}))
	}
	c.server = server
	return nil
}

// runTUI starts the full screen client. Guests can use it without a token.
func (c *cli) runTUI(cmd *cobra.Command, _ []string) error {
	if !isTerminal(cmd.OutOrStdout()) {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cli.root.noTerminal"))
		return errReported
	}
	store := auth.NewStore()
	cred := auth.Resolve(store, c.server)
	client, err := api.New(api.Options{Server: c.server, Token: cred.Token, UserAgent: buildinfo.UserAgent(), Context: cmd.Context()})
	if err != nil {
		return err
	}
	d := ui.Deps{API: client, Cred: cred, Store: store, Server: c.server, Config: &c.cfg, NoAnim: c.noAnim}
	return tui.Run(cmd.Context(), d, tui.Options{Start: tui.StartSplash, Sleep: c.env.sleep, Locale: c.lang})
}

// guestClient returns an API client without a token.
func (c *cli) guestClient() (*api.Client, error) {
	return api.New(api.Options{Server: c.server, UserAgent: buildinfo.UserAgent()})
}
