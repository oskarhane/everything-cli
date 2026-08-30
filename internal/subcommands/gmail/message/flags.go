package message

import (
	"strings"
)

// SplitCSV splits a comma-separated flag value into trimmed, non-empty items,
// the CSV convention for --to, --cc, --bcc, and the label-id flags. Every
// gmail leaf that takes a comma-separated flag goes through this, so the
// trimming and empty-item semantics are identical across subtrees.
func SplitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var items []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			items = append(items, p)
		}
	}
	return items
}
