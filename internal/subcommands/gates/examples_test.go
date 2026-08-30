package gates

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// exampleViolations returns the documentation violations of one runnable
// leaf. Each message names the offending command path.
func exampleViolations(cmd *cobra.Command) []string {
	name := cmd.CommandPath()
	if cmd.Example == "" {
		return []string{fmt.Sprintf("%s: runnable leaf has no Example", name)}
	}
	var violations []string
	if !firstLineFlushLeft(cmd.Example) {
		violations = append(violations,
			fmt.Sprintf("%s: Example must start flush-left with \"#\" or \"google-cli\"", name))
	}
	if n := countInvocations(cmd.Example); n < 2 {
		violations = append(violations,
			fmt.Sprintf("%s: Example has %d google-cli invocation(s), want >= 2", name, n))
	}
	return violations
}

// firstLineFlushLeft reports whether the example's first line starts at
// column zero with a comment or an invocation (no leading whitespace).
func firstLineFlushLeft(example string) bool {
	first, _, _ := strings.Cut(example, "\n")
	return strings.HasPrefix(first, "#") || strings.HasPrefix(example, "google-cli")
}

// TestAllLeafCommands_HaveExamples walks every command under the mounted
// root (account + gmail + calendar trees) and requires every runnable leaf
// to carry a flush-left Example with at least two google-cli invocations.
// Per-package gates cover their own leaves; this whole-tree gate also
// catches leaves a package test forgot and future subtrees automatically.
func TestAllLeafCommands_HaveExamples(t *testing.T) {
	_, _, leaves := mountAndCheck(t)
	var violations []string
	walkTree(newWholeTree(), func(cmd *cobra.Command) {
		if isRunnableLeaf(cmd) {
			violations = append(violations, exampleViolations(cmd)...)
		}
	})
	if leaves < minLeaves {
		t.Fatalf("only %d runnable leaves in mounted tree, want >= %d — tree mount is broken", leaves, minLeaves)
	}
	for _, v := range violations {
		t.Error(v)
	}
}

// minLeaves is the known minimum of runnable leaves; a whole-tree walk that
// finds fewer means the tree failed to mount or silently atrophied and the
// gate must not pass vacuously.
const minLeaves = 40
