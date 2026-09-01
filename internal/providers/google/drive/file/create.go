package file

import (
	"fmt"

	"github.com/spf13/cobra"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// newCreateCmd returns `drive file create`: a metadata-only file — a folder,
// an empty Google-native document, or any other empty Drive file. Exactly one
// of --type or --mime-type names what to create. Contentful uploads go
// through upload.
func newCreateCmd(cfg *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	var (
		typeName    string
		mimeType    string
		parentID    string
		description string
	)
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a Drive file (folder, doc, sheet, slide, or raw MIME type)",
		Example: `# Create a folder
everything-cli drive file create "Reports" --format json

# Create an empty Google Sheet inside a parent folder
everything-cli drive file create "Q3 budget" --type sheet --parent 1AbCdEfGh

# Create a file with a raw MIME type
everything-cli drive file create "notes.md" --mime-type text/markdown`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			if f.Changed("type") && f.Changed("mime-type") {
				return fmt.Errorf("--type (%s) and --mime-type (%s) are mutually exclusive: pass exactly one", typeName, mimeType)
			}
			resolved, err := resolveCreateMime(typeName, mimeType, f.Changed("mime-type"))
			if err != nil {
				return err
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			file := &drive.File{Name: args[0], MimeType: resolved, Description: description}
			if parentID != "" {
				file.Parents = []string{parentID}
			}
			created, err := svc.CreateFile(cmd.Context(), file)
			if err != nil {
				return err
			}
			printFileView(cmd, cfg, created)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&typeName, "type", "folder", "Shorthand type to create: folder, doc, sheet, or slide")
	f.StringVar(&mimeType, "mime-type", "", "Raw MIME type for the new file (mutually exclusive with --type)")
	f.StringVar(&parentID, "parent", "", "Id of the parent folder")
	f.StringVar(&description, "description", "", "File description")
	return cmd
}

// resolveCreateMime maps the --type shorthand to its Drive MIME type. A raw
// --mime-type wins when given; an unknown --type is a usage error naming the
// valid shorthands.
func resolveCreateMime(typeName, rawMime string, hasRaw bool) (string, error) {
	if hasRaw {
		return rawMime, nil
	}
	mime, ok := mimeShorthands[typeName]
	if !ok {
		return "", fmt.Errorf("unsupported --type %q: want folder, doc, sheet, or slide", typeName)
	}
	return mime, nil
}
