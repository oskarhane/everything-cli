package comment

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/spf13/cobra"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeCommentService is the hermetic service.CommentService double: it serves
// seeded comments and records every call for assertions. The embedded nil
// service.CommentService satisfies the whole CommentService surface, so
// unseeded results stay nil and fail loudly if a leaf ever renders them.
type fakeCommentService struct {
	service.CommentService

	err error // when set, every call fails

	comments []*drive.Comment // served by ListComments
	listID   string           // last ListComments file id

	createFileID string         // last CreateComment file id
	createText   string         // last CreateComment text
	created      *drive.Comment // served by CreateComment

	replyFileID    string       // last CreateReply file id
	replyCommentID string       // last CreateReply comment id
	replyText      string       // last CreateReply content
	replyAction    string       // last CreateReply action
	replied        *drive.Reply // served by CreateReply

	deleteFileID    string // last DeleteComment file id
	deleteCommentID string // last DeleteComment comment id
	deleteCalls     int
}

func (f *fakeCommentService) ListComments(_ context.Context, fileID string) ([]*drive.Comment, error) {
	f.listID = fileID
	if f.err != nil {
		return nil, f.err
	}
	return f.comments, nil
}

func (f *fakeCommentService) CreateComment(_ context.Context, fileID, content string) (*drive.Comment, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.createFileID, f.createText = fileID, content
	return f.created, nil
}

func (f *fakeCommentService) CreateReply(_ context.Context, fileID, commentID, content, action string) (*drive.Reply, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.replyFileID, f.replyCommentID, f.replyText, f.replyAction = fileID, commentID, content, action
	return f.replied, nil
}

func (f *fakeCommentService) DeleteComment(_ context.Context, fileID, commentID string) error {
	if f.err != nil {
		return f.err
	}
	f.deleteFileID, f.deleteCommentID = fileID, commentID
	f.deleteCalls++
	return nil
}

// newCommentLeafCmd builds a comment leaf against a fake CommentService,
// ready to execute hermetically with no network and no real account store.
func newCommentLeafCmd(build func(*app.Config, service.Dialer[service.CommentService]) *cobra.Command, svc *fakeCommentService, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), func(context.Context) (service.CommentService, error) { return svc, nil })
}

// seedComments returns a small realistic comment set: one unresolved with a
// quoted region and two replies, one unresolved with no author, quoted
// region, or replies (the nil-branch coverage), and one resolved.
func seedComments() []*drive.Comment {
	return []*drive.Comment{
		{
			Id:                "comment_1",
			Author:            &drive.User{DisplayName: "Oskar Hane"},
			CreatedTime:       "2026-09-08T09:15:00Z",
			QuotedFileContent: &drive.CommentQuotedFileContent{Value: "revenue is up 12%"},
			Content:           "Needs a source for this claim",
			Replies: []*drive.Reply{
				{
					Id:          "reply_1",
					Author:      &drive.User{DisplayName: "Ada Lovelace"},
					CreatedTime: "2026-09-08T10:00:00Z",
					Content:     "Source added in v2",
				},
				{
					Id:          "reply_2",
					Author:      &drive.User{DisplayName: "Oskar Hane"},
					CreatedTime: "2026-09-09T08:30:00Z",
					Action:      "resolve",
				},
			},
		},
		{
			Id:          "comment_2",
			CreatedTime: "2026-09-07T16:45:00Z",
			Content:     "Typo in the header",
		},
		{
			Id:          "comment_3",
			Author:      &drive.User{DisplayName: "Ada Lovelace"},
			CreatedTime: "2026-09-06T11:00:00Z",
			Resolved:    true,
			Content:     "Title typo",
			Replies: []*drive.Reply{
				{
					Id:          "reply_3",
					Author:      &drive.User{DisplayName: "Oskar Hane"},
					CreatedTime: "2026-09-06T11:30:00Z",
					Action:      "resolve",
				},
			},
		},
	}
}

// seedCreatedComment is the comment CreateComment hands back.
func seedCreatedComment() *drive.Comment {
	return &drive.Comment{
		Id:          "comment_new",
		Author:      &drive.User{DisplayName: "Oskar Hane"},
		CreatedTime: "2026-09-10T09:00:00Z",
		Content:     "Needs a source for this claim",
	}
}

// seedReplied is the reply CreateReply hands back, mirroring action.
func seedReplied(action string) *drive.Reply {
	r := &drive.Reply{
		Id:          "reply_new",
		Author:      &drive.User{DisplayName: "Oskar Hane"},
		CreatedTime: "2026-09-10T09:05:00Z",
	}
	if action != "" {
		r.Action = action
	} else {
		r.Content = "Source added in v2"
	}
	return r
}

// errAPI stands in for a Google API failure any leaf must propagate.
var errAPI = errors.New("googleapi: Error 403: access denied")
