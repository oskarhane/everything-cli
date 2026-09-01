package auth

import (
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/config"
)

// noAccountsErr / noDefaultErr are the exact actionable account-selection
// error texts; client packages assert against them, so they are pinned here
// verbatim.
const (
	noAccountsErr = "no Google accounts configured; run `google-cli account add`"
	noDefaultErr  = "no default account set; run `google-cli account use <name>` or pass --account"
)

// newDialStore returns a store on fs rooted at the default config dir,
// mirroring how Dial constructs the store (config.NewStore on cfg.Fs).
func newDialStore(t *testing.T, fs afero.Fs) *config.Store {
	t.Helper()
	store, err := config.NewStore(fs, "")
	require.NoError(t, err)
	return store
}

// seedAccount persists an account so account resolution has something to
// find; withToken also gives it a still-valid cached token so Dial builds a
// TokenSource without refreshing.
func seedDialAccount(t *testing.T, store *config.Store, name string, withToken bool) {
	t.Helper()
	acct := &config.Account{Name: name, Email: name + "@example.com"}
	if withToken {
		acct.Token = &oauth2.Token{
			AccessToken:  "access-" + name,
			RefreshToken: "refresh-" + name,
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(time.Hour),
		}
	}
	require.NoError(t, store.Save(acct))
}

func TestResolveAccount(t *testing.T) {
	t.Run("flag wins over the stored default", func(t *testing.T) {
		store := newTestStore(t)
		seedDialAccount(t, store, "personal", false)
		require.NoError(t, store.SetDefaultAccount("personal"))

		account, err := ResolveAccount(&app.Config{Account: "work"}, store)
		require.NoError(t, err)
		require.Equal(t, "work", account)
	})

	t.Run("falls back to the store default", func(t *testing.T) {
		store := newTestStore(t)
		seedDialAccount(t, store, "personal", false)
		seedDialAccount(t, store, "work", false)
		require.NoError(t, store.SetDefaultAccount("work"))

		account, err := ResolveAccount(&app.Config{}, store)
		require.NoError(t, err)
		require.Equal(t, "work", account)
	})

	t.Run("no accounts configured", func(t *testing.T) {
		_, err := ResolveAccount(&app.Config{}, newTestStore(t))
		require.EqualError(t, err, noAccountsErr)
	})

	t.Run("accounts exist but no default set", func(t *testing.T) {
		store := newTestStore(t)
		seedDialAccount(t, store, "personal", false)

		_, err := ResolveAccount(&app.Config{}, store)
		require.EqualError(t, err, noDefaultErr)
	})
}

func TestDial(t *testing.T) {
	t.Run("fails fast with the account error before touching credentials", func(t *testing.T) {
		_, _, err := Dial(&app.Config{Fs: afero.NewMemMapFs()})
		require.EqualError(t, err, noAccountsErr)
	})

	t.Run("propagates credentials resolution failure", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		store := newDialStore(t, fs)
		seedDialAccount(t, store, "work", true)
		require.NoError(t, store.SetDefaultAccount("work"))

		_, _, err := Dial(&app.Config{Fs: fs})
		require.ErrorContains(t, err, "no OAuth credentials file found; tried: ")
	})

	t.Run("default account with valid token dial end to end", func(t *testing.T) {
		fs, credentialsPath := writeCredentialsFile(t)
		store := newDialStore(t, fs)
		seedDialAccount(t, store, "work", true)
		require.NoError(t, store.SetDefaultAccount("work"))

		ts, account, err := Dial(&app.Config{Fs: fs, Credentials: credentialsPath})
		require.NoError(t, err)
		require.Equal(t, "work", account)
		require.NotNil(t, ts)

		tok, err := ts.Token()
		require.NoError(t, err)
		require.Equal(t, "access-work", tok.AccessToken)
	})

	t.Run("flag wins through the full chain", func(t *testing.T) {
		fs, credentialsPath := writeCredentialsFile(t)
		store := newDialStore(t, fs)
		seedDialAccount(t, store, "personal", false)
		seedDialAccount(t, store, "work", true)
		require.NoError(t, store.SetDefaultAccount("personal"))

		_, account, err := Dial(&app.Config{Fs: fs, Credentials: credentialsPath, Account: "work"})
		require.NoError(t, err)
		require.Equal(t, "work", account)
	})
}

func TestDialAccount(t *testing.T) {
	t.Run("returns the resolved account record with its scopes", func(t *testing.T) {
		fs, credentialsPath := writeCredentialsFile(t)
		store := newDialStore(t, fs)
		seedDialAccount(t, store, "work", true)
		require.NoError(t, store.SetDefaultAccount("work"))

		acct, ts, err := DialAccount(&app.Config{Fs: fs, Credentials: credentialsPath})
		require.NoError(t, err)
		require.NotNil(t, ts)
		require.Equal(t, "work", acct.Name)
	})

	t.Run("propagates account resolution failure", func(t *testing.T) {
		_, _, err := DialAccount(&app.Config{Fs: afero.NewMemMapFs()})
		require.EqualError(t, err, noAccountsErr)
	})

	t.Run("Dial returns the name DialAccount resolved", func(t *testing.T) {
		fs, credentialsPath := writeCredentialsFile(t)
		store := newDialStore(t, fs)
		seedDialAccount(t, store, "work", true)
		require.NoError(t, store.SetDefaultAccount("work"))

		_, account, err := Dial(&app.Config{Fs: fs, Credentials: credentialsPath})
		require.NoError(t, err)
		require.Equal(t, "work", account)
	})
}
