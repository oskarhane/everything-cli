package auth

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// OAuthProfile bundles everything about an OAuth provider that the
// generalized flow machinery must take from the provider and never from a
// user-supplied file: the pinned authorization/token endpoints, the
// identity (userinfo) URL that resolves the account email after the flow,
// the scope that guarantees email access there, and the provider's default
// scope set. Providers supply one profile each; Google's is GoogleOAuth.
type OAuthProfile struct {
	// Name is the provider's display name, used in flow status lines.
	Name string
	// Endpoint is the provider's pinned authorization and token URLs. A
	// credentials file can never override these.
	Endpoint oauth2.Endpoint
	// UserinfoURL is the identity endpoint queried with the freshly
	// exchanged token to resolve the account email.
	UserinfoURL string
	// EmailScope is appended to every flow's scope set so UserinfoURL is
	// authorized to return the account email.
	EmailScope string
	// DefaultScopes is the scope set onboarded when the caller passes none.
	DefaultScopes []string
}

// GoogleOAuth is the OAuth profile for Google: endpoints pinned to Google's,
// the userinfo v2 identity endpoint, and the CLI's full default scope set
// (Gmail, Calendar, Drive, Docs, Sheets, Slides, plus userinfo.email).
var GoogleOAuth = OAuthProfile{
	Name:        "everything-cli",
	Endpoint:    google.Endpoint,
	UserinfoURL: "https://www.googleapis.com/oauth2/v2/userinfo",
	EmailScope:  ScopeUserEmail,
	DefaultScopes: func() []string {
		scopes := append([]string{}, ScopesGmail...)
		scopes = append(scopes, ScopesCalendar...)
		scopes = append(scopes, ScopesDrive...)
		scopes = append(scopes, ScopesDocs...)
		scopes = append(scopes, ScopesSheets...)
		scopes = append(scopes, ScopesSlides...)
		return append(scopes, ScopeUserEmail)
	}(),
}
