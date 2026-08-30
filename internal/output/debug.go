package output

import (
	"io"
	"os"
)

// debugOn gates debug emission and defaults to off; debugW is the sink so
// tests can capture output.
var (
	debugOn bool
	debugW  io.Writer = os.Stderr
)

// SetDebug enables or disables debug output. It is called once, from the
// root command's PersistentPreRun, with the --debug flag value.
func SetDebug(on bool) {
	debugOn = on
}

// Debug writes one debug line to stderr when debug is enabled; it no-ops
// otherwise. The caller composes msg from non-secret data only — token
// values must never be passed. Every line passes through StripControl so
// raw data cannot inject terminal escapes.
func Debug(msg string) {
	if !debugOn {
		return
	}
	writeLine(debugW, StripControl(msg))
}
