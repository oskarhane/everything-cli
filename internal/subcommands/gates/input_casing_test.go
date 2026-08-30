package gates

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// kebabRe is the required shape for input identifiers: command names
// (Use first word), aliases, and flag long names. Single-character flag
// shorthands are exempt.
var kebabRe = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// inputViolations returns the casing violations of one command's Use word,
// aliases, and flags. Each message names the offending item.
func inputViolations(cmd *cobra.Command) []string {
	path := cmd.CommandPath()
	var violations []string

	if use := useFirstWord(cmd.Use); use != "" && !kebabRe.MatchString(use) {
		violations = append(violations, fmt.Sprintf("%s: Use %q is not kebab-case", path, use))
	}
	for _, alias := range cmd.Aliases {
		if !kebabRe.MatchString(alias) {
			violations = append(violations,
				fmt.Sprintf("%s: alias %q is not kebab-case", path, alias))
		}
	}

	for _, fs := range []*pflag.FlagSet{cmd.Flags(), cmd.PersistentFlags()} {
		seen := map[string]bool{}
		fs.VisitAll(func(f *pflag.Flag) {
			if seen[f.Name] {
				return
			}
			seen[f.Name] = true
			if !kebabRe.MatchString(f.Name) {
				violations = append(violations,
					fmt.Sprintf("%s: flag --%s is not kebab-case", path, f.Name))
			}
		})
	}
	return violations
}

// TestInputIdentifiers_AreKebabCase walks the mounted tree and asserts every
// command's Use first word, aliases, and flag long names are kebab-case.
// Root persistent flags are covered because the root itself is in the walk;
// inherited flags are exempt from the per-command set but are checked at
// their definition site.
func TestInputIdentifiers_AreKebabCase(t *testing.T) {
	_, commands, _ := mountAndCheck(t)
	var violations []string
	walkTree(newWholeTree(), func(cmd *cobra.Command) {
		violations = append(violations, inputViolations(cmd)...)
	})
	if commands < minTreeCommands {
		t.Fatalf("walked only %d commands, want >= %d — tree mount is broken", commands, minTreeCommands)
	}
	for _, v := range violations {
		t.Error(v)
	}
}

// minTreeCommands is the known minimum command count of the mounted tree;
// fewer means the mount is broken or the tree silently atrophied and the
// gate must not pass vacuously.
const minTreeCommands = 50
