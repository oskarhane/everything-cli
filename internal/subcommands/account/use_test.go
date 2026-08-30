package account

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUseSetsDefault: account use persists the default account.
func TestUseSetsDefault(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	seedAccount(t, cfg, "personal", "me@example.com")
	seedAccount(t, cfg, "work", "work@example.com")

	outStr, err := execute(t, root, out, "account", "use", "work")
	require.NoError(t, err)
	assert.Contains(t, outStr, "work")

	def, err := newStore(t, cfg).DefaultAccount()
	require.NoError(t, err)
	assert.Equal(t, "work", def)
}

// TestUseUnknownAccountErrors: using an unknown name errors and leaves the
// default unchanged.
func TestUseUnknownAccountErrors(t *testing.T) {
	cfg, root, out := newAccountEnv(t)
	seedAccount(t, cfg, "work", "work@example.com")
	require.NoError(t, newStore(t, cfg).SetDefaultAccount("work"))

	_, err := execute(t, root, out, "account", "use", "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
	assert.Contains(t, err.Error(), "setting default account")

	def, err := newStore(t, cfg).DefaultAccount()
	require.NoError(t, err)
	assert.Equal(t, "work", def, "the default must survive a failed use")
}
