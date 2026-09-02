package label

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	gmail "google.golang.org/api/gmail/v1"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/providers/google/gmail/service"
)

// newGetCmd returns `gmail label get`: one label by id or name.
func newGetCmd(cfg *app.Config, newSvc service.Dialer[service.GmailService]) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id-or-name>",
		Short: "Show a Gmail label by id or name",
		Example: `# Show the INBOX label as JSON
everything-cli google gmail label get INBOX --format json

# Show a label by name as a table
everything-cli google gmail label get Travel --format table`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			label, err := getLabel(cmd.Context(), svc, args[0])
			if err != nil {
				return err
			}
			printLabel(cmd, cfg, label)
			return nil
		},
	}
}

// getLabel fetches the label by id, falling back to a name match against the
// listed labels, so users can address labels either way.
func getLabel(ctx context.Context, svc service.GmailService, idOrName string) (*gmail.Label, error) {
	label, getErr := svc.GetLabel(ctx, idOrName)
	if getErr == nil {
		return label, nil
	}
	labels, listErr := svc.ListLabels(ctx)
	if listErr != nil {
		return nil, getErr
	}
	for _, l := range labels {
		if l.Name == idOrName {
			return l, nil
		}
	}
	return nil, fmt.Errorf("label %q not found by id or name", idOrName)
}
