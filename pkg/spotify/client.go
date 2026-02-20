package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// SpotifyAPIClient handles communication with the Spotify Web API
type SpotifyAPIClient struct {
	httpClient  *http.Client
	rateLimiter *RateLimiter
	baseURL     string
}

// NewSpotifyAPIClient creates a new Spotify API client with rate limiting
func NewSpotifyAPIClient() *SpotifyAPIClient {
	return &SpotifyAPIClient{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: NewRateLimiter(180, time.Minute), // 180 requests per minute
		baseURL:     "https://api.spotify.com/v1",
	}
}

// TrackMetadataResponse represents the Spotify API response for track metadata
type TrackMetadataResponse struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Duration         int32                  `json:"duration_ms"`
	Explicit         bool                   `json:"explicit"`
	ExternalIDs      map[string]string      `json:"external_ids"`
	ExternalURLs     map[string]string      `json:"external_urls"`
	URI              string                 `json:"uri"`
	Popularity       int32                  `json:"popularity"`
	PreviewURL       string                 `json:"preview_url"`
	TrackNumber      int32                  `json:"track_number"`
	DiscNumber       int32                  `json:"disc_number"`
	Album            AlbumResponse          `json:"album"`
	Artists          []ArtistResponse       `json:"artists"`
}

// AlbumResponse represents album info in API response
type AlbumResponse struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	ReleaseDate  string           `json:"release_date"`
	URI          string           `json:"uri"`
	Images       []ImageResponse  `json:"images"`
	Artists      []ArtistResponse `json:"artists"`
}

// ImageResponse represents image info in API response
type ImageResponse struct {
	URL    string `json:"url"`
	Height int32  `json:"height"`
	Width  int32  `json:"width"`
}

// ArtistResponse represents artist info in API response
type ArtistResponse struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	URI              string            `json:"uri"`
	ExternalURLs     map[string]string `json:"external_urls"`
}

// AudioFeaturesResponse represents the Spotify API response for audio features
type AudioFeaturesResponse struct {
	ID               string  `json:"id"`
	Acousticness     float32 `json:"acousticness"`
	Danceability     float32 `json:"danceability"`
	Duration         int32   `json:"duration_ms"`
	Energy           float32 `json:"energy"`
	Instrumentalness float32 `json:"instrumentalness"`
	Key              int32   `json:"key"`
	Liveness         float32 `json:"liveness"`
	Loudness         float32 `json:"loudness"`
	Mode             int32   `json:"mode"`
	Speechiness      float32 `json:"speechiness"`
	Tempo            float32 `json:"tempo"`
	TimeSignature    int32   `json:"time_signature"`
	Valence          float32 `json:"valence"`
}

// FetchTrackMetadata fetches track metadata from Spotify API
func (c *SpotifyAPIClient) FetchTrackMetadata(ctx context.Context, trackID string, accessToken string) (*TrackMetadataResponse, error) {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, RateLimited(fmt.Errorf("rate limit exceeded: %w", err))
	}

	url := fmt.Sprintf("%s/tracks/%s", c.baseURL, trackID)
	resp, err := c.doRequest(ctx, "GET", url, accessToken, nil)
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, fmt.Errorf("request timeout: %w", err)
		}
		return nil, fmt.Errorf("failed to fetch track metadata: %w", err)
	}

	var result TrackMetadataResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse track metadata response: %w", err)
	}

	return &result, nil
}

// FetchAudioFeatures fetches audio features from Spotify API
func (c *SpotifyAPIClient) FetchAudioFeatures(ctx context.Context, trackID string, accessToken string) (*AudioFeaturesResponse, error) {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, RateLimited(fmt.Errorf("rate limit exceeded: %w", err))
	}

	url := fmt.Sprintf("%s/audio-features/%s", c.baseURL, trackID)
	resp, err := c.doRequest(ctx, "GET", url, accessToken, nil)
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, fmt.Errorf("request timeout: %w", err)
		}
		return nil, fmt.Errorf("failed to fetch audio features: %w", err)
	}

	var result AudioFeaturesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse audio features response: %w", err)
	}

	return &result, nil
}

// doRequest performs an HTTP request to the Spotify API
func (c *SpotifyAPIClient) doRequest(ctx context.Context, method string, url string, accessToken string, body []byte) ([]byte, error) {
	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle HTTP errors
	if resp.StatusCode >= 400 {
		var errResp struct {
			Error struct {
				Status  int    `json:"status"`
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(respBody, &errResp)

		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, TokenExpired(fmt.Errorf("unauthorized: token may be expired"))
		case http.StatusTooManyRequests:
			return nil, RateLimited(fmt.Errorf("rate limit exceeded from Spotify API"))
		case http.StatusNotFound:
			return nil, fmt.Errorf("resource not found")
		default:
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error.Message)
		}
	}

	return respBody, nil
}

// RateLimiter implements a rate limiter with exponential backoff for queued requests
type RateLimiter struct {
	maxRequests  int
	timeWindow   time.Duration
	requests     []time.Time
	waitCh       chan struct{}
	mu           sync.Mutex
	backoffBase  time.Duration
	maxBackoff   time.Duration
	currentRetry int
}

// NewRateLimiter creates a new rate limiter
// maxRequests: maximum number of requests allowed in the time window
// timeWindow: the time window (e.g., 1 minute for Spotify's 180 requests/minute)
func NewRateLimiter(maxRequests int, timeWindow time.Duration) *RateLimiter {
	return &RateLimiter{
		maxRequests: maxRequests,
		timeWindow:  timeWindow,
		requests:    make([]time.Time, 0, maxRequests),
		waitCh:      make(chan struct{}, 1),
		backoffBase: 100 * time.Millisecond,
		maxBackoff:  30 * time.Second,
	}
}

// Wait blocks until a request can be made within the rate limit
// Implements exponential backoff for queued requests
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Remove requests older than the time window
	for len(rl.requests) > 0 && now.Sub(rl.requests[0]) > rl.timeWindow {
		rl.requests = rl.requests[1:]
	}

	// If we haven't reached the limit, allow the request immediately
	if len(rl.requests) < rl.maxRequests {
		rl.requests = append(rl.requests, now)
		rl.currentRetry = 0
		return nil
	}

	// Calculate wait time until the oldest request leaves the window
	oldestRequest := rl.requests[0]
	waitTime := oldestRequest.Add(rl.timeWindow).Sub(now)

	// Add exponential backoff
	backoff := rl.calculateBackoff()
	totalWait := waitTime + backoff

	// Check context deadline
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(totalWait):
		// After waiting, add the new request
		rl.requests = append(rl.requests, time.Now())
		rl.currentRetry = 0
		return nil
	}
}

// calculateBackoff implements exponential backoff with jitter
func (rl *RateLimiter) calculateBackoff() time.Duration {
	// Exponential backoff: base * 2^retry
	backoff := rl.backoffBase
	for i := 0; i < rl.currentRetry; i++ {
		backoff *= 2
		if backoff > rl.maxBackoff {
			backoff = rl.maxBackoff
			break
		}
	}
	rl.currentRetry++
	return backoff
}

// Close shuts down the rate limiter
func (rl *RateLimiter) Close() error {
	return nil
}
