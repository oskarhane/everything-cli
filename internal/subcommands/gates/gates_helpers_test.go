package gates

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// These tests pin the failure-message format of each gate's helper so the
// offending command/flag/field is always named in gate output. They use a
// synthetic tree — never the real one — to prove the gates catch violations.

// TestInputViolations_NamesOffender: a command with a bad Use word, a bad
// alias, and a bad flag name produces one message per offense, each naming
// the command and the offending item.
func TestInputViolations_NamesOffender(t *testing.T) {
	cmd := &cobra.Command{Use: "get_by_id <id>", Aliases: []string{"BadAlias"}}
	cmd.Flags().String("someFlag", "", "")
	vs := inputViolations(cmd)

	assert.Len(t, vs, 3)
	joined := strings.Join(vs, "\n")
	assert.Contains(t, joined, `Use "get_by_id" is not kebab`)
	assert.Contains(t, joined, `alias "BadAlias" is not kebab`)
	assert.Contains(t, joined, `flag --someFlag is not kebab`)
	assert.Contains(t, joined, cmd.CommandPath())
}

// TestInputViolations_AllowOk: kebab names, single-char shorthand flags, and
// empty Use produce no violations.
func TestInputViolations_AllowOk(t *testing.T) {
	cmd := &cobra.Command{Use: "get <id>"}
	cmd.Flags().StringP("query", "q", "", "")
	assert.Empty(t, inputViolations(cmd))
}

// TestExampleViolations_NamesOffender: a leaf with a one-invocation example
// and an indented example is reported with its command path.
func TestExampleViolations_NamesOffender(t *testing.T) {
	bad := &cobra.Command{Use: "get", RunE: func(*cobra.Command, []string) error { return nil }}
	bad.Example = "  google-cli demo get\n  google-cli demo get --format json"
	assert.Contains(t, exampleViolations(bad)[0], "get: Example must start flush-left")

	single := &cobra.Command{Use: "get", RunE: func(*cobra.Command, []string) error { return nil }}
	single.Example = "# one\n" + "google-cli app get"
	vs := exampleViolations(single)
	assert.Len(t, vs, 1)
	assert.Contains(t, vs[0], "1 google-cli invocation(s), want >= 2")

	empty := &cobra.Command{Use: "get", RunE: func(*cobra.Command, []string) error { return nil }}
	assert.Contains(t, exampleViolations(empty)[0], "no Example")
}

// TestFieldViolations_NamesOffender: a non-snake field is reported with its
// context string.
func TestFieldViolations_NamesOffender(t *testing.T) {
	vs := fieldViolations("render.go:10 PrintTable", []string{"messages_count", "camelCase"})
	assert.Len(t, vs, 1)
	assert.Contains(t, vs[0], "camelCase")
	assert.Contains(t, vs[0], "not snake_case")

	assert.Empty(t, fieldViolations("ctx", []string{"messages_count", "id"}))
}
