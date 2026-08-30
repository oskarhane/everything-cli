package skill

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBundleContents: the embedded Bundle is re-rooted at SKILL.md plus the
// three reference files — the flat layout the installer walks.
func TestBundleContents(t *testing.T) {
	want := map[string]bool{
		"SKILL.md":               false,
		"references/account.md":  false,
		"references/gmail.md":    false,
		"references/calendar.md": false,
	}

	err := fs.WalkDir(Bundle, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			if seen, ok := want[p]; ok {
				require.False(t, seen, "%s must appear exactly once", p)
				want[p] = true
			}
		}
		return nil
	})
	require.NoError(t, err)

	for p, seen := range want {
		assert.True(t, seen, "bundle must contain %s", p)
	}
}

// TestBundleFrontmatter: SKILL.md identifies the skill as google-cli and
// carries a version line for the installer to rewrite.
func TestBundleFrontmatter(t *testing.T) {
	data, err := fs.ReadFile(Bundle, "SKILL.md")
	require.NoError(t, err)

	body := string(data)
	assert.Equal(t, "---", firstLine(body))
	assert.Contains(t, body, "name: google-cli")

	// The bundle ships `version: dev`; Install rewrites the line.
	assert.Regexp(t, `(?m)^version: dev[ \t]*$`, body)
}

// firstLine returns the first line of s.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimRight(s[:i], "\r")
	}
	return s
}
