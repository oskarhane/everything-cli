package values

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// newAppendCmd returns `sheets values append`: add rows after the last row of
// the table containing the A1 range. Values come from --values (inline JSON)
// or --values-file (json/csv/tsv), exactly one.
func newAppendCmd(cfg *app.Config, newSvc service.Dialer[service.SheetValuesService]) *cobra.Command {
	var valuesFlag, valuesFile, inputOption string
	cmd := &cobra.Command{
		Use:   "append <spreadsheet-id>",
		Short: "Append rows after the last row of a range's table",
		Example: `# Append rows from an inline JSON array of arrays
google-cli sheets values append 1AbCdEfGh --range "Sheet1!A1:D" --values '[[1,"a",true],[2,"b",false]]' --format json

# Append rows from a CSV file
google-cli sheets values append 1AbCdEfGh --range "Sheet1!A1:D" --values-file ./rows.csv

# Append values as raw strings (no formula parsing)
google-cli sheets values append 1AbCdEfGh --range "Sheet1!A1:B" --values '[[=SUM(A1:A2)]]' --input-option RAW`,
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
			updatedRange, updatedRows, updatedCols, err := svc.AppendValues(cmd.Context(), args[0], a1Range, vals, inputOption)
			if err != nil {
				return err
			}
			printAppend(cmd, cfg, updatedRange, updatedRows, updatedCols)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&valuesFlag, "values", "", "Inline JSON array of arrays of cell values, e.g. [[1,\"a\"],[2,\"b\"]]")
	f.StringVar(&valuesFile, "values-file", "", "Path to a .json/.csv/.tsv values file (read via the config FS)")
	f.StringVar(&inputOption, "input-option", "USER_ENTERED", "How values are interpreted: RAW or USER_ENTERED")
	f.String("range", "", "A1 range locating the table to append to, e.g. Sheet1!A1:D")
	_ = cmd.MarkFlagRequired("range")
	return cmd
}
