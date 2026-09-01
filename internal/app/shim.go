package app

import (
	"fmt"
	"io"
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

// RewriteLegacyArgs maps pre-provider invocations onto the provider-first
// tree: a bare Google resource word (`gmail list`) or a moved account verb
// (`account add work`) is rewritten to `google <args...>`, with a
// deprecation warning on errw. Anything else passes through unchanged —
// including `account list` (a real top-level command) and flag-first
// invocations, which the shim deliberately does not chase.
func RewriteLegacyArgs(binary string, args []string, errw io.Writer) []string {
	if len(args) == 0 {
		return args
	}
	rewrite := legacyGoogleResources[args[0]] ||
		(args[0] == "account" && len(args) > 1 && legacyAccountVerbs[args[1]])
	if !rewrite {
		return args
	}
	_, _ = fmt.Fprintf(errw,
		"warning: `%s %s` is deprecated; use `%s google %s` "+
			"(the back-compat shim will be removed in a future release)\n",
		binary, joinArgs(args), binary, joinArgs(args))
	out := make([]string, 0, len(args)+1)
	out = append(out, "google")
	return append(out, args...)
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
