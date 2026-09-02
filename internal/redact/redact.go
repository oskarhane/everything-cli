// Package redact holds the process-wide redaction registry: the set of
// secret values (OAuth access/refresh tokens, OAuth app client secrets,
// provider API keys) that must never appear in CLI output. It lives in a
// leaf package so both registration points (internal/auth and the provider
// strategies) and emission points (internal/output, the top-level error
// print) can consult it without an import cycle.
package redact

import (
	"sort"
	"strings"
	"sync"
)

var (
	secretsMu sync.Mutex
	secrets   = map[string]struct{}{}
)

// RegisterSecret adds value to the redaction registry. Empty values are
// ignored (an absent refresh token must not redact every empty string). A
// secret is registered at its mint/read point — the moment the value enters
// the process — never at print time, so no table cell, TOON row, or debug
// line can leak it.
func RegisterSecret(value string) {
	if value == "" {
		return
	}
	secretsMu.Lock()
	defer secretsMu.Unlock()
	secrets[value] = struct{}{}
}

// Redact replaces every registered secret in s with "***". Emission points
// pass rendered text through Redact before writing. With no secrets
// registered it short-circuits, so normal output pays no redaction cost and
// passes through untouched.
func Redact(s string) string {
	if s == "" {
		return s
	}
	secretsMu.Lock()
	defer secretsMu.Unlock()
	if len(secrets) == 0 {
		return s
	}
	// Snapshot longest-first: if one secret is a prefix of another,
	// replacing the shorter first would leak the longer's tail.
	sorted := make([]string, 0, len(secrets))
	for secret := range secrets {
		sorted = append(sorted, secret)
	}
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	for _, secret := range sorted {
		if strings.Contains(s, secret) {
			s = strings.ReplaceAll(s, secret, "***")
		}
	}
	return s
}
