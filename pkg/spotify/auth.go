package spotify

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// OAuthConfig holds Spotify OAuth configuration
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
}

// AuthManager manages the OAuth flow with Spotify
type AuthManager struct {
	config            *OAuthConfig
	tokenStore        TokenStore
	encryptionKey     []byte
	stateTokenTimeout time.Duration
	stateMutex        map[string]int64 // state -> expiration timestamp
}

// NewAuthManager creates a new OAuth authentication manager
func NewAuthManager(config *OAuthConfig, tokenStore TokenStore, encryptionKey []byte) *AuthManager {
	if len(encryptionKey) != 32 {
		panic("encryption key must be 32 bytes for AES-256")
	}
	return &AuthManager{
		config:            config,
		tokenStore:        tokenStore,
		encryptionKey:     encryptionKey,
		stateTokenTimeout: 10 * time.Minute,
		stateMutex:        make(map[string]int64),
	}
}

// GetAuthorizationURL generates a Spotify authorization URL with CSRF protection
func (am *AuthManager) GetAuthorizationURL(ctx context.Context) (string, string, error) {
	// Generate a secure random state token
	stateToken, err := generateSecureToken(32)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate state token: %w", err)
	}

	// Store state token with expiration for validation
	am.stateMutex[stateToken] = time.Now().Add(am.stateTokenTimeout).UnixNano()

	// Build the authorization URL
	authURL := url.URL{
		Scheme: "https",
		Host:   "accounts.spotify.com",
		Path:   "/authorize",
	}

	params := url.Values{}
	params.Add("client_id", am.config.ClientID)
	params.Add("response_type", "code")
	params.Add("redirect_uri", am.config.RedirectURI)
	params.Add("state", stateToken)
	params.Add("scope", formatScopes(am.config.Scopes))
	params.Add("show_dialog", "true")

	authURL.RawQuery = params.Encode()

	return authURL.String(), stateToken, nil
}

// ValidateState validates the state token from the OAuth callback
func (am *AuthManager) ValidateState(state string) error {
	expiration, exists := am.stateMutex[state]
	if !exists {
		return fmt.Errorf("invalid state token")
	}

	// Check if token has expired
	if time.Now().UnixNano() > expiration {
		delete(am.stateMutex, state)
		return fmt.Errorf("state token expired")
	}

	// Remove used state token
	delete(am.stateMutex, state)
	return nil
}

// ExchangeCodeForToken exchanges authorization code for access and refresh tokens
func (am *AuthManager) ExchangeCodeForToken(ctx context.Context, code string) (*Token, error) {
	// Create token request to Spotify
	tokenReq := url.Values{}
	tokenReq.Add("grant_type", "authorization_code")
	tokenReq.Add("code", code)
	tokenReq.Add("redirect_uri", am.config.RedirectURI)
	tokenReq.Add("client_id", am.config.ClientID)
	tokenReq.Add("client_secret", am.config.ClientSecret)

	// Make request to Spotify token endpoint
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.PostForm("https://accounts.spotify.com/api/token", tokenReq)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	defer resp.Body.Close()

	// Handle API errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, AuthFailed(fmt.Errorf("spotify API error (status %d): %s", resp.StatusCode, string(body)))
	}

	// Parse response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int32  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Create token object
	token := &Token{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		ExpiresIn:    tokenResp.ExpiresIn,
		RefreshToken: tokenResp.RefreshToken,
		Scope:        tokenResp.Scope,
		Timestamp:    time.Now().Unix(),
	}

	return token, nil
}

// RefreshAccessToken refreshes an expired access token
func (am *AuthManager) RefreshAccessToken(ctx context.Context, token *Token) (*Token, error) {
	if token.RefreshToken == "" {
		return nil, fmt.Errorf("refresh token is empty")
	}

	// Create refresh token request
	tokenReq := url.Values{}
	tokenReq.Add("grant_type", "refresh_token")
	tokenReq.Add("refresh_token", token.RefreshToken)
	tokenReq.Add("client_id", am.config.ClientID)
	tokenReq.Add("client_secret", am.config.ClientSecret)

	// Make request to Spotify token endpoint
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.PostForm("https://accounts.spotify.com/api/token", tokenReq)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	// Handle API errors
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, WrapUnauthorized(fmt.Errorf("refresh token invalid or expired"))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("spotify API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var refreshResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int32  `json:"expires_in"`
		Scope       string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&refreshResp); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	// Return new token with preserved refresh token
	newToken := &Token{
		AccessToken:  refreshResp.AccessToken,
		TokenType:    refreshResp.TokenType,
		ExpiresIn:    refreshResp.ExpiresIn,
		RefreshToken: token.RefreshToken, // Keep the original refresh token
		Scope:        refreshResp.Scope,
		Timestamp:    time.Now().Unix(),
	}

	return newToken, nil
}

// EncryptToken encrypts a token using AES-256-GCM
func (am *AuthManager) EncryptToken(token *Token) (string, error) {
	// Serialize token to JSON
	data, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(am.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM instance
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// Return base64-encoded ciphertext
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptToken decrypts a token encrypted with AES-256-GCM
func (am *AuthManager) DecryptToken(encryptedData string) (*Token, error) {
	// Decode base64
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted data: %w", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(am.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM instance
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce from ciphertext
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt token: %w", err)
	}

	// Unmarshal token
	var token Token
	if err := json.Unmarshal(plaintext, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}

// IsTokenExpiringSoon checks if token is expiring soon
func (am *AuthManager) IsTokenExpiringSoon(token *Token, bufferSeconds int32) bool {
	expirationTime := time.Unix(token.Timestamp, 0).Add(time.Duration(token.ExpiresIn) * time.Second)
	checkTime := time.Now().Add(time.Duration(bufferSeconds) * time.Second)
	return checkTime.After(expirationTime)
}

// StoreEncryptedToken stores an encrypted token in the token store
func (am *AuthManager) StoreEncryptedToken(ctx context.Context, key string, token *Token) error {
	encrypted, err := am.EncryptToken(token)
	if err != nil {
		return fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Store encrypted token as a new token object
	storageToken := &Token{
		AccessToken: encrypted, // Store encrypted data in AccessToken field
		Timestamp:   time.Now().Unix(),
	}

	return am.tokenStore.StoreToken(ctx, key, storageToken)
}

// RetrieveDecryptedToken retrieves and decrypts a token from the token store
func (am *AuthManager) RetrieveDecryptedToken(ctx context.Context, key string) (*Token, error) {
	storageToken, err := am.tokenStore.RetrieveToken(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve token from store: %w", err)
	}

	if storageToken == nil {
		return nil, fmt.Errorf("token not found")
	}

	// Decrypt the token
	token, err := am.DecryptToken(storageToken.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt token: %w", err)
	}

	return token, nil
}

// Helper functions

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	token := make([]byte, length)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(token), nil
}

// formatScopes formats scopes slice into space-separated string
func formatScopes(scopes []string) string {
	result := ""
	for i, scope := range scopes {
		if i > 0 {
			result += " "
		}
		result += scope
	}
	return result
}
