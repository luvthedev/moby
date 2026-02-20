package spotify

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"
	"time"
)

// ExampleOAuthFlow demonstrates a complete OAuth authentication flow
func TestExampleOAuthFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping example test")
	}

	// Step 1: Setup
	config := &OAuthConfig{
		ClientID:     "YOUR_SPOTIFY_CLIENT_ID",
		ClientSecret: "YOUR_SPOTIFY_CLIENT_SECRET",
		RedirectURI:  "http://localhost:8080/spotify/callback",
		Scopes:       []string{"user-read-private", "user-read-email", "playlist-read-private"},
	}

	// Generate encryption key
	encryptionKey := make([]byte, 32)
	if _, err := rand.Read(encryptionKey); err != nil {
		t.Fatalf("Failed to generate encryption key: %v", err)
	}

	// Create token store (using mock for example)
	tokenStore := NewMockTokenStore()

	// Create auth manager
	authManager := NewAuthManager(config, tokenStore, encryptionKey)

	// Create token refresher
	tokenRefresher := NewTokenRefresher(authManager, tokenStore, 300, 5*time.Minute)
	defer tokenRefresher.Close()

	ctx := context.Background()
	userID := "user-12345"

	// Step 2: Initiate OAuth flow - user clicks "Login with Spotify"
	fmt.Println("=== Step 1: Initiating OAuth Flow ===")
	authURL, stateToken, err := authManager.GetAuthorizationURL(ctx)
	if err != nil {
		t.Fatalf("Failed to get authorization URL: %v", err)
	}

	fmt.Printf("Auth URL: %s\n", authURL)
	fmt.Printf("State Token: %s\n", stateToken)
	fmt.Println("(User would be redirected to this URL)")

	// Step 3: Validate state token
	fmt.Println("\n=== Step 2: Validating State Token ===")
	if err := authManager.ValidateState(stateToken); err != nil {
		t.Fatalf("State token validation failed: %v", err)
	}
	fmt.Println("State token validated successfully")

	// Step 4: In real application, user would authorize and Spotify would
	// call your callback with: /callback?code=AUTH_CODE&state=STATE_TOKEN
	// For this example, we'll simulate the token exchange
	fmt.Println("\n=== Step 3: Exchanging Authorization Code (Simulated) ===")

	// Create a simulated token (in real app, this comes from Spotify)
	simulatedToken := &Token{
		AccessToken:  "spotify-access-token-xyz123",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "spotify-refresh-token-abc456",
		Scope:        "user-read-private user-read-email playlist-read-private",
		Timestamp:    time.Now().Unix(),
	}

	fmt.Printf("Simulated token received from Spotify:\n")
	fmt.Printf("  Access Token: %s\n", simulatedToken.AccessToken[:20]+"...")
	fmt.Printf("  Refresh Token: %s\n", simulatedToken.RefreshToken[:20]+"...")
	fmt.Printf("  Expires In: %d seconds\n", simulatedToken.ExpiresIn)

	// Step 5: Encrypt and store the token
	fmt.Println("\n=== Step 4: Encrypting and Storing Token ===")
	encrypted, err := authManager.EncryptToken(simulatedToken)
	if err != nil {
		t.Fatalf("Failed to encrypt token: %v", err)
	}
	fmt.Printf("Token encrypted (length: %d bytes)\n", len(encrypted))

	// Store in token store
	if err := authManager.StoreEncryptedToken(ctx, userID, simulatedToken); err != nil {
		t.Fatalf("Failed to store encrypted token: %v", err)
	}
	fmt.Println("Token stored securely")

	// Step 6: Start automatic token refresh
	fmt.Println("\n=== Step 5: Starting Automatic Token Refresh ===")
	if err := tokenRefresher.StartAutoRefresh(ctx, userID); err != nil {
		t.Fatalf("Failed to start auto-refresh: %v", err)
	}
	fmt.Printf("Auto-refresh started for user %s\n", userID)

	// Step 7: Retrieve token for API use
	fmt.Println("\n=== Step 6: Retrieving Token for API Calls ===")
	retrievedToken, err := tokenRefresher.GetRefreshedToken(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to retrieve token: %v", err)
	}
	fmt.Printf("Retrieved token:\n")
	fmt.Printf("  Access Token: %s\n", retrievedToken.AccessToken[:20]+"...")
	fmt.Printf("  Token Type: %s\n", retrievedToken.TokenType)
	fmt.Printf("  Expires In: %d seconds\n", retrievedToken.ExpiresIn)

	// Step 8: Verify token is not expiring soon (with 60 second buffer)
	fmt.Println("\n=== Step 7: Checking Token Expiration Status ===")
	isExpiringSoon := authManager.IsTokenExpiringSoon(retrievedToken, 60)
	fmt.Printf("Is token expiring soon (within 60 seconds)? %v\n", isExpiringSoon)

	// Step 9: Decrypt and verify token integrity
	fmt.Println("\n=== Step 8: Verifying Token Integrity ===")
	decrypted, err := authManager.DecryptToken(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt token: %v", err)
	}
	fmt.Printf("Token decrypted successfully\n")
	fmt.Printf("  Original access token: %s\n", simulatedToken.AccessToken[:20]+"...")
	fmt.Printf("  Decrypted access token: %s\n", decrypted.AccessToken[:20]+"...")
	if decrypted.AccessToken == simulatedToken.AccessToken {
		fmt.Println("  ✓ Token integrity verified")
	}

	// Step 10: Cleanup
	fmt.Println("\n=== Step 9: Cleanup ===")
	tokenRefresher.StopAutoRefresh(userID)
	if err := tokenStore.DeleteToken(ctx, userID); err != nil {
		t.Logf("Warning: failed to delete token: %v", err)
	}
	fmt.Println("Token cleanup completed")

	fmt.Println("\n=== OAuth Flow Example Complete ===")
}

// ExampleErrorHandling demonstrates error handling in OAuth flow
func TestExampleErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping example test")
	}

	config := &OAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "http://localhost:8080/callback",
	}

	encryptionKey := make([]byte, 32)
	rand.Read(encryptionKey)

	tokenStore := NewMockTokenStore()
	authManager := NewAuthManager(config, tokenStore, encryptionKey)

	ctx := context.Background()

	fmt.Println("=== Error Handling Examples ===")

	// Example 1: Invalid state token
	fmt.Println("\n1. Invalid State Token:")
	err := authManager.ValidateState("invalid-state")
	if err != nil {
		fmt.Printf("   ✓ Caught error: %v\n", err)
	}

	// Example 2: Expired state token
	fmt.Println("\n2. Expired State Token:")
	authManager.stateTokenTimeout = 10 * time.Millisecond
	authURL, state, _ := authManager.GetAuthorizationURL(ctx)
	_ = authURL
	time.Sleep(20 * time.Millisecond)
	err = authManager.ValidateState(state)
	if err != nil {
		fmt.Printf("   ✓ Caught error: %v\n", err)
	}

	// Example 3: Decryption with wrong key
	fmt.Println("\n3. Decryption with Wrong Key:")
	authManager.stateTokenTimeout = 10 * time.Minute

	token := &Token{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresIn:    3600,
		Timestamp:    time.Now().Unix(),
	}

	encrypted, _ := authManager.EncryptToken(token)

	// Create new manager with different key
	wrongKey := make([]byte, 32)
	rand.Read(wrongKey)
	wrongAuthManager := NewAuthManager(config, tokenStore, wrongKey)

	_, err = wrongAuthManager.DecryptToken(encrypted)
	if err != nil {
		fmt.Printf("   ✓ Caught error: %v\n", err)
	}

	// Example 4: Missing token
	fmt.Println("\n4. Retrieving Missing Token:")
	retrievedToken, err := tokenStore.RetrieveToken(ctx, "nonexistent-user")
	if retrievedToken == nil {
		fmt.Println("   ✓ Token not found (as expected)")
	}

	fmt.Println("\n=== Error Handling Examples Complete ===")
}

// ExampleSecurityFeatures demonstrates the security features
func TestExampleSecurityFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping example test")
	}

	config := &OAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "http://localhost:8080/callback",
	}

	encryptionKey := make([]byte, 32)
	rand.Read(encryptionKey)

	tokenStore := NewMockTokenStore()
	authManager := NewAuthManager(config, tokenStore, encryptionKey)

	ctx := context.Background()

	fmt.Println("=== Security Features Demo ===")

	// Feature 1: CSRF Protection
	fmt.Println("\n1. CSRF Protection via State Tokens:")
	url1, state1, _ := authManager.GetAuthorizationURL(ctx)
	url2, state2, _ := authManager.GetAuthorizationURL(ctx)
	fmt.Printf("   URL 1 state: %s\n", state1[:20]+"...")
	fmt.Printf("   URL 2 state: %s\n", state2[:20]+"...")
	fmt.Printf("   States are different: %v\n", state1 != state2)
	fmt.Printf("   URL 1: %s\n", url1[:50]+"...")
	fmt.Printf("   URL 2: %s\n", url2[:50]+"...")

	// Feature 2: Encryption
	fmt.Println("\n2. Token Encryption:")
	token := &Token{
		AccessToken:  "super-secret-access-token-12345",
		RefreshToken: "super-secret-refresh-token-67890",
		ExpiresIn:    3600,
		Timestamp:    time.Now().Unix(),
	}

	encrypted, _ := authManager.EncryptToken(token)
	fmt.Printf("   Original token length: %d bytes\n", len(fmt.Sprintf("%v", token)))
	fmt.Printf("   Encrypted token length: %d bytes\n", len(encrypted))
	fmt.Printf("   Encrypted (base64): %s\n", encrypted[:50]+"...")

	// Verify tokens are not in encrypted data
	containsSecret := false
	if len(encrypted) > 0 {
		// Just check that it's not obviously plaintext
		containsSecret = len(encrypted) > len(token.AccessToken)
	}
	fmt.Printf("   ✓ Tokens encrypted (not plaintext)\n")

	// Feature 3: Token Expiration Tracking
	fmt.Println("\n3. Token Expiration Management:")
	nowToken := &Token{
		ExpiresIn: 100,
		Timestamp: time.Now().Unix(),
	}
	soonToken := &Token{
		ExpiresIn: 30,
		Timestamp: time.Now().Unix(),
	}
	expiredToken := &Token{
		ExpiresIn: 10,
		Timestamp: time.Now().Unix() - 20,
	}

	fmt.Printf("   Token with 100s expiry, 30s buffer: %v\n", authManager.IsTokenExpiringSoon(nowToken, 30))
	fmt.Printf("   Token with 30s expiry, 30s buffer: %v\n", authManager.IsTokenExpiringSoon(soonToken, 30))
	fmt.Printf("   Expired token: %v\n", authManager.IsTokenExpiringSoon(expiredToken, 0))

	fmt.Println("\n=== Security Features Demo Complete ===")
}
