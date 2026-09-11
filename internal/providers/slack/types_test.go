package slack

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMessageJSONIsSnakeCase pins the shared output shape and its tags.
func TestMessageJSONIsSnakeCase(t *testing.T) {
	msg := Message{
		TS:         "1512085950.000216",
		ChannelID:  "C0B3HMXFEUV",
		User:       "U02H6ECK2",
		Text:       "hello",
		ThreadTS:   "1512085940.000100",
		ReplyCount: 2,
		Reactions:  []Reaction{{Name: "eyes", Count: 3}},
		Edited:     true,
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"ts": "1512085950.000216",
		"channel_id": "C0B3HMXFEUV",
		"user": "U02H6ECK2",
		"text": "hello",
		"thread_ts": "1512085940.000100",
		"reply_count": 2,
		"reactions": [{"name": "eyes", "count": 3}],
		"edited": true
	}`, string(data))
}

// TestMessageOptionalFieldsOmit: thread_ts and reactions disappear when
// empty; reply_count and edited always render.
func TestMessageOptionalFieldsOmit(t *testing.T) {
	data, err := json.Marshal(Message{TS: "1.0", ChannelID: "C1", User: "U1", Text: "x"})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "thread_ts")
	assert.NotContains(t, string(data), "reactions")
	assert.Contains(t, string(data), `"reply_count":0`)
	assert.Contains(t, string(data), `"edited":false`)
}

// TestWireMessageMapsEditedObjectToBool: the wire's edited: {...} marker maps
// to edited: true, and reactions flatten to name/count.
func TestWireMessageMapsEditedObjectToBool(t *testing.T) {
	var wire wireMessage
	require.NoError(t, json.Unmarshal([]byte(`{
		"ts": "1512085950.000216",
		"user": "U02H6ECK2",
		"text": "hello",
		"thread_ts": "1512085940.000100",
		"reply_count": 2,
		"reactions": [{"name": "eyes", "count": 3, "users": ["U1", "U2"]}],
		"edited": {"user": "U02H6ECK2", "ts": "1512085960.000300"},
		"unmapped_future_field": true
	}`), &wire))

	msg := wire.view("C0B3HMXFEUV")
	assert.Equal(t, "1512085950.000216", msg.TS)
	assert.Equal(t, "C0B3HMXFEUV", msg.ChannelID)
	assert.Equal(t, "U02H6ECK2", msg.User)
	assert.Equal(t, "hello", msg.Text)
	assert.Equal(t, "1512085940.000100", msg.ThreadTS)
	assert.Equal(t, 2, msg.ReplyCount)
	require.Len(t, msg.Reactions, 1)
	assert.Equal(t, Reaction{Name: "eyes", Count: 3}, msg.Reactions[0])
	assert.True(t, msg.Edited)

	var untouched wireMessage
	require.NoError(t, json.Unmarshal([]byte(`{"ts": "2.0", "user": "U1", "text": "x"}`), &untouched))
	assert.False(t, untouched.view("C1").Edited)
}

// TestWireChannelMapsToView: is_private and the im counterpart user survive
// the mapping; the view's user field is omitted when empty.
func TestWireChannelMapsToView(t *testing.T) {
	var channel wireChannel
	require.NoError(t, json.Unmarshal([]byte(`{"id": "C1", "name": "general", "is_private": true}`), &channel))
	assert.Equal(t, Channel{ID: "C1", Name: "general", IsPrivate: true}, channel.view())

	data, err := json.Marshal(channel.view())
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"user"`)

	var im wireChannel
	require.NoError(t, json.Unmarshal([]byte(`{"id": "D1", "is_im": true, "user": "U1"}`), &im))
	assert.Equal(t, Channel{ID: "D1", User: "U1"}, im.view())
}

// TestWireUserMapsToView: display_name comes from the nested profile object.
func TestWireUserMapsToView(t *testing.T) {
	var user wireUser
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "U1",
		"name": "oskar",
		"real_name": "Oskar Hane",
		"profile": {"display_name": "oskarh", "title": "unmapped"}
	}`), &user))
	assert.Equal(t, User{ID: "U1", Name: "oskar", RealName: "Oskar Hane", DisplayName: "oskarh"}, user.view())
}

// TestCursorEnvelope pins response_metadata.next_cursor extraction.
func TestCursorEnvelope(t *testing.T) {
	var env cursorEnvelope
	require.NoError(t, json.Unmarshal([]byte(`{"response_metadata": {"next_cursor": "dGVhbTpDMD"}}`), &env))
	assert.Equal(t, "dGVhbTpDMD", env.nextCursor())

	require.NoError(t, json.Unmarshal([]byte(`{"response_metadata": {"next_cursor": ""}}`), &env))
	assert.Empty(t, env.nextCursor())
}

// TestSearchMessagesPaging pins search.messages' page counter shape.
func TestSearchMessagesPaging(t *testing.T) {
	var msgs searchMessages
	require.NoError(t, json.Unmarshal([]byte(`{
		"total": 3,
		"paging": {"count": 2, "total": 3, "page": 1, "pages": 2},
		"matches": [{"ts": "1.0"}]
	}`), &msgs))
	assert.Equal(t, 3, msgs.Total)
	assert.Equal(t, searchPaging{Count: 2, Total: 3, Page: 1, Pages: 2}, msgs.Paging)
}
