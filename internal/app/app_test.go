package app

import (
	"bytes"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRootCommand(t *testing.T) {
	root := NewRootCommand(NewConfig())

	assert.Equal(t, "google-cli", root.Use)
	assert.NotEmpty(t, root.Short, "root command should have a short description")
}

func TestRootCommandPersistentFlags(t *testing.T) {
	root := NewRootCommand(NewConfig())
	flags := root.PersistentFlags()

	for _, name := range []string{"account", "format", "debug"} {
		assert.NotNil(t, flags.Lookup(name), "expected persistent flag --%s", name)
	}
	assert.Nil(t, flags.Lookup("credentials"),
		"--credentials is Google-specific: it lives on the google provider command, not the root")
}

func TestPersistentFlagsBindToConfig(t *testing.T) {
	cfg := &Config{Fs: afero.NewMemMapFs()}
	root := NewRootCommand(cfg)

	err := root.ParseFlags([]string{
		"--account", "user@example.com",
		"--format", "json",
		"--debug",
	})
	require.NoError(t, err)

	assert.Equal(t, "user@example.com", cfg.Account)
	assert.Equal(t, "json", cfg.Format)
	assert.True(t, cfg.Debug)
}

func TestRootWiresDebugFlagToOutput(t *testing.T) {
	cfg := &Config{Fs: afero.NewMemMapFs()}
	root := NewRootCommand(cfg)
	require.NotNil(t, root.PersistentPreRunE, "root must define the single cfg.Debug consumer")

	require.NoError(t, root.PersistentPreRunE(root, nil))
}

func TestRootCommandVersionFlag(t *testing.T) {
	prev := Version
	Version = "v9.9.9-test"
	t.Cleanup(func() { Version = prev })

	cfg := &Config{Fs: afero.NewMemMapFs()}
	root := NewRootCommand(cfg)
	out := &bytes.Buffer{}
	root.SetOut(out)

	root.SetArgs([]string{"--version"})
	require.NoError(t, root.Execute(), "--version must exit without error")

	assert.Equal(t, "google-cli version v9.9.9-test\n", out.String())
}

func TestNewConfigDefaults(t *testing.T) {
	cfg := NewConfig()

	assert.Empty(t, cfg.Account)
	assert.Empty(t, cfg.Format)
	assert.False(t, cfg.Debug)
	assert.Empty(t, cfg.Credentials)
	assert.NotNil(t, cfg.Fs)
}
