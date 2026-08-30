package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripControl(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"empty string unchanged", "", ""},
		{"plain ascii unchanged", "Hello, world", "Hello, world"},
		{"unicode unchanged", "café — ok", "café — ok"},
		{"tab preserved", "col1\tcol2", "col1\tcol2"},
		{"newline preserved", "line1\nline2", "line1\nline2"},
		{"carriage return preserved", "line1\rline2", "line1\rline2"},
		{"SOH 0x01 replaced", "a\x01b", "a?b"},
		{"ESC from ANSI escape replaced", "foo\x1b[31mbar", "foo?[31mbar"},
		{"BEL replaced", "ring\x07bell", "ring?bell"},
		{"DEL 0x7F replaced", "del\x7fbyte", "del?byte"},
		{"mixed controls and whitespace", "a\x01\tb\x02\nc\x7fd", "a?\tb?\nc?d"},
		{
			name: "every other C0 byte replaced",
			in:   "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f",
			want: "?????????\t\n??\r??????????????????",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, StripControl(tc.in))
		})
	}
}
