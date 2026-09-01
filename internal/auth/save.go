package auth

import (
	"github.com/oskarhane/google-cli/internal/config"
	"github.com/oskarhane/google-cli/internal/output"
	"golang.org/x/oauth2"
)

// SaveAccount persists an authenticated account in store and returns its
// canonical name. When an account with the same email already exists, that
// record is updated under its original name instead of creating a
// duplicate; the returned name reflects that.
func SaveAccount(store *config.Store, name, email string, scopes []string, tok *oauth2.Token) (string, error) {
	// Register the token's secrets for redaction at the save point too, so
	// callers that persist a token minted outside RunFlowWith (tests, other
	// strategies) are covered.
	registerTokenSecrets(tok)
	acct := &config.Account{
		Name:   name,
		Email:  email,
		Scopes: scopes,
		Token:  tok,
	}
	if err := store.Save(acct); err != nil {
		return "", err
	}
	// Account names/emails are non-secret metadata; never log token values.
	output.Debug("saved account " + acct.Name)
	return acct.Name, nil
}
