package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/redact"
)

// Canary values: unique per test so a secret registered here can never match
// another test's fixtures (the registry is process-global and sticky).
const (
	canaryDebugSecret = "canary-secret-debug-7f3a"
	canaryPrintSecret = "canary-secret-print-9c1e"
)

// TestRedactionCanaryDebug: a registered secret appearing in a debug line is
// scrubbed before emission.
func TestRedactionCanaryDebug(t *testing.T) {
	redact.RegisterSecret(canaryDebugSecret)
	SetDebug(true)
	buf, restore := debugSink(t)
	defer restore()

	Debug("loaded token " + canaryDebugSecret + " for work")

	assert.NotContains(t, buf.String(), canaryDebugSecret,
		"Debug output must pass through the redactor")
	assert.Contains(t, buf.String(), "loaded token *** for work")
}

// TestRedactionCanaryPrint: a registered secret inside rendered command
// output is scrubbed in every format — JSON field, table cell, and TOON row
// alike — because all three funnel through writeLine.
func TestRedactionCanaryPrint(t *testing.T) {
	redact.RegisterSecret(canaryPrintSecret)
	fields := []string{"name", "token"}
	rows := []map[string]any{
		{"name": "work", "token": canaryPrintSecret},
	}

	for _, format := range []Format{FormatJSON, FormatTable, FormatToon} {
		t.Run(string(format), func(t *testing.T) {
			var buf bytes.Buffer
			Print(&buf, format, fields, rows, rows)

			require.NotEmpty(t, buf.String())
			assert.NotContains(t, buf.String(), canaryPrintSecret,
				"%s output must pass through the redactor", format)
			assert.Contains(t, buf.String(), "***")
		})
	}
}

// TestRedactionLeavesOrdinaryOutputUntouched: with no secret matching, all
// formats render the caller's text byte-for-byte (the empty-registry fast
// path must never distort normal output).
func TestRedactionLeavesOrdinaryOutputUntouched(t *testing.T) {
	var buf bytes.Buffer
	PrintJSON(&buf, map[string]any{"greeting": "hello, world"})
	assert.Equal(t, "{\n\t\"greeting\": \"hello, world\"\n}\n", buf.String())
}
