package values

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

	"github.com/oskarhane/google-cli/internal/app"
)

// resolveValuesInput parses cell values from exactly one source: the inline
// --values JSON flag or the --values-file path read through the config FS.
// The file's extension selects json/csv/tsv parsing; ParseValues enforces
// the exactly-one-source rule and the rectangular 2D shape.
func resolveValuesInput(cfg *app.Config, valuesFlag, valuesFile string) ([][]any, error) {
	if strings.TrimSpace(valuesFlag) != "" && valuesFile != "" {
		return nil, fmt.Errorf("provide values from one source only: either --values (inline JSON array of arrays like %s) or --values-file, not both", shapeExample)
	}
	if valuesFile != "" {
		data, err := afero.ReadFile(cfg.Fs, valuesFile)
		if err != nil {
			return nil, fmt.Errorf("reading values file %s: %w", valuesFile, err)
		}
		return ParseValues("", data, filepath.Ext(valuesFile))
	}
	return ParseValues(valuesFlag, nil, "")
}

// validateInputOption rejects anything but the two ValueInputOption values
// the Sheets API accepts; the default is USER_ENTERED.
func validateInputOption(option string) error {
	switch option {
	case "RAW", "USER_ENTERED":
		return nil
	}
	return fmt.Errorf("invalid --input-option %q: expected RAW or USER_ENTERED", option)
}
