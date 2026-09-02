// package app_test (external) — internal/app cannot import internal/auth from
// its in-package tests: auth imports app for Config, so a test-side import
// would close an import cycle.
package app_test

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
	"github.com/oskarhane/everything-cli/internal/config"
)

// newAccountFlagRoot mounts a dummy leaf under the root. The leaf's RunE
// resolves the account the way API-backed leaves do (auth.ResolveAccountFor
// on an empty in-memory store) so the tests can tell the flag gate from the
// later account-resolution failure. Its ran output reports whether RunE
// executed.
func newAccountFlagRoot(t *testing.T) (*cobra.Command, *app.Config, *bool) {
	t.Helper()

	cfg := &app.Config{Fs: afero.NewMemMapFs()}
	root := app.NewRootCommand(cfg)
	ran := false
	dummy := &cobra.Command{
		Use: "dummy",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ran = true
			store, err := config.NewStore(cfg.Fs, "")
			if err != nil {
				return err
			}
			_, err = auth.ResolveAccountFor(cfg, store, config.ProviderGoogle)
			return err
		},
	}
	root.AddCommand(dummy)
	return root, cfg, &ran
}

func TestAccountFlagSetButEmptyFailsClosed(t *testing.T) {
	root, _, ran := newAccountFlagRoot(t)
	root.SetArgs([]string{"dummy", "--account", ""})

	err := root.Execute()

	require.Error(t, err)
	assert.EqualError(t, err, "--account is empty: pass an account name or drop the flag")
	assert.False(t, *ran, "empty --account must fail closed before the subcommand RunE")
}

func TestAccountFlagUnsetKeepsDefaultResolution(t *testing.T) {
	root, cfg, ran := newAccountFlagRoot(t)
	root.SetArgs([]string{"dummy"})

	err := root.Execute()

	// RunE runs; the failure is the later resolution error on the empty
	// store, NOT the empty-flag error.
	require.Error(t, err)
	assert.ErrorContains(t, err, "no google accounts configured")
	assert.True(t, *ran, "no --account must not trip the flag gate")
	assert.Empty(t, cfg.Account)
}

func TestAccountFlagNamedAccountRuns(t *testing.T) {
	root, cfg, ran := newAccountFlagRoot(t)

	// ResolveAccountFor verifies the account record exists, so seed the
	// named account on the same store the dummy leaf resolves against.
	store, err := config.NewStore(cfg.Fs, "")
	require.NoError(t, err)
	require.NoError(t, store.Save(&config.Account{Name: "work"}))

	root.SetArgs([]string{"--account", "work", "dummy"})

	err = root.Execute()

	require.NoError(t, err, "a named --account resolves without a default set")
	assert.Equal(t, "work", cfg.Account)
	assert.True(t, *ran)
}
