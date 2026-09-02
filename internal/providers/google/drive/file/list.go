package file

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newListCmd returns `drive file list`: files matching a Drive q, composed
// client-side from --query, --name, --parent, --mime, and --trashed. All
// terms are ANDed.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	var (
		query    string
		name     string
		parentID string
		mimeType string
		trashed  bool
		max      int64
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List files in Drive",
		Example: `# List the 25 most recent non-trashed files as JSON
everything-cli google drive file list --format json

# List folders inside a parent folder, as a table
everything-cli google drive file list --parent 1AbC --mime folder --format table

# Search by name substring with a raw query ANDed in
everything-cli google drive file list --name "invoice" --query "owner = 'me'" --max 100`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			q := composeQuery(query, name, parentID, resolveMime(mimeType), trashed)
			files, err := svc.ListFiles(cmd.Context(), q, max)
			if err != nil {
				return err
			}
			printFileList(cmd, cfg, files)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVarP(&query, "query", "q", "", "Raw Drive q term passed through verbatim (e.g. \"owner = 'me'\")")
	f.StringVar(&name, "name", "", "Substring the file name must contain")
	f.StringVar(&parentID, "parent", "", "Id of the folder whose children to list")
	f.StringVar(&mimeType, "mime", "", "MIME type filter: folder, doc, sheet, slide, or a raw MIME type")
	f.BoolVar(&trashed, "trashed", false, "Include trashed files (default lists only non-trashed)")
	f.Int64Var(&max, "max", 25, "Maximum files to return (0 = unlimited)")
	return cmd
}
