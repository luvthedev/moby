package spotify

import (
	"context"
	"errors"
	"fmt"
)

// Sentinel errors for Spotify-specific error conditions.
var (
	// ErrAuthFailed indicates that Spotify authentication failed,
	// for example due to invalid credentials or an expired authorization code.
	ErrAuthFailed = errors.New("spotify: authentication failed")

	// ErrRateLimited indicates that the Spotify API rate limit has been exceeded.
	ErrRateLimited = errors.New("spotify: rate limited")

	// ErrPreviewUnavailable indicates that no 30-second preview clip
	// is available for the requested track.
	ErrPreviewUnavailable = errors.New("spotify: preview unavailable")

	// ErrTokenExpired indicates that the access token has expired
	// and must be refreshed using RefreshToken.
	ErrTokenExpired = errors.New("spotify: token expired")
)

// SpotifyError represents a Spotify-specific error with a code
// that maps to one of the sentinel error conditions.
type SpotifyError struct {
	// Code is the sentinel error identifying the error class.
	Code error
	// Message provides additional detail about the error.
	Message string
}

// Error implements the error interface.
func (e *SpotifyError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Code.Error()
}

// Unwrap returns the underlying sentinel error code so that
// errors.Is works with SpotifyError values.
func (e *SpotifyError) Unwrap() error {
	return e.Code
}

// IsAuthFailed reports whether err is or wraps ErrAuthFailed.
func IsAuthFailed(err error) bool {
	return errors.Is(err, ErrAuthFailed)
}

// IsRateLimited reports whether err is or wraps ErrRateLimited.
func IsRateLimited(err error) bool {
	return errors.Is(err, ErrRateLimited)
}

// IsPreviewUnavailable reports whether err is or wraps ErrPreviewUnavailable.
func IsPreviewUnavailable(err error) bool {
	return errors.Is(err, ErrPreviewUnavailable)
}

// IsTokenExpired reports whether err is or wraps ErrTokenExpired.
func IsTokenExpired(err error) bool {
	return errors.Is(err, ErrTokenExpired)
}

// Token represents an OAuth2 token pair for Spotify API access.
type Token struct {
	// AccessToken is the OAuth2 access token.
	AccessToken string
	// RefreshToken is the OAuth2 refresh token used to obtain new access tokens.
	RefreshToken string
	// ExpiresAt is the Unix timestamp when the access token expires.
	ExpiresAt int64
}

// TokenStore abstracts the storage backend for Spotify OAuth2 tokens.
// Implementations may use AWS S3, Google Cloud Storage, Azure Blob Storage,
// or any other persistent storage mechanism.
type TokenStore interface {
	// SaveToken persists the token for the given user.
	SaveToken(ctx context.Context, userID string, token *Token) error

	// GetToken retrieves the stored token for the given user.
	// Returns ErrNotFound (or an equivalent) if no token exists for the user.
	GetToken(ctx context.Context, userID string) (*Token, error)

	// DeleteToken removes the stored token for the given user.
	// It is not an error to delete a token that does not exist.
	DeleteToken(ctx context.Context, userID string) error
}
