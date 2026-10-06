package docs

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newFormatCmd returns `docs format`: apply text and paragraph styling to the
// content range [--start, --end) — Docs-API UTF-16 content indices, zero-based
// with the end exclusive, so --start 1 --end 12 styles from the very start of
// the body — in a tab (--tab, by tab ID or exact title; default the first
// tab). The style switches (--bold, --italic, --underline, --strikethrough)
// turn those styles on over the range and --heading restyles the covered
// paragraphs as HEADING_<n>; at least one of them is required, and only the
// named styles are touched.
func newFormatCmd(_ *app.Config, newSvc service.Dialer[service.DocService]) *cobra.Command {
	var (
		start         int64
		end           int64
		bold          bool
		italic        bool
		underline     bool
		strikethrough bool
		heading       int64
		tab           string
	)
	cmd := &cobra.Command{
		Use:   "format <doc-id>",
		Short: "Style a content range in a Google Doc",
		Example: `# Bold the range [1, 12)
everything-cli google docs format 1AbCdEfGh --start 1 --end 12 --bold

# Restyle the covered paragraphs as H2 and italicize them, in a named tab
everything-cli google docs format 1AbCdEfGh --start 120 --end 240 --heading 2 --italic --tab "Changelog"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if start < 1 {
				return fmt.Errorf("--start is required and must be a positive Docs-API content index: the range starts there")
			}
			if end <= start {
				return fmt.Errorf("--end is required and must be greater than --start: the range end is exclusive")
			}
			if heading < 0 || heading > 6 {
				return fmt.Errorf("--heading must be between 1 and 6 (HEADING_<n>)")
			}
			if !bold && !italic && !underline && !strikethrough && heading == 0 {
				return fmt.Errorf("at least one of --bold, --italic, --underline, --strikethrough, or --heading is required")
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			// FormatDocRange forwards the tab ID as-is and makes no read to
			// resolve a title, so a --tab key is resolved here first via
			// the service's shared resolution; an empty tab lets the API
			// apply the format to the first tab.
			tabID := ""
			if tab != "" {
				resolved, err := svc.ResolveDocTab(cmd.Context(), args[0], tab)
				if err != nil {
					return err
				}
				tabID = resolved.TabID
			}
			format := service.DocRangeFormat{
				StartIndex:    start,
				EndIndex:      end,
				TabID:         tabID,
				Bold:          bold,
				Italic:        italic,
				Strikethrough: strikethrough,
				Underline:     underline,
				HeadingLevel:  heading,
			}
			if err := svc.FormatDocRange(cmd.Context(), args[0], format); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Formatted document %s range [%d, %d)\n", args[0], start, end); err != nil {
				return err
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.Int64Var(&start, "start", 0, "Docs-API content index the range starts at (required, >=1)")
	f.Int64Var(&end, "end", 0, "Docs-API content index the range ends before (required, >--start)")
	f.BoolVar(&bold, "bold", false, "Bold the range")
	f.BoolVar(&italic, "italic", false, "Italicize the range")
	f.BoolVar(&underline, "underline", false, "Underline the range")
	f.BoolVar(&strikethrough, "strikethrough", false, "Strikethrough the range")
	f.Int64Var(&heading, "heading", 0, "Restyle the covered paragraphs as HEADING_<n> (1-6)")
	f.StringVar(&tab, "tab", "", "Format within this tab, by tab ID or exact title (default: the first tab)")
	return cmd
}
