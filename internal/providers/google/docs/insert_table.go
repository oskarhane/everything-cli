package docs

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newInsertTableCmd returns `docs insert-table`: insert a table before the
// given Docs-API content index (--index, default 0 = the end of the tab body,
// which the service computes) in a tab (--tab, by tab ID or exact title;
// default the first tab). The table is either empty — sized by --rows and
// --columns, both required and >= 1 — or filled from CSV cell data via --csv
// or --csv-file (mutually exclusive, mirroring --text/--text-file): then the
// dimensions derive from the data (rows = record count, columns = the widest
// record) and --rows/--columns must not be set. The tab key is forwarded to
// the service, which resolves it, exactly like append.
func newInsertTableCmd(cfg *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	var (
		rows    int64
		columns int64
		csvData string
		csvFile string
		index   int64
		tab     string
	)
	cmd := &cobra.Command{
		Use:   "insert-table <doc-id>",
		Short: "Insert a table into a Google Doc",
		Example: `# Insert an empty 3x4 table at the end of the document's first tab
everything-cli google docs insert-table 1AbCdEfGh --rows 3 --columns 4

# Insert a filled table from inline CSV (a real CSV parse, so quoted commas work)
everything-cli google docs insert-table 1AbCdEfGh --csv "name,note
Oskar,\"likes, commas\""

# Insert a table from a CSV file before index 120 in a named tab
everything-cli google docs insert-table 1AbCdEfGh --csv-file cells.csv --index 120 --tab Changelog`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if csvData != "" && csvFile != "" {
				return fmt.Errorf("--csv and --csv-file are mutually exclusive")
			}
			if index < 0 {
				return fmt.Errorf("--index must be >= 0: 0 (the default) inserts at the end of the tab body")
			}
			spec := service.DocTableSpec{Index: index, TabKey: tab}
			if csvData != "" || csvFile != "" {
				if cmd.Flags().Changed("rows") || cmd.Flags().Changed("columns") {
					return fmt.Errorf("--rows and --columns must not be set with --csv or --csv-file: the dimensions derive from the cell data")
				}
				data := csvData
				if csvFile != "" {
					b, err := afero.ReadFile(cfg.Fs, csvFile)
					if err != nil {
						return fmt.Errorf("reading --csv-file %s: %w", csvFile, err)
					}
					data = string(b)
				}
				cells, err := parseTableCSV(data)
				if err != nil {
					return err
				}
				spec.Cells = cells
				spec.Rows = int64(len(cells))
				for _, row := range cells {
					if int64(len(row)) > spec.Columns {
						spec.Columns = int64(len(row))
					}
				}
			} else {
				if rows < 1 || columns < 1 {
					return fmt.Errorf("--rows and --columns are required and must be >= 1: give the table dimensions, or cell data via --csv or --csv-file")
				}
				spec.Rows, spec.Columns = rows, columns
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			// InsertDocTable resolves the tab key itself (exact tab ID,
			// then exact title); an empty tab targets the first tab.
			start, err := svc.InsertDocTable(cmd.Context(), args[0], spec)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Inserted %dx%d table into document %s at index %d\n",
				spec.Rows, spec.Columns, args[0], start); err != nil {
				return err
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.Int64Var(&rows, "rows", 0, "Table row count (required without --csv/--csv-file, >= 1)")
	f.Int64Var(&columns, "columns", 0, "Table column count (required without --csv/--csv-file, >= 1)")
	f.StringVar(&csvData, "csv", "", "Inline CSV cell data: dimensions derive from it (rows = records, columns = widest record)")
	f.StringVar(&csvFile, "csv-file", "", "Read CSV cell data from this file instead of --csv")
	f.Int64Var(&index, "index", 0, "Docs-API content index to insert before (default 0: the end of the tab body)")
	f.StringVar(&tab, "tab", "", "Insert into this tab, by tab ID or exact title (default: the first tab)")
	return cmd
}

// parseTableCSV parses CSV cell data with a real CSV parse (encoding/csv, so
// quoted commas work). Ragged rows are allowed — the column count is the
// widest record, and shorter rows leave their missing cells empty.
func parseTableCSV(data string) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(data))
	r.FieldsPerRecord = -1 // ragged rows allowed: columns derive from the widest record
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV cell data: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("the CSV cell data carried zero records: give at least one row")
	}
	return records, nil
}
