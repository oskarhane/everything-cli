package service

import (
	"context"
	"fmt"

	drive "google.golang.org/api/drive/v3"
)

// PermissionService is the Drive API surface the permission leaves use:
// permissions.list, permissions.create, permissions.delete. Thin wrappers,
// so fakes model permissions, not call objects.
type PermissionService interface {
	ListPermissions(ctx context.Context, fileID string) ([]*drive.Permission, error)
	GrantPermission(ctx context.Context, fileID string, perm *drive.Permission) (*drive.Permission, error)
	DeletePermission(ctx context.Context, fileID, permissionID string) error
}

// ListPermissions pages permissions.list for one file and returns every
// permission across all pages.
func (s *realDriveService) ListPermissions(ctx context.Context, fileID string) ([]*drive.Permission, error) {
	call := s.drive.Permissions.List(fileID)
	return pageAll(func(page string) ([]*drive.Permission, string, error) {
		if page != "" {
			call = call.PageToken(page)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, "", fmt.Errorf("listing permissions for file %s: %w", fileID, err)
		}
		return resp.Permissions, resp.NextPageToken, nil
	})
}

// GrantPermission adds perm to the file and returns the created permission.
// Fields is pinned to the grant-relevant fields so the response carries
// exactly what the leaves display.
func (s *realDriveService) GrantPermission(ctx context.Context, fileID string, perm *drive.Permission) (*drive.Permission, error) {
	granted, err := s.drive.Permissions.Create(fileID, perm).
		Fields("id,type,role,emailAddress,displayName,expirationTime,deleted").
		Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("granting permission on file %s: %w", fileID, err)
	}
	return granted, nil
}

// DeletePermission revokes one permission from a file.
func (s *realDriveService) DeletePermission(ctx context.Context, fileID, permissionID string) error {
	if err := s.drive.Permissions.Delete(fileID, permissionID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("deleting permission %s from file %s: %w", permissionID, fileID, err)
	}
	return nil
}
