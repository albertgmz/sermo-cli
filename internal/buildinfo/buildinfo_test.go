package buildinfo

import (
	"os"
	"runtime"
	"testing"
)

func TestUserAgent(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })
	Version = "1.2.3"

	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	want := "sermo-cli/1.2.3 (" + runtime.GOOS + "/" + runtime.GOARCH + "; " + host + ")"
	if got := UserAgent(); got != want {
		t.Fatalf("UserAgent() = %q, want %q", got, want)
	}
}

func TestVersionDefault(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("Version = %q, want dev", Version)
	}
}
