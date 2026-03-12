package spotify

import (
	"context"
	"errors"
	"testing"
)

func TestSpotifyErrorCodes(t *testing.T) {
	tests := []struct {
		name    string
		code    error
		checkFn func(error) bool
	}{
		{"AuthFailed", ErrAuthFailed, IsAuthFailed},
		{"RateLimited", ErrRateLimited, IsRateLimited},
		{"PreviewUnavailable", ErrPreviewUnavailable, IsPreviewUnavailable},
		{"TokenExpired", ErrTokenExpired, IsTokenExpired},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := &SpotifyError{Code: tc.code, Message: "test"}
			if !tc.checkFn(err) {
				t.Errorf("expected %s check to return true for SpotifyError with code %v", tc.name, tc.code)
			}
			if !errors.Is(err, tc.code) {
				t.Errorf("expected errors.Is to match code %v", tc.code)
			}
		})
	}
}

func TestSpotifyErrorMessage(t *testing.T) {
	err := &SpotifyError{Code: ErrAuthFailed, Message: "invalid credentials"}
	expected := "spotify: authentication failed: invalid credentials"
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}

	errNoMsg := &SpotifyError{Code: ErrRateLimited}
	if errNoMsg.Error() != ErrRateLimited.Error() {
		t.Errorf("got %q, want %q", errNoMsg.Error(), ErrRateLimited.Error())
	}
}

func TestSpotifyErrorUnwrap(t *testing.T) {
	err := &SpotifyError{Code: ErrTokenExpired, Message: "token expired at noon"}
	if !errors.Is(err, ErrTokenExpired) {
		t.Error("Unwrap should allow errors.Is to match ErrTokenExpired")
	}
	if errors.Is(err, ErrAuthFailed) {
		t.Error("errors.Is should not match a different code")
	}
}

func TestCheckFunctionsWithNil(t *testing.T) {
	if IsAuthFailed(nil) {
		t.Error("IsAuthFailed(nil) should be false")
	}
	if IsRateLimited(nil) {
		t.Error("IsRateLimited(nil) should be false")
	}
	if IsPreviewUnavailable(nil) {
		t.Error("IsPreviewUnavailable(nil) should be false")
	}
	if IsTokenExpired(nil) {
		t.Error("IsTokenExpired(nil) should be false")
	}
}

func TestConfigValidate(t *testing.T) {
	store := &mockTokenStore{}
	tests := []struct {
		name    string
		config  SpotifyConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: SpotifyConfig{
				ClientID:     "id",
				ClientSecret: "secret",
				RedirectURI:  "http://localhost/callback",
				TokenStore:   store,
			},
			wantErr: false,
		},
		{
			name: "missing client_id",
			config: SpotifyConfig{
				ClientSecret: "secret",
				RedirectURI:  "http://localhost/callback",
				TokenStore:   store,
			},
			wantErr: true,
		},
		{
			name: "missing client_secret",
			config: SpotifyConfig{
				ClientID:    "id",
				RedirectURI: "http://localhost/callback",
				TokenStore:  store,
			},
			wantErr: true,
		},
		{
			name: "missing redirect_uri",
			config: SpotifyConfig{
				ClientID:     "id",
				ClientSecret: "secret",
				TokenStore:   store,
			},
			wantErr: true,
		},
		{
			name: "missing token_store",
			config: SpotifyConfig{
				ClientID:     "id",
				ClientSecret: "secret",
				RedirectURI:  "http://localhost/callback",
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// mockTokenStore is a minimal TokenStore implementation for testing.
type mockTokenStore struct{}

func (m *mockTokenStore) SaveToken(_ context.Context, _ string, _ *Token) error {
	return nil
}

func (m *mockTokenStore) GetToken(_ context.Context, _ string) (*Token, error) {
	return &Token{}, nil
}

func (m *mockTokenStore) DeleteToken(_ context.Context, _ string) error {
	return nil
}

// Compile-time check that mockTokenStore implements TokenStore.
var _ TokenStore = (*mockTokenStore)(nil)
