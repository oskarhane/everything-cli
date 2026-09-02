package app

import (
	"fmt"
	"io"

	"github.com/oskarhane/everything-cli/internal/output"
)

// legacyGoogleResources are the bare top-level resource words that moved
// under the `google` provider command when the CLI went provider-first.
var legacyGoogleResources = map[string]bool{
	"gmail":    true,
	"calendar": true,
	"drive":    true,
	"docs":     true,
	"sheets":   true,
	"slides":   true,
	"youtube":  true,
}

// legacyAccountVerbs are the account-management verbs that moved under each
// provider (`google account add`, ...). `account list` is deliberately
// absent: the top-level account command keeps a read-only cross-provider
// list, so it needs no redirect.
var legacyAccountVerbs = map[string]bool{
	"add":    true,
	"use":    true,
	"get":    true,
	"remove": true,
}

// legacyValueFlags take a separate value token (`--format json ...`), so
// the shim must skip that token when scanning for the first command word.
// `--debug` is boolean and needs no skip; unknown flags stop the scan so
// cobra reports them exactly as before.
var legacyValueFlags = map[string]bool{
	"format":      true,
	"account":     true,
	"credentials": true,
}

// RewriteLegacyArgs maps pre-provider invocations onto the provider-first
// tree: a bare Google resource word (`gmail list`) or a moved account verb
// (`account add work`) is rewritten by inserting `google` ahead of the
// first command word, with a deprecation warning on errw. Leading root
// flags are skipped (`--format json gmail list` rewrites too; `--format`,
// `--account`, and `--credentials` consume their value token, `--debug`
// does not). Anything else passes through unchanged — including
// `account list` (a real top-level command) and argv whose first flag is
// unknown to the shim (cobra will error on it as before).
func RewriteLegacyArgs(binary string, args []string, errw io.Writer) []string {
	if len(args) == 0 {
		return args
	}
	idx := firstCommandWord(args)
	if idx < 0 {
		return args
	}
	word := args[idx]
	rewrite := legacyGoogleResources[word] ||
		(word == "account" && len(args) > idx+1 && legacyAccountVerbs[args[idx+1]])
	if !rewrite {
		return args
	}
	out := make([]string, 0, len(args)+1)
	out = append(out, args[:idx]...)
	out = append(out, "google")
	out = append(out, args[idx:]...)
	// argv is echoed into the warning; strip control bytes so a crafted
	// argument cannot inject ANSI escapes into the terminal.
	_, _ = fmt.Fprintf(errw,
		"warning: `%s %s` is deprecated; use `%s %s` "+
			"(the back-compat shim will be removed in a future release)\n",
		binary, output.StripControl(joinArgs(args)), binary, output.StripControl(joinArgs(out)))
	return out
}

// firstCommandWord returns the index of the first non-flag token, skipping
// the value tokens of known value-taking flags. It returns -1 when argv is
// all flags, and stops at the first unknown flag (returning -1) so cobra
// reports it exactly as it would without the shim.
func firstCommandWord(args []string) int {
	for i := 0; i < len(args); {
		tok := args[i]
		if len(tok) < 2 || tok[0] != '-' {
			return i
		}
		name := tok
		for len(name) > 0 && name[0] == '-' {
			name = name[1:]
		}
		hasValue := false
		for j, c := range name {
			if c == '=' {
				hasValue = true
				name = name[:j]
				break
			}
		}
		switch {
		case name == "debug":
			i++
		case legacyValueFlags[name]:
			if hasValue {
				i++
			} else {
				i += 2
			}
		default:
			return -1
		}
	}
	return -1
}

// joinArgs renders argv for the deprecation warning.
func joinArgs(args []string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}
