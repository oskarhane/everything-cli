package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestScopes pins the exact scope sets granted per service.
func TestScopes(t *testing.T) {
	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/gmail.send",
		"https://www.googleapis.com/auth/gmail.compose",
	}, ScopesGmail)

	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/calendar",
	}, ScopesCalendar)

	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/drive",
	}, ScopesDrive)

	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/documents",
	}, ScopesDocs)

	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/spreadsheets",
	}, ScopesSheets)

	assert.Equal(t, []string{
		"https://www.googleapis.com/auth/presentations",
	}, ScopesSlides)

	assert.Equal(t, "https://www.googleapis.com/auth/userinfo.email", ScopeUserEmail)
}
