package slack

import (
	"context"
	"net/url"
	"strconv"
)

// threadPageLimit is the conversations.replies page size requested when the
// item budget does not ask for fewer: 200 keeps the round trips low without
// oversized responses.
const threadPageLimit = 200

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
// channelID because Slack's wire messages have no channel of their own.
func (s *httpService) ThreadReplies(ctx context.Context, channelID, ts string, maxItems int) ([]Message, error) {
	out := []Message{}
	cursor := ""
	for page := 0; page < maxListPages; page++ {
		limit := threadPageLimit
		if maxItems > 0 && maxItems-len(out) < limit {
			limit = maxItems - len(out)
		}
		q := url.Values{}
		q.Set("channel", channelID)
		q.Set("ts", ts)
		q.Set("limit", strconv.Itoa(limit))
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var resp threadRepliesResponse
		if err := s.apiCall(ctx, "/conversations.replies", q, &resp); err != nil {
			return nil, err
		}
		for _, m := range resp.Messages {
			out = append(out, m.view(channelID))
			if maxItems > 0 && len(out) >= maxItems {
				return out, nil
			}
		}
		cursor = resp.nextCursor()
		if cursor == "" {
			break
		}
	}
	return out, nil
}
