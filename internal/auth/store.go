package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/albertgmz/sermo-cli/internal/config"
)

const (
	keyringService = "sermo-cli"
	tokensFile     = "tokens.json"
)

// ErrNoToken tells that the store has no valid token for the server.
var ErrNoToken = errors.New("auth: no stored token")

// Store keeps one token for each server URL.
// It uses the system keyring first. When the keyring is not available, it uses config.Dir()/tokens.json.
type Store struct{}

// NewStore returns a Store.
func NewStore() *Store {
	return &Store{}
}

// Load returns the token for server. It returns ErrNoToken when no token is stored.
// Load deletes an expired token and returns ErrNoToken.
func (s *Store) Load(server string) (Token, error) {
	t, err := load(server)
	if err != nil {
		return Token{}, err
	}
	if t.expired(time.Now()) {
		if err := s.Delete(server); err != nil {
			return Token{}, err
		}
		return Token{}, ErrNoToken
	}
	return t, nil
}

// load reads the keyring first. When the keyring has no token or fails, load reads the file.
func load(server string) (Token, error) {
	v, err := keyring.Get(keyringService, server)
	if err != nil {
		return loadFile(server)
	}
	var t Token
	if err := json.Unmarshal([]byte(v), &t); err != nil {
		return Token{}, fmt.Errorf("auth: decode keyring token: %w", err)
	}
	return t, nil
}

func loadFile(server string) (Token, error) {
	m, err := readTokens()
	if err != nil {
		return Token{}, err
	}
	t, ok := m[server]
	if !ok {
		return Token{}, ErrNoToken
	}
	return t, nil
}

// Save stores t for server. It writes the file only when the keyring fails.
func (s *Store) Save(server string, t Token) error {
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("auth: encode token: %w", err)
	}
	if keyring.Set(keyringService, server, string(data)) == nil {
		return nil
	}
	m, err := readTokens()
	if err != nil {
		return err
	}
	m[server] = t
	return writeTokens(m)
}

// Delete removes the token for server from the keyring and from the file.
// It returns no error when no token is stored.
func (s *Store) Delete(server string) error {
	// An error here means that no token is in the keyring, or that no keyring is available.
	_ = keyring.Delete(keyringService, server)
	m, err := readTokens()
	if err != nil {
		return err
	}
	if _, ok := m[server]; !ok {
		return nil
	}
	delete(m, server)
	return writeTokens(m)
}

func readTokens() (map[string]Token, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	m := map[string]Token{}
	data, err := os.ReadFile(filepath.Join(dir, tokensFile))
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("auth: read %s: %w", tokensFile, err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("auth: decode %s: %w", tokensFile, err)
	}
	return m, nil
}

func writeTokens(m map[string]Token) error {
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("auth: encode %s: %w", tokensFile, err)
	}
	if err := writeFileAtomic(dir, tokensFile, data); err != nil {
		return fmt.Errorf("auth: write %s: %w", tokensFile, err)
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
