package service

import (
	"context"
	"fmt"
	"io"

	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// FileService is the Drive API surface the file leaves use. Thin wrappers
// over Files, so fakes model file resources, not call objects.
type FileService interface {
	ListFiles(ctx context.Context, query string, maxResults int64) ([]*drive.File, error)
	GetFile(ctx context.Context, fileID string) (*drive.File, error)
	CreateFile(ctx context.Context, f *drive.File) (*drive.File, error)
	UploadFile(ctx context.Context, f *drive.File, mimeType string, content io.Reader) (*drive.File, error)
	TrashFile(ctx context.Context, fileID string) (*drive.File, error)
	UntrashFile(ctx context.Context, fileID string) (*drive.File, error)
	DeleteFile(ctx context.Context, fileID string) error
	DownloadTo(ctx context.Context, fileID string, w io.Writer) error
	ExportTo(ctx context.Context, fileID, exportMime string, w io.Writer) error
}

// ListFiles pages files.list (filtered by query when non-empty) until
// maxResults items are collected or the pages run out. maxResults <= 0 means
// an unlimited listing; the per-request page size is clamped to
// maxFilePageSize.
func (s *realDriveService) ListFiles(ctx context.Context, query string, maxResults int64) ([]*drive.File, error) {
	return pageAllBudgeted(maxResults, func(page string, remaining int64) ([]*drive.File, string, error) {
		call := s.drive.Files.List()
		if query != "" {
			call = call.Q(query)
		}
		if page != "" {
			call = call.PageToken(page)
		}
		if remaining > 0 {
			call = call.PageSize(remaining)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, "", fmt.Errorf("listing files: %w", err)
		}
		return resp.Files, resp.NextPageToken, nil
	})
}

// GetFile returns the file's metadata. The API returns full metadata by
// default, so no Fields projection is needed here.
func (s *realDriveService) GetFile(ctx context.Context, fileID string) (*drive.File, error) {
	file, err := s.drive.Files.Get(fileID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting file %s: %w", fileID, err)
	}
	return file, nil
}

// CreateFile creates an empty (or metadata-only) file — e.g. a folder, or a
// Google-native document seeded with properties. Contentful uploads go
// through UploadFile.
func (s *realDriveService) CreateFile(ctx context.Context, f *drive.File) (*drive.File, error) {
	created, err := s.drive.Files.Create(f).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("creating file: %w", err)
	}
	return created, nil
}

// UploadFile creates a file with content from r, labeled with mimeType
// (Drive uses the declared type, not sniffing, so an empty mimeType must be
// handled by the caller — this sends what it is given, untyped when empty).
func (s *realDriveService) UploadFile(ctx context.Context, f *drive.File, mimeType string, content io.Reader) (*drive.File, error) {
	call := s.drive.Files.Create(f)
	if mimeType == "" {
		call = call.Media(content)
	} else {
		call = call.Media(content, googleapi.ContentType(mimeType))
	}
	uploaded, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("uploading file: %w", err)
	}
	return uploaded, nil
}

// TrashFile moves the file to the trash. Drive v3 has no trash verb; the
// trash state lives on the File and is set through files.update.
func (s *realDriveService) TrashFile(ctx context.Context, fileID string) (*drive.File, error) {
	updated, err := s.drive.Files.Update(fileID, &drive.File{Trashed: true}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("trashing file %s: %w", fileID, err)
	}
	return updated, nil
}

// UntrashFile restores the file from the trash.
func (s *realDriveService) UntrashFile(ctx context.Context, fileID string) (*drive.File, error) {
	updated, err := s.drive.Files.Update(fileID, &drive.File{Trashed: false}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("untrashing file %s: %w", fileID, err)
	}
	return updated, nil
}

// DeleteFile permanently deletes the file (bypassing the trash).
func (s *realDriveService) DeleteFile(ctx context.Context, fileID string) error {
	if err := s.drive.Files.Delete(fileID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("deleting file %s: %w", fileID, err)
	}
	return nil
}

// DownloadTo streams the file's binary content into w. Drive serves the
// media via alt=media on files.get; the response body is fully consumed and
// closed here, so callers only supply the destination writer.
func (s *realDriveService) DownloadTo(ctx context.Context, fileID string, w io.Writer) error {
	resp, err := s.drive.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		return fmt.Errorf("downloading file %s: %w", fileID, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("downloading file %s: copying content: %w", fileID, err)
	}
	return nil
}

// ExportTo streams a Google Workspace file converted to exportMime into w.
// Only Google-native types (Docs, Sheets, Slides) export; binary blobs are
// downloaded with DownloadTo.
func (s *realDriveService) ExportTo(ctx context.Context, fileID, exportMime string, w io.Writer) error {
	resp, err := s.drive.Files.Export(fileID, exportMime).Context(ctx).Download()
	if err != nil {
		return fmt.Errorf("exporting file %s to %s: %w", fileID, exportMime, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("exporting file %s to %s: copying content: %w", fileID, exportMime, err)
	}
	return nil
}
