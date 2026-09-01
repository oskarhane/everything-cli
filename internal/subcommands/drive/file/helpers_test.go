package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeService is the hermetic service.FileService double: it serves seeded
// files, streams seeded content, and records every call for assertions. The
// embedded nil service.FileService satisfies any surface the parent hands
// down that these leaves never call, so it stays nil.
type fakeService struct {
	service.FileService

	files []*drive.File // served by ListFiles and GetFile-by-id
	err   error         // when set, every call fails

	listQ         string      // last ListFiles query
	listMax       int64       // last ListFiles max
	created       *drive.File // last CreateFile request
	uploaded      *drive.File // last UploadFile request
	uploadMime    string      // last UploadFile mime type
	uploadBytes   []byte      // bytes UploadFile received
	trashedID     string
	untrashedID   string
	deleted       bool
	deletedID     string
	perms         []*drive.Permission // served by ListPermissions
	listedFileID  string              // last ListPermissions file id
	grantedFileID string              // last GrantPermission file id
	grantedPerm   *drive.Permission   // last GrantPermission request
	deletedFileID string
	deletedPermID string
	downloaded    string // last DownloadTo file id
	downloadedB   []byte // bytes DownloadTo writes
	exportedID    string // last ExportTo file id
	exportMime    string // last ExportTo export mime
	exportBytes   []byte // bytes ExportTo writes
}

func (f *fakeService) ListFiles(_ context.Context, q string, maxResults int64) ([]*drive.File, error) {
	f.listQ, f.listMax = q, maxResults
	if f.err != nil {
		return nil, f.err
	}
	if maxResults > 0 && int64(len(f.files)) > maxResults {
		return f.files[:maxResults], nil
	}
	return f.files, nil
}

func (f *fakeService) GetFile(_ context.Context, id string) (*drive.File, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, file := range f.files {
		if file.Id == id {
			return file, nil
		}
	}
	return nil, fmt.Errorf("googleapi: Error 404: file %s not found", id)
}

func (f *fakeService) CreateFile(_ context.Context, file *drive.File) (*drive.File, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.created = file
	created := *file
	created.Id = "file_new"
	return &created, nil
}

func (f *fakeService) UploadFile(_ context.Context, file *drive.File, mimeType string, content io.Reader) (*drive.File, error) {
	if f.err != nil {
		return nil, f.err
	}
	body, err := io.ReadAll(content)
	if err != nil {
		return nil, err
	}
	f.uploaded, f.uploadMime, f.uploadBytes = file, mimeType, body
	uploaded := *file
	uploaded.Id = "file_new"
	return &uploaded, nil
}

func (f *fakeService) TrashFile(_ context.Context, id string) (*drive.File, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.trashedID = id
	return &drive.File{Id: id, Name: "Report", Trashed: true}, nil
}

func (f *fakeService) UntrashFile(_ context.Context, id string) (*drive.File, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.untrashedID = id
	return &drive.File{Id: id, Name: "Report", Trashed: false}, nil
}

func (f *fakeService) DeleteFile(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	f.deleted, f.deletedID = true, id
	return nil
}

func (f *fakeService) DownloadTo(_ context.Context, id string, w io.Writer) error {
	if f.err != nil {
		return f.err
	}
	f.downloaded = id
	_, err := w.Write(f.downloadedB)
	return err
}

func (f *fakeService) ExportTo(_ context.Context, id, exportMime string, w io.Writer) error {
	if f.err != nil {
		return f.err
	}
	f.exportedID, f.exportMime = id, exportMime
	_, err := w.Write(f.exportBytes)
	return err
}

// Permission fakes: ListPermissions serves perms, GrantPermission records
// the request and echoes an id, DeletePermission records the revoke. err,
// when set, fails every call like the file surfaces above.

func (f *fakeService) ListPermissions(_ context.Context, fileID string) ([]*drive.Permission, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.listedFileID = fileID
	return f.perms, nil
}

func (f *fakeService) GrantPermission(_ context.Context, fileID string, perm *drive.Permission) (*drive.Permission, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.grantedFileID, f.grantedPerm = fileID, perm
	granted := *perm
	granted.Id = "perm_new"
	return &granted, nil
}

func (f *fakeService) DeletePermission(_ context.Context, fileID, permissionID string) error {
	if f.err != nil {
		return f.err
	}
	f.deletedFileID, f.deletedPermID = fileID, permissionID
	return nil
}

// seedPermissions returns a realistic permission set: a user, a group, and
// the anyone-with-link constant id.
func seedPermissions() []*drive.Permission {
	return []*drive.Permission{
		{Id: "perm_1", Type: "user", Role: "writer", EmailAddress: "alice@example.com", DisplayName: "Alice"},
		{Id: "perm_2", Type: "group", EmailAddress: "team@example.com", DisplayName: "Team"},
		{Id: "anyoneWithLink", Type: "anyone", Role: "reader", Deleted: false},
	}
}

// fakeNewSvc returns a service.Dialer[service.FileService] handing out svc,
// so leaves run hermetically with no network and no real account store.
func fakeNewSvc(svc *fakeService) service.Dialer[service.FileService] {
	return func(context.Context) (service.FileService, error) { return svc, nil }
}

// newLeafCmd builds a leaf against a fake service, ready to execute.
func newLeafCmd(build func(*app.Config, service.Dialer[service.FileService]) *cobra.Command, svc *fakeService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), fakeNewSvc(svc))
}

// newLeafCmdWithFs builds a leaf against a fake service and a supplied FS,
// for leaves that read or write files (upload, download --out).
func newLeafCmdWithFs(build func(*app.Config, service.Dialer[service.FileService]) *cobra.Command, svc *fakeService, format string, fs afero.Fs) *cobra.Command {
	cfg := cmdtest.NewTestConfig(format)
	cfg.Fs = fs
	return build(cfg, fakeNewSvc(svc))
}

// seedFiles returns a small realistic file set: one binary blob, one
// Google-native doc, one trashed folder.
func seedFiles() []*drive.File {
	return []*drive.File{
		{Id: "file_1", Name: "report.pdf", MimeType: "application/pdf", Size: 42, Parents: []string{"folder_1"}, Owners: []*drive.User{{EmailAddress: "me@example.com"}}},
		{Id: "file_2", Name: "Q3 notes", MimeType: "application/vnd.google-apps.document", Owners: []*drive.User{{EmailAddress: "me@example.com"}}, Shared: true},
		{Id: "file_3", Name: "Archive", MimeType: "application/vnd.google-apps.folder", Trashed: true},
	}
}

// seedDetailFile returns one file carrying every rendered field, for get and
// the mutation echoes.
func seedDetailFile() *drive.File {
	return &drive.File{
		Id:           "file_1",
		Name:         "Report",
		MimeType:     "application/pdf",
		Size:         1234,
		Owners:       []*drive.User{{EmailAddress: "me@example.com"}},
		Parents:      []string{"parent_1", "parent_2"},
		Trashed:      true,
		Shared:       true,
		ModifiedTime: "2026-08-24T09:00:00.000Z",
		WebViewLink:  "https://drive.google.com/file/d/file_1/view",
		Description:  "Quarterly report",
	}
}

// seedBlobFile returns a file whose bytes are seeded for streaming tests:
// mimeType decides binary download vs native export branching.
func seedBlobFile(mimeType string) *drive.File {
	return &drive.File{Id: "file_1", Name: "content", MimeType: mimeType}
}

// readAll returns the full contents of a file on the test FS.
func readAll(t *testing.T, fs afero.Fs, path string) []byte {
	t.Helper()
	data, err := afero.ReadFile(fs, path)
	require.NoError(t, err)
	return data
}
