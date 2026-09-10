package service

import (
	"net/http"
	"testing"

	drive "google.golang.org/api/drive/v3"
)

// TestListComments covers comments.list over the fake drive endpoint: a
// single-page listing returns every comment, and a multi-page listing
// follows NextPageToken until it runs out. The response's fields projection
// must keep nextPageToken, or pageAll would stop after the first page.
func TestListComments(t *testing.T) {
	tests := []struct {
		name       string
		pages      []*drive.CommentList
		wantIDs    []string
		wantTokens []string // pageToken query values, one per request
	}{
		{
			name: "single page",
			pages: []*drive.CommentList{{
				Comments: []*drive.Comment{
					{Id: "c1", Content: "first"},
					{Id: "c2", Content: "second"},
				},
			}},
			wantIDs:    []string{"c1", "c2"},
			wantTokens: []string{""},
		},
		{
			name: "follows page tokens across pages",
			pages: []*drive.CommentList{
				{Comments: []*drive.Comment{{Id: "c1"}}, NextPageToken: "p2"},
				{Comments: []*drive.Comment{{Id: "c2"}}, NextPageToken: "p3"},
				{Comments: []*drive.Comment{{Id: "c3"}}},
			},
			wantIDs:    []string{"c1", "c2", "c3"},
			wantTokens: []string{"", "p2", "p3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tokens []string
			svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/files/doc-1/comments" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				if r.URL.Query().Get("fields") == "" {
					t.Error("comments.list sent no fields projection")
				}
				tokens = append(tokens, r.URL.Query().Get("pageToken"))
				page := &drive.CommentList{}
				if len(tokens) <= len(tt.pages) {
					page = tt.pages[len(tokens)-1]
				}
				writeJSON(w, page)
			})

			got, err := svc.ListComments(t.Context(), "doc-1")
			if err != nil {
				t.Fatalf("ListComments: %v", err)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("comments = %d, want %d", len(got), len(tt.wantIDs))
			}
			for i, c := range got {
				if c.Id != tt.wantIDs[i] {
					t.Errorf("comment[%d].id = %q, want %q", i, c.Id, tt.wantIDs[i])
				}
			}
			if len(tokens) != len(tt.wantTokens) {
				t.Fatalf("list requests = %d, want %d", len(tokens), len(tt.wantTokens))
			}
			for i, tok := range tokens {
				if tok != tt.wantTokens[i] {
					t.Errorf("request[%d].pageToken = %q, want %q", i, tok, tt.wantTokens[i])
				}
			}
		})
	}
}

// TestListCommentsError guards the error path: an API failure must surface
// wrapped, naming the file.
func TestListCommentsError(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": {"code": 403, "message": "no access"}}`, http.StatusForbidden)
	})

	if _, err := svc.ListComments(t.Context(), "doc-1"); err == nil {
		t.Fatal("ListComments: want error on API failure, got nil")
	}
}

// TestCreateComment drives comments.create: the POSTed body must carry
// exactly the caller's content and no anchor, and the created comment must
// come back.
func TestCreateComment(t *testing.T) {
	var body *drive.Comment
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/files/doc-1/comments" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		body = &drive.Comment{}
		decodeInto(t, r, body)
		writeJSON(w, &drive.Comment{Id: "c9", Content: "hello"})
	})

	created, err := svc.CreateComment(t.Context(), "doc-1", "hello")
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if body == nil {
		t.Fatal("no comments.create request was sent")
	}
	if body.Content != "hello" {
		t.Errorf("sent content = %q, want %q", body.Content, "hello")
	}
	if body.Anchor != "" {
		t.Errorf("sent anchor = %q, want empty (no anchor on whole-file comments)", body.Anchor)
	}
	if created == nil || created.Id != "c9" {
		t.Fatalf("created comment = %+v, want id c9", created)
	}
}

// TestCreateCommentError guards the error path: an API failure must surface
// wrapped, naming the file.
func TestCreateCommentError(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": {"code": 403, "message": "no access"}}`, http.StatusForbidden)
	})

	if _, err := svc.CreateComment(t.Context(), "doc-1", "hello"); err == nil {
		t.Fatal("CreateComment: want error on API failure, got nil")
	}
}

// TestCreateReply drives replies.create: a plain reply must send content
// and no action, while an action reply ("resolve"/"reopen") must send the
// action and no content — the API requires content only when no action is
// given.
func TestCreateReply(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		action      string
		wantContent string
		wantAction  string
	}{
		{
			name:        "content reply sends content and no action",
			content:     "a reply",
			wantContent: "a reply",
			wantAction:  "",
		},
		{
			name:       "resolve reply sends action and no content",
			action:     "resolve",
			wantAction: "resolve",
		},
		{
			name:       "reopen reply sends action and no content",
			action:     "reopen",
			wantAction: "reopen",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *drive.Reply
			svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/files/doc-1/comments/c1/replies" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				body = &drive.Reply{}
				decodeInto(t, r, body)
				writeJSON(w, &drive.Reply{Id: "r1", Action: tt.wantAction})
			})

			created, err := svc.CreateReply(t.Context(), "doc-1", "c1", tt.content, tt.action)
			if err != nil {
				t.Fatalf("CreateReply: %v", err)
			}
			if body == nil {
				t.Fatal("no replies.create request was sent")
			}
			if body.Content != tt.wantContent {
				t.Errorf("sent content = %q, want %q", body.Content, tt.wantContent)
			}
			if body.Action != tt.wantAction {
				t.Errorf("sent action = %q, want %q", body.Action, tt.wantAction)
			}
			if created == nil || created.Id != "r1" {
				t.Fatalf("created reply = %+v, want id r1", created)
			}
		})
	}
}

// TestCreateReplyError guards the error path: an API failure must surface
// wrapped, naming the comment and the file.
func TestCreateReplyError(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": {"code": 500, "message": "boom"}}`, http.StatusInternalServerError)
	})

	if _, err := svc.CreateReply(t.Context(), "doc-1", "c1", "a reply", ""); err == nil {
		t.Fatal("CreateReply: want error on API failure, got nil")
	}
}

// TestDeleteComment drives comments.delete: the DELETE must land on the
// comment's path.
func TestDeleteComment(t *testing.T) {
	var deletedPath bool
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/files/doc-1/comments/c1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		deletedPath = true
		w.WriteHeader(http.StatusNoContent)
	})

	if err := svc.DeleteComment(t.Context(), "doc-1", "c1"); err != nil {
		t.Fatalf("DeleteComment: %v", err)
	}
	if !deletedPath {
		t.Error("no comments.delete request was sent")
	}
}

// TestDeleteCommentError guards the error path: an API failure must surface
// wrapped, naming the comment and the file.
func TestDeleteCommentError(t *testing.T) {
	svc := newDocsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": {"code": 404, "message": "no comment"}}`, http.StatusNotFound)
	})

	if err := svc.DeleteComment(t.Context(), "doc-1", "c1"); err == nil {
		t.Fatal("DeleteComment: want error on API failure, got nil")
	}
}
