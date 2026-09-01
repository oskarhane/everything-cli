package app

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/spf13/afero"
)

// WriteToFile creates the destination's parent dirs as needed, then lets write
// stream into the newly opened file through the given afero FS. It is the
// shared --out plumbing for read commands that can send their bytes to a file
// instead of stdout; write receives the file and must return its own error.
func WriteToFile(fs afero.Fs, out string, write func(io.Writer) error) error {
	if dir := path.Dir(out); dir != "" && dir != "." {
		if err := fs.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory for --out %s: %w", out, err)
		}
	}
	f, err := fs.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("opening --out %s: %w", out, err)
	}
	if err := write(f); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing --out %s: %w", out, err)
	}
	return nil
}
