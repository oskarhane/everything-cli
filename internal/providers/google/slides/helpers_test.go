package slides

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeSlideService is the hermetic service.SlideService double: it serves
// seeded shapes, layouts, and placeholders and records every call for
// assertions.
type fakeSlideService struct {
	shapes       []service.SlideShape       // served by GetSlideText
	layouts      []service.SlideLayout      // served by ListSlideLayouts
	placeholders []service.SlidePlaceholder // served by ListSlidePlaceholders
	err          error                      // when set, every call fails
	getID        string                     // last GetSlideText presentation id
	replaceID    string                     // last ReplaceSlideText presentation id
	find         string                     // last ReplaceSlideText find text
	replaceWith  string                     // last ReplaceSlideText replacement text
	matchCase    bool                       // last ReplaceSlideText match-case flag

	layoutsID      string // last ListSlideLayouts presentation id
	placeholdersID string // last ListSlidePlaceholders presentation id

	createCalls  int    // CreateSlide invocation count
	createID     string // last CreateSlide presentation id
	createLayout string // last CreateSlide resolved layout object ID
	createIndex  *int64 // last CreateSlide insertion index (nil = appended)
	createResult string // CreateSlide reply object ID

	insertCalls   int    // InsertSlideText invocation count
	insertID      string // last InsertSlideText presentation id
	insertShapeID string // last InsertSlideText shape object ID
	insertText    string // last InsertSlideText text
}

func (f *fakeSlideService) GetSlideText(_ context.Context, id string) ([]service.SlideShape, error) {
	f.getID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.shapes, nil
}

func (f *fakeSlideService) ListSlideLayouts(_ context.Context, id string) ([]service.SlideLayout, error) {
	f.layoutsID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.layouts, nil
}

func (f *fakeSlideService) ListSlidePlaceholders(_ context.Context, id string) ([]service.SlidePlaceholder, error) {
	f.placeholdersID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.placeholders, nil
}

func (f *fakeSlideService) CreateSlide(_ context.Context, id, layoutID string, index *int64) (string, error) {
	f.createCalls++
	if f.err != nil {
		return "", f.err
	}
	f.createID, f.createLayout, f.createIndex = id, layoutID, index
	if f.createResult != "" {
		return f.createResult, nil
	}
	return "new_slide_id", nil
}

func (f *fakeSlideService) InsertSlideText(_ context.Context, id, shapeID, text string) error {
	f.insertCalls++
	if f.err != nil {
		return f.err
	}
	f.insertID, f.insertShapeID, f.insertText = id, shapeID, text
	return nil
}

func (f *fakeSlideService) ReplaceSlideText(_ context.Context, id, find, replaceWith string, matchCase bool) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.replaceID = id
	f.find, f.replaceWith, f.matchCase = find, replaceWith, matchCase
	return 3, nil
}

// newSlideLeafCmd builds a slide-service leaf against a fake, ready to
// execute hermetically with no network and no real account store.
func newSlideLeafCmd(build func(*app.Config, service.Dialer[service.SlideService]) *cobra.Command, svc *fakeSlideService, format string) *cobra.Command {
	return newSlideLeafCmdCfg(build, svc, cmdtest.NewTestConfig(format))
}

// newSlideLeafCmdCfg is newSlideLeafCmd with a caller-supplied config, so a
// test can seed files (--text-file) on the config's in-memory FS.
func newSlideLeafCmdCfg(build func(*app.Config, service.Dialer[service.SlideService]) *cobra.Command, svc *fakeSlideService, cfg *app.Config) *cobra.Command {
	return build(cfg, func(context.Context) (service.SlideService, error) { return svc, nil })
}

// newFileLeafCmd builds a FileService leaf against a fake.
func newFileLeafCmd(build func(*app.Config, service.Dialer[service.FileService]) *cobra.Command, svc *cmdtest.DeleteRecorder, format string) *cobra.Command {
	return build(cmdtest.NewTestConfig(format), func(context.Context) (service.FileService, error) { return svc, nil })
}

// seedShapes returns a small realistic shape set spanning two slides.
func seedShapes() []service.SlideShape {
	return []service.SlideShape{
		{SlideNumber: 1, ShapeID: "shape_title_1", Text: "Quarterly Review"},
		{SlideNumber: 1, ShapeID: "shape_body_1", Text: "Revenue is up 12% QoQ"},
		{SlideNumber: 2, ShapeID: "shape_title_2", Text: "Roadmap"},
		{SlideNumber: 2, ShapeID: "shape_bullets_2", Text: "Ship beta\nHire two engineers"},
	}
}

// seedLayouts returns a small realistic layout list: names and object IDs
// that differ, so a test can tell which one the resolver picked.
func seedLayouts() []service.SlideLayout {
	return []service.SlideLayout{
		{ObjectID: "layout_title_only", Name: "TITLE_ONLY"},
		{ObjectID: "layout_title_and_body", Name: "TITLE_AND_BODY"},
	}
}

// seedPlaceholders returns two placeholders on each of two slides, keyed by
// slide position and object ID, so set-text can be driven either way.
func seedPlaceholders() []service.SlidePlaceholder {
	return []service.SlidePlaceholder{
		{SlideNumber: 1, SlideID: "slide_1", PlaceholderIndex: 0, ShapeID: "shape_title_1"},
		{SlideNumber: 1, SlideID: "slide_1", PlaceholderIndex: 1, ShapeID: "shape_body_1"},
		{SlideNumber: 2, SlideID: "slide_2", PlaceholderIndex: 0, ShapeID: "shape_title_2"},
		{SlideNumber: 2, SlideID: "slide_2", PlaceholderIndex: 1, ShapeID: "shape_body_2"},
	}
}
