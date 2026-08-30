package gmail

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/app"
)

func TestResolveAccountFlagWins(t *testing.T) {
	store := newTestStore(t)
	seedAccount(t, store, "personal")
	require.NoError(t, store.SetDefaultAccount("personal"))

	got, err := resolveAccount(&app.Config{Account: "work"}, store)
	require.NoError(t, err)
	require.Equal(t, "work", got)
}

func TestResolveAccountDefault(t *testing.T) {
	store := newTestStore(t)
	seedAccount(t, store, "personal")
	seedAccount(t, store, "work")
	require.NoError(t, store.SetDefaultAccount("work"))

	got, err := resolveAccount(&app.Config{}, store)
	require.NoError(t, err)
	require.Equal(t, "work", got)
}

func TestResolveAccountNoAccounts(t *testing.T) {
	store := newTestStore(t)

	_, err := resolveAccount(&app.Config{}, store)
	require.ErrorContains(t, err, "no Google accounts configured")
	require.ErrorContains(t, err, "google-cli account add")
}

func TestResolveAccountNoDefaultSet(t *testing.T) {
	store := newTestStore(t)
	seedAccount(t, store, "personal")

	_, err := resolveAccount(&app.Config{}, store)
	require.ErrorContains(t, err, "no default account set")
	require.ErrorContains(t, err, "--account")
}

func TestDialFailsWithoutAccounts(t *testing.T) {
	// dial must fail fast with the actionable account error before touching
	// credentials or the network.
	_, err := dial(context.Background(), &app.Config{Fs: afero.NewMemMapFs()})
	require.ErrorContains(t, err, "no Google accounts configured")
}
