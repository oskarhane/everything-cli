package auth

import (
	"golang.org/x/oauth2"

	"github.com/oskarhane/everything-cli/internal/redact"
)

// RegisterSecret adds value to the process-wide redaction registry in
// internal/redact. It is re-exported here so the auth mint/read points and
// provider auth strategies register secrets without importing the leaf
// package directly.
func RegisterSecret(value string) { redact.RegisterSecret(value) }

// Redact replaces every registered secret in s with "***".
func Redact(s string) string { return redact.Redact(s) }

// registerTokenSecrets registers both values of an OAuth token. Both are as
// sensitive as a refresh token and must never print.
func registerTokenSecrets(tok *oauth2.Token) {
	if tok == nil {
		return
	}
	RegisterSecret(tok.AccessToken)
	RegisterSecret(tok.RefreshToken)
}
