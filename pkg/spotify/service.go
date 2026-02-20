package spotify

import (
	"context"
	"fmt"

	pb "github.com/moby/moby/pkg/spotify/v1"
)

// SpotifyServiceServer implements the gRPC SpotifyService
type SpotifyServiceServer struct {
	pb.UnimplementedSpotifyServiceServer
	authManager   *AuthManager
	tokenRefresh  *TokenRefresher
	tokenStore    TokenStore
	apiClient     *SpotifyAPIClient
}

// NewSpotifyServiceServer creates a new Spotify service server
func NewSpotifyServiceServer(authMgr *AuthManager, tokenRefresher *TokenRefresher, store TokenStore) *SpotifyServiceServer {
	return &SpotifyServiceServer{
		authManager:  authMgr,
		tokenRefresh: tokenRefresher,
		tokenStore:   store,
		apiClient:    NewSpotifyAPIClient(),
	}
}

// GetAuthorizationURL generates a Spotify authorization URL for the user to visit
// Returns the authorization URL and a state token for CSRF protection
func (s *SpotifyServiceServer) GetAuthorizationURL(ctx context.Context, req *pb.GetAuthorizationURLRequest) (*pb.GetAuthorizationURLResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	authURL, stateToken, err := s.authManager.GetAuthorizationURL(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate authorization URL: %w", err)
	}

	return &pb.GetAuthorizationURLResponse{
		AuthUrl:    authURL,
		StateToken: stateToken,
	}, nil
}

// Authenticate performs the OAuth code exchange to get access and refresh tokens
func (s *SpotifyServiceServer) Authenticate(ctx context.Context, req *pb.AuthRequest) (*pb.AuthResponse, error) {
	// Validate the authorization code
	if req.Code == "" {
		return nil, fmt.Errorf("authorization code is required")
	}

	// Exchange authorization code for tokens
	token, err := s.authManager.ExchangeCodeForToken(ctx, req.Code)
	if err != nil {
		if IsAuthFailed(err) {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// Store the token (encrypted)
	if err := s.authManager.StoreEncryptedToken(ctx, req.UserId, token); err != nil {
		return nil, fmt.Errorf("failed to store token: %w", err)
	}

	// Start automatic token refresh
	if err := s.tokenRefresh.StartAutoRefresh(ctx, req.UserId); err != nil {
		return nil, fmt.Errorf("failed to start token refresh: %w", err)
	}

	return &pb.AuthResponse{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		ExpiresIn:    token.ExpiresIn,
		RefreshToken: token.RefreshToken,
		Scope:        token.Scope,
		Timestamp:    token.Timestamp,
	}, nil
}

// RefreshToken refreshes an expired access token using the refresh token
func (s *SpotifyServiceServer) RefreshToken(ctx context.Context, req *pb.TokenRefreshRequest) (*pb.TokenRefreshResponse, error) {
	// Retrieve stored token or use provided refresh token
	var token *Token
	var err error

	if req.RefreshToken != "" {
		// Use provided refresh token
		token = &Token{
			RefreshToken: req.RefreshToken,
		}
	} else if req.UserId != "" {
		// Retrieve stored token
		token, err = s.authManager.RetrieveDecryptedToken(ctx, req.UserId)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve stored token: %w", err)
		}
	} else {
		return nil, fmt.Errorf("either refresh_token or user_id must be provided")
	}

	if token == nil {
		return nil, fmt.Errorf("token not found")
	}

	// Refresh the token
	newToken, err := s.authManager.RefreshAccessToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Store refreshed token if user_id provided
	if req.UserId != "" {
		if err := s.authManager.StoreEncryptedToken(ctx, req.UserId, newToken); err != nil {
			return nil, fmt.Errorf("failed to store refreshed token: %w", err)
		}
	}

	return &pb.TokenRefreshResponse{
		AccessToken: newToken.AccessToken,
		TokenType:   newToken.TokenType,
		ExpiresIn:   newToken.ExpiresIn,
		Scope:       newToken.Scope,
		Timestamp:   newToken.Timestamp,
	}, nil
}

// GetMetadata retrieves track metadata from Spotify API
// Implements AC1: Track metadata (name, artist, duration, preview URL) is retrieved from Spotify API and returned via gRPC
func (s *SpotifyServiceServer) GetMetadata(ctx context.Context, req *pb.GetMetadataRequest) (*pb.TrackMetadata, error) {
	if req == nil || req.TrackId == "" {
		return nil, fmt.Errorf("track_id is required")
	}

	// Check if token is provided directly or needs to be refreshed
	accessToken := req.AccessToken
	if accessToken == "" {
		return nil, fmt.Errorf("access_token is required")
	}

	// Fetch metadata from Spotify API
	metadata, err := s.apiClient.FetchTrackMetadata(ctx, req.TrackId, accessToken)
	if err != nil {
		// Check if token is expired and needs refresh
		if IsTokenExpired(err) {
			return nil, TokenExpired(fmt.Errorf("access token expired: %w", err))
		}
		return nil, fmt.Errorf("failed to fetch track metadata: %w", err)
	}

	// Map API response to protobuf message
	return mapMetadataToProto(metadata), nil
}

// GetFeatures retrieves audio features for a track from Spotify API
// Implements AC2: Audio features (tempo, key, energy, danceability, valence) are fetched and mapped to protobuf messages
func (s *SpotifyServiceServer) GetFeatures(ctx context.Context, req *pb.GetFeaturesRequest) (*pb.AudioFeatures, error) {
	if req == nil || req.TrackId == "" {
		return nil, fmt.Errorf("track_id is required")
	}

	// Check if token is provided directly or needs to be refreshed
	accessToken := req.AccessToken
	if accessToken == "" {
		return nil, fmt.Errorf("access_token is required")
	}

	// Fetch audio features from Spotify API
	features, err := s.apiClient.FetchAudioFeatures(ctx, req.TrackId, accessToken)
	if err != nil {
		// Check if token is expired and needs refresh
		if IsTokenExpired(err) {
			return nil, TokenExpired(fmt.Errorf("access token expired: %w", err))
		}
		return nil, fmt.Errorf("failed to fetch audio features: %w", err)
	}

	// Map API response to protobuf message
	return mapFeaturesToProto(features), nil
}

// GetPreviewURL retrieves the preview URL for a track
// Implements AC4: Preview unavailable error is returned when a track has no preview URL instead of returning null
func (s *SpotifyServiceServer) GetPreviewURL(ctx context.Context, req *pb.GetPreviewURLRequest) (*pb.PreviewURLResponse, error) {
	if req == nil || req.TrackId == "" {
		return nil, fmt.Errorf("track_id is required")
	}

	// Check if token is provided directly or needs to be refreshed
	accessToken := req.AccessToken
	if accessToken == "" {
		return nil, fmt.Errorf("access_token is required")
	}

	// Fetch track metadata to get preview URL
	metadata, err := s.apiClient.FetchTrackMetadata(ctx, req.TrackId, accessToken)
	if err != nil {
		// Check if token is expired and needs refresh
		if IsTokenExpired(err) {
			return nil, TokenExpired(fmt.Errorf("access token expired: %w", err))
		}
		return nil, fmt.Errorf("failed to fetch track metadata: %w", err)
	}

	// Check if preview URL is available
	if metadata.PreviewURL == "" {
		return nil, PreviewUnavailable(fmt.Errorf("preview URL is not available for track %s", req.TrackId))
	}

	return &pb.PreviewURLResponse{
		TrackId:     req.TrackId,
		PreviewUrl:  metadata.PreviewURL,
		IsAvailable: true,
	}, nil
}

// mapMetadataToProto converts API response to protobuf TrackMetadata message
func mapMetadataToProto(apiResp *TrackMetadataResponse) *pb.TrackMetadata {
	// Map artists
	artists := make([]*pb.Artist, 0, len(apiResp.Artists))
	for _, artist := range apiResp.Artists {
		artists = append(artists, &pb.Artist{
			Id:                   artist.ID,
			Name:                 artist.Name,
			Uri:                  artist.URI,
			ExternalUrlsSpotify:  artist.ExternalURLs["spotify"],
		})
	}

	// Map album artists
	albumArtists := make([]*pb.Artist, 0, len(apiResp.Album.Artists))
	for _, artist := range apiResp.Album.Artists {
		albumArtists = append(albumArtists, &pb.Artist{
			Id:                   artist.ID,
			Name:                 artist.Name,
			Uri:                  artist.URI,
			ExternalUrlsSpotify:  artist.ExternalURLs["spotify"],
		})
	}

	// Map album images
	images := make([]*pb.Image, 0, len(apiResp.Album.Images))
	for _, img := range apiResp.Album.Images {
		images = append(images, &pb.Image{
			Url:    img.URL,
			Height: img.Height,
			Width:  img.Width,
		})
	}

	// Map album
	album := &pb.Album{
		Id:           apiResp.Album.ID,
		Name:         apiResp.Album.Name,
		ReleaseDate:  apiResp.Album.ReleaseDate,
		Uri:          apiResp.Album.URI,
		Images:       images,
		Artists:      albumArtists,
	}

	// Create and return protobuf message
	return &pb.TrackMetadata{
		Id:                   apiResp.ID,
		Name:                 apiResp.Name,
		Album:                album,
		Artists:              artists,
		DurationMs:           apiResp.Duration,
		Explicit:             apiResp.Explicit,
		ExternalIdsIsrc:      apiResp.ExternalIDs["isrc"],
		ExternalUrlsSpotify:  apiResp.ExternalURLs["spotify"],
		Uri:                  apiResp.URI,
		Popularity:           apiResp.Popularity,
		PreviewUrl:           apiResp.PreviewURL,
		TrackNumber:          apiResp.TrackNumber,
		DiscNumber:           apiResp.DiscNumber,
	}
}

// mapFeaturesToProto converts API response to protobuf AudioFeatures message
func mapFeaturesToProto(apiResp *AudioFeaturesResponse) *pb.AudioFeatures {
	return &pb.AudioFeatures{
		Id:               apiResp.ID,
		Acousticness:     apiResp.Acousticness,
		Danceability:     apiResp.Danceability,
		DurationMs:       apiResp.Duration,
		Energy:           apiResp.Energy,
		Instrumentalness: apiResp.Instrumentalness,
		Key:              apiResp.Key,
		Liveness:         apiResp.Liveness,
		Loudness:         apiResp.Loudness,
		Mode:             apiResp.Mode,
		Speechiness:      apiResp.Speechiness,
		Tempo:            apiResp.Tempo,
		TimeSignature:    apiResp.TimeSignature,
		Valence:          apiResp.Valence,
	}
}

// GetStoredToken retrieves the currently stored token for a user (without exposing secrets in logs)
func (s *SpotifyServiceServer) GetStoredToken(ctx context.Context, userID string) (*Token, error) {
	return s.tokenRefresh.GetRefreshedToken(ctx, userID)
}

// ValidateStateToken validates the state token from OAuth callback
func (s *SpotifyServiceServer) ValidateStateToken(state string) error {
	return s.authManager.ValidateState(state)
}

// DeleteToken removes a user's token from storage
func (s *SpotifyServiceServer) DeleteToken(ctx context.Context, userID string) error {
	// Stop automatic refresh if running
	s.tokenRefresh.StopAutoRefresh(userID)

	// Delete token from storage
	return s.tokenStore.DeleteToken(ctx, userID)
}

// Close shuts down the service and releases resources
func (s *SpotifyServiceServer) Close() error {
	var errs []error

	if err := s.tokenRefresh.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close token refresher: %w", err))
	}

	if err := s.tokenStore.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close token store: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	return nil
}

// Helper method for stub implementation - not needed for actual gRPC calls
// This is used by service initialization
func (s *SpotifyServiceServer) mustEmbedUnimplementedSpotifyServiceServer() {
	// This is a compile-time marker
}
