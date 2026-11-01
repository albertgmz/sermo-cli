// Package config reads and writes the user configuration file and resolves the server URL.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// DefaultServer is the server URL when no other value is set.
const DefaultServer = "https://fcuba.net"

const fileName = "config.json"

// Config is the content of config.json.
type Config struct {
	Server        string `json:"server,omitempty"`
	Theme         string `json:"theme,omitempty"`         // theme ID; "" means the default theme
	Effect        string `json:"effect,omitempty"`        // effect name; "" means a random effect
	ReducedMotion bool   `json:"reducedMotion,omitempty"` // true stops animations
	Locale        string `json:"locale,omitempty"`        // "es" or "en"; "" means "es"
	Prefetch      string `json:"prefetch,omitempty"`      // a Prefetch value; "" means PrefetchOff
	Interface     string `json:"interface,omitempty"`     // InterfaceBBS or "" for the modern interface
	Mouse         string `json:"mouse,omitempty"`         // MouseOff or "" for mouse clicks
}

// MouseOff is the value of Config.Mouse that stops the mouse clicks. Then
// the terminal selects text with the mouse.
const MouseOff = "off"

// MouseOn reports whether the app uses the mouse.
func (c Config) MouseOn() bool { return c.Mouse != MouseOff }

// InterfaceBBS is the value of Config.Interface for the BBS interface.
const InterfaceBBS = "bbs"

// BBS reports whether the setting selects the BBS interface.
func (c Config) BBS() bool { return c.Interface == InterfaceBBS }

// The values of Config.Prefetch. PrefetchLight gets the next page of a list in the background.
// PrefetchFull also gets the first thread page of the unread forums.
const (
	PrefetchOff   = "off"
	PrefetchLight = "light"
	PrefetchFull  = "full"
)

// LightPrefetch reports whether the setting is PrefetchLight or PrefetchFull.
func (c Config) LightPrefetch() bool { return c.Prefetch == PrefetchLight || c.FullPrefetch() }

// FullPrefetch reports whether the setting is PrefetchFull.
func (c Config) FullPrefetch() bool { return c.Prefetch == PrefetchFull }

// Dir returns the configuration directory.
// It is $SERMO_CONFIG_DIR when set, else os.UserConfigDir()/sermo.
func Dir() (string, error) {
	if d := os.Getenv("SERMO_CONFIG_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: find config dir: %w", err)
	}
	return filepath.Join(base, "sermo"), nil
}

// ThemesDir returns the directory of the user theme files: Dir()/themes.
func ThemesDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "themes"), nil
}

// LocalesDir returns the directory of the user translation files: Dir()/locales.
// It holds one directory for each language, for example locales/pt/core.json.
func LocalesDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "locales"), nil
}

// Load reads config.json. A missing file gives a zero Config and no error.
func Load() (Config, error) {
	var c Config
	dir, err := Dir()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("config: read: %w", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("config: decode %s: %w", fileName, err)
	}
	return c, nil
}

// Save writes config.json with mode 0600 in a directory with mode 0700.
// It writes a temporary file first and then renames it.
func Save(c Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}
	if err := writeFileAtomic(dir, fileName, data); err != nil {
		return fmt.Errorf("config: write: %w", err)
	}
	return nil
}

// writeFileAtomic replaces dir/name with data. os.CreateTemp gives the file mode 0600.
func writeFileAtomic(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, name+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}

// ResolveServer selects the server URL. The order is flag, $SERMO_SERVER, c.Server, DefaultServer.
// The URL must use http or https. ResolveServer removes a trailing "/".
func ResolveServer(flag string, c Config) (string, error) {
	s := firstNonEmpty(flag, os.Getenv("SERMO_SERVER"), c.Server, DefaultServer)
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("config: invalid server URL %q: use an http or https URL", s)
	}
	return strings.TrimRight(s, "/"), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
