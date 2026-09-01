package app

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// TestWriteToFile pins the shared --out plumbing: parent dirs are created,
// write streams into the file, and write's error propagates verbatim.
func TestWriteToFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, WriteToFile(fs, "out/nested/doc.txt", func(w io.Writer) error {
		_, err := io.WriteString(w, "hello")
		return err
	}))
	got, err := afero.ReadFile(fs, "out/nested/doc.txt")
	require.NoError(t, err)
	require.Equal(t, "hello", string(got))
}

// TestWriteToFileTruncates checks an existing destination is overwritten, not
// appended to.
func TestWriteToFileTruncates(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "doc.txt", []byte("old contents"), 0o644))

	require.NoError(t, WriteToFile(fs, "doc.txt", func(w io.Writer) error {
		_, err := io.WriteString(w, "new")
		return err
	}))
	got, err := afero.ReadFile(fs, "doc.txt")
	require.NoError(t, err)
	require.Equal(t, "new", string(got))
}

// TestWriteToFilePropagatesWriteError checks the write callback's error is
// returned as-is, with the file closed behind it.
func TestWriteToFilePropagatesWriteError(t *testing.T) {
	fs := afero.NewMemMapFs()
	boom := errors.New("boom")

	err := WriteToFile(fs, "doc.txt", func(w io.Writer) error { return boom })
	require.ErrorIs(t, err, boom)
}

// TestWriteToFileOpenError checks an un-openable destination surfaces the
// "opening --out" wrap.
func TestWriteToFileOpenError(t *testing.T) {
	// A read-only FS fails OpenFile; a bare filename skips MkdirAll so the
	// open failure is the first error surfaced.
	fs := afero.NewReadOnlyFs(afero.NewMemMapFs())

	err := WriteToFile(fs, "doc.txt", func(w io.Writer) error { return nil })
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "opening --out doc.txt"), err)
}
