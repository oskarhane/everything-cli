// Package update implements the google-cli update command: check GitHub
// releases for a newer version, replace the running binary, and (user
// permitting) reinstall the refreshed agent-skill bundle.
package update

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	updateapi "github.com/oskarhane/google-cli/internal/update"
)

// newClient is the seam for the GitHub releases client, so command tests can
// inject a fake without network access.
var newClient = func() updateapi.Client { return updateapi.NewClient("", "") }

// runUpdate is the seam over the update.Run orchestration, so command tests
// can observe the computed Options (notably SkipSkillInstall and
// AgentFilter) without executing the real download/replace pipeline.
var runUpdate = func(ctx context.Context, client updateapi.Client, current string, opts updateapi.Options) (updateapi.Result, error) {
	return updateapi.Run(ctx, client, current, opts)
}

// readYesNo reads a yes/no answer from os.Stdin: y/yes (case-insensitive) is
// true; EOF, a read error, or anything else is false. Seam for tests.
var readYesNo = func() (bool, error) {
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// checkFields is the output field order for --check output.
var checkFields = []string{"current_version", "latest_version", "update_available"}

// NewCmd builds the top-level update command: it is its own leaf.
func NewCmd(cfg *app.Config) *cobra.Command {
	var (
		yes       bool
		checkOnly bool
		agent     string
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install the latest google-cli release",
		Long: "Check GitHub releases for a newer google-cli version and, if one " +
			"exists, download it, verify it against the release checksum manifest, " +
			"and replace the running binary atomically. After a successful " +
			"replacement the refreshed skill bundle is installed: automatically " +
			"with --yes, on confirmation at an interactive terminal, or not at all " +
			"(a hint is printed instead). --check reports versions only.",
		Example: `# Report the current and latest versions without changing anything
google-cli update --check

# Update and auto-install the refreshed skill bundle without prompting
google-cli update --yes

# Update and install the refreshed skill bundle only into Claude Code
google-cli update --yes --agent claude-code`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newClient()
			format := output.ResolveOutput(cfg.Format)
			ctx := cmd.Context()
			if checkOnly {
				rel, available, err := updateapi.Check(ctx, client, app.Version)
				if err != nil {
					return wrapUpdateError(err)
				}
				printCheck(cmd.OutOrStdout(), format, rel, available)
				return nil
			}
			res, err := runUpdate(ctx, client, app.Version, updateapi.Options{
				AgentFilter:      agent,
				Yes:              yes,
				SkipSkillInstall: skipSkillInstall(yes, cmd),
				FS:               cfg.Fs,
			})
			if err != nil {
				if errors.Is(err, updateapi.ErrUpToDate) {
					// Already current: exit 0, still render the comparison.
					printResult(cmd.OutOrStdout(), format, res)
					return nil
				}
				return wrapAgentFilterError(wrapUpdateError(err))
			}
			printResult(cmd.OutOrStdout(), format, res)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation and auto-install the updated skill bundle")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "only report current and latest versions; change nothing")
	addAgentFlag(cmd, &agent)
	return cmd
}

// skipSkillInstall pre-decides the post-update skill reinstall for Run
// (Run never prompts: it may run non-interactively). --yes auto-installs;
// an interactive non-agent terminal is asked; anything else — non-TTY stdin
// or agent harness — gets no prompt and Run fills skill_hint instead.
func skipSkillInstall(yes bool, cmd *cobra.Command) bool {
	if yes {
		return false
	}
	if output.StdinIsTerminal() && !output.IsAgent() {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), "Install the refreshed skill bundle? [y/N] ")
		ok, err := readYesNo()
		return err != nil || !ok
	}
	return true
}

// printCheck renders the --check result: versions and availability only.
func printCheck(w io.Writer, format output.Format, rel *updateapi.Release, available bool) {
	row := map[string]any{
		"current_version":  app.Version,
		"latest_version":   rel.Tag,
		"update_available": available,
	}
	output.Print(w, format, checkFields, row, []map[string]any{row})
}

// printResult renders a Run Result (success or already-up-to-date).
func printResult(w io.Writer, format output.Format, res updateapi.Result) {
	output.Print(w, format, res.Fields(), res.Row(), []map[string]any{res.Row()})
}

// wrapUpdateError maps internal/update sentinel errors to user-facing
// messages. Rate limiting gets a token hint; the rest already read well.
func wrapUpdateError(err error) error {
	if errors.Is(err, updateapi.ErrRateLimited) {
		return fmt.Errorf(
			"%w — export GITHUB_TOKEN (or GH_TOKEN) to raise the limit", err)
	}
	return err
}
