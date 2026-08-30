package output

import (
	"os"
	"testing"
)

// TestMain seeds IsAgent false so the host machine's agent-harness env (this
// suite runs inside one) cannot flip format-resolution expectations. Tests
// that need agent detection override the seam and restore it via t.Cleanup.
func TestMain(m *testing.M) {
	IsAgent = func() bool { return false }
	os.Exit(m.Run())
}
