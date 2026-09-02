// Package cmdtree is the single tree-assembly seam for everything-cli:
// New builds the full command tree from the provider registry plus the
// CLI-own commands, and WalkTree is the one depth-first walk. main.go,
// the whole-tree gates, and the skill drift test all consume this package,
// so the shipped tree and the tested trees cannot drift apart.
//
// Provider registration stays explicit at each consumption site: main.go
// and the test files side-effect-import the provider packages themselves,
// and New reads only provider.List(). This package deliberately imports no
// provider — importing it never changes the registry.
package cmdtree

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/provider"
	"github.com/oskarhane/everything-cli/internal/subcommands/account"
	"github.com/oskarhane/everything-cli/internal/subcommands/skill"
	"github.com/oskarhane/everything-cli/internal/subcommands/update"
)

// New builds the complete command tree: the root command with persistent
// flags bound to cfg, every registered provider's tree under its provider
// command, and the CLI-own commands top-level (the read-only
// cross-provider account aggregate, the embedded skill, and self-update).
func New(cfg *app.Config) *cobra.Command {
	root := app.NewRootCommand(cfg)
	for _, p := range provider.List() {
		root.AddCommand(p.NewCmd(cfg))
	}
	root.AddCommand(
		account.NewCmd(cfg),
		skill.NewCmd(cfg),
		update.NewCmd(cfg),
	)
	return root
}

// WalkTree visits cmd and every descendant, depth-first. It is the single
// tree walk shared by every whole-tree consumer.
func WalkTree(cmd *cobra.Command, visit func(*cobra.Command)) {
	walkTree(cmd, visit)
}

// walkTree is the WalkTree implementation, kept unexported so callers go
// through the one exported entry point.
func walkTree(cmd *cobra.Command, visit func(*cobra.Command)) {
	visit(cmd)
	for _, sub := range cmd.Commands() {
		walkTree(sub, visit)
	}
}
