package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// debugSink swaps debugW for a capture buffer and returns a restore func.
func debugSink(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prevW, prevOn := debugW, debugOn
	debugW = &buf
	return &buf, func() {
		debugW = prevW
		SetDebug(prevOn)
	}
}

func TestDebugOffByDefault(t *testing.T) {
	SetDebug(false)
	buf, restore := debugSink(t)
	defer restore()

	Debug("credentials: using config dir path\x1b[31m")

	assert.Empty(t, buf.String(), "debug must not emit when disabled")
}

func TestDebugStripsControlBytes(t *testing.T) {
	SetDebug(true)
	buf, restore := debugSink(t)
	defer restore()

	Debug("using credentials \x1b[31mcredentials.json\x1b[0m")

	assert.Equal(t, "using credentials ?[31mcredentials.json?[0m\n", buf.String())
}
