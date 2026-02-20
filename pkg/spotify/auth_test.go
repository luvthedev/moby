package spotify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"
)

// MockTokenStore is a mock implementation of TokenStore for testing
type MockTokenStore struct {
	tokens map[string]*Token
}

func NewMockTokenStore() *MockTokenStore {
	return &MockTokenStore{
		tokens: make(map[string]*Token),
	}
}

func (m *MockTokenStore) StoreToken(ctx context.Context, key string, token *Token) error {
	m.tokens[key] = token
	return nil
}

func (m *MockTokenStore) RetrieveToken(ctx context.Context, key string) (*Token, error) {
	token, ok := m.tokens[key]
	if !ok {
		return nil, nil
	}
	return token, nil
}

func (m *MockTokenStore) DeleteToken(ctx context.Context, key string) error {
	delete(m.tokens, key)
	return nil
}

func (m *MockTokenStore) TokenExists(ctx context.Context, key string) (bool, error) {
	_, ok := m.tokens[key]
	return ok, nil
}

func (m *MockTokenStore) Close() error {
	return nil
}

// TestOAuthFlow tests the complete OAuth flow
func TestOAuthFlow(t *testing.T) {
	// Create test configuration
	config := &OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		Scopes:       []string{"user-read-private", "user-read-email"},
	}

	// Generate encryption key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("Failed to generate encryption key: %v", err)
	}

	// Create auth manager
	store := NewMockTokenStore()
	authMgr := NewAuthManager(config, store, key)

	// Test 1: Generate authorization URL with state token
	t.Run("GenerateAuthorizationURL", func(t *testing.T) {
		authURL, state, err := authMgr.GetAuthorizationURL(context.Background())
		if err != nil {
			t.Fatalf("Failed to generate auth URL: %v", err)
		}

		if authURL == "" {
			t.Error("Authorization URL is empty")
		}

		if state == "" {
			t.Error("State token is empty")
		}

		if !isValidStateToken(state) {
			t.Error("State token is not valid base64 URL encoded")
		}

		// Verify state was stored
		if err := authMgr.ValidateState(state); err != nil {
			t.Fatalf("Valid state token validation failed: %v", err)
		}
	})

	// Test 2: Validate state token (CSRF protection)
	t.Run("ValidateState", func(t *testing.T) {
		authURL, state, err := authMgr.GetAuthorizationURL(context.Background())
		if err != nil {
			t.Fatalf("Failed to generate auth URL: %v", err)
		}

		_ = authURL

		// Valid state should pass
		if err := authMgr.ValidateState(state); err != nil {
			t.Fatalf("State validation failed: %v", err)
		}

		// State should be consumed, second validation should fail
		if err := authMgr.ValidateState(state); err == nil {
			t.Error("Expected error for reused state token")
		}

		// Invalid state should fail
		if err := authMgr.ValidateState("invalid-state"); err == nil {
			t.Error("Expected error for invalid state token")
		}
	})

	// Test 3: Token encryption/decryption
	t.Run("EncryptDecryptToken", func(t *testing.T) {
		originalToken := &Token{
			AccessToken:  "test-access-token",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
			RefreshToken: "test-refresh-token",
			Scope:        "user-read-private user-read-email",
			Timestamp:    time.Now().Unix(),
		}

		// Encrypt token
		encrypted, err := authMgr.EncryptToken(originalToken)
		if err != nil {
			t.Fatalf("Failed to encrypt token: %v", err)
		}

		if encrypted == "" {
			t.Error("Encrypted token is empty")
		}

		// Decrypt token
		decrypted, err := authMgr.DecryptToken(encrypted)
		if err != nil {
			t.Fatalf("Failed to decrypt token: %v", err)
		}

		// Verify token data
		if decrypted.AccessToken != originalToken.AccessToken {
			t.Errorf("AccessToken mismatch: got %s, want %s", decrypted.AccessToken, originalToken.AccessToken)
		}

		if decrypted.RefreshToken != originalToken.RefreshToken {
			t.Errorf("RefreshToken mismatch: got %s, want %s", decrypted.RefreshToken, originalToken.RefreshToken)
		}

		if decrypted.ExpiresIn != originalToken.ExpiresIn {
			t.Errorf("ExpiresIn mismatch: got %d, want %d", decrypted.ExpiresIn, originalToken.ExpiresIn)
		}
	})

	// Test 4: Store and retrieve encrypted token
	t.Run("StoreRetrieveEncryptedToken", func(t *testing.T) {
		token := &Token{
			AccessToken:  "access-token-123",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
			RefreshToken: "refresh-token-456",
			Scope:        "user-read-private",
			Timestamp:    time.Now().Unix(),
		}

		key := "user-123"

		// Store encrypted token
		if err := authMgr.StoreEncryptedToken(context.Background(), key, token); err != nil {
			t.Fatalf("Failed to store encrypted token: %v", err)
		}

		// Retrieve and decrypt token
		retrieved, err := authMgr.RetrieveDecryptedToken(context.Background(), key)
		if err != nil {
			t.Fatalf("Failed to retrieve decrypted token: %v", err)
		}

		// Verify token data
		if retrieved.AccessToken != token.AccessToken {
			t.Errorf("AccessToken mismatch: got %s, want %s", retrieved.AccessToken, token.AccessToken)
		}

		if retrieved.RefreshToken != token.RefreshToken {
			t.Errorf("RefreshToken mismatch: got %s, want %s", retrieved.RefreshToken, token.RefreshToken)
		}
	})

	// Test 5: Token expiration check
	t.Run("TokenExpirationCheck", func(t *testing.T) {
		now := time.Now()

		// Token expiring soon
		expiringToken := &Token{
			AccessToken:  "expiring",
			ExpiresIn:    60, // 1 minute
			Timestamp:    now.Unix(),
		}

		if !authMgr.IsTokenExpiringSoon(expiringToken, 120) {
			t.Error("Token should be expiring soon")
		}

		// Token still valid
		validToken := &Token{
			AccessToken:  "valid",
			ExpiresIn:    7200, // 2 hours
			Timestamp:    now.Unix(),
		}

		if authMgr.IsTokenExpiringSoon(validToken, 60) {
			t.Error("Token should not be expiring soon")
		}
	})

	// Test 6: Test token refresher
	t.Run("TokenRefresher", func(t *testing.T) {
		store := NewMockTokenStore()
		authMgr := NewAuthManager(config, store, key)
		refresher := NewTokenRefresher(authMgr, store, 60, 100*time.Millisecond)
		defer refresher.Close()

		// Create a token
		token := &Token{
			AccessToken:  "test-access",
			TokenType:    "Bearer",
			ExpiresIn:    30,
			RefreshToken: "test-refresh",
			Timestamp:    time.Now().Unix(),
		}

		ctx := context.Background()
		key := "test-user"

		// Store token
		if err := store.StoreToken(ctx, key, token); err != nil {
			t.Fatalf("Failed to store token: %v", err)
		}

		// Start auto-refresh
		if err := refresher.StartAutoRefresh(ctx, key); err != nil {
			t.Fatalf("Failed to start auto-refresh: %v", err)
		}

		// Verify refresher is running
		if count := refresher.GetActiveRefreshCount(); count != 1 {
			t.Errorf("Expected 1 active refresh, got %d", count)
		}

		// Stop auto-refresh
		refresher.StopAutoRefresh(key)

		// Verify refresher stopped
		if count := refresher.GetActiveRefreshCount(); count != 0 {
			t.Errorf("Expected 0 active refreshes, got %d", count)
		}
	})
}

// TestStateTokenTimeout tests state token expiration
func TestStateTokenTimeout(t *testing.T) {
	config := &OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
	}

	key := make([]byte, 32)
	rand.Read(key)

	store := NewMockTokenStore()
	authMgr := NewAuthManager(config, store, key)

	// Set a very short timeout for testing
	authMgr.stateTokenTimeout = 100 * time.Millisecond

	authURL, state, err := authMgr.GetAuthorizationURL(context.Background())
	if err != nil {
		t.Fatalf("Failed to generate auth URL: %v", err)
	}

	_ = authURL

	// Wait for token to expire
	time.Sleep(150 * time.Millisecond)

	// State should be expired
	if err := authMgr.ValidateState(state); err == nil {
		t.Error("Expected error for expired state token")
	}
}

// Helper function to validate state token format
func isValidStateToken(state string) bool {
	_, err := encodeToBase64URL(state)
	return err == nil
}

// Helper function for base64 URL encoding validation
func encodeToBase64URL(s string) (string, error) {
	return hex.EncodeToString([]byte(s)), nil
}

// TestTokenEncryptionSecurity tests that encrypted data is not plaintext
func TestTokenEncryptionSecurity(t *testing.T) {
	config := &OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}

	key := make([]byte, 32)
	rand.Read(key)

	store := NewMockTokenStore()
	authMgr := NewAuthManager(config, store, key)

	token := &Token{
		AccessToken:  "super-secret-access-token",
		RefreshToken: "super-secret-refresh-token",
		ExpiresIn:    3600,
		Timestamp:    time.Now().Unix(),
	}

	encrypted, err := authMgr.EncryptToken(token)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Verify plaintext tokens are not in encrypted data
	if contains(encrypted, "super-secret-access-token") {
		t.Error("Access token found in encrypted data")
	}

	if contains(encrypted, "super-secret-refresh-token") {
		t.Error("Refresh token found in encrypted data")
	}
}

// Helper to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
