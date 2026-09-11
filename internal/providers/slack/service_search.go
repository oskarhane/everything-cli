package slack

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// searchPageSize is Slack's per-request maximum for search.messages (count).
// Every page asks for a full page; --max budgets items across pages.
const searchPageSize = 100

// SearchResult is the `slack search messages` view: the query Slack echoed
// back plus the matches collected across pages. Messages is never nil, so an
// empty search renders as [] rather than null.
type SearchResult struct {
	Query    string        `json:"query"`
	Messages []SearchMatch `json:"messages"`
}

// SearchMatch is one search.messages match: the shared Message view plus the
// search-specific fields. Slack nests the conversation under the match, so
// the view flattens channel.id/name into channel_id (from Message) and
// channel_name; Username is the match's display handle and Permalink the
// workspace link back to the message.
type SearchMatch struct {
	Message
	ChannelName string `json:"channel_name"`
	Username    string `json:"username"`
	Permalink   string `json:"permalink"`
}

// searchMatches is the messages object of a search.messages response: the
// shared total/paging counter (types.go) plus the search-specific matches.
type searchMatches struct {
	searchMessages
	Matches []wireSearchMatch `json:"matches"`
}

// wireSearchMatch is the pinned subset of one search.messages match. The
// embedded shared wireMessage supplies ts/user/text/thread_ts/reply_count/
// reactions/edited; only the fields search adds are pinned here.
type wireSearchMatch struct {
	wireMessage
	Channel   wireChannel `json:"channel"`
	Username  string      `json:"username"`
	Permalink string      `json:"permalink"`
}

// view maps a wire match into the search view, taking the channel identity
// from the match's nested channel object.
func (m wireSearchMatch) view() SearchMatch {
	return SearchMatch{
		Message:     m.wireMessage.view(m.Channel.ID),
		ChannelName: m.Channel.Name,
		Username:    m.Username,
		Permalink:   m.Permalink,
	}
}

// searchMessagesResponse is the search.messages envelope: the query Slack
// echoes plus the messages object.
type searchMessagesResponse struct {
	Query    string        `json:"query"`
	Messages searchMatches `json:"messages"`
}

// SearchMessages runs search.messages (user tokens only), following Slack's
// 1-based page counter. It collects matches until max items are gathered
// (max <= 0 = no cap) or messages.paging.page reaches pages, then truncates
// any overshoot from the last page. sort is passed through when non-empty
// (Slack: score|timestamp). Pagination is bounded by maxListPages so a
// misbehaving endpoint cannot loop forever.
func (s *httpService) SearchMessages(ctx context.Context, query, sort string, max int) (*SearchResult, error) {
	result := &SearchResult{Messages: []SearchMatch{}}
	for page := 1; ; page++ {
		resp, err := s.searchMessagesPage(ctx, query, sort, page)
		if err != nil {
			return nil, err
		}
		result.Query = resp.Query
		for _, match := range resp.Messages.Matches {
			result.Messages = append(result.Messages, match.view())
		}
		if max > 0 && len(result.Messages) >= max {
			result.Messages = result.Messages[:max]
			return result, nil
		}
		if resp.Messages.Paging.Page >= resp.Messages.Paging.Pages {
			return result, nil
		}
		if page >= maxListPages {
			return nil, fmt.Errorf("search results did not terminate after %d pages", maxListPages)
		}
	}
}

// searchMessagesPage fetches one 1-based page of search.messages.
func (s *httpService) searchMessagesPage(ctx context.Context, query, sort string, page int) (*searchMessagesResponse, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("count", strconv.Itoa(searchPageSize))
	q.Set("page", strconv.Itoa(page))
	if sort != "" {
		q.Set("sort", sort)
	}
	var out searchMessagesResponse
	if err := s.apiCall(ctx, "/search.messages", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
