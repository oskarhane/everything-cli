// Package output renders command results as JSON, a terminal table, or TOON.
//
// Format auto-detection precedence: explicit --format flag > agent harness
// (toon) > TTY stdout (table) > json.
package output

import (
	"fmt"
	"os"

	"github.com/oskarhane/google-cli/internal/agent"
	"golang.org/x/term"
)

// Format identifies an output rendering mode.
type Format string

const (
	// FormatJSON renders indented JSON for non-TTY (piped) consumers.
	FormatJSON Format = "json"
	// FormatTable renders a go-pretty table for TTY consumers.
	FormatTable Format = "table"
	// FormatToon renders TOON for agent-harness consumers.
	FormatToon Format = "toon"
)

// IsFormat reports whether s is a valid explicit --format value.
func IsFormat(s string) bool {
	switch Format(s) {
	case FormatJSON, FormatTable, FormatToon:
		return true
	}
	return false
}

// StdoutIsTerminal is the package-level test seam for terminal detection. It
// reads the real os.Stdout file descriptor directly so wrapping the command's
// writer (cmd.OutOrStdout) cannot affect format resolution. Tests may replace
// this var and restore it via t.Cleanup.
var StdoutIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// StdinIsTerminal is the package-level test seam for terminal detection on
// os.Stdin, so interactive prompts (e.g. the update confirmation) can be
// driven hermetically. Tests may replace this var and restore it via
// t.Cleanup.
var StdinIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// IsAgent is the package-level test seam for agent-harness detection. When it
// reports true and no explicit --format is given, output resolves to toon.
var IsAgent = agent.DetectFn

// Warnf is the seam for resolution warnings, so tests can capture the
// invalid-format warning without reading process stderr.
var Warnf = func(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}

// ResolveOutput returns the effective output format for a --format flag
// value. A non-empty valid flag always wins. An invalid non-empty value warns
// and falls through to auto-detection: an agent harness yields toon, a TTY
// stdout yields table, and anything else (piped/redirected) yields json.
func ResolveOutput(formatFlag string) Format {
	if formatFlag != "" {
		if IsFormat(formatFlag) {
			return Format(formatFlag)
		}
		Warnf("ignoring invalid --format %q (expected json, table, or toon)\n", formatFlag)
	}
	if IsAgent() {
		return FormatToon
	}
	if StdoutIsTerminal() {
		return FormatTable
	}
	return FormatJSON
}
