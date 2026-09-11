package slides

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestAddResolvesLayoutNameAndPrintsObjectID is acceptance 1: `slides add
// pres --layout TITLE_ONLY` makes exactly one CreateSlide call carrying the
// layout object ID resolved from the fake's layout list (the single
// batchUpdate the service issues), and the reply's object ID is printed.
func TestAddResolvesLayoutNameAndPrintsObjectID(t *testing.T) {
	svc := &fakeSlideService{layouts: seedLayouts(), createResult: "new_slide_id"}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newAddCmd, svc, "json"), "pres", "--layout", "TITLE_ONLY")

	require.Equal(t, 1, svc.createCalls, "exactly one batchUpdate")
	require.Equal(t, "pres", svc.createID)
	require.Equal(t, "layout_title_only", svc.createLayout, "the object ID resolved from the layout list, not the name")
	require.Nil(t, svc.createIndex, "no --index means the slide is appended")

	row, ok := cmdtest.DecodeJSON(t, out).(map[string]any)
	require.True(t, ok, "expected a JSON object, got: %s", out)
	require.Equal(t, "new_slide_id", row["slide_id"])
}

func TestAddAcceptsLayoutObjectID(t *testing.T) {
	svc := &fakeSlideService{layouts: seedLayouts()}
	cmdtest.RunCmd(t, newSlideLeafCmd(newAddCmd, svc, "json"), "pres", "--layout", "layout_title_and_body")

	require.Equal(t, "layout_title_and_body", svc.createLayout)
}

func TestAddLayoutNameIsCaseInsensitive(t *testing.T) {
	svc := &fakeSlideService{layouts: seedLayouts()}
	cmdtest.RunCmd(t, newSlideLeafCmd(newAddCmd, svc, "json"), "pres", "--layout", "title_only")

	require.Equal(t, "layout_title_only", svc.createLayout)
}

func TestAddForwardsInsertionIndex(t *testing.T) {
	svc := &fakeSlideService{layouts: seedLayouts()}
	cmdtest.RunCmd(t, newSlideLeafCmd(newAddCmd, svc, "json"), "pres", "--layout", "TITLE_ONLY", "--index", "2")

	require.NotNil(t, svc.createIndex)
	require.EqualValues(t, 2, *svc.createIndex)
}

// TestAddIndexZeroIsForwarded guards the pointer seam: --index 0 is a valid
// position (the front) and must reach the service, unlike an omitted flag.
func TestAddIndexZeroIsForwarded(t *testing.T) {
	svc := &fakeSlideService{layouts: seedLayouts()}
	cmdtest.RunCmd(t, newSlideLeafCmd(newAddCmd, svc, "json"), "pres", "--layout", "TITLE_ONLY", "--index", "0")

	require.NotNil(t, svc.createIndex)
	require.EqualValues(t, 0, *svc.createIndex)
}

// TestAddUnknownLayoutRefused is acceptance 4: an unknown layout key fails
// naming the offending value, before any batchUpdate is issued.
func TestAddUnknownLayoutRefused(t *testing.T) {
	svc := &fakeSlideService{layouts: seedLayouts()}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newAddCmd, svc, "json"), "pres", "--layout", "NOPE")

	require.Contains(t, err.Error(), `"NOPE"`)
	require.Equal(t, 0, svc.createCalls, "no batchUpdate on an unresolved layout")
}

func TestAddLayoutRequired(t *testing.T) {
	svc := &fakeSlideService{layouts: seedLayouts()}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newAddCmd, svc, "json"), "pres")

	require.Contains(t, err.Error(), "--layout is required")
}

func TestAddPropagatesAPIError(t *testing.T) {
	svc := &fakeSlideService{err: errors.New("googleapi: Error 403")}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newAddCmd, svc, "json"), "pres", "--layout", "TITLE_ONLY")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}
