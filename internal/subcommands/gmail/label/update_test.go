package label

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdatePartialRename(t *testing.T) {
	svc := &fakeService{}
	out := runCmd(t, newLeafCmd(newUpdateCmd, svc, "json"), "Label_7", "--name", "Travel 2026")

	require.Equal(t, "Label_7", svc.updatedID)
	require.NotNil(t, svc.updated)
	require.Equal(t, "Travel 2026", svc.updated.Name)
	// Partial update: nothing else is sent.
	require.Nil(t, svc.updated.Color)
	require.Empty(t, svc.updated.LabelListVisibility)
	require.Empty(t, svc.updated.MessageListVisibility)

	row, ok := decodeJSON(t, out).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Travel 2026", row["name"])
}

func TestUpdateColorOnly(t *testing.T) {
	svc := &fakeService{}
	runCmd(t, newLeafCmd(newUpdateCmd, svc, "json"),
		"Label_7",
		"--color-text", "#ffffff", "--color-bg", "#039be5",
	)

	require.Equal(t, "Label_7", svc.updatedID)
	require.NotNil(t, svc.updated.Color)
	require.Equal(t, "#ffffff", svc.updated.Color.TextColor)
	require.Equal(t, "#039be5", svc.updated.Color.BackgroundColor)
	require.Empty(t, svc.updated.Name, "color-only update must not rename")
}

func TestUpdateVisibilities(t *testing.T) {
	svc := &fakeService{}
	runCmd(t, newLeafCmd(newUpdateCmd, svc, "json"),
		"Label_7",
		"--label-list-visibility", "labelHide",
		"--message-list-visibility", "hide",
	)

	require.Equal(t, "labelHide", svc.updated.LabelListVisibility)
	require.Equal(t, "hide", svc.updated.MessageListVisibility)
}

func TestUpdateNothing(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"), "Label_7")

	require.Contains(t, err.Error(), "nothing to update")
	require.Nil(t, svc.updated, "empty update must not reach the API")
}

func TestUpdateInvalidVisibility(t *testing.T) {
	svc := &fakeService{}
	_, err := runCmdErr(t, newLeafCmd(newUpdateCmd, svc, "json"),
		"Label_7", "--label-list-visibility", "bogus")

	require.Contains(t, err.Error(), "invalid --label-list-visibility")
	require.Nil(t, svc.updated)
}
