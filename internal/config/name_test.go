package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidAccountName: S12 — the validator must reject path escapes, ':',
// NUL, C0 control bytes and DEL, while keeping ordinary account names.
func TestValidAccountName(t *testing.T) {
	for _, tt := range []struct {
		name string
		want bool
	}{
		// Ordinary names still pass.
		{"work", true},
		{"work-2", true},
		{"oskar.hane", true},
		{"a_b", true},
		{"UPPER", true},
		{"work.backup", true},
		// Path escapes and empties.
		{"", false},
		{".", false},
		{"..", false},
		{"a/b", false},
		{"a\\b", false},
		// S12 additions: colon, NUL, control chars, DEL.
		{"a:b", false},
		{":", false},
		{"a\x00b", false},
		{"a\x01b", false},
		{"a\x1fb", false},
		{"\x1f", false},
		{"a\x7f", false},
		{"\n", false},
		{"\t", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, validAccountName(tt.name))
		})
	}
}
