package service

import (
	"context"
	"encoding/json"
	"fmt"
)

// Viewer is the authenticated account's own user record, as decoded from the
// GraphQL API. The JSON tags are the wire shape; leaves map it to
// snake_case views.
type Viewer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ViewerService is the viewer surface the `linear account` subtree consumes
// post-auth. oauth.go has its own pre-auth queryViewer on a bare access
// token; this one goes through the account client so it shares exec's
// timeout, error, and pagination discipline.
type ViewerService interface {
	GetViewer(ctx context.Context) (Viewer, error)
}

// Compile-time proof that Service satisfies the viewer surface. It lives
// here rather than in service.go's assertion block because each concern
// file owns its own seam.
var _ ViewerService = (*Service)(nil)

// GetViewer returns the account the current API key authenticates as. An
// empty email is treated as no viewer — Linear returns a null viewer for an
// invalid or revoked key, and reporting "some viewer with no identity" would
// be worse than a clear failure.
func (s *Service) GetViewer(ctx context.Context) (Viewer, error) {
	const query = `query { viewer { id name email } }`
	data, err := s.exec(ctx, query, nil)
	if err != nil {
		return Viewer{}, err
	}
	raw, err := dig(data, "viewer")
	if err != nil {
		return Viewer{}, err
	}
	if string(raw) == "null" {
		return Viewer{}, fmt.Errorf("linear API reported no viewer")
	}
	var viewer Viewer
	if err := json.Unmarshal(raw, &viewer); err != nil {
		return Viewer{}, fmt.Errorf("decoding viewer: %w", err)
	}
	if viewer.Email == "" {
		return Viewer{}, fmt.Errorf("linear API reported no viewer")
	}
	return viewer, nil
}
