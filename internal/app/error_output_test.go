package app

import (
	"bytes"
	"errors"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFailingTree mounts a leaf whose RunE fails under a NewRootCommand root
// and wires both of cobra's output streams to buffers. On error cobra prints
// the usage block via Println → OutOrStderr and the error line via
// PrintErrln → ErrOrStderr, so usage noise can land in either buffer; both
// are returned for asserting on. The tree deliberately leaves SilenceErrors
// unset so cobra's own error print exercises the same channel a usage dump
// would go through.
func newFailingTree() (root *cobra.Command, out, errOut *bytes.Buffer) {
	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}
	root = NewRootCommand(&Config{Fs: afero.NewMemMapFs()})
	root.AddCommand(&cobra.Command{
		Use:   "boom",
		Short: "Fails at runtime on purpose",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return errors.New("boom failed")
		},
	})
	root.SetOut(out)
	root.SetErr(errOut)
	return root, out, errOut
}

// TestRuntimeErrorPrintsErrorLineOnly pins the live bug: a runtime failure
// (e.g. a rate-limited `update`) used to dump cobra's entire usage block
// ahead of the error line. The root's SilenceUsage — set in NewRootCommand —
// is the guard, so a failed run prints the error line and nothing else.
func TestRuntimeErrorPrintsErrorLineOnly(t *testing.T) {
	root, out, errOut := newFailingTree()
	root.SetArgs([]string{"boom"})

	err := root.Execute()

	require.Error(t, err, "the runtime error must still reach the caller")
	assert.ErrorContains(t, err, "boom failed")
	assert.Contains(t, errOut.String(), "Error: boom failed",
		"the error line itself still prints")
	assert.Empty(t, out.String(), "stdout stays clean; a usage dump is the only thing cobra would print here")
	assert.NotContains(t, errOut.String(), "Usage:", "stderr must not carry the usage block")
}

// TestUsageErrorsPrintNoUsageBlock covers the parse-time failures — an
// unknown flag, an extra positional arg on a NoArgs leaf — where cobra's
// on-error usage print historically dumped the help text. The error is
// returned naming the offender and neither stream carries "Usage:".
func TestUsageErrorsPrintNoUsageBlock(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknown flag",
			args:    []string{"boom", "--bogus"},
			wantErr: "unknown flag: --bogus",
		},
		{
			name:    "extra positional arg",
			args:    []string{"boom", "surprise"},
			wantErr: `unknown command "surprise" for "everything-cli boom"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, out, errOut := newFailingTree()
			root.SetArgs(tt.args)

			err := root.Execute()

			require.Error(t, err, "the error must still reach the caller")
			assert.ErrorContains(t, err, tt.wantErr)
			assert.NotContains(t, out.String(), "Usage:", "stdout must not carry the usage block")
			assert.NotContains(t, errOut.String(), "Usage:", "stderr must not carry the usage block")
		})
	}
}
