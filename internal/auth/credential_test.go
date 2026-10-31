package auth

import (
	"testing"

	"github.com/zalando/go-keyring"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		stored string
		want   Credential
	}{
		{"api key wins", "key-1", "tok-1", Credential{Token: "key-1", Source: SourceAPIKey}},
		{"stored token", "", "tok-1", Credential{Token: "tok-1", Source: SourceDevice}},
		{"none", "", "", Credential{Source: SourceNone}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyring.MockInit()
			useConfigDir(t)
			t.Setenv("SERMO_API_KEY", tt.apiKey)
			s := NewStore()
			if tt.stored != "" {
				if err := s.Save(testServer, Token{AccessToken: tt.stored}); err != nil {
					t.Fatal(err)
				}
			}
			if got := Resolve(s, testServer); got != tt.want {
				t.Fatalf("Resolve() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
