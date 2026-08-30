package config

import "golang.org/x/oauth2"

// Account is a persisted Google account: identity, granted scopes and the
// cached OAuth token. Disk format uses snake_case JSON keys; Token reuses
// oauth2.Token's wire tags (access_token, refresh_token, token_type, expiry).
type Account struct {
	Name   string        `json:"name"`
	Email  string        `json:"email"`
	Scopes []string      `json:"scopes"`
	Token  *oauth2.Token `json:"token"`
}
