package auth

import "os"

// Source tells where a credential comes from.
type Source int

// Credential sources.
const (
	SourceNone Source = iota
	SourceAPIKey
	SourceDevice
)

// Credential is the token to send to the server and its source.
type Credential struct {
	Token  string
	Source Source
}

// Resolve selects the credential for server. A non-empty $SERMO_API_KEY has priority.
// Then Resolve uses the stored token. When no token is available, it returns SourceNone.
func Resolve(s *Store, server string) Credential {
	if key := os.Getenv("SERMO_API_KEY"); key != "" {
		return Credential{Token: key, Source: SourceAPIKey}
	}
	t, err := s.Load(server)
	if err != nil {
		return Credential{Source: SourceNone}
	}
	return Credential{Token: t.AccessToken, Source: SourceDevice}
}
