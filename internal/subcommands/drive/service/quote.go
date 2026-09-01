package service

import "strings"

// DoubleSingleQuotes doubles every embedded single quote in s. Both the
// Drive q syntax and A1-quoted sheet titles escape a literal quote by
// doubling it, so the drive, sheets, and docs trees share this one escaping
// helper here instead of growing a copy per resource package.
func DoubleSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
