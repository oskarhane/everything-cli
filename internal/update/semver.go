package update

import (
	"strconv"
	"strings"
)

// Version is a parsed semantic version.
type Version struct {
	Major, Minor, Patch int
	// Prerelease is the prerelease identifier without the leading dash,
	// empty for a release (e.g. "1.2.0" -> "", "1.2.0-rc.1" -> "rc.1").
	Prerelease string
}

// ParseVersion parses "vMAJOR.MINOR.PATCH[-prerelease]". Unparseable input
// returns ok=false.
func ParseVersion(s string) (Version, bool) {
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return Version{}, false
	}
	// Build metadata is ignored for ordering purposes.
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var pre string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
		if !validPrerelease(pre) {
			return Version{}, false
		}
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || p != strconv.Itoa(n) {
			return Version{}, false
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2], Prerelease: pre}, true
}

// Compare returns -1, 0, or 1 as a sorts before, equal to, or after b.
// Comparison is numeric (1.10.0 > 1.2.0); a prerelease sorts before its
// release, and prerelease identifiers follow semver precedence (numeric
// identifiers compare numerically, fewer identifiers sort lower).
func Compare(a, b Version) int {
	for _, pair := range [][2]int{
		{a.Major, b.Major},
		{a.Minor, b.Minor},
		{a.Patch, b.Patch},
	} {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}
			return 1
		}
	}
	return comparePrerelease(a.Prerelease, b.Prerelease)
}

// validPrerelease checks a prerelease identifier string (without the leading
// dash): dot-separated identifiers, each non-empty and made of [0-9A-Za-z-].
// An identifier that starts with a digit must be all digits, which rejects
// git-describe-style suffixes like "5-gabc".
func validPrerelease(s string) bool {
	if s == "" {
		return false
	}
	for _, ident := range strings.Split(s, ".") {
		if ident == "" {
			return false
		}
		allDigits := true
		for _, r := range ident {
			if !strings.ContainsRune("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-", r) {
				return false
			}
			if r < '0' || r > '9' {
				allDigits = false
			}
		}
		if ident[0] >= '0' && ident[0] <= '9' && !allDigits {
			return false
		}
	}
	return true
}

func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1 // release sorts after prerelease
	}
	if b == "" {
		return -1
	}
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := comparePrereleaseIdent(as[i], bs[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	default:
		return 0
	}
}

func comparePrereleaseIdent(a, b string) int {
	an, aerr := strconv.Atoi(a)
	bn, berr := strconv.Atoi(b)
	switch {
	case aerr == nil && berr == nil:
		switch {
		case an < bn:
			return -1
		case an > bn:
			return 1
		}
		return 0
	case aerr == nil:
		return -1 // numeric identifiers sort before alphanumeric
	case berr == nil:
		return 1
	default:
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		}
		return 0
	}
}
