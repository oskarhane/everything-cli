package comment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestDialRequiresDriveScope pins the scope guard: the Drive comments
// endpoints reject the documents/presentations scopes, so an account without
// a drive grant must fail with the re-consent guidance before any service is
// built or API call made, instead of surfacing a raw 403 from Google.
func TestDialRequiresDriveScope(t *testing.T) {
	tests := []struct {
		name    string
		scopes  []string
		missing bool
	}{
		{
			name:    "account with the full drive scope dials",
			scopes:  []string{auth.ScopeUserEmail, auth.ScopesDrive[0]},
			missing: false,
		},
		{
			name:    "account with the minimal drive.file scope dials",
			scopes:  []string{auth.ScopeUserEmail, auth.ScopeDriveFile},
			missing: false,
		},
		{
			name:    "account with only the docs scope is told to re-consent",
			scopes:  []string{auth.ScopeUserEmail, auth.ScopesDocs[0]},
			missing: true,
		},
		{
			name:    "account with only the slides scope is told to re-consent",
			scopes:  []string{auth.ScopeUserEmail, auth.ScopesSlides[0]},
			missing: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := dial(context.Background(), cmdtest.NewDialConfig(t, "work", tc.scopes))
			if !tc.missing {
				require.NoError(t, err)
				require.NotNil(t, svc)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), `account "work"`)
			assert.Contains(t, err.Error(), "account auth", "error must name the re-consent action")
			assert.Contains(t, err.Error(), auth.ScopesDrive[0], "error must name the full drive scope alternative")
		})
	}
}
