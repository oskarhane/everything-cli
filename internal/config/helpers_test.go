package config

import (
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// keysOf lists a JSON object's keys, for schema assertions.
func keysOf(raw map[string]any) []string {
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	return keys
}

// newTestStore returns a hermetic store on an in-memory FS: tests must never
// read or write the real ~/.config/google-cli.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(afero.NewMemMapFs(), "/config")
	require.NoError(t, err)
	return store
}

// testToken builds a token for round-trip assertions.
func testToken(access string) *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  access,
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}
}
