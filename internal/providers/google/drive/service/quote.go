package service

import "strings"

// DoubleSingleQuotes doubles every embedded single quote in s. A1-quoted
// sheet titles escape a literal quote by writing it twice in a row, so the
// sheets and docs trees share this one escaping helper here instead of
// growing a copy per resource package. Drive q does NOT use this grammar —
// it escapes with backslashes ('O\'Brien', '\\') — so drive q terms must
// not route through this helper.
func DoubleSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
