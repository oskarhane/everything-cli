package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/config"
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

// TestParseScopes pins the canonical --scopes parsing shared by every
// provider leaf: comma-split, trimmed, blanks dropped, empty (or
// blank-only) input yielding nil / no entries.
func TestParseScopes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: nil},
		{name: "blank only", in: " , , ", want: []string{}},
		{name: "trims and drops blanks", in: " read,write , ,issues:create", want: []string{"read", "write", "issues:create"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ParseScopes(tc.in))
		})
	}
}

// TestMissingScopes pins the shared set-comparison helper: it returns only
// required entries absent from the account's grants, in required's order.
func TestMissingScopes(t *testing.T) {
	acct := &config.Account{Name: "work", Scopes: []string{ScopeUserEmail, ScopesDrive[0]}}

	assert.Nil(t, missingScopes(acct, []string{ScopesDrive[0], ScopeUserEmail}), "all granted → no missing")
	assert.Equal(t,
		[]string{ScopesDocs[0], ScopesSheets[0]},
		missingScopes(acct, []string{ScopesDocs[0], ScopesDrive[0], ScopesSheets[0]}),
		"granted entry skipped, missing listed in required order")
	assert.Nil(t, missingScopes(acct, nil), "empty required → no missing")
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
			wantErr:  `account "work" is missing scope https://www.googleapis.com/auth/drive: re-run "everything-cli google account auth <name>" to consent (accounts added before Drive support need this once)`,
		},
		{
			name:     "every missing scope is listed",
			acct:     &config.Account{Name: "legacy", Scopes: nil},
			required: []string{ScopesDocs[0], ScopesSheets[0]},
			wantErr:  `account "legacy" is missing scopes https://www.googleapis.com/auth/documents, https://www.googleapis.com/auth/spreadsheets: re-run "everything-cli google account auth <name>" to consent (accounts added before Drive support need this once)`,
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
			assert.Contains(t, err.Error(), "account auth", "error must name the re-consent action")
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
			wantErr:  `account "work" is missing scope https://www.googleapis.com/auth/drive or https://www.googleapis.com/auth/drive.file: re-run "everything-cli google account auth <name>" to consent (accounts added before Drive support need this once)`,
		},
		{
			name:     "no account errors before any alternative check",
			acct:     nil,
			required: ScopesDriveDial,
			wantErr:  `no account: run "everything-cli google account add <name>" first`,
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
			if tc.acct == nil {
				assert.Contains(t, err.Error(), "account add", "no-account error must name the account-creation action")
				return
			}
			assert.Contains(t, err.Error(), "account auth", "error must name the re-consent action")
			for _, s := range tc.required {
				assert.Contains(t, err.Error(), s, "error must name alternative scope %s", s)
			}
		})
	}
}
