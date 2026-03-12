# Project
Moby enables developers and creators to generate and synthesize audio content while visualizing data in real-time through interactive web interfaces powered by WebSocket connections, streamlining the workflow for building audio-driven applications.
Stack: Go · Docker · Cloud SDKs (AWS, Google Cloud, Azure) · gRPC · Protocol Buffers
Patterns:
- Error Handling: Direct error checking with early returns and error propagation. Errors are checked immediately after operations and returned up the call stack. Uses standard Go error interface with custom error types from errdefs package.
- Testing Patterns: Uses Go's standard testing package with table-driven tests and helper functions. Tests are organized in _test.go files alongside implementation. Integration tests use testing utilities and assertion libraries like gotest.tools.
- Async Patterns: Uses goroutines and synchronization primitives for concurrency. Employs sync.Mutex and sync.Cond for thread-safe access to shared resources. Publisher-subscriber pattern used for event distribution.
- File Organization: Organized by functional domains with package-based structure. Related functionality grouped in packages (daemon, client, integration, pkg). Platform-specific code separated with _windows, _linux suffixes. Tests colocated with implementation in _test.go files.
- Code Style: Follows Go conventions with CamelCase for exported identifiers and lowercase for unexported. Functional options pattern used for configuration. Interfaces defined for abstraction. Comments provided for exported types and functions.

# Goal: Spotify Audio Source Integration
Integrate Spotify as an audio source within an audio generation and visualization platform built on Go, Docker, and cloud infrastructure. The integration should enable users to authenticate with Spotify, retrieve song metadata and pre-computed audio features (tempo, key, energy), and utilize 30-second preview clips as input for audio analysis and synthesis workflows. The implementation must work within Spotify's API constraints (preview-only access, no full audio downloads) and leverage the existing gRPC/Protocol Buffers architecture for inter-service communication. The solution should treat Spotify as a pluggable audio source alongside existing inputs, requiring minimal architectural changes to the current pipeline while maintaining low maintenance overhead.

# Your Task: Define gRPC service contract and protobuf messages for Spotify authentication and track data

## Description
Define the gRPC service contract and data structures for Spotify integration, including authentication state, track metadata, and audio features. This establishes the boundary between Spotify-specific logic and the rest of the Moby pipeline.

## Acceptance Criteria
- SpotifyService gRPC interface is defined with all required methods (authenticate, get metadata, get features, get preview URL, refresh token)
- Protobuf messages for authentication, track metadata, and audio features are defined and compile without errors
- Custom error types (AuthFailed, RateLimited, PreviewUnavailable, TokenExpired) are defined in errdefs-compatible style
- TokenStore interface abstracts cloud storage backend (AWS S3, Google Cloud Storage, or Azure Blob)

## Implementation Notes
- Define SpotifyService gRPC service with methods: AuthenticateUser, GetTrackMetadata, GetAudioFeatures, GetPreviewURL, RefreshToken.
- Create protobuf messages: AuthRequest (client_id, redirect_uri), AuthResponse (access_token, refresh_token, expires_at), TrackMetadata (id, name, artist, duration_ms, preview_url), AudioFeatures (tempo, key, energy, danceability, valence).
- Add SpotifyError custom error type in internal/spotify/types.go with codes: ErrAuthFailed, ErrRateLimited, ErrPreviewUnavailable, ErrTokenExpired.
- Define TokenStore interface in internal/spotify/types.go with methods: SaveToken(ctx, userID, token), GetToken(ctx, userID), DeleteToken(ctx, userID) to abstract cloud storage.
- Create internal/spotify/config.go with SpotifyConfig struct holding client_id, client_secret, redirect_uri, and cloud storage backend reference.
- Add protobuf option for Go package paths: option go_package = "github.com/moby/pkg/spotify".

## Completion
Verify your changes work — run relevant tests or checks appropriate for this project.

Then create `.codepoet/stories/09546035-85bf-419b-bbba-0bf3e96df755/done.json` with this exact structure:
```json
{
  "status": "completed",
  "summary": "<brief summary of what you did>",
  "files_changed": ["list", "of", "files"]
}
```
IMPORTANT: The file MUST be at exactly `.codepoet/stories/09546035-85bf-419b-bbba-0bf3e96df755/done.json`.
Do not create this file until you are fully done.
Do NOT perform any git operations (no git add, commit, or push).