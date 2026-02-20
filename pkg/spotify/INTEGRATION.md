# Spotify OAuth Integration Guide

This document explains how to use the Spotify OAuth authentication system with secure token storage and automatic refresh.

## Overview

The Spotify authentication package provides:

1. **OAuth Flow Management**: Secure OAuth 2.0 flow with CSRF protection via state tokens
2. **Token Encryption**: AES-256-GCM encryption for tokens at rest
3. **Cloud Storage Support**: AWS S3, Google Cloud Storage, and Azure Blob Storage backends
4. **Automatic Token Refresh**: Background service that automatically refreshes tokens before expiration
5. **gRPC Service**: Production-ready gRPC endpoints for authentication

## Security Features

### 1. CSRF Protection
- Each authorization request generates a unique, cryptographically secure state token
- State tokens have a 10-minute expiration by default
- State tokens are validated before exchanging the authorization code
- Invalid or expired state tokens are rejected

### 2. Encryption at Rest
- Access and refresh tokens are encrypted using AES-256-GCM
- Random nonce generated for each encryption
- Tokens are serialized and encrypted before storage
- Only the encryption key is needed to decrypt tokens

### 3. Secure Token Exchange
- Authorization codes are exchanged directly with Spotify servers (server-to-server)
- Client secrets are never exposed in URLs or logs
- Tokens are stored encrypted immediately after receipt
- Automatic refresh occurs without user interaction

### 4. Token Lifecycle Management
- Tokens are automatically refreshed before expiration
- Default buffer time: 5 minutes before actual expiration
- Background refresh checks every 5 minutes per token
- Failed refresh attempts are logged but don't block token retrieval

## Setup Instructions

### 1. Generate Encryption Key

```go
import "crypto/rand"

// Generate a 32-byte key for AES-256
encryptionKey := make([]byte, 32)
if _, err := rand.Read(encryptionKey); err != nil {
    log.Fatal(err)
}
// Store this key securely (e.g., in environment variables or secrets manager)
```

### 2. Create Token Store

#### AWS S3
```go
import (
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/moby/moby/pkg/spotify"
)

// Initialize S3 client (with proper configuration)
s3Client := s3.NewFromConfig(cfg)
tokenStore := spotify.NewS3TokenStore(s3Client, "spotify-tokens-bucket")
```

#### Google Cloud Storage
```go
import (
    "cloud.google.com/go/storage"
    "github.com/moby/moby/pkg/spotify"
)

// Initialize GCS client
gcsClient, err := storage.NewClient(ctx)
if err != nil {
    log.Fatal(err)
}
tokenStore := spotify.NewGCSTokenStore(gcsClient, "spotify-tokens-bucket")
```

#### Azure Blob Storage
```go
import (
    "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
    "github.com/moby/moby/pkg/spotify"
)

// Initialize Azure client
containerClient, err := azblob.NewContainerClient(containerURL, credential)
if err != nil {
    log.Fatal(err)
}
tokenStore := spotify.NewAzureBlobTokenStore(containerClient)
```

### 3. Initialize Auth Manager

```go
config := &spotify.OAuthConfig{
    ClientID:     "your-spotify-client-id",
    ClientSecret: "your-spotify-client-secret",
    RedirectURI:  "https://yourapp.com/spotify/callback",
    Scopes:       []string{"user-read-private", "user-read-email"},
}

authManager := spotify.NewAuthManager(config, tokenStore, encryptionKey)
```

### 4. Set Up Token Refresher

```go
// Create token refresher with 5-minute buffer and 5-minute check interval
tokenRefresher := spotify.NewTokenRefresher(authManager, tokenStore, 300, 5*time.Minute)
defer tokenRefresher.Close()
```

### 5. Implement OAuth Flow

#### Step 1: Initiate OAuth (User clicks "Login with Spotify")
```go
authURL, stateToken, err := authManager.GetAuthorizationURL(ctx)
if err != nil {
    return err
}

// Store stateToken in session for validation later
// Redirect user to authURL
return redirectToURL(authURL)
```

#### Step 2: Handle Callback
```go
// After user authorizes, Spotify redirects to your callback URL
// Extract code and state from query parameters
code := r.URL.Query().Get("code")
state := r.URL.Query().Get("state")

// Validate state token
if err := authManager.ValidateState(state); err != nil {
    return fmt.Errorf("invalid state token: %w", err)
}

// Exchange code for tokens
token, err := authManager.ExchangeCodeForToken(ctx, code)
if err != nil {
    return err
}

// Store encrypted token
userID := "user-123" // Get from your session/database
if err := authManager.StoreEncryptedToken(ctx, userID, token); err != nil {
    return err
}

// Start automatic refresh
if err := tokenRefresher.StartAutoRefresh(ctx, userID); err != nil {
    return err
}
```

## Usage Examples

### Get User's Token
```go
// Automatically refreshes if needed
token, err := tokenRefresher.GetRefreshedToken(ctx, userID)
if err != nil {
    return err
}

// Use token for API calls
spotifyAPICall(token.AccessToken)
```

### Manual Token Refresh
```go
// Refresh without storing
newToken, err := authManager.RefreshAccessToken(ctx, oldToken)
if err != nil {
    return err
}
```

### Clean Up User's Token
```go
// Stop auto-refresh and delete token
tokenRefresher.StopAutoRefresh(userID)
err := tokenStore.DeleteToken(ctx, userID)
```

## gRPC Service Usage

The package provides a gRPC service implementation:

```go
// Initialize service
service := spotify.NewSpotifyServiceServer(authManager, tokenRefresher, tokenStore)
defer service.Close()

// Register with gRPC server
pb.RegisterSpotifyServiceServer(grpcServer, service)

// Now clients can call:
// - GetAuthorizationURL(ctx, request)
// - Authenticate(ctx, request)
// - RefreshToken(ctx, request)
```

## Error Handling

### Auth Failures
```go
if spotify.IsAuthFailed(err) {
    // Handle authentication failure
    // Could be invalid credentials, network error, etc.
}
```

### Token Expiration
```go
if spotify.IsTokenExpired(err) {
    // Token has expired - request user to re-authenticate
}
```

### Rate Limiting
```go
if spotify.IsRateLimited(err) {
    // Spotify API rate limit exceeded
    // Implement exponential backoff retry logic
}
```

## Configuration Best Practices

1. **Encryption Key**:
   - Store in secure key management system (AWS KMS, Google Cloud KMS, Azure Key Vault)
   - Never hardcode or commit to version control
   - Rotate regularly

2. **OAuth Credentials**:
   - Store in environment variables or secrets manager
   - Use separate credentials for development and production
   - Regenerate if accidentally exposed

3. **Redirect URI**:
   - Match exactly with Spotify application settings
   - Use HTTPS in production
   - Whitelist allowed domains

4. **Scopes**:
   - Request only necessary scopes
   - Document why each scope is needed
   - Users can revoke individual scopes

## Troubleshooting

### State Token Validation Fails
- Ensure token is being validated within the 10-minute window
- Check that state token wasn't already used
- Verify OAuth callback includes state parameter

### Encryption/Decryption Errors
- Verify encryption key is correct (32 bytes for AES-256)
- Check that stored tokens weren't corrupted
- Ensure JSON serialization/deserialization works

### Token Refresh Fails
- Check refresh token hasn't been revoked by user
- Verify Spotify API connectivity
- Check rate limiting hasn't been exceeded
- Review error logs for specific Spotify API errors

## Testing

The package includes comprehensive tests in `auth_test.go`:

```bash
go test -v ./pkg/spotify
```

Test coverage includes:
- OAuth flow with state tokens
- CSRF protection validation
- Token encryption/decryption
- Storage backends (mock implementation)
- Token refresh logic
- Error handling

## Production Considerations

1. **Monitoring**: Log and alert on token refresh failures
2. **Metrics**: Track refresh rates, latency, and failure rates
3. **Audit**: Log all authentication events for security auditing
4. **Backup**: Consider backup strategies for token storage
5. **Disaster Recovery**: Plan for storage backend failures
6. **Scaling**: Use cloud storage backends for multi-instance deployments
