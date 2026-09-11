package slack

import (
	"context"
	"net/url"
	"strconv"
)

// channelPageSize is the per-request page size both conversations endpoints
// ask for when no smaller --max budget remains. Slack's documented default is
// 100; 200 halves the request count for uncapped listings while keeping each
// payload modest.
const channelPageSize = 200

// ChannelHistoryOptions maps the conversations.history query params. Max is
// the total message budget across pages (0 = no cap).
type ChannelHistoryOptions struct {
	Channel string
	Oldest  string
	Latest  string
	Max     int
}

// ChannelListOptions maps the conversations.list query params. Types is the
// comma-separated conversation kinds to include. Max is the total channel
// budget across pages (0 = no cap).
type ChannelListOptions struct {
	Types string
	Max   int
}

// channelHistoryResponse is the GET /conversations.history envelope; the
// embedded cursorEnvelope promotes response_metadata.next_cursor.
type channelHistoryResponse struct {
	cursorEnvelope
	Messages []wireMessage `json:"messages"`
}

// channelListResponse is the GET /conversations.list envelope.
type channelListResponse struct {
	cursorEnvelope
	Channels []wireChannel `json:"channels"`
}

// ChannelHistory returns the messages of one conversation, newest first,
// following response_metadata.next_cursor across pages until the listing is
// exhausted or the Max item budget is reached. Messages map to the shared
// Message view with ChannelID filled from the request. Max <= 0 means no cap
// (still bounded by maxListPages against a cursor-looping endpoint). The
// per-request page size is channelPageSize, clamped to the remaining budget.
func (s *httpService) ChannelHistory(ctx context.Context, opts ChannelHistoryOptions) ([]Message, error) {
	return collectCursorPages(ctx, opts.Max, "channel history",
		func(ctx context.Context, cursor string, limit int) ([]Message, string, error) {
			q := url.Values{}
			q.Set("channel", opts.Channel)
			if opts.Oldest != "" {
				q.Set("oldest", opts.Oldest)
			}
			if opts.Latest != "" {
				q.Set("latest", opts.Latest)
			}
			q.Set("limit", strconv.Itoa(pageLimit(channelPageSize, limit)))
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			var out channelHistoryResponse
			if err := s.apiCall(ctx, "/conversations.history", q, &out); err != nil {
				return nil, "", err
			}
			messages := make([]Message, 0, len(out.Messages))
			for _, m := range out.Messages {
				messages = append(messages, m.view(opts.Channel))
			}
			return messages, out.nextCursor(), nil
		})
}

// ChannelList returns the conversations visible to the token, following
// response_metadata.next_cursor across pages until the listing is exhausted
// or the Max item budget is reached. Max <= 0 means no cap (still bounded by
// maxListPages). Types defaults to the caller's value; the leaf always
// supplies one. The per-request page size is channelPageSize, clamped to the
// remaining budget.
func (s *httpService) ChannelList(ctx context.Context, opts ChannelListOptions) ([]Channel, error) {
	return collectCursorPages(ctx, opts.Max, "channel listing",
		func(ctx context.Context, cursor string, limit int) ([]Channel, string, error) {
			q := url.Values{}
			if opts.Types != "" {
				q.Set("types", opts.Types)
			}
			q.Set("limit", strconv.Itoa(pageLimit(channelPageSize, limit)))
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			var out channelListResponse
			if err := s.apiCall(ctx, "/conversations.list", q, &out); err != nil {
				return nil, "", err
			}
			channels := make([]Channel, 0, len(out.Channels))
			for _, c := range out.Channels {
				channels = append(channels, c.view())
			}
			return channels, out.nextCursor(), nil
		})
}
