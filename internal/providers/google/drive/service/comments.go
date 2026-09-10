package service

import (
	"context"
	"fmt"

	drive "google.golang.org/api/drive/v3"
)

// CommentService is the Drive API comment surface the docs and slides
// comment leaves use: comments.list, comments.create, comments.delete, and
// replies.create. Comments on Google Docs and Slides files are managed
// through the Drive API, so they live in this shared seam rather than a
// per-resource package. Thin wrappers, so fakes model comments, not call
// objects.
type CommentService interface {
	ListComments(ctx context.Context, fileID string) ([]*drive.Comment, error)
	CreateComment(ctx context.Context, fileID, content string) (*drive.Comment, error)
	CreateReply(ctx context.Context, fileID, commentID, content, action string) (*drive.Reply, error)
	DeleteComment(ctx context.Context, fileID, commentID string) error
}

// commentListFields is the Fields projection ListComments asks for: the
// comment fields the leaves display (content, author, times, resolve state,
// the quoted region, and the replies) plus nextPageToken, which pageAll
// needs to keep paging — a projection without it would silently truncate
// the listing at the first page.
const commentListFields = "nextPageToken,comments(id,content,htmlContent,author,createdTime,modifiedTime,resolved,quotedFileContent,replies(id,content,htmlContent,author,createdTime,action))"

// commentCreateFields is the Fields projection CreateComment asks for:
// what the created-comment response carries back for display.
const commentCreateFields = "id,content,htmlContent,author,createdTime,resolved,replies"

// replyCreateFields is the Fields projection CreateReply asks for: the
// reply fields the leaves display, including the action the reply performed
// (resolve/reopen).
const replyCreateFields = "id,content,htmlContent,author,createdTime,action"

// ListComments pages comments.list for one file and returns every comment
// across all pages.
func (s *realDriveService) ListComments(ctx context.Context, fileID string) ([]*drive.Comment, error) {
	call := s.drive.Comments.List(fileID).Fields(commentListFields)
	return pageAll(func(page string) ([]*drive.Comment, string, error) {
		if page != "" {
			call = call.PageToken(page)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, "", fmt.Errorf("listing comments for file %s: %w", fileID, err)
		}
		return resp.Comments, resp.NextPageToken, nil
	})
}

// CreateComment posts content as a new top-level comment on the file and
// returns the created comment. No anchor is sent — the leaves comment on
// the file as a whole, not a document region.
func (s *realDriveService) CreateComment(ctx context.Context, fileID, content string) (*drive.Comment, error) {
	created, err := s.drive.Comments.Create(fileID, &drive.Comment{Content: content}).
		Fields(commentCreateFields).
		Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("creating comment on file %s: %w", fileID, err)
	}
	return created, nil
}

// CreateReply posts a reply to one comment and returns the created reply.
// The API requires content when action is empty ("resolve" and "reopen"
// replies may omit it), so Content is set only for plain-text replies.
func (s *realDriveService) CreateReply(ctx context.Context, fileID, commentID, content, action string) (*drive.Reply, error) {
	reply := &drive.Reply{Action: action}
	if action == "" {
		reply.Content = content
	}
	created, err := s.drive.Replies.Create(fileID, commentID, reply).
		Fields(replyCreateFields).
		Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("replying to comment %s on file %s: %w", commentID, fileID, err)
	}
	return created, nil
}

// DeleteComment removes one comment (with its replies) from a file.
func (s *realDriveService) DeleteComment(ctx context.Context, fileID, commentID string) error {
	if err := s.drive.Comments.Delete(fileID, commentID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("deleting comment %s from file %s: %w", commentID, fileID, err)
	}
	return nil
}
