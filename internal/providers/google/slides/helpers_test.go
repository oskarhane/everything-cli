package slides

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/output"
	"github.com/oskarhane/google-cli/internal/providers/google/drive/service"
	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

// TestMain neutralizes format auto-detection so the host's harness env and
// TTY cannot flip output expectations.
func TestMain(m *testing.M) {
	output.IsAgent = func() bool { return false }
	output.StdoutIsTerminal = func() bool { return false }
	os.Exit(m.Run())
}

// fakeSlideService is the hermetic service.SlideService double: it serves
// seeded shapes and records every call for assertions.
type fakeSlideService struct {
	shapes      []service.SlideShape // served by GetSlideText
	err         error                // when set, every call fails
	getID       string               // last GetSlideText presentation id
	replaceID   string               // last ReplaceSlideText presentation id
	find        string               // last ReplaceSlideText find text
	replaceWith string               // last ReplaceSlideText replacement text
	matchCase   bool                 // last ReplaceSlideText match-case flag
}

func (f *fakeSlideService) GetSlideText(_ context.Context, id string) ([]service.SlideShape, error) {
	f.getID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.shapes, nil
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
	return build(cmdtest.NewTestConfig(format), func(context.Context) (service.SlideService, error) { return svc, nil })
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
