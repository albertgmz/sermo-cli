package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/update"
)

// setup gives each test an empty keyring, an empty config directory and
// no server or API key from the environment.
func setup(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	t.Setenv("SERMO_CONFIG_DIR", t.TempDir())
	t.Setenv("SERMO_SERVER", "")
	t.Setenv("SERMO_API_KEY", "")
	i18n.SetLocale("es")
	t.Cleanup(func() { i18n.SetLocale("es") })
}

// testEnv does not wait, does not open a browser and finds no release.
func testEnv() *env {
	return &env{
		sleep:   func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		openURL: func(string) error { return nil },
		checkUpdate: func(_ context.Context, current string) (update.Result, error) {
			return update.Result{Current: current}, errors.New("offline")
		},
	}
}

type result struct {
	out, err string
	runErr   error
}

func run(t *testing.T, e *env, args ...string) result {
	t.Helper()
	cmd := newRootCmd(e)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return result{out: out.String(), err: errOut.String(), runErr: err}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
