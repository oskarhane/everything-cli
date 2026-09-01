package apikey

import (
	"strings"
	"sync"
)

// The redaction registry records captured API keys so output paths can
// scrub them. It lives at the strategy seam until the output layer grows a
// shared registry; Redact is the scrub function that wiring calls.
var (
	secretsMu sync.Mutex
	secrets   []string
)

// redacted is the replacement marker for scrubbed secrets.
const redacted = "***"

// RegisterSecret adds value to the redaction registry. Empty values are
// ignored (scrubbing the empty string would destroy all output), and a
// value is recorded once. Register at the mint/read point, before anything
// could print the value.
func RegisterSecret(value string) {
	if value == "" {
		return
	}
	secretsMu.Lock()
	defer secretsMu.Unlock()
	for _, s := range secrets {
		if s == value {
			return
		}
	}
	secrets = append(secrets, value)
}

// Redact returns s with every registered secret replaced by "***". A
// secret appearing several times is scrubbed at every occurrence.
func Redact(s string) string {
	secretsMu.Lock()
	defer secretsMu.Unlock()
	for _, secret := range secrets {
		s = strings.ReplaceAll(s, secret, redacted)
	}
	return s
}
