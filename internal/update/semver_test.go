package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Version
		ok   bool
	}{
		{name: "plain release", in: "1.2.3", want: Version{Major: 1, Minor: 2, Patch: 3}, ok: true},
		{name: "v prefix", in: "v1.2.3", want: Version{Major: 1, Minor: 2, Patch: 3}, ok: true},
		{name: "prerelease", in: "v1.2.3-rc.1", want: Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1"}, ok: true},
		{name: "dev is not a release", in: "dev", ok: false},
		{name: "empty", in: "", ok: false},
		{name: "bare short sha", in: "abc1234", ok: false},
		{name: "git describe suffix", in: "v1.2.3-5-gabc", ok: false},
		{name: "too few fields", in: "1.2", ok: false},
		{name: "too many fields", in: "1.2.3.4", ok: false},
		{name: "non numeric", in: "1.2.x", ok: false},
		{name: "negative component", in: "1.-2.3", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseVersion(tt.in)
			assert.Equal(t, tt.ok, ok, "ParseVersion(%q) ok", tt.in)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{name: "equal", a: "1.2.3", b: "v1.2.3", want: 0},
		{name: "numeric not lexical", a: "1.10.0", b: "1.2.0", want: 1},
		{name: "numeric not lexical reversed", a: "1.2.0", b: "1.10.0", want: -1},
		{name: "major wins", a: "2.0.0", b: "1.99.99", want: 1},
		{name: "minor wins over patch", a: "1.3.0", b: "1.2.9", want: 1},
		{name: "patch", a: "1.2.4", b: "1.2.3", want: 1},
		{name: "prerelease before release", a: "1.2.3-rc.1", b: "1.2.3", want: -1},
		{name: "release after prerelease", a: "1.2.3", b: "1.2.3-rc.1", want: 1},
		{name: "prerelease numeric ids", a: "1.2.3-rc.9", b: "1.2.3-rc.10", want: -1},
		{name: "prerelease more ids lower", a: "1.2.3-alpha", b: "1.2.3-alpha.1", want: -1},
		{name: "prerelease equal", a: "1.2.3-rc.1", b: "1.2.3-rc.1", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			av, ok := ParseVersion(tt.a)
			assert.True(t, ok, "parse %q", tt.a)
			bv, ok := ParseVersion(tt.b)
			assert.True(t, ok, "parse %q", tt.b)
			assert.Equal(t, tt.want, Compare(av, bv))
		})
	}
}
