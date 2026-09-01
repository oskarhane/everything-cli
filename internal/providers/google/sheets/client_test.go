package sheets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/subcommands/cmdtest"
)

// TestDialRequiresSheetsScope pins the scope guard: an account narrowed to
// non-sheets scopes must fail with the re-consent guidance before any service
// is built or API call made, instead of surfacing a raw 403 from Google.
func TestDialRequiresSheetsScope(t *testing.T) {
	tests := []struct {
		name    string
		scopes  []string
		missing bool
	}{
		{
			name:    "account with the sheets scope dials",
			scopes:  []string{auth.ScopeUserEmail, auth.ScopesSheets[0]},
			missing: false,
		},
		{
			name:    "account without the sheets scope is told to re-consent",
			scopes:  []string{auth.ScopeUserEmail, auth.ScopesGmail[0]},
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
			assert.Contains(t, err.Error(), "account add", "error must name the re-consent action")
			assert.Contains(t, err.Error(), auth.ScopesSheets[0], "error must name the missing scope")
		})
	}
}
