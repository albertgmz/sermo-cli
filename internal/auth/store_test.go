package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

const testServer = "https://forum.example"

var errNoKeychain = errors.New("no keychain")

func useConfigDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "sermo")
	t.Setenv("SERMO_CONFIG_DIR", dir)
	return dir
}

func TestStoreKeyring(t *testing.T) {
	keyring.MockInit()
	dir := useConfigDir(t)
	s := NewStore()
	want := Token{AccessToken: "tok-1", ExpiresAt: time.Now().Add(time.Hour).Truncate(time.Second).UTC()}
	if err := s.Save(testServer, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := keyring.Get("sermo-cli", testServer); err != nil {
		t.Fatalf("keyring.Get() error = %v, want token in keyring", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tokens.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tokens.json exists, want keyring only (err = %v)", err)
	}
	got, err := s.Load(testServer)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.AccessToken != want.AccessToken || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
	if err := s.Delete(testServer); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.Load(testServer); !errors.Is(err, ErrNoToken) {
		t.Fatalf("Load() after Delete error = %v, want ErrNoToken", err)
	}
}

func TestStoreLoadMissing(t *testing.T) {
	keyring.MockInit()
	useConfigDir(t)
	if _, err := NewStore().Load(testServer); !errors.Is(err, ErrNoToken) {
		t.Fatalf("Load() error = %v, want ErrNoToken", err)
	}
}

func TestStoreDeleteAbsent(t *testing.T) {
	for name, init := range map[string]func(){
		"keyring": keyring.MockInit,
		"file":    func() { keyring.MockInitWithError(errNoKeychain) },
	} {
		t.Run(name, func(t *testing.T) {
			init()
			useConfigDir(t)
			if err := NewStore().Delete(testServer); err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
		})
	}
}

func TestStoreFileFallback(t *testing.T) {
	keyring.MockInitWithError(errNoKeychain)
	dir := useConfigDir(t)
	s := NewStore()
	if err := s.Save(testServer, Token{AccessToken: "tok-1"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.Save("https://other.example", Token{AccessToken: "tok-2"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	assertPrivateFile(t, filepath.Join(dir, "tokens.json"))
	got, err := s.Load(testServer)
	if err != nil || got.AccessToken != "tok-1" {
		t.Fatalf("Load() = %+v, %v", got, err)
	}
	if err := s.Delete(testServer); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.Load(testServer); !errors.Is(err, ErrNoToken) {
		t.Fatalf("Load() after Delete error = %v, want ErrNoToken", err)
	}
	if got, err := s.Load("https://other.example"); err != nil || got.AccessToken != "tok-2" {
		t.Fatalf("other server Load() = %+v, %v", got, err)
	}
}

func assertPrivateFile(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("%s mode = %o, want 600", path, fi.Mode().Perm())
	}
}

func TestStoreLoadFileWhenKeyringEmpty(t *testing.T) {
	keyring.MockInitWithError(errNoKeychain)
	useConfigDir(t)
	if err := NewStore().Save(testServer, Token{AccessToken: "tok-1"}); err != nil {
		t.Fatal(err)
	}
	keyring.MockInit()
	got, err := NewStore().Load(testServer)
	if err != nil || got.AccessToken != "tok-1" {
		t.Fatalf("Load() = %+v, %v, want token from file", got, err)
	}
	if err := NewStore().Delete(testServer); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore().Load(testServer); !errors.Is(err, ErrNoToken) {
		t.Fatalf("Load() after Delete error = %v, want ErrNoToken", err)
	}
}

func TestStoreExpiredToken(t *testing.T) {
	for name, init := range map[string]func(){
		"keyring": keyring.MockInit,
		"file":    func() { keyring.MockInitWithError(errNoKeychain) },
	} {
		t.Run(name, func(t *testing.T) {
			init()
			useConfigDir(t)
			s := NewStore()
			if err := s.Save(testServer, Token{AccessToken: "old", ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Load(testServer); !errors.Is(err, ErrNoToken) {
				t.Fatalf("Load() error = %v, want ErrNoToken", err)
			}
			if _, err := keyring.Get("sermo-cli", testServer); err == nil {
				t.Fatal("expired token is still in the keyring")
			}
			keyring.MockInit()
			if _, err := s.Load(testServer); !errors.Is(err, ErrNoToken) {
				t.Fatalf("expired token is still in the file: %v", err)
			}
		})
	}
}

func TestStoreCorruptFile(t *testing.T) {
	keyring.MockInitWithError(errNoKeychain)
	dir := useConfigDir(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tokens.json"), []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewStore().Load(testServer)
	if err == nil || errors.Is(err, ErrNoToken) {
		t.Fatalf("Load() error = %v, want decode error", err)
	}
}
