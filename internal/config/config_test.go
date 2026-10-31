package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func useTempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "sermo")
	t.Setenv("SERMO_CONFIG_DIR", dir)
	return dir
}

func TestDirUsesEnv(t *testing.T) {
	dir := useTempDir(t)
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("Dir() = %q, want %q", got, dir)
	}
}

func TestDirDefault(t *testing.T) {
	t.Setenv("SERMO_CONFIG_DIR", "")
	base, err := os.UserConfigDir()
	if err != nil {
		t.Skip("no user config dir:", err)
	}
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(base, "sermo"); got != want {
		t.Fatalf("Dir() = %q, want %q", got, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	useTempDir(t)
	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if c != (Config{}) {
		t.Fatalf("Load() = %+v, want zero Config", c)
	}
}

func TestLoadCorruptJSON(t *testing.T) {
	dir := useTempDir(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error for corrupt JSON")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := useTempDir(t)
	want := Config{Server: "https://example.org", Theme: "malecon", Effect: "fire", ReducedMotion: true, Locale: "en"}
	if err := Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}

	want.Theme = "ambar-crt"
	if err := Save(want); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	if got, _ := Load(); got != want {
		t.Fatalf("Load() after overwrite = %+v, want %+v", got, want)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("dir has %d entries, want only config.json", len(entries))
	}
	if runtime.GOOS == "windows" {
		return
	}
	assertMode(t, filepath.Join(dir, "config.json"), 0o600)
	assertMode(t, dir, 0o700)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}

func TestResolveServerPrecedence(t *testing.T) {
	file := Config{Server: "https://file.example"}
	tests := []struct {
		name, flag, env string
		cfg             Config
		want            string
	}{
		{"flag wins", "https://flag.example", "https://env.example", file, "https://flag.example"},
		{"env over file", "", "https://env.example", file, "https://env.example"},
		{"file over default", "", "", file, "https://file.example"},
		{"default", "", "", Config{}, DefaultServer},
		{"trailing slash removed", "http://localhost:3000/", "", Config{}, "http://localhost:3000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SERMO_SERVER", tt.env)
			got, err := ResolveServer(tt.flag, tt.cfg)
			if err != nil {
				t.Fatalf("ResolveServer() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveServer() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveServerRejectsBadURL(t *testing.T) {
	t.Setenv("SERMO_SERVER", "")
	for _, in := range []string{"ftp://example.org", "example.org", "https://", "http://%zz", "::"} {
		t.Run(in, func(t *testing.T) {
			if got, err := ResolveServer(in, Config{}); err == nil {
				t.Fatalf("ResolveServer(%q) = %q, want error", in, got)
			}
		})
	}
}
