package message

import (
	"strings"
)

// splitCSV splits a comma-separated flag value into trimmed, non-empty items,
// the CSV convention for --to, --cc, --bcc, and the label-id flags.
func splitCSV(s string) []string {
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
