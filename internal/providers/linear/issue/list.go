package issue

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/dates"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// nowFunc is the clock seam: tests pin it so --updated-since relative
// values resolve deterministically.
var nowFunc = time.Now

// newListCmd returns `linear issue list`: issues, most recently updated
// first, optionally scoped by team, assignee, creator, or update time.
func newListCmd(cfg *app.Config, newSvc service.Dialer[service.IssueService], viewerSvc service.Dialer[service.ViewerService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Linear issues, optionally scoped by team, assignee, creator, or update time",
		Example: `# List workspace issues as JSON
everything-cli linear issue list --format json

# List one team's issues as a table
everything-cli linear issue list --team 9c1e2f3a-... --format table

# Issues assigned to you, changed since yesterday
everything-cli linear issue list --assignee me --updated-since -1d --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			filter, err := listFilter(cmd, viewerSvc)
			if err != nil {
				return err
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			issues, err := svc.ListIssues(cmd.Context(), filter)
			if err != nil {
				return err
			}
			printIssueList(cmd, cfg, issues)
			return nil
		},
	}
	f := cmd.Flags()
	f.String("team", "", "Team ID to scope the listing (empty = workspace-wide)")
	f.String("assignee", "", "Assignee user ID, or me for the current account (empty = any)")
	f.String("created-by", "", "Creator user ID, or me for the current account (empty = any)")
	f.String("updated-since", "", "Only issues updated since: RFC3339 (offset optional), date, or relative (now, -1d); empty = no filter")
	return cmd
}

// listFilter composes the list flags into one server-side filter. Values are
// parsed and resolved before the issue service is dialed so a bad value
// fails fast.
func listFilter(cmd *cobra.Command, viewerSvc service.Dialer[service.ViewerService]) (service.IssueFilter, error) {
	f := cmd.Flags()
	teamID, _ := f.GetString("team")
	assignee, _ := f.GetString("assignee")
	createdBy, _ := f.GetString("created-by")
	updatedSinceRaw, _ := f.GetString("updated-since")

	filter := service.IssueFilter{TeamID: teamID}

	if updatedSinceRaw != "" {
		updatedSince, err := dates.ParseWindowTime(updatedSinceRaw, nowFunc())
		if err != nil {
			return service.IssueFilter{}, fmt.Errorf("invalid --updated-since %q: %w", updatedSinceRaw, err)
		}
		filter.UpdatedSince = updatedSince
	}

	me := &meResolver{newSvc: viewerSvc}
	var err error
	if filter.AssigneeID, err = me.resolve(cmd.Context(), assignee); err != nil {
		return service.IssueFilter{}, err
	}
	if filter.CreatorID, err = me.resolve(cmd.Context(), createdBy); err != nil {
		return service.IssueFilter{}, err
	}
	return filter, nil
}

// meResolver maps a --assignee/--created-by value to a user ID: a literal
// ID passes through untouched; the me keyword resolves through one
// GetViewer call shared by both flags for the whole run.
type meResolver struct {
	newSvc service.Dialer[service.ViewerService]
	svc    service.ViewerService
	id     string
	loaded bool
}

// resolve maps one flag value to a user ID.
func (r *meResolver) resolve(ctx context.Context, value string) (string, error) {
	if value != "me" {
		return value, nil
	}
	if !r.loaded {
		if r.svc == nil {
			svc, err := r.newSvc(ctx)
			if err != nil {
				return "", err
			}
			r.svc = svc
		}
		viewer, err := r.svc.GetViewer(ctx)
		if err != nil {
			return "", fmt.Errorf("resolving me: %w", err)
		}
		r.id, r.loaded = viewer.ID, true
	}
	return r.id, nil
}
