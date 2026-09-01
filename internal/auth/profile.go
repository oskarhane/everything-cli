package auth

import (
	"context"

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
	// ScopeSeparator joins the scope list on the authorization URL; empty
	// means the oauth2 default (space). Linear's authorize endpoint
	// documents a comma-separated scope parameter.
	ScopeSeparator string
	// IdentityResolver, when non-nil, resolves the account email after the
	// code exchange instead of the UserinfoURL GET — for providers whose
	// identity endpoint is not a plain JSON GET (Linear resolves identity
	// through its GraphQL viewer query). It is called with the freshly
	// exchanged token.
	IdentityResolver func(ctx context.Context, tok *oauth2.Token) (string, error)
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
