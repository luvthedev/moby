// Package spotify provides gRPC service definitions and helpers for Spotify API integration
//
// This package includes:
// - gRPC service definitions for Spotify authentication and track data retrieval
// - Protocol Buffer message definitions for authentication, track metadata, and audio features
// - Custom error types compatible with moby/errdefs
// - TokenStore interface for abstracting cloud storage backends (AWS S3, GCS, Azure Blob)
//
// Usage example:
//
//	// Create a token store (e.g., AWS S3)
//	tokenStore := NewS3TokenStore("my-bucket", "us-west-2")
//	defer tokenStore.Close()
//
//	// Store a token
//	token := &Token{
//	    AccessToken: "...",
//	    RefreshToken: "...",
//	}
//	err := tokenStore.StoreToken(ctx, "user-123", token)
//
//	// Retrieve a token
//	retrievedToken, err := tokenStore.RetrieveToken(ctx, "user-123")
package spotify
