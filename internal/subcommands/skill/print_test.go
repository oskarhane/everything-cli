package skill

import (
	"bytes"
	"io/fs"
	"slices"
	"strings"
	"testing"

	skillapi "github.com/oskarhane/google-cli/internal/skill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedPrintBundle builds the byte-exact stdout contract of skill print:
// SKILL.md first, then every references/*.md in sorted order, each preceded
// by a `===== references/<name>.md =====` separator line.
func expectedPrintBundle(t *testing.T) string {
	t.Helper()
	skillMD, err := fs.ReadFile(skillapi.Bundle, "SKILL.md")
	require.NoError(t, err)

	refs, err := fs.Glob(skillapi.Bundle, "references/*.md")
	require.NoError(t, err)
	slices.Sort(refs)

	var want bytes.Buffer
	want.Write(skillMD)
	for _, name := range refs {
		data, rerr := fs.ReadFile(skillapi.Bundle, name)
		require.NoError(t, rerr)
		want.WriteString("===== " + name + " =====\n")
		want.Write(data)
	}
	return want.String()
}

// TestPrintEmitsExactBundleBytes: stdout equals SKILL.md followed by every
// reference in sorted order, each behind its separator line — byte for byte.
func TestPrintEmitsExactBundleBytes(t *testing.T) {
	_, root, out, _ := newSkillEnv(t)

	stdout, stderr, err := execute(t, root, out, nil, "skill", "print")
	require.NoError(t, err)

	assert.Equal(t, expectedPrintBundle(t), stdout)
	assert.Empty(t, stderr)
}

// TestPrintSeparatorOrder: the separators appear once each, in sorted
// reference order, after the SKILL.md content.
func TestPrintSeparatorOrder(t *testing.T) {
	_, root, out, _ := newSkillEnv(t)

	stdout, _, err := execute(t, root, out, nil, "skill", "print")
	require.NoError(t, err)

	skillMD, err := fs.ReadFile(skillapi.Bundle, "SKILL.md")
	require.NoError(t, err)
	skillEnd := strings.Index(stdout, string(skillMD))
	require.Equal(t, 0, skillEnd, "SKILL.md bytes must come first")

	var sepIdx []int
	for _, name := range []string{
		"references/account.md", "references/calendar.md", "references/gmail.md",
	} {
		sep := "===== " + name + " =====\n"
		idx := strings.Index(stdout, sep)
		require.NotEqual(t, -1, idx, "missing separator for %s", name)
		sepIdx = append(sepIdx, idx)
	}
	assert.True(t, slices.IsSorted(sepIdx), "separators must be in sorted order")
}

// TestPrintIgnoresFormatFlag: print bypasses output formatting — markdown
// bytes are never marshalled through toon/table/json.
func TestPrintIgnoresFormatFlag(t *testing.T) {
	_, root, out, _ := newSkillEnv(t)

	stdout, _, err := execute(t, root, out, nil, "skill", "print", "--format", "table")
	require.NoError(t, err)
	assert.Equal(t, expectedPrintBundle(t), stdout)
}

// TestPrintRejectsPositionalArgs: print takes no positionals.
func TestPrintRejectsPositionalArgs(t *testing.T) {
	_, root, out, _ := newSkillEnv(t)

	_, _, err := execute(t, root, out, nil, "skill", "print", "extra")
	require.Error(t, err)
}
