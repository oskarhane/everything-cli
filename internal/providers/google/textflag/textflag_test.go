package textflag

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestResolveReturnsInlineText(t *testing.T) {
	got, err := Resolve(afero.NewMemMapFs(), "inline", "", "append")

	require.NoError(t, err)
	require.Equal(t, "inline", got)
}

func TestResolveReadsTextFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "notes.txt", []byte("from file"), 0o644))

	got, err := Resolve(fs, "", "notes.txt", "append")

	require.NoError(t, err)
	require.Equal(t, "from file", got)
}

func TestResolveRejectsBothSources(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "notes.txt", []byte("from file"), 0o644))

	_, err := Resolve(fs, "inline", "notes.txt", "append")

	require.EqualError(t, err, "--text and --text-file are mutually exclusive")
}

func TestResolveRequiresASource(t *testing.T) {
	_, err := Resolve(afero.NewMemMapFs(), "", "", "insert")

	require.EqualError(t, err, "--text or --text-file is required: give the text to insert inline or via a file")
}

func TestResolveMissingTextFile(t *testing.T) {
	_, err := Resolve(afero.NewMemMapFs(), "", "nope.txt", "set")

	require.ErrorContains(t, err, "reading --text-file nope.txt")
}
