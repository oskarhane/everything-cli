package slides

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oskarhane/google-cli/internal/subcommands/cmdtest"
)

func TestReplaceFlags(t *testing.T) {
	svc := &fakeSlideService{shapes: seedShapes()}
	out := cmdtest.RunCmd(t, newSlideLeafCmd(newReplaceCmd, svc, "json"),
		"pres_1", "--find", "Acme", "--replace-with", "Zenith")

	require.Equal(t, "pres_1", svc.replaceID)
	require.Equal(t, "Acme", svc.find)
	require.Equal(t, "Zenith", svc.replaceWith)
	require.False(t, svc.matchCase, "match-case defaults to false")
	require.Equal(t, "Replaced 3 occurrence(s)\n", out)
}

func TestReplaceMatchCase(t *testing.T) {
	svc := &fakeSlideService{shapes: seedShapes()}
	cmdtest.RunCmd(t, newSlideLeafCmd(newReplaceCmd, svc, "json"),
		"pres_1", "--find", "Acme", "--replace-with", "Zenith", "--match-case")

	require.True(t, svc.matchCase)
}

func TestReplaceEmptyFindRefused(t *testing.T) {
	svc := &fakeSlideService{shapes: seedShapes()}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newReplaceCmd, svc, "json"),
		"pres_1", "--find", "", "--replace-with", "Zenith")

	require.Contains(t, err.Error(), "--find must be non-empty")
}

func TestReplaceFindRequired(t *testing.T) {
	svc := &fakeSlideService{shapes: seedShapes()}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newReplaceCmd, svc, "json"),
		"pres_1", "--replace-with", "Zenith")

	require.Contains(t, err.Error(), `required flag(s) "find" not set`)
}

func TestReplacePropagatesAPIError(t *testing.T) {
	svc := &fakeSlideService{err: errors.New("googleapi: Error 403")}
	_, err := cmdtest.RunCmdErr(t, newSlideLeafCmd(newReplaceCmd, svc, "json"),
		"pres_1", "--find", "Acme", "--replace-with", "Zenith")

	require.Contains(t, err.Error(), "googleapi: Error 403")
}
