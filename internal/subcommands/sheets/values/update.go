package values

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// newUpdateCmd returns `sheets values update`: write rows starting at the
// top-left of the A1 range, overwriting what is there. Values come from
// --values (inline JSON) or --values-file (json/csv/tsv), exactly one.
func newUpdateCmd(cfg *app.Config, newSvc service.Dialer[service.SheetValuesService]) *cobra.Command {
	var valuesFlag, valuesFile, inputOption string
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
			if err := validateInputOption(inputOption); err != nil {
				return err
			}
			vals, err := resolveValuesInput(cfg, valuesFlag, valuesFile)
			if err != nil {
				return err
			}
			a1Range, _ := cmd.Flags().GetString("range")
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			updatedRange, updatedCells, err := svc.UpdateValues(cmd.Context(), args[0], a1Range, vals, inputOption)
			if err != nil {
				return err
			}
			printUpdate(cmd, cfg, updatedRange, updatedCells)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&valuesFlag, "values", "", "Inline JSON array of arrays of cell values, e.g. [[1,\"a\"],[2,\"b\"]]")
	f.StringVar(&valuesFile, "values-file", "", "Path to a .json/.csv/.tsv values file (read via the config FS)")
	f.StringVar(&inputOption, "input-option", "USER_ENTERED", "How values are interpreted: RAW or USER_ENTERED")
	f.String("range", "", "A1 range to write into, e.g. Sheet1!A1:D10")
	_ = cmd.MarkFlagRequired("range")
	return cmd
}
