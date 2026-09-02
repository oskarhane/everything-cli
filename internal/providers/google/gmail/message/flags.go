package message

import (
	"fmt"
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

// SplitRecipients splits a recipient flag value (--to, --cc, --bcc) like
// SplitCSV and additionally rejects any item containing C0 control characters
// (CR, LF, NUL, ...): a crafted recipient could otherwise smuggle extra
// headers into the composed message. The error names the offending item so
// the caller sees exactly which flag value failed.
func SplitRecipients(s string) ([]string, error) {
	items := SplitCSV(s)
	for _, item := range items {
		if containsControl(item) {
			return nil, fmt.Errorf("recipient %q contains control characters", item)
		}
	}
	return items, nil
}
