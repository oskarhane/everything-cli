package service

import (
	"net/http"
	"strings"
	"testing"

	drive "google.golang.org/api/drive/v3"
)

// testFileFields enumerates the Drive field names backing every field
// file/render.go renders (fileRow plus the fileView description). The list and
// get projections must carry all of them, plus trashed, which the API default
// omits.
var testFileFields = []string{
	"id", "name", "mimeType", "size", "owners", "parents",
	"trashed", "shared", "modifiedTime", "webViewLink", "description",
}

// TestListFilesPinsFields drives ListFiles over a two-page fake files.list and
// asserts every page's request pins the envelope plus the rendered field set.
// A projection without nextPageToken would truncate to page one; one without
// trashed would always report trashed=false.
func TestListFilesPinsFields(t *testing.T) {
	var sawFields []string
	svc := newPagedTestServer(t, map[string]http.HandlerFunc{
		"/files": func(w http.ResponseWriter, r *http.Request) {
			sawFields = append(sawFields, r.URL.Query().Get("fields"))
			switch r.URL.Query().Get("pageToken") {
			case "":
				writeJSON(w, drive.FileList{
					Files:         []*drive.File{{Id: "f-1", Trashed: true}},
					NextPageToken: "tok-2",
				})
			case "tok-2":
				writeJSON(w, drive.FileList{
					Files:         []*drive.File{{Id: "f-2"}},
					NextPageToken: "",
				})
			default:
				http.Error(w, "unexpected pageToken", http.StatusBadRequest)
			}
		},
	})

	files, err := svc.ListFiles(t.Context(), "", 0)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}
	if len(sawFields) != 2 {
		t.Fatalf("list requests = %d, want 2", len(sawFields))
	}
	for i, fields := range sawFields {
		if fields != fileListFields {
			t.Errorf("page %d fields = %q, want %q", i+1, fields, fileListFields)
		}
		for _, want := range append([]string{"nextPageToken"}, testFileFields...) {
			if !strings.Contains(fields, want) {
				t.Errorf("page %d fields = %q, want it to contain %s", i+1, fields, want)
			}
		}
	}
}

// TestGetFilePinsFields drives GetFile over a fake files.get and asserts the
// request pins the rendered field set including trashed, which the API default
// omits.
func TestGetFilePinsFields(t *testing.T) {
	var sawFields string
	svc := newPagedTestServer(t, map[string]http.HandlerFunc{
		"/files/f-1": func(w http.ResponseWriter, r *http.Request) {
			sawFields = r.URL.Query().Get("fields")
			writeJSON(w, &drive.File{Id: "f-1", Trashed: true})
		},
	})

	file, err := svc.GetFile(t.Context(), "f-1")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if file == nil || file.Id != "f-1" {
		t.Fatalf("file = %+v, want id f-1", file)
	}
	if sawFields != fileFields {
		t.Errorf("fields = %q, want %q", sawFields, fileFields)
	}
	for _, want := range testFileFields {
		if !strings.Contains(sawFields, want) {
			t.Errorf("fields = %q, want it to contain %s", sawFields, want)
		}
	}
}
