package values

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newClearCmd returns `sheets values clear`: empty every cell in the A1
// range (formatting is kept). Clearing is bounded to the given range and
// recoverable by rewriting values, so unlike delete there is no --force.
func newClearCmd(cfg *app.Config, newSvc service.Dialer[service.SheetValuesService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear <spreadsheet-id>",
		Short: "Clear the values in an A1 range",
		Example: `# Clear a bounded range
everything-cli google sheets values clear 1AbCdEfGh --range "Sheet1!A2:D10"

# Clear everything below the header in one column
everything-cli google sheets values clear 1AbCdEfGh --range "Sheet1!B2:B" --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a1Range, _ := cmd.Flags().GetString("range")
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			clearedRange, err := svc.ClearValues(cmd.Context(), args[0], a1Range)
			if err != nil {
				return err
			}
			printClear(cmd, cfg, clearedRange)
			return nil
		},
	}
	f := cmd.Flags()
	f.String("range", "", "A1 range to clear, e.g. Sheet1!A1:D10")
	_ = cmd.MarkFlagRequired("range")
	return cmd
}
