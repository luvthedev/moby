package spotify

// SpotifyConfig holds the configuration required for the Spotify
// integration, including OAuth2 credentials and the storage backend
// for persisting user tokens.
type SpotifyConfig struct {
	// ClientID is the Spotify application client identifier.
	ClientID string

	// ClientSecret is the Spotify application client secret.
	ClientSecret string

	// RedirectURI is the OAuth2 callback URI registered with Spotify.
	RedirectURI string

	// TokenStore is the storage backend used to persist OAuth2 tokens.
	// Implementations may target AWS S3, Google Cloud Storage,
	// Azure Blob Storage, or any other persistent store.
	TokenStore TokenStore
}

// Validate checks that the required configuration fields are populated
// and returns an error if any are missing.
func (c *SpotifyConfig) Validate() error {
	if c.ClientID == "" {
		return &SpotifyError{Code: ErrAuthFailed, Message: "client_id is required"}
	}
	if c.ClientSecret == "" {
		return &SpotifyError{Code: ErrAuthFailed, Message: "client_secret is required"}
	}
	if c.RedirectURI == "" {
		return &SpotifyError{Code: ErrAuthFailed, Message: "redirect_uri is required"}
	}
	if c.TokenStore == nil {
		return &SpotifyError{Code: ErrAuthFailed, Message: "token_store is required"}
	}
	return nil
}
