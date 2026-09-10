package comment

import (
	"github.com/spf13/cobra"

	drive "google.golang.org/api/drive/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// listFields is the field order for comment list output; the same names are
// the snake_case JSON and TOON keys. commentViewFields and replyViewFields
// are the single-object field orders the mutating leaves report. go-pretty's
// StyleLight upper-cases the headers when rendering.
var (
	listFields        = []string{"comment_id", "author", "created", "resolved", "quoted", "content", "replies"}
	commentViewFields = []string{"comment_id", "created"}
	replyViewFields   = []string{"reply_id", "created", "action"}
)

// commentRow maps one comment to its output row. replies nests the full
// reply objects — {reply_id, author, created, action, content} — for JSON
// and TOON; table cells compact them to a reply count via commentTableRows.
func commentRow(c *drive.Comment) map[string]any {
	return map[string]any{
		"comment_id": c.Id,
		"author":     authorName(c.Author),
		"created":    c.CreatedTime,
		"resolved":   c.Resolved,
		"quoted":     quotedValue(c.QuotedFileContent),
		"content":    c.Content,
		"replies":    replyRows(c.Replies),
	}
}

// commentTableRows copies rows with replies compacted to a reply count for
// table cells; JSON and TOON keep the nested objects.
func commentTableRows(rows []map[string]any) []map[string]any {
	compacted := make([]map[string]any, len(rows))
	for i, row := range rows {
		out := make(map[string]any, len(row))
		for k, v := range row {
			if k == "replies" {
				if replies, ok := v.([]map[string]any); ok {
					v = len(replies)
				}
			}
			out[k] = v
		}
		compacted[i] = out
	}
	return compacted
}

// replyRows maps a comment's replies to nested output objects. A nil reply
// list yields an empty array, never null.
func replyRows(replies []*drive.Reply) []map[string]any {
	rows := make([]map[string]any, 0, len(replies))
	for _, r := range replies {
		if r == nil {
			continue
		}
		rows = append(rows, replyRow(r))
	}
	return rows
}

// replyRow maps one reply to its nested output object.
func replyRow(r *drive.Reply) map[string]any {
	return map[string]any{
		"reply_id": r.Id,
		"author":   authorName(r.Author),
		"created":  r.CreatedTime,
		"action":   r.Action,
		"content":  r.Content,
	}
}

// printCommentList renders zero or more comments: a JSON/TOON array with
// nested replies, or a table with one row per comment and the replies
// compacted to a count.
func printCommentList(cmd *cobra.Command, cfg *app.Config, comments []*drive.Comment) {
	rows := make([]map[string]any, 0, len(comments))
	for _, c := range comments {
		rows = append(rows, commentRow(c))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), listFields, rows, commentTableRows(rows))
}

// printCommentView renders a single created comment: an object in JSON/TOON,
// a one-row table.
func printCommentView(cmd *cobra.Command, cfg *app.Config, c *drive.Comment) {
	view := map[string]any{
		"comment_id": c.Id,
		"created":    c.CreatedTime,
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), commentViewFields, view, []map[string]any{view})
}

// printReplyView renders a single created reply: an object in JSON/TOON, a
// one-row table.
func printReplyView(cmd *cobra.Command, cfg *app.Config, r *drive.Reply) {
	view := map[string]any{
		"reply_id": r.Id,
		"created":  r.CreatedTime,
		"action":   r.Action,
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), replyViewFields, view, []map[string]any{view})
}

// authorName returns the author's display name, or "" when absent.
func authorName(u *drive.User) string {
	if u == nil {
		return ""
	}
	return u.DisplayName
}

// quotedValue returns the quoted file content's text, or "" when absent.
func quotedValue(q *drive.CommentQuotedFileContent) string {
	if q == nil {
		return ""
	}
	return q.Value
}
