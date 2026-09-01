package cmdtest

import (
	"context"

	"github.com/oskarhane/google-cli/internal/subcommands/drive/service"
)

// DeleteRecorder is the hermetic service.FileService double for the delete
// leaves of the docs, sheets, and slides trees: it records every deleted
// file id and fails on demand. The embedded nil service.FileService
// satisfies the surface the parent hands down that delete never calls, so
// it stays nil.
type DeleteRecorder struct {
	service.FileService

	Err error // when set, DeleteFile fails with it

	DeletedIDs []string // every deleted file id, in call order
}

// NewDeleteRecorder returns an empty recorder, ready to hand to a delete
// leaf's Dialer.
func NewDeleteRecorder() *DeleteRecorder {
	return &DeleteRecorder{}
}

func (f *DeleteRecorder) DeleteFile(_ context.Context, fileID string) error {
	if f.Err != nil {
		return f.Err
	}
	f.DeletedIDs = append(f.DeletedIDs, fileID)
	return nil
}
