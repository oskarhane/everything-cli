package slack

import (
	"context"
	"net/url"
	"strconv"
)

// threadPageSize is the conversations.replies page size requested when the
// item budget does not ask for fewer: 200 keeps the round trips low without
// oversized responses.
const threadPageSize = 200

// threadRepliesResponse is the pinned subset of GET /conversations.replies.
// Slack returns the parent message first, then the thread's replies;
// response_metadata.next_cursor (via cursorEnvelope) carries the follow-up
// page.
type threadRepliesResponse struct {
	cursorEnvelope
	Messages []wireMessage `json:"messages"`
}

// ThreadReplies returns one thread of channelID: the parent message followed
// by its replies in Slack's order, following response_metadata.next_cursor
// across pages up to maxItems total (0 = no cap). Every message carries
// channelID because Slack's wire messages have no channel of their own. Like
// the other cursor listings, a cursor that never empties surfaces the shared
// runaway-cursor error instead of truncating silently.
func (s *httpService) ThreadReplies(ctx context.Context, channelID, ts string, maxItems int) ([]Message, error) {
	return collectCursorPages(ctx, maxItems, "thread replies",
		func(ctx context.Context, cursor string, limit int) ([]Message, string, error) {
			q := url.Values{}
			q.Set("channel", channelID)
			q.Set("ts", ts)
			q.Set("limit", strconv.Itoa(pageLimit(threadPageSize, limit)))
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			var resp threadRepliesResponse
			if err := s.apiCall(ctx, "/conversations.replies", q, &resp); err != nil {
				return nil, "", err
			}
			messages := make([]Message, 0, len(resp.Messages))
			for _, m := range resp.Messages {
				messages = append(messages, m.view(channelID))
			}
			return messages, resp.nextCursor(), nil
		})
}
