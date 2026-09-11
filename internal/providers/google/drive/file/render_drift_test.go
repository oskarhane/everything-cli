package file

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/providers/google/drive/service"
)

// renderedToWire maps every rendered key (fileListFields/fileViewFields, the
// snake_case output of fileRow/fileView) to the Drive wire field it reads.
// Encoded once here — the drift guard — so a rendered key added without
// extending service.FileFields fails this test instead of silently rendering
// empty.
var renderedToWire = map[string]string{
	"id":            "id",
	"name":          "name",
	"mime_type":     "mimeType",
	"size":          "size",
	"owner":         "owners",
	"parent_ids":    "parents",
	"trashed":       "trashed",
	"shared":        "shared",
	"modified_time": "modifiedTime",
	"web_link":      "webViewLink",
	"description":   "description",
}

// TestRenderProjectionDrift pins the render-to-projection mapping: every key
// rendered by fileRow/fileView must map to a wire field present in the
// service's Fields projection, and the projection must carry no field that
// nothing renders.
func TestRenderProjectionDrift(t *testing.T) {
	projected := map[string]bool{}
	for _, f := range strings.Split(service.FileFields, ",") {
		projected[f] = true
	}

	rendered := append(append([]string{}, fileListFields...), fileViewFields...)
	for _, key := range rendered {
		wire, ok := renderedToWire[key]
		require.True(t, ok, "rendered key %q has no wire-field mapping in renderedToWire", key)
		require.True(t, projected[wire], "rendered key %q maps to wire field %q, which is missing from service.FileFields", key, wire)
	}

	for wire := range projected {
		used := false
		for _, mapped := range renderedToWire {
			if mapped == wire {
				used = true
				break
			}
		}
		require.True(t, used, "service.FileFields projects wire field %q that no rendered key maps to", wire)
	}
}
