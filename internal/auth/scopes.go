package auth

import (
	"fmt"
	"strings"

	"github.com/oskarhane/google-cli/internal/config"
)

// ScopeUserEmail is requested on every flow so the account's email can be
// resolved from the userinfo endpoint after authorization.
const ScopeUserEmail = "https://www.googleapis.com/auth/userinfo.email"

// ScopesGmail grants full Gmail access: modify, send and compose.
var ScopesGmail = []string{
	"https://www.googleapis.com/auth/gmail.modify",
	"https://www.googleapis.com/auth/gmail.send",
	"https://www.googleapis.com/auth/gmail.compose",
}

// ScopesCalendar grants full Google Calendar access.
var ScopesCalendar = []string{
	"https://www.googleapis.com/auth/calendar",
}

// ScopesDrive grants full Google Drive access: files and sharing.
var ScopesDrive = []string{
	"https://www.googleapis.com/auth/drive",
}

// ScopeDriveFile grants access only to files this app created or opened: the
// minimal Drive profile. It cannot reach account-wide files, so sharing
// leaves still demand the full ScopesDrive grant.
const ScopeDriveFile = "https://www.googleapis.com/auth/drive.file"

// ScopesDriveDial are the scope alternatives a drive service dial accepts:
// the full drive grant, or the minimal drive.file profile for accounts that
// opt out of account-wide sharing power.
var ScopesDriveDial = []string{ScopesDrive[0], ScopeDriveFile}

// ScopesDocs grants full Google Docs access.
var ScopesDocs = []string{
	"https://www.googleapis.com/auth/documents",
}

// ScopesSheets grants full Google Sheets access.
var ScopesSheets = []string{
	"https://www.googleapis.com/auth/spreadsheets",
}

// ScopesSlides grants full Google Slides access.
var ScopesSlides = []string{
	"https://www.googleapis.com/auth/presentations",
}

// RequireScopes fails fast when the account lacks a scope a command needs,
// before any service is built or API call made: without it, accounts consented
// before Drive support only surface raw 403s from Google. It errors with a
// re-consent action listing every missing scope. Accounts added via account
// add without --scopes always pass; the guard only trips for narrowed grants.
func RequireScopes(acct *config.Account, required []string) error {
	if acct == nil {
		return fmt.Errorf("no account: run \"google-cli account add <name>\" first")
	}
	granted := make(map[string]bool, len(acct.Scopes))
	for _, s := range acct.Scopes {
		granted[s] = true
	}
	var missing []string
	for _, s := range required {
		if !granted[s] {
			missing = append(missing, s)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	label := "scope"
	if len(missing) > 1 {
		label = "scopes"
	}
	return fmt.Errorf("account %q is missing %s %s: re-run \"google-cli account add <name>\" to consent (accounts added before Drive support need this once)",
		acct.Name, label, strings.Join(missing, ", "))
}

// RequireAnyScopes is the alternatives-set twin of RequireScopes: it passes
// when the account holds ANY ONE of required, and fails only when it holds
// none, before any service is built or API call made. It errors with the same
// re-consent action, naming every acceptable alternative. Accounts added via
// account add without --scopes always pass; the guard only trips for narrowed
// grants.
func RequireAnyScopes(acct *config.Account, required []string) error {
	if acct == nil {
		return fmt.Errorf("no account: run \"google-cli account add <name>\" first")
	}
	granted := make(map[string]bool, len(acct.Scopes))
	for _, s := range acct.Scopes {
		granted[s] = true
	}
	for _, s := range required {
		if granted[s] {
			return nil
		}
	}
	return fmt.Errorf("account %q is missing scope %s: re-run \"google-cli account add <name>\" to consent (accounts added before Drive support need this once)",
		acct.Name, strings.Join(required, " or "))
}
