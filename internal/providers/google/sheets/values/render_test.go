package values

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJoinCells(t *testing.T) {
	tests := []struct {
		name string
		in   []any
		want string
	}{
		{"empty", []any{}, ""},
		{"scalars", []any{"a", int64(1), nil, true}, "a\t1\t<nil>\ttrue"},
		{"single", []any{"only"}, "only"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, JoinCells(tt.in))
		})
	}
}
