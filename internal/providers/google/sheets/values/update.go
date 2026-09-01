package values

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/service"
)

// newUpdateCmd returns `sheets values update`: write rows starting at the
// top-left of the A1 range, overwriting what is there. Values come from
// --values (inline JSON) or --values-file (json/csv/tsv), exactly one.
func newUpdateCmd(cfg *app.Config, newSvc service.Dialer[service.SheetValuesService]) *cobra.Command {
	var in *valuesInputFlags
	cmd := &cobra.Command{
		Use:   "update <spreadsheet-id>",
		Short: "Write cell values starting at the top-left of a range",
		Example: `# Overwrite a range from an inline JSON array of arrays
google-cli sheets values update 1AbCdEfGh --range "Sheet1!A1:B2" --values '[[1,"a"],[2,"b"]]' --format json

# Write cells from a TSV file
google-cli sheets values update 1AbCdEfGh --range "Sheet1!A1:B" --values-file ./cells.tsv

# Write literal text where the value looks like a formula
google-cli sheets values update 1AbCdEfGh --range "Sheet1!C1" --values '[[=SUM(A1:A2)]]' --input-option RAW`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := in.validateInputOption(); err != nil {
				return err
			}
			vals, err := in.resolve(cfg)
			if err != nil {
				return err
			}
			a1Range, _ := cmd.Flags().GetString("range")
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			updatedRange, updatedCells, err := svc.UpdateValues(cmd.Context(), args[0], a1Range, vals, in.inputOption)
			if err != nil {
				return err
			}
			printUpdate(cmd, cfg, updatedRange, updatedCells)
			return nil
		},
	}
	in = registerValuesFlags(cmd.Flags())
	f := cmd.Flags()
	f.String("range", "", "A1 range to write into, e.g. Sheet1!A1:D10")
	_ = cmd.MarkFlagRequired("range")
	return cmd
}
