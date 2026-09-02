package redact

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRedactMasksRegisteredSecrets: a registered secret never survives
// Redact; unregistered text passes through untouched.
func TestRedactMasksRegisteredSecrets(t *testing.T) {
	RegisterSecret("redact-test-access-token")
	assert.Equal(t, "header *** trailer", Redact("header redact-test-access-token trailer"))
	assert.Equal(t, "nothing secret here", Redact("nothing secret here"))
	assert.Equal(t, "", Redact(""))
}

// TestRegisterSecretIgnoresEmpty: an absent token value must not redact
// every string.
func TestRegisterSecretIgnoresEmpty(t *testing.T) {
	RegisterSecret("")
	assert.Equal(t, "still here", Redact("still here"))
}

// TestRedactScrubsEveryOccurrence: a secret appearing several times is
// scrubbed at every occurrence.
func TestRedactScrubsEveryOccurrence(t *testing.T) {
	RegisterSecret("redact-test-multi")
	out := Redact("a redact-test-multi b redact-test-multi")
	assert.NotContains(t, out, "redact-test-multi")
	assert.Equal(t, "a *** b ***", out)
}

// TestRegisterSecretIsIdempotent: re-registering the same value changes
// nothing (the registry is a set).
func TestRegisterSecretIsIdempotent(t *testing.T) {
	RegisterSecret("redact-test-once")
	RegisterSecret("redact-test-once")
	assert.Equal(t, "***", Redact("redact-test-once"))
}
