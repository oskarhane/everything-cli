package apikey

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactRegistry(t *testing.T) {
	t.Run("empty value is not registered", func(t *testing.T) {
		RegisterSecret("")
		assert.Equal(t, "nothing to scrub", Redact("nothing to scrub"),
			"registering the empty string must not destroy output")
	})
	t.Run("every occurrence is scrubbed", func(t *testing.T) {
		RegisterSecret("test-key-multi")
		out := Redact("a test-key-multi b test-key-multi")
		assert.NotContains(t, out, "test-key-multi")
		assert.Equal(t, "a "+redacted+" b "+redacted, out)
	})
	t.Run("re-registering the same value is idempotent", func(t *testing.T) {
		RegisterSecret("test-key-once")
		RegisterSecret("test-key-once")
		assert.Equal(t, redacted, Redact("test-key-once"))
	})
}
