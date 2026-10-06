package docs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

func TestFormatValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "start missing",
			args:    []string{"doc_1", "--end", "5", "--bold"},
			wantErr: "--start is required",
		},
		{
			name:    "start zero",
			args:    []string{"doc_1", "--start", "0", "--end", "5", "--bold"},
			wantErr: "--start is required",
		},
		{
			name:    "start negative",
			args:    []string{"doc_1", "--start", "-2", "--end", "5", "--bold"},
			wantErr: "--start is required",
		},
		{
			name:    "end missing",
			args:    []string{"doc_1", "--start", "1", "--bold"},
			wantErr: "--end is required",
		},
		{
			name:    "end equals start",
			args:    []string{"doc_1", "--start", "5", "--end", "5", "--bold"},
			wantErr: "--end is required and must be greater than --start",
		},
		{
			name:    "end before start",
			args:    []string{"doc_1", "--start", "5", "--end", "2", "--bold"},
			wantErr: "--end is required and must be greater than --start",
		},
		{
			name:    "no style requested",
			args:    []string{"doc_1", "--start", "1", "--end", "5"},
			wantErr: "at least one of --bold, --italic, --underline, --strikethrough, or --heading is required",
		},
		{
			name:    "heading above range",
			args:    []string{"doc_1", "--start", "1", "--end", "5", "--heading", "7"},
			wantErr: "--heading must be between 1 and 6",
		},
		{
			name:    "heading negative",
			args:    []string{"doc_1", "--start", "1", "--end", "5", "--heading", "-1"},
			wantErr: "--heading must be between 1 and 6",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeDocService{}
			_, err := cmdtest.RunCmdErr(t, newLeafCmd(newFormatCmd, svc, "json"), tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, svc.formatID) // zero write calls
		})
	}
}

func TestFormatStyleFlagMapping(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want service.DocRangeFormat
	}{
		{
			name: "bold",
			args: []string{"--bold"},
			want: service.DocRangeFormat{StartIndex: 2, EndIndex: 9, Bold: true},
		},
		{
			name: "italic",
			args: []string{"--italic"},
			want: service.DocRangeFormat{StartIndex: 2, EndIndex: 9, Italic: true},
		},
		{
			name: "underline",
			args: []string{"--underline"},
			want: service.DocRangeFormat{StartIndex: 2, EndIndex: 9, Underline: true},
		},
		{
			name: "strikethrough",
			args: []string{"--strikethrough"},
			want: service.DocRangeFormat{StartIndex: 2, EndIndex: 9, Strikethrough: true},
		},
		{
			name: "all styles combined",
			args: []string{"--bold", "--italic", "--underline", "--strikethrough"},
			want: service.DocRangeFormat{StartIndex: 2, EndIndex: 9, Bold: true, Italic: true, Underline: true, Strikethrough: true},
		},
		{
			name: "heading level",
			args: []string{"--heading", "3"},
			want: service.DocRangeFormat{StartIndex: 2, EndIndex: 9, HeadingLevel: 3},
		},
		{
			name: "heading with a text style",
			args: []string{"--heading", "1", "--bold"},
			want: service.DocRangeFormat{StartIndex: 2, EndIndex: 9, Bold: true, HeadingLevel: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeDocService{}
			args := append([]string{"doc_1", "--start", "2", "--end", "9"}, tt.args...)
			cmdtest.RunCmd(t, newLeafCmd(newFormatCmd, svc, "json"), args...)

			require.Equal(t, "doc_1", svc.formatID)
			require.Equal(t, tt.want, svc.formatSpec)
		})
	}
}

func TestFormatSuccessLineNamesDocAndRange(t *testing.T) {
	svc := &fakeDocService{}
	out := cmdtest.RunCmd(t, newLeafCmd(newFormatCmd, svc, "json"),
		"doc_1", "--start", "1", "--end", "12", "--bold")

	require.Equal(t, "Formatted document doc_1 range [1, 12)\n", out)
}

func TestFormatTabByIDReachesService(t *testing.T) {
	svc := &fakeDocService{docTabs: seedDocTabs()}
	cmdtest.RunCmd(t, newLeafCmd(newFormatCmd, svc, "json"),
		"doc_1", "--start", "1", "--end", "5", "--bold", "--tab", "t.def456")

	require.Equal(t, "t.def456", svc.formatSpec.TabID)
	require.Equal(t, int64(1), svc.formatSpec.StartIndex)
	require.Equal(t, int64(5), svc.formatSpec.EndIndex)
}

func TestFormatTabByTitleSendsResolvedID(t *testing.T) {
	svc := &fakeDocService{docTabs: seedDocTabs()}
	cmdtest.RunCmd(t, newLeafCmd(newFormatCmd, svc, "json"),
		"doc_1", "--start", "1", "--end", "5", "--bold", "--tab", "Changelog")

	// FormatDocRange makes no read of its own: the leaf resolves the title
	// via ResolveDocTab and sends the resolved tab ID.
	require.Equal(t, "t.def456", svc.formatSpec.TabID)
}

func TestFormatUnknownTabWritesNothing(t *testing.T) {
	svc := &fakeDocService{docTabs: seedDocTabs()}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newFormatCmd, svc, "json"),
		"doc_1", "--start", "1", "--end", "5", "--bold", "--tab", "nope")

	require.ErrorContains(t, err, `no tab with ID or title "nope"`)
	require.Empty(t, svc.formatID) // zero write calls
}

func TestFormatPropagatesAPIError(t *testing.T) {
	svc := &fakeDocService{err: errAPI}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newFormatCmd, svc, "json"),
		"doc_1", "--start", "1", "--end", "5", "--bold")

	require.ErrorIs(t, err, errAPI)
}

func TestFormatRequiresExactlyOneArg(t *testing.T) {
	svc := &fakeDocService{}
	_, err := cmdtest.RunCmdErr(t, newLeafCmd(newFormatCmd, svc, "json"),
		"--start", "1", "--end", "5", "--bold")

	require.Contains(t, err.Error(), "accepts 1 arg")
}
