// Package textflag resolves the --text / --text-file source pair shared by
// commands that take their body text from exactly one of the two.
package textflag

import (
	"fmt"

	"github.com/spf13/afero"
)

// Resolve validates the --text / --text-file pair (exactly one set) and reads
// the file variant through fs. The text comes back verbatim; callers that need
// trailing newline handling (for example append) add it themselves. The action
// names what the text is for in the neither-set error, so the message reads
// naturally per command.
func Resolve(fs afero.Fs, text, textFile, action string) (string, error) {
	if text != "" && textFile != "" {
		return "", fmt.Errorf("--text and --text-file are mutually exclusive")
	}
	if text == "" && textFile == "" {
		return "", fmt.Errorf("--text or --text-file is required: give the text to %s inline or via a file", action)
	}
	if textFile != "" {
		b, err := afero.ReadFile(fs, textFile)
		if err != nil {
			return "", fmt.Errorf("reading --text-file %s: %w", textFile, err)
		}
		text = string(b)
	}
	return text, nil
}
