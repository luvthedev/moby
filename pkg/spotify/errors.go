package spotify

import (
	"github.com/moby/moby/errdefs"
)

// ErrAuthFailed signals that Spotify authentication failed
type ErrAuthFailed interface {
	AuthFailed()
}

// ErrRateLimited signals that the Spotify API rate limit has been exceeded
type ErrRateLimited interface {
	RateLimited()
}

// ErrPreviewUnavailable signals that the preview URL is not available for the track
type ErrPreviewUnavailable interface {
	PreviewUnavailable()
}

// ErrTokenExpired signals that the access token has expired
type ErrTokenExpired interface {
	TokenExpired()
}

// Custom error implementations

type errAuthFailed struct{ error }

func (errAuthFailed) AuthFailed() {}

func (e errAuthFailed) Cause() error {
	return e.error
}

func (e errAuthFailed) Unwrap() error {
	return e.error
}

// AuthFailed creates an [ErrAuthFailed] error from the given error.
// It returns the error as-is if it is either nil (no error) or already implements [ErrAuthFailed].
func AuthFailed(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(ErrAuthFailed); ok {
		return err
	}
	return errAuthFailed{err}
}

// IsAuthFailed returns true if the error implements [ErrAuthFailed]
func IsAuthFailed(err error) bool {
	_, ok := err.(ErrAuthFailed)
	return ok
}

type errRateLimited struct{ error }

func (errRateLimited) RateLimited() {}

func (e errRateLimited) Cause() error {
	return e.error
}

func (e errRateLimited) Unwrap() error {
	return e.error
}

// RateLimited creates an [ErrRateLimited] error from the given error.
// It returns the error as-is if it is either nil (no error) or already implements [ErrRateLimited].
func RateLimited(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(ErrRateLimited); ok {
		return err
	}
	return errRateLimited{err}
}

// IsRateLimited returns true if the error implements [ErrRateLimited]
func IsRateLimited(err error) bool {
	_, ok := err.(ErrRateLimited)
	return ok
}

type errPreviewUnavailable struct{ error }

func (errPreviewUnavailable) PreviewUnavailable() {}

func (e errPreviewUnavailable) Cause() error {
	return e.error
}

func (e errPreviewUnavailable) Unwrap() error {
	return e.error
}

// PreviewUnavailable creates an [ErrPreviewUnavailable] error from the given error.
// It returns the error as-is if it is either nil (no error) or already implements [ErrPreviewUnavailable].
func PreviewUnavailable(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(ErrPreviewUnavailable); ok {
		return err
	}
	return errPreviewUnavailable{err}
}

// IsPreviewUnavailable returns true if the error implements [ErrPreviewUnavailable]
func IsPreviewUnavailable(err error) bool {
	_, ok := err.(ErrPreviewUnavailable)
	return ok
}

type errTokenExpired struct{ error }

func (errTokenExpired) TokenExpired() {}

func (e errTokenExpired) Cause() error {
	return e.error
}

func (e errTokenExpired) Unwrap() error {
	return e.error
}

// TokenExpired creates an [ErrTokenExpired] error from the given error.
// It returns the error as-is if it is either nil (no error) or already implements [ErrTokenExpired].
func TokenExpired(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(ErrTokenExpired); ok {
		return err
	}
	return errTokenExpired{err}
}

// IsTokenExpired returns true if the error implements [ErrTokenExpired]
func IsTokenExpired(err error) bool {
	_, ok := err.(ErrTokenExpired)
	return ok
}

// Helper function to map HTTP status codes to Spotify errors
// This is useful when converting HTTP responses from Spotify API

// WrapUnauthorized wraps an error as Unauthorized using errdefs
func WrapUnauthorized(err error) error {
	return errdefs.Unauthorized(err)
}

// WrapUnavailable wraps an error as Unavailable using errdefs
func WrapUnavailable(err error) error {
	return errdefs.Unavailable(err)
}

// WrapInvalidParameter wraps an error as InvalidParameter using errdefs
func WrapInvalidParameter(err error) error {
	return errdefs.InvalidParameter(err)
}
