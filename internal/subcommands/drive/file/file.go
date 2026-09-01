// Package file builds the `drive file` command subtree.
package file

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// NewCmd returns the `drive file` parent with every file leaf attached.
// Each leaf lives in its own file: list.go, get.go, create.go, upload.go,
// download.go, trash.go, untrash.go, delete.go, and the sharing leaves
// (permissions, share, unshare), one AddCommand line each.
//
// The sharing leaves get a narrowing dial: the base dialer accepts
// drive.file accounts, but sharing any file on the account demands the full
// drive grant, so newSharingSvc resolves the account once, checks the scope
// before those three leaves can dial, and builds the service itself —
// mirroring how the drive parent narrows the shared seam per subtree.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.FileService]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Manage files in Drive",
	}
	shareSvc := newSharingSvc(cfg)
	cmd.AddCommand(newListCmd(cfg, newSvc))
	cmd.AddCommand(newGetCmd(cfg, newSvc))
	cmd.AddCommand(newCreateCmd(cfg, newSvc))
	cmd.AddCommand(newUploadCmd(cfg, newSvc))
	cmd.AddCommand(newDownloadCmd(cfg, newSvc))
	cmd.AddCommand(newTrashCmd(cfg, newSvc))
	cmd.AddCommand(newUntrashCmd(cfg, newSvc))
	cmd.AddCommand(newDeleteCmd(cfg, newSvc))
	cmd.AddCommand(newPermissionsCmd(cfg, shareSvc))
	cmd.AddCommand(newShareCmd(cfg, shareSvc))
	cmd.AddCommand(newUnshareCmd(cfg, shareSvc))
	return cmd
}

// newSharingSvc narrows the dial for the sharing leaves: granting or revoking
// permissions reaches account-wide files, which the minimal drive.file
// profile must never allow. It resolves the account and its token once
// (token cache read only, no API call), demands the full drive scope, and
// builds the service from the same token — mirroring drive's dial, which
// composes DialAccount → scope guard → service.New. A narrowed grant fails
// fast — before any service is built or API call made — with the same
// re-consent guidance as the other scope guards.
func newSharingSvc(cfg *app.Config) service.Dialer[service.FileService] {
	return func(ctx context.Context) (service.FileService, error) {
		acct, ts, err := auth.DialAccount(cfg)
		if err != nil {
			return nil, err
		}
		if err := auth.RequireScopes(acct, auth.ScopesDrive); err != nil {
			return nil, err
		}
		return service.New(ctx, ts)
	}
}
