package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
)

func (c *cli) localesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "locales", Short: i18n.T("cli.locales.short"), Args: cobra.NoArgs}
	cmd.AddCommand(&cobra.Command{
		Use:   "export [lang]",
		Short: i18n.T("cli.locales.export.short"),
		Args:  cobra.MaximumNArgs(1),
		RunE:  c.exportLocale,
	})
	return cmd
}

// exportLocale writes the texts of a language to the translations folder.
// Without an argument it uses the current language.
func (c *cli) exportLocale(cmd *cobra.Command, args []string) error {
	lang := i18n.Locale()
	if len(args) == 1 {
		lang = args[0]
	}
	dir, err := config.LocalesDir()
	if err != nil {
		return err
	}
	written, skipped, err := i18n.Export(dir, lang)
	if errors.Is(err, i18n.ErrLanguage) {
		return errors.New(i18n.Tf("cli.locales.export.badLanguage", map[string]any{"Lang": lang}))
	}
	if err != nil {
		return errors.New(i18n.Tf("cli.locales.export.failed", map[string]any{"Error": err}))
	}
	out := cmd.OutOrStdout()
	for _, f := range written {
		_, _ = fmt.Fprintln(out, i18n.Tf("cli.locales.export.written", map[string]any{"File": f}))
	}
	for _, f := range skipped {
		_, _ = fmt.Fprintln(out, i18n.Tf("cli.locales.export.skipped", map[string]any{"File": f}))
	}
	_, _ = fmt.Fprintln(out, i18n.T("cli.locales.export.done"))
	return nil
}

// loadUserLocales loads the user translation files for a command. It
// writes one line to w about the files that it did not use.
func loadUserLocales(w io.Writer) {
	dir, err := config.LocalesDir()
	if err != nil {
		return
	}
	problems := i18n.LoadDir(dir)
	if len(problems) == 0 {
		return
	}
	files := make([]string, len(problems))
	for i, p := range problems {
		files[i] = p.File
	}
	_, _ = fmt.Fprintln(w, i18n.Plural("locales.loadFailed", len(problems), map[string]any{"Files": strings.Join(files, "; ")}))
}
