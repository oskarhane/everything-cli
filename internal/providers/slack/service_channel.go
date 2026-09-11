package slack

import (
	"context"
	"fmt"
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
	Max     int64
}

// ChannelListOptions maps the conversations.list query params. Types is the
// comma-separated conversation kinds to include. Max is the total channel
// budget across pages (0 = no cap).
type ChannelListOptions struct {
	Types string
	Max   int64
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
// (still bounded by maxListPages against a cursor-looping endpoint).
func (s *httpService) ChannelHistory(ctx context.Context, opts ChannelHistoryOptions) ([]Message, error) {
	messages := make([]Message, 0)
	cursor := ""
	for page := 0; ; page++ {
		q := url.Values{}
		q.Set("channel", opts.Channel)
		if opts.Oldest != "" {
			q.Set("oldest", opts.Oldest)
		}
		if opts.Latest != "" {
			q.Set("latest", opts.Latest)
		}
		q.Set("limit", channelPageLimit(opts.Max, len(messages)))
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var out channelHistoryResponse
		if err := s.apiCall(ctx, "/conversations.history", q, &out); err != nil {
			return nil, err
		}
		for _, m := range out.Messages {
			messages = append(messages, m.view(opts.Channel))
		}
		if opts.Max > 0 && int64(len(messages)) >= opts.Max {
			break
		}
		if cursor = out.nextCursor(); cursor == "" {
			break
		}
		if page+1 >= maxListPages {
			return nil, fmt.Errorf("channel history did not terminate after %d pages", maxListPages)
		}
	}
	if opts.Max > 0 && int64(len(messages)) > opts.Max {
		messages = messages[:opts.Max]
	}
	return messages, nil
}

// ChannelList returns the conversations visible to the token, following
// response_metadata.next_cursor across pages until the listing is exhausted
// or the Max item budget is reached. Max <= 0 means no cap (still bounded by
// maxListPages). Types defaults to the caller's value; the leaf always
// supplies one.
func (s *httpService) ChannelList(ctx context.Context, opts ChannelListOptions) ([]Channel, error) {
	channels := make([]Channel, 0)
	cursor := ""
	for page := 0; ; page++ {
		q := url.Values{}
		if opts.Types != "" {
			q.Set("types", opts.Types)
		}
		q.Set("limit", channelPageLimit(opts.Max, len(channels)))
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var out channelListResponse
		if err := s.apiCall(ctx, "/conversations.list", q, &out); err != nil {
			return nil, err
		}
		for _, c := range out.Channels {
			channels = append(channels, c.view())
		}
		if opts.Max > 0 && int64(len(channels)) >= opts.Max {
			break
		}
		if cursor = out.nextCursor(); cursor == "" {
			break
		}
		if page+1 >= maxListPages {
			return nil, fmt.Errorf("channel listing did not terminate after %d pages", maxListPages)
		}
	}
	if opts.Max > 0 && int64(len(channels)) > opts.Max {
		channels = channels[:opts.Max]
	}
	return channels, nil
}

// channelPageLimit renders the per-request limit: the remaining budget when
// one is set, else the full page size. The result is at least 1 because the
// page loop exits before requesting a page with no budget left.
func channelPageLimit(max int64, have int) string {
	limit := int64(channelPageSize)
	if max > 0 {
		remaining := max - int64(have)
		if remaining < 1 {
			remaining = 1
		}
		if remaining < limit {
			limit = remaining
		}
	}
	return strconv.FormatInt(limit, 10)
}
