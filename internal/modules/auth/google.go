package auth

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// NewGoogleOAuthConfig builds the Google OAuth client from explicit arguments
// instead of a package-level global.
//
// The old `var googleConfig = &oauth2.Config{...}` ran at package init and read
// config.AppConfig there, so the client was frozen the moment the process
// started and could not be varied per environment or per test. Taking the
// settings as parameters turns it into an ordinary dependency the composition
// root can supply, and keeps the auth package free of the config global.
func NewGoogleOAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			"openid",
			"email",
			"profile",
		},
	}
}