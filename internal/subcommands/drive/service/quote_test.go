package service

import "testing"

func TestDoubleSingleQuotes(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"plain", "plain"},
		{"O'Brien report", "O''Brien report"},
		{"it's Bob's", "it''s Bob''s"},
	}
	for _, tt := range tests {
		if got := DoubleSingleQuotes(tt.in); got != tt.want {
			t.Errorf("DoubleSingleQuotes(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
