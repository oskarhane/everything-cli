package label

import (
	"testing"

	"github.com/stretchr/testify/require"

	gmail "google.golang.org/api/gmail/v1"
)

func TestCreateMinimal(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newCreateCmd, svc, "json"), "Travel")

	require.NotNil(t, svc.created)
	require.Equal(t, "Travel", svc.created.Name)
	require.Nil(t, svc.created.Color)
	require.Empty(t, svc.created.LabelListVisibility)
	require.Empty(t, svc.created.MessageListVisibility)

	// The created label is echoed as output.
	row, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Label_99", row["id"])
	require.Equal(t, "Travel", row["name"])
}

func TestCreateFull(t *testing.T) {
	svc := &fakeService{}
	runCmd(t, newLeafCmd(newCreateCmd, svc, "json"),
		"Travel",
		"--color-text", "#ffffff",
		"--color-bg", "#039be5",
		"--label-list-visibility", "labelHide",
		"--message-list-visibility", "hide",
	)

	require.NotNil(t, svc.created)
	require.Equal(t, &gmail.LabelColor{TextColor: "#ffffff", BackgroundColor: "#039be5"}, svc.created.Color)
	require.Equal(t, "labelHide", svc.created.LabelListVisibility)
	require.Equal(t, "hide", svc.created.MessageListVisibility)
}

func TestCreateInvalidVisibility(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"),
		"Travel", "--label-list-visibility", "bogus")

	require.Contains(t, err.Error(), "invalid --label-list-visibility")
	require.Nil(t, svc.created, "invalid input must not reach the API")
}

func TestCreateInvalidMessageVisibility(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newCreateCmd, svc, "json"),
		"Travel", "--message-list-visibility", "bogus")

	require.Contains(t, err.Error(), "invalid --message-list-visibility")
	require.Nil(t, svc.created)
}
