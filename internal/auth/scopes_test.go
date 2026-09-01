package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/config"
)

// TestScopes pins the exact scope sets granted per service.
func TestScopes(t *testing.T) {
	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/gmail.send",
		"https://www.googleapis.com/auth/gmail.compose",
	}, ScopesGmail)

	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/calendar",
	}, ScopesCalendar)

	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/drive",
	}, ScopesDrive)

	assert.Equal(t, "https://www.googleapis.com/auth/drive.file", ScopeDriveFile)

	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/drive",
		ScopeDriveFile,
	}, ScopesDriveDial)

	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/documents",
	}, ScopesDocs)

	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/spreadsheets",
	}, ScopesSheets)

	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/presentations",
	}, ScopesSlides)

	assert.Equal(t, "https://www.googleapis.com/auth/userinfo.email", ScopeUserEmail)
}

// TestRequireScopes pins the scope guard: full-grant accounts pass, narrowed
// grants fail with the re-consent action naming the account and every
// missing scope.
func TestRequireScopes(t *testing.T) {
	fullGrants := []string{ScopeUserEmail, ScopesGmail[0], ScopesCalendar[0], ScopesDrive[0]}

	tests := []struct {
		name     string
		acct     *config.Account
		required []string
		wantErr  string
	}{
		{
			name:     "account with the family scope passes",
			acct:     &config.Account{Name: "work", Scopes: fullGrants},
			required: ScopesDrive,
		},
		{
			name: "default full grant passes every family",
			acct: &config.Account{Name: "work", Scopes: append(append(append(append(append(append([]string{ScopeUserEmail},
				ScopesGmail...), ScopesCalendar...), ScopesDrive...), ScopesDocs...), ScopesSheets...), ScopesSlides...)},
			required: append(append(append(ScopesDrive, ScopesDocs...), ScopesSheets...), ScopesSlides...),
		},
		{
			name:     "narrowed grant names the missing scope and the re-consent action",
			acct:     &config.Account{Name: "work", Scopes: []string{ScopeUserEmail, ScopesGmail[0]}},
			required: ScopesDrive,
			wantErr:  `account "work" is missing scope https://www.googleapis.com/auth/drive: re-run "google-cli account add <name>" to consent (accounts added before Drive support need this once)`,
		},
		{
			name:     "every missing scope is listed",
			acct:     &config.Account{Name: "legacy", Scopes: nil},
			required: []string{ScopesDocs[0], ScopesSheets[0]},
			wantErr:  `account "legacy" is missing scopes https://www.googleapis.com/auth/documents, https://www.googleapis.com/auth/spreadsheets: re-run "google-cli account add <name>" to consent (accounts added before Drive support need this once)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireScopes(tc.acct, tc.required)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
			assert.Contains(t, err.Error(), "account add", "error must name the re-consent action")
			for _, s := range tc.required {
				assert.Contains(t, err.Error(), s, "error must name missing scope %s", s)
			}
		})
	}
}

// TestRequireAnyScopes pins the alternatives guard: an account holding any
// one of the required scopes passes, an account holding none fails with the
// re-consent action naming every acceptable alternative.
func TestRequireAnyScopes(t *testing.T) {
	tests := []struct {
		name     string
		acct     *config.Account
		required []string
		wantErr  string
	}{
		{
			name:     "full drive grant passes the drive dial set",
			acct:     &config.Account{Name: "work", Scopes: []string{ScopeUserEmail, ScopesDrive[0]}},
			required: ScopesDriveDial,
		},
		{
			name:     "minimal drive.file-only grant passes the drive dial set",
			acct:     &config.Account{Name: "work", Scopes: []string{ScopeUserEmail, ScopeDriveFile}},
			required: ScopesDriveDial,
		},
		{
			name:     "grant outside the set fails naming both alternatives",
			acct:     &config.Account{Name: "work", Scopes: []string{ScopeUserEmail, ScopesGmail[0]}},
			required: ScopesDriveDial,
			wantErr:  `account "work" is missing scope https://www.googleapis.com/auth/drive or https://www.googleapis.com/auth/drive.file: re-run "google-cli account add <name>" to consent (accounts added before Drive support need this once)`,
		},
		{
			name:     "no account errors before any alternative check",
			acct:     nil,
			required: ScopesDriveDial,
			wantErr:  `no account: run "google-cli account add <name>" first`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireAnyScopes(tc.acct, tc.required)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
			assert.Contains(t, err.Error(), "account add", "error must name the re-consent action")
			if tc.acct == nil {
				return
			}
			for _, s := range tc.required {
				assert.Contains(t, err.Error(), s, "error must name alternative scope %s", s)
			}
		})
	}
}
