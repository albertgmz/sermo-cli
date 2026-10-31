// Command sermo is a terminal client for Sermo forums.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/cli/browser"

	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/update"
)

// errReported tells that the command already wrote its error message.
var errReported = errors.New("error already reported")

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := execute(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}

// execute runs the command line and returns the exit code.
func execute(ctx context.Context, args []string) int {
	i18n.SetLocale(startLocale(args))
	cmd := newRootCmd(&env{openURL: browser.OpenURL, checkUpdate: update.Check})
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	if !errors.Is(err, errReported) {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), err)
	}
	return 1
}

// startLocale returns the language for the help texts before cobra reads
// the flags. The --lang flag has priority over the config file.
func startLocale(args []string) string {
	for i, a := range args {
		if v, ok := strings.CutPrefix(a, "--lang="); ok {
			return v
		}
		if a == "--lang" && i+1 < len(args) {
			return args[i+1]
		}
	}
	cfg, _ := config.Load()
	return cfg.Locale
}
