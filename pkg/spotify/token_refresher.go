package spotify

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TokenRefresher manages automatic token refresh before expiration
type TokenRefresher struct {
	authManager    *AuthManager
	tokenStore     TokenStore
	bufferSeconds  int32 // Seconds before actual expiration to refresh
	checkInterval  time.Duration
	activeRefresh  map[string]*time.Ticker
	mu             sync.RWMutex
	stopChan       chan struct{}
}

// NewTokenRefresher creates a new token refresher service
// bufferSeconds: number of seconds before actual expiration to refresh the token
// checkInterval: interval at which to check for tokens needing refresh
func NewTokenRefresher(authManager *AuthManager, tokenStore TokenStore, bufferSeconds int32, checkInterval time.Duration) *TokenRefresher {
	if bufferSeconds < 30 {
		bufferSeconds = 300 // Default to 5 minutes
	}
	if checkInterval < 1*time.Minute {
		checkInterval = 5 * time.Minute
	}

	return &TokenRefresher{
		authManager:   authManager,
		tokenStore:    tokenStore,
		bufferSeconds: bufferSeconds,
		checkInterval: checkInterval,
		activeRefresh: make(map[string]*time.Ticker),
		stopChan:      make(chan struct{}),
	}
}

// StartAutoRefresh starts automatic token refresh for a specific user/session
func (tr *TokenRefresher) StartAutoRefresh(ctx context.Context, key string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	// Check if already refreshing this token
	if _, exists := tr.activeRefresh[key]; exists {
		return nil // Already refreshing
	}

	// Retrieve current token to validate
	token, err := tr.tokenStore.RetrieveToken(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to retrieve token for auto-refresh: %w", err)
	}

	if token == nil {
		return fmt.Errorf("token not found")
	}

	// Start ticker for this token
	ticker := time.NewTicker(tr.checkInterval)
	tr.activeRefresh[key] = ticker

	// Start goroutine to handle refreshing
	go tr.refreshLoop(ctx, key, ticker)

	return nil
}

// StopAutoRefresh stops automatic token refresh for a specific user/session
func (tr *TokenRefresher) StopAutoRefresh(key string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if ticker, exists := tr.activeRefresh[key]; exists {
		ticker.Stop()
		delete(tr.activeRefresh, key)
	}
}

// refreshLoop handles the periodic refresh checking and execution
func (tr *TokenRefresher) refreshLoop(ctx context.Context, key string, ticker *time.Ticker) {
	defer func() {
		tr.mu.Lock()
		ticker.Stop()
		delete(tr.activeRefresh, key)
		tr.mu.Unlock()
	}()

	for {
		select {
		case <-tr.stopChan:
			return
		case <-ticker.C:
			// Attempt to refresh token if needed
			if err := tr.checkAndRefreshToken(ctx, key); err != nil {
				// Log error but continue trying
				// In production, would log this to observability system
				fmt.Printf("Token refresh check failed for key %s: %v\n", key, err)
			}
		}
	}
}

// checkAndRefreshToken checks if token needs refresh and performs it
func (tr *TokenRefresher) checkAndRefreshToken(ctx context.Context, key string) error {
	// Retrieve stored token
	storedToken, err := tr.tokenStore.RetrieveToken(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to retrieve token: %w", err)
	}

	if storedToken == nil {
		return fmt.Errorf("token not found")
	}

	// Check if token is expiring soon
	if !tr.authManager.IsTokenExpiringSoon(storedToken, tr.bufferSeconds) {
		return nil // Token still valid
	}

	// Token is expiring, refresh it
	newToken, err := tr.authManager.RefreshAccessToken(ctx, storedToken)
	if err != nil {
		return fmt.Errorf("failed to refresh access token: %w", err)
	}

	// Store the refreshed token
	if err := tr.tokenStore.StoreToken(ctx, key, newToken); err != nil {
		return fmt.Errorf("failed to store refreshed token: %w", err)
	}

	return nil
}

// GetRefreshedToken gets the current token, refreshing if necessary
func (tr *TokenRefresher) GetRefreshedToken(ctx context.Context, key string) (*Token, error) {
	token, err := tr.tokenStore.RetrieveToken(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve token: %w", err)
	}

	if token == nil {
		return nil, fmt.Errorf("token not found")
	}

	// Check if token needs refresh
	if tr.authManager.IsTokenExpiringSoon(token, tr.bufferSeconds) {
		newToken, err := tr.authManager.RefreshAccessToken(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}

		// Store refreshed token
		if err := tr.tokenStore.StoreToken(ctx, key, newToken); err != nil {
			return nil, fmt.Errorf("failed to store refreshed token: %w", err)
		}

		return newToken, nil
	}

	return token, nil
}

// Close stops all active refreshers and cleans up resources
func (tr *TokenRefresher) Close() error {
	close(tr.stopChan)

	tr.mu.Lock()
	defer tr.mu.Unlock()

	// Stop all active tickers
	for _, ticker := range tr.activeRefresh {
		ticker.Stop()
	}
	tr.activeRefresh = make(map[string]*time.Ticker)

	return nil
}

// GetActiveRefreshCount returns the number of actively refreshing tokens
func (tr *TokenRefresher) GetActiveRefreshCount() int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return len(tr.activeRefresh)
}
