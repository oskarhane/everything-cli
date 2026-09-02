package values

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newGetCmd returns `sheets values get`: read an A1 range and render one row
// per spreadsheet row. --range is required because a bare spreadsheet id has
// no meaningful default range.
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.SheetValuesService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <spreadsheet-id>",
		Short: "Read cell values from an A1 range",
		Example: `# Read a range as JSON
everything-cli google sheets values get 1AbCdEfGh --range "Sheet1!A1:D10" --format json

# Show the same range as a table
everything-cli google sheets values get 1AbCdEfGh --range "Budget!A1:C20" --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			a1Range, _ := cmd.Flags().GetString("range")
			vals, err := svc.GetValues(cmd.Context(), args[0], a1Range)
			if err != nil {
				return err
			}
			printValuesGet(cmd, cfg, a1Range, vals)
			return nil
		},
	}
	cmd.Flags().String("range", "", "A1 range to read, e.g. Sheet1!A1:D10")
	_ = cmd.MarkFlagRequired("range")
	return cmd
}
