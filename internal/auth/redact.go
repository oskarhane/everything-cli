package auth

import (
	"strings"
	"sync"

	"golang.org/x/oauth2"
)

// The redaction registry holds secret values (OAuth access/refresh tokens
// and, for other strategies, API keys or bearer tokens) so output paths can
// scrub them before emission. Per AGENTS.md, a secret is registered at its
// mint/read point — the moment the value enters the process — never at
// print time, so no table cell, TOON row, or debug line can leak it.
var (
	secretsMu sync.Mutex
	secrets   = map[string]struct{}{}
)

// RegisterSecret adds value to the redaction registry. Empty values are
// ignored (an absent refresh token must not redact every empty string).
func RegisterSecret(value string) {
	if value == "" {
		return
	}
	secretsMu.Lock()
	defer secretsMu.Unlock()
	secrets[value] = struct{}{}
}

// Redact replaces every registered secret in s with "***". Output paths
// pass rendered text through Redact before emission.
func Redact(s string) string {
	if s == "" {
		return s
	}
	secretsMu.Lock()
	defer secretsMu.Unlock()
	for secret := range secrets {
		if strings.Contains(s, secret) {
			s = strings.ReplaceAll(s, secret, "***")
		}
	}
	return s
}

// registerTokenSecrets registers both values of an OAuth token. Both are as
// sensitive as a refresh token and must never print.
func registerTokenSecrets(tok *oauth2.Token) {
	if tok == nil {
		return
	}
	RegisterSecret(tok.AccessToken)
	RegisterSecret(tok.RefreshToken)
}
