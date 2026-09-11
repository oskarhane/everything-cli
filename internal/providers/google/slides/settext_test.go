package slides

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestSetTextFlagHelpDescribesInsertion is acceptance 1: --text must not claim
// to replace the placeholder's content. InsertSlideText issues an
// InsertTextRequest at index 0, so the help says the text is inserted at the
// start of the existing text.
func TestSetTextFlagHelpDescribesInsertion(t *testing.T) {
	cmd := newSetTextCmd(cmdtest.NewTestConfig("json"), nil)
	flag := cmd.Flags().Lookup("text")
	require.NotNil(t, flag)

	usage := strings.ToLower(flag.Usage)
	require.NotContains(t, usage, "replace")
	require.Contains(t, usage, "insert")
	require.Contains(t, usage, "start of the placeholder's existing text")
	require.Contains(t, usage, "index 0")
}

// TestSetTextResolvesPlaceholder is acceptance 2: `slides set-text pres
// --slide 2 --placeholder-idx 0 --text Hello` makes exactly one InsertText
// call against the shape object ID that is placeholder index 0 on the fake's
// second slide.
func TestSetTextResolvesPlaceholder(t *testing.T) {
	svc := &fakeSlideService{placeholders: seedPlaceholders()}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newSetTextCmd, svc, "json"),
		"pres", "--slide", "2", "--placeholder-idx", "0", "--text", "Hello")

	require.Equal(t, 1, svc.insertCalls, "exactly one batchUpdate")
	require.Equal(t, "pres", svc.insertID)
	require.Equal(t, "shape_title_2", svc.insertShapeID, "placeholder 0 on slide 2")
	require.Equal(t, "Hello", svc.insertText)
	require.Contains(t, out, "shape_title_2")
}

func TestSetTextBySlideObjectID(t *testing.T) {
	svc := &fakeSlideService{placeholders: seedPlaceholders()}
	cmdtest.RunCmd(t, newSlideLeafCmd(newSetTextCmd, svc, "json"),
		"pres", "--slide", "slide_2", "--placeholder-idx", "1", "--text", "Body")

	require.Equal(t, "shape_body_2", svc.insertShapeID)
}

func TestSetTextTextFile(t *testing.T) {
	cfg := cmdtest.NewTestConfig("json")
	require.NoError(t, afero.WriteFile(cfg.Fs, "body.txt", []byte("Hello from file"), 0o644))
	svc := &fakeSlideService{placeholders: seedPlaceholders()}

	cmdtest.RunCmd(t, newSlideLeafCmdCfg(newSetTextCmd, svc, cfg),
		"pres", "--slide", "2", "--placeholder-idx", "0", "--text-file", "body.txt")

	require.Equal(t, "Hello from file", svc.insertText)
}

// TestSetTextOutOfRangeSlide is acceptance 4: an out-of-range slide number
// fails naming the offending value, before any batchUpdate is issued.
func TestSetTextOutOfRangeSlide(t *testing.T) {
	svc := &fakeSlideService{placeholders: seedPlaceholders()}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newSetTextCmd, svc, "json"),
		"pres", "--slide", "9", "--placeholder-idx", "0", "--text", "Hello")

	require.Contains(t, err.Error(), "9")
	require.Equal(t, 0, svc.insertCalls, "no batchUpdate when the slide is unknown")
}

// TestSetTextOutOfRangePlaceholder is acceptance 4's other half: a
// placeholder index that no shape on the slide has fails naming it.
func TestSetTextOutOfRangePlaceholder(t *testing.T) {
	svc := &fakeSlideService{placeholders: seedPlaceholders()}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newSetTextCmd, svc, "json"),
		"pres", "--slide", "2", "--placeholder-idx", "7", "--text", "Hello")

	require.Contains(t, err.Error(), "7")
	require.Equal(t, 0, svc.insertCalls)
}

func TestSetTextUnknownSlideObjectID(t *testing.T) {
	svc := &fakeSlideService{placeholders: seedPlaceholders()}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newSetTextCmd, svc, "json"),
		"pres", "--slide", "nope", "--placeholder-idx", "0", "--text", "Hello")

	require.Contains(t, err.Error(), `"nope"`)
	require.Equal(t, 0, svc.insertCalls)
}

func TestSetTextMutuallyExclusiveSources(t *testing.T) {
	svc := &fakeSlideService{placeholders: seedPlaceholders()}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newSetTextCmd, svc, "json"),
		"pres", "--slide", "2", "--placeholder-idx", "0", "--text", "Hello", "--text-file", "body.txt")

	require.Contains(t, err.Error(), "--text and --text-file are mutually exclusive")
	require.Equal(t, 0, svc.insertCalls)
}

func TestSetTextSourceRequired(t *testing.T) {
	svc := &fakeSlideService{placeholders: seedPlaceholders()}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newSetTextCmd, svc, "json"),
		"pres", "--slide", "2", "--placeholder-idx", "0")

	require.Contains(t, err.Error(), "--text or --text-file is required")
}

func TestSetTextSlideRequired(t *testing.T) {
	svc := &fakeSlideService{placeholders: seedPlaceholders()}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newSetTextCmd, svc, "json"),
		"pres", "--placeholder-idx", "0", "--text", "Hello")

	require.Contains(t, err.Error(), "--slide is required")
}

func TestSetTextPlaceholderIdxRequired(t *testing.T) {
	svc := &fakeSlideService{placeholders: seedPlaceholders()}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newSetTextCmd, svc, "json"),
		"pres", "--slide", "2", "--text", "Hello")

	require.Contains(t, err.Error(), "--placeholder-idx is required")
}

func TestSetTextPropagatesAPIError(t *testing.T) {
	svc := &fakeSlideService{err: errors.New("googleapi: Error 403")}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newSetTextCmd, svc, "json"),
		"pres", "--slide", "2", "--placeholder-idx", "0", "--text", "Hello")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}
