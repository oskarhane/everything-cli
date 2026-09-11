package slack

import (
	"fmt"
	"strings"
)

// maxListPages caps how many pages one listing may follow before giving up.
// A well-behaved server ends with an empty next_cursor, so the cap only ever
// fires on a misbehaving endpoint looping cursors forever. 100 pages at
// Slack's page sizes is far beyond any real listing.
const maxListPages = 100

// Message is the shared view of one Slack message, returned by search,
// channel history, and thread output alike. Slack's wire message carries no
// channel of its own — the enclosing listing supplies ChannelID. ThreadTS is
// set only when the message belongs to a thread; Reactions is the emoji
// tally; Edited flattens the wire's edited object to a bool.
type Message struct {
	TS         string     `json:"ts"`
	ChannelID  string     `json:"channel_id"`
	User       string     `json:"user"`
	Text       string     `json:"text"`
	ThreadTS   string     `json:"thread_ts,omitempty"`
	ReplyCount int        `json:"reply_count"`
	Reactions  []Reaction `json:"reactions,omitempty"`
	Files      []File     `json:"files,omitempty"`
	Edited     bool       `json:"edited"`
}

// Reaction is one emoji's tally on a message.
type Reaction struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// File is one attachment on a message. Slack's wire file object carries many
// more fields; only the four the shared view surfaces are pinned here. Slack
// spells the content type "mimetype" (no underscore).
type File struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Mimetype string `json:"mimetype"`
	Size     int64  `json:"size"`
}

// reactionCell is the reactions value one message row carries. It marshals as
// the shared []Reaction array (JSON and TOON) while String renders the table
// cell: the comma-joined name:count list channel history has always shown.
// output.PrintTable's default %v formatting honors the String method, so one
// messageRow serves both the table and the JSON/TOON render formats.
type reactionCell []Reaction

// String renders the table cell form, e.g. "eyes:3,fire:1"; an empty tally
// renders as "".
func (r reactionCell) String() string {
	parts := make([]string, 0, len(r))
	for _, reaction := range r {
		parts = append(parts, fmt.Sprintf("%s:%d", reaction.Name, reaction.Count))
	}
	return strings.Join(parts, ",")
}

// fileCell is the attachments value one message row carries. It marshals as
// the shared []File array (JSON and TOON) while String renders the table cell:
// the comma-joined name:id list. It mirrors reactionCell so one messageRow
// serves both the table and the JSON/TOON render formats.
type fileCell []File

// String renders the table cell form, e.g. "deploy.log:F0B3HMXFEUV"; an empty
// attachment list renders as "".
func (f fileCell) String() string {
	parts := make([]string, 0, len(f))
	for _, file := range f {
		parts = append(parts, fmt.Sprintf("%s:%s", file.Name, file.ID))
	}
	return strings.Join(parts, ",")
}

// messageRow maps one message to its shared output row. It is the single row
// shape behind both channel history and thread output: JSON/TOON mirror the
// Message tags (thread_ts and reactions omit when empty, reply_count and
// edited always render) and each leaf's field list addresses the table cells.
func messageRow(m Message) map[string]any {
	row := map[string]any{
		"ts":          m.TS,
		"channel_id":  m.ChannelID,
		"user":        m.User,
		"text":        m.Text,
		"reply_count": m.ReplyCount,
		"edited":      m.Edited,
	}
	if m.ThreadTS != "" {
		row["thread_ts"] = m.ThreadTS
	}
	if len(m.Reactions) > 0 {
		row["reactions"] = reactionCell(m.Reactions)
	}
	if len(m.Files) > 0 {
		row["files"] = fileCell(m.Files)
	}
	return row
}

// Channel is the shared view of one conversation (conversations.list). User
// is the counterpart for im entries and is omitted elsewhere; IsPrivate is
// false for ims.
type Channel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
	User      string `json:"user,omitempty"`
}

// User is the shared view of one workspace member (users.list/users.info).
type User struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RealName    string `json:"real_name"`
	DisplayName string `json:"display_name"`
}

// wireMessage is the pinned subset of Slack's wire message object. Slack
// messages are subtype-heavy (bot_message, channel_join, ...) and carry many
// more fields, so decoding is deliberately non-strict: unmapped fields are
// ignored and the view is built from what is here.
type wireMessage struct {
	TS         string         `json:"ts"`
	User       string         `json:"user"`
	Text       string         `json:"text"`
	ThreadTS   string         `json:"thread_ts"`
	ReplyCount int            `json:"reply_count"`
	Reactions  []wireReaction `json:"reactions"`
	Files      []wireFile     `json:"files"`
	Edited     *wireEdited    `json:"edited"`
}

// wireReaction is one reactions element; its users list is not part of the
// shared view.
type wireReaction struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// wireFile is the pinned subset of one message attachment; Slack's wire object
// exposes many more fields and spells the content type "mimetype".
type wireFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Mimetype string `json:"mimetype"`
	Size     int64  `json:"size"`
}

// wireEdited is the marker Slack attaches to messages changed after posting;
// its presence maps to Message.Edited.
type wireEdited struct {
	User string `json:"user"`
	TS   string `json:"ts"`
}

// view maps m into the shared Message view for the channel it was read from.
func (m wireMessage) view(channelID string) Message {
	msg := Message{
		TS:         m.TS,
		ChannelID:  channelID,
		User:       m.User,
		Text:       m.Text,
		ThreadTS:   m.ThreadTS,
		ReplyCount: m.ReplyCount,
		Edited:     m.Edited != nil,
	}
	for _, r := range m.Reactions {
		msg.Reactions = append(msg.Reactions, Reaction(r))
	}
	for _, f := range m.Files {
		msg.Files = append(msg.Files, File(f))
	}
	return msg
}

// wireChannel is the pinned subset of a conversation object.
type wireChannel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
	User      string `json:"user"`
}

// view maps c into the shared Channel view.
func (c wireChannel) view() Channel {
	return Channel(c)
}

// wireUser is the pinned subset of a user object; display_name lives under
// the profile object.
type wireUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	RealName string `json:"real_name"`
	Profile  struct {
		DisplayName string `json:"display_name"`
	} `json:"profile"`
}

// view maps u into the shared User view.
func (u wireUser) view() User {
	return User{ID: u.ID, Name: u.Name, RealName: u.RealName, DisplayName: u.Profile.DisplayName}
}

// responseMetadata is the cursor carrier of Slack's paginated list
// responses.
type responseMetadata struct {
	NextCursor string `json:"next_cursor"`
}

// cursorEnvelope is embedded by cursor-paginated list responses
// (conversations.history, conversations.list, users.list, ...). Embedding it
// promotes response_metadata onto the enclosing response type.
type cursorEnvelope struct {
	ResponseMetadata responseMetadata `json:"response_metadata"`
}

// nextCursor returns the cursor to follow, or "" when the listing is
// exhausted.
func (e cursorEnvelope) nextCursor() string { return e.ResponseMetadata.NextCursor }

// searchPaging is the page counter of a search.messages response
// (messages.paging): search paginates by page number, not by cursor.
type searchPaging struct {
	Count int `json:"count"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Pages int `json:"pages"`
}

// searchMessages is the messages object of a search.messages response. The
// match element shape belongs to the search resource; only the paging
// counter is shared.
type searchMessages struct {
	Total  int          `json:"total"`
	Paging searchPaging `json:"paging"`
}
