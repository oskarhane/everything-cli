package docs

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestInsertTableEmptyTableDimensions(t *testing.T) {
	svc := &fakeDocService{}
	cmdtest.RunCmd(t, newLeafCmd(newInsertTableCmd, svc, "json"),
		"doc_1", "--rows", "2", "--columns", "3")

	require.Equal(t, "doc_1", svc.insertTableID)
	// No cell data: Cells stays nil, and --index defaults to 0, which the
	// service reads as "the end of the tab body".
	require.Equal(t, service.DocTableSpec{Rows: 2, Columns: 3}, svc.insertTableSpec)
}

func TestInsertTableInlineCSV(t *testing.T) {
	svc := &fakeDocService{}
	cmdtest.RunCmd(t, newLeafCmd(newInsertTableCmd, svc, "json"),
		"doc_1", "--csv", "name,note\nOskar,\"likes, commas\"")

	// A real CSV parse: the quoted comma stays inside the second cell, and
	// the dimensions derive from the data.
	require.Equal(t, [][]string{{"name", "note"}, {"Oskar", "likes, commas"}}, svc.insertTableSpec.Cells)
	require.Equal(t, int64(2), svc.insertTableSpec.Rows)
	require.Equal(t, int64(2), svc.insertTableSpec.Columns)
}

func TestInsertTableRaggedCSVDerivesWidestColumns(t *testing.T) {
	svc := &fakeDocService{}
	cmdtest.RunCmd(t, newLeafCmd(newInsertTableCmd, svc, "json"),
		"doc_1", "--csv", "a,b,c\nx")

	// Ragged rows are allowed: the column count is the widest record.
	require.Equal(t, [][]string{{"a", "b", "c"}, {"x"}}, svc.insertTableSpec.Cells)
	require.Equal(t, int64(2), svc.insertTableSpec.Rows)
	require.Equal(t, int64(3), svc.insertTableSpec.Columns)
}

func TestInsertTableReadsCSVFile(t *testing.T) {
	svc := &fakeDocService{}
	fs := afero.NewMemMapFs()
	seedTextFile(t, fs, "cells.csv", "a,b\nc,d\n")
	cmd := newLeafCmdWithFs(newInsertTableCmd, svc, "json", fs)

	cmdtest.RunCmd(t, cmd, "doc_1", "--csv-file", "cells.csv")

	require.Equal(t, [][]string{{"a", "b"}, {"c", "d"}}, svc.insertTableSpec.Cells)
	require.Equal(t, int64(2), svc.insertTableSpec.Rows)
	require.Equal(t, int64(2), svc.insertTableSpec.Columns)
}

func TestInsertTableValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		files   map[string]string // seeded on the test FS
		wantErr string
	}{
		{
			name:    "csv and csv-file are mutually exclusive",
			args:    []string{"doc_1", "--csv", "a,b", "--csv-file", "cells.csv"},
			files:   map[string]string{"cells.csv": "a,b"},
			wantErr: "--csv and --csv-file are mutually exclusive",
		},
		{
			name:    "rows must not be set with csv",
			args:    []string{"doc_1", "--csv", "a,b", "--rows", "2"},
			wantErr: "--rows and --columns must not be set with --csv or --csv-file",
		},
		{
			name:    "columns must not be set with csv-file",
			args:    []string{"doc_1", "--csv-file", "cells.csv", "--columns", "2"},
			files:   map[string]string{"cells.csv": "a,b"},
			wantErr: "--rows and --columns must not be set with --csv or --csv-file",
		},
		{
			name:    "no cell data and no dimensions",
			args:    []string{"doc_1"},
			wantErr: "--rows and --columns are required",
		},
		{
			name:    "rows without columns",
			args:    []string{"doc_1", "--rows", "2"},
			wantErr: "--rows and --columns are required",
		},
		{
			name:    "zero dimensions are rejected",
			args:    []string{"doc_1", "--rows", "0", "--columns", "2"},
			wantErr: "--rows and --columns are required",
		},
		{
			name:    "negative index is rejected",
			args:    []string{"doc_1", "--rows", "1", "--columns", "1", "--index", "-1"},
			wantErr: "--index must be >= 0",
		},
		{
			name:    "csv-file with zero records",
			args:    []string{"doc_1", "--csv-file", "empty.csv"},
			files:   map[string]string{"empty.csv": ""},
			wantErr: "zero records",
		},
		{
			name:    "missing csv-file",
			args:    []string{"doc_1", "--csv-file", "nope.csv"},
			wantErr: "reading --csv-file nope.csv",
		},
		{
			name:    "malformed csv",
			args:    []string{"doc_1", "--csv", "\"unclosed"},
			wantErr: "parsing CSV cell data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeDocService{}
			fs := afero.NewMemMapFs()
			for path, content := range tt.files {
				seedTextFile(t, fs, path, content)
			}
			cmd := newLeafCmdWithFs(newInsertTableCmd, svc, "json", fs)

			_, err := cmdtest.RunCmdErr(t, cmd, tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, svc.insertTableID) // zero write calls
		})
	}
}

func TestInsertTablePropagatesAPIError(t *testing.T) {
	svc := &fakeDocService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInsertTableCmd, svc, "json"),
		"doc_1", "--rows", "1", "--columns", "1")

	require.ErrorIs(t, err, errAPI)
}

func TestInsertTableRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInsertTableCmd, svc, "json"),
		"--rows", "1", "--columns", "1")

	require.Contains(t, err.Error(), "accepts 1 arg")
}

func TestInsertTableForwardsTabKey(t *testing.T) {
	tests := []struct {
		name   string
		tabKey string
	}{
		{name: "by ID", tabKey: "t.def456"},
		{name: "by title", tabKey: "Changelog"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeDocService{docTabs: seedDocTabs()}
			cmdtest.RunCmd(t, newLeafCmd(newInsertTableCmd, svc, "json"),
				"doc_1", "--rows", "1", "--columns", "1", "--tab", tt.tabKey)

			// insert-table forwards the --tab key as-is; InsertDocTable
			// resolves it (exact tab ID first, then exact title), like
			// append — no leaf-side ResolveDocTab call.
			require.Equal(t, tt.tabKey, svc.insertTableSpec.TabKey)
		})
	}
}

func TestInsertTableUnknownTabWritesNothing(t *testing.T) {
	svc := &fakeDocService{docTabs: seedDocTabs()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newInsertTableCmd, svc, "json"),
		"doc_1", "--rows", "1", "--columns", "1", "--tab", "nope")

	require.ErrorContains(t, err, `no tab with ID or title "nope"`)
	require.Empty(t, svc.insertTableID) // zero write calls
}

func TestInsertTableSuccessLine(t *testing.T) {
	svc := &fakeDocService{insertTableStart: 42}
	out := cmdtest.RunCmd(t, newLeafCmd(newInsertTableCmd, svc, "json"),
		"doc_1", "--rows", "2", "--columns", "3", "--index", "10")

	// The line reports the effective dimensions and the start index the
	// service returned.
	require.Equal(t, "Inserted 2x3 table into document doc_1 at index 42\n", out)
	require.Equal(t, int64(10), svc.insertTableSpec.Index)
}
