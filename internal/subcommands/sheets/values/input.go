package values

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
	"github.com/spf13/pflag"

	"github.com/oskarhane/google-cli/internal/app"
)

// valuesInputFlags holds the input flags shared by the value-writing leaves
// (`sheets values append` and `sheets values update`).
type valuesInputFlags struct {
	values      string
	valuesFile  string
	inputOption string
}

// registerValuesFlags binds the shared input flags on f and returns the
// holder the leaf's RunE reads. The exactly-one-source rule is not checked
// here: ParseValues owns it.
func registerValuesFlags(f *pflag.FlagSet) *valuesInputFlags {
	in := &valuesInputFlags{inputOption: "USER_ENTERED"}
	f.StringVar(&in.values, "values", "", "Inline JSON array of arrays of cell values, e.g. [[1,\"a\"],[2,\"b\"]]")
	f.StringVar(&in.valuesFile, "values-file", "", "Path to a .json/.csv/.tsv values file (read via the config FS)")
	f.StringVar(&in.inputOption, "input-option", "USER_ENTERED", "How values are interpreted: RAW or USER_ENTERED")
	return in
}

// validateInputOption rejects anything but the two ValueInputOption values
// the Sheets API accepts; the default is USER_ENTERED.
func (in *valuesInputFlags) validateInputOption() error {
	switch in.inputOption {
	case "RAW", "USER_ENTERED":
		return nil
	}
	return fmt.Errorf("invalid --input-option %q: expected RAW or USER_ENTERED", in.inputOption)
}

// resolve reads and parses cell values from exactly one source: the inline
// --values JSON flag or the --values-file path read through the config FS.
// The file's extension selects json/csv/tsv parsing; ParseValues enforces
// the exactly-one-source rule and the rectangular 2D shape.
func (in *valuesInputFlags) resolve(cfg *app.Config) ([][]any, error) {
	if in.valuesFile != "" {
		data, err := afero.ReadFile(cfg.Fs, in.valuesFile)
		if err != nil {
			return nil, fmt.Errorf("reading values file %s: %w", in.valuesFile, err)
		}
		return ParseValues(in.values, data, filepath.Ext(in.valuesFile))
	}
	return ParseValues(in.values, nil, "")
}
