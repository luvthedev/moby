package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"cloud.google.com/go/storage"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/moby/moby/errdefs"
)

// Token represents a Spotify API token with metadata
type Token struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    int32
	RefreshToken string
	Scope        string
	Timestamp    int64
}

// TokenStore abstracts the storage backend for Spotify authentication tokens
// It supports multiple cloud storage backends including AWS S3, Google Cloud Storage, and Azure Blob
type TokenStore interface {
	// StoreToken saves a token to the backing storage
	// The key parameter identifies the token (e.g., user ID, session ID)
	// The context parameter allows for cancellation and timeouts
	StoreToken(ctx context.Context, key string, token *Token) error

	// RetrieveToken retrieves a stored token from the backing storage
	// It returns the token and any error encountered
	RetrieveToken(ctx context.Context, key string) (*Token, error)

	// DeleteToken removes a token from the backing storage
	DeleteToken(ctx context.Context, key string) error

	// TokenExists checks if a token exists in the backing storage
	TokenExists(ctx context.Context, key string) (bool, error)

	// Close closes the token store and releases any resources
	Close() error
}

// S3TokenStore implements TokenStore using AWS S3
type S3TokenStore struct {
	bucketName string
	client     *s3.Client
}

// NewS3TokenStore creates a new S3-based token store
// client: configured AWS S3 client
// bucketName: the S3 bucket name where tokens will be stored
func NewS3TokenStore(client *s3.Client, bucketName string) *S3TokenStore {
	return &S3TokenStore{
		bucketName: bucketName,
		client:     client,
	}
}

// StoreToken saves a token to S3 with encryption at rest
func (s *S3TokenStore) StoreToken(ctx context.Context, key string, token *Token) error {
	data, err := json.Marshal(token)
	if err != nil {
		return errdefs.System(fmt.Errorf("failed to marshal token: %w", err))
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
		// Enable server-side encryption for security
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	})
	if err != nil {
		return errdefs.System(fmt.Errorf("failed to store token in S3: %w", err))
	}

	return nil
}

// RetrieveToken retrieves a token from S3
func (s *S3TokenStore) RetrieveToken(ctx context.Context, key string) (*Token, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, errdefs.NotFound(fmt.Errorf("token not found: %w", err))
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, errdefs.System(fmt.Errorf("failed to read token from S3: %w", err))
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, errdefs.System(fmt.Errorf("failed to unmarshal token: %w", err))
	}

	return &token, nil
}

// DeleteToken removes a token from S3
func (s *S3TokenStore) DeleteToken(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return errdefs.System(fmt.Errorf("failed to delete token from S3: %w", err))
	}
	return nil
}

// TokenExists checks if a token exists in S3
func (s *S3TokenStore) TokenExists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

// Close closes the S3 token store
func (s *S3TokenStore) Close() error {
	return nil
}

// GCSTokenStore implements TokenStore using Google Cloud Storage
type GCSTokenStore struct {
	bucketName string
	client     *storage.Client
}

// NewGCSTokenStore creates a new GCS-based token store
// client: configured Google Cloud Storage client
// bucketName: the GCS bucket name where tokens will be stored
func NewGCSTokenStore(client *storage.Client, bucketName string) *GCSTokenStore {
	return &GCSTokenStore{
		bucketName: bucketName,
		client:     client,
	}
}

// StoreToken saves a token to GCS with encryption at rest
func (g *GCSTokenStore) StoreToken(ctx context.Context, key string, token *Token) error {
	data, err := json.Marshal(token)
	if err != nil {
		return errdefs.System(fmt.Errorf("failed to marshal token: %w", err))
	}

	wc := g.client.Bucket(g.bucketName).Object(key).NewWriter(ctx)
	wc.ContentType = "application/json"

	if _, err := wc.Write(data); err != nil {
		return errdefs.System(fmt.Errorf("failed to store token in GCS: %w", err))
	}

	if err := wc.Close(); err != nil {
		return errdefs.System(fmt.Errorf("failed to close GCS writer: %w", err))
	}

	return nil
}

// RetrieveToken retrieves a token from GCS
func (g *GCSTokenStore) RetrieveToken(ctx context.Context, key string) (*Token, error) {
	rc, err := g.client.Bucket(g.bucketName).Object(key).NewReader(ctx)
	if err != nil {
		return nil, errdefs.NotFound(fmt.Errorf("token not found: %w", err))
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, errdefs.System(fmt.Errorf("failed to read token from GCS: %w", err))
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, errdefs.System(fmt.Errorf("failed to unmarshal token: %w", err))
	}

	return &token, nil
}

// DeleteToken removes a token from GCS
func (g *GCSTokenStore) DeleteToken(ctx context.Context, key string) error {
	if err := g.client.Bucket(g.bucketName).Object(key).Delete(ctx); err != nil {
		return errdefs.System(fmt.Errorf("failed to delete token from GCS: %w", err))
	}
	return nil
}

// TokenExists checks if a token exists in GCS
func (g *GCSTokenStore) TokenExists(ctx context.Context, key string) (bool, error) {
	_, err := g.client.Bucket(g.bucketName).Object(key).Attrs(ctx)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// Close closes the GCS token store
func (g *GCSTokenStore) Close() error {
	return g.client.Close()
}

// AzureBlobTokenStore implements TokenStore using Azure Blob Storage
type AzureBlobTokenStore struct {
	containerName string
	client        *azblob.ContainerClient
}

// NewAzureBlobTokenStore creates a new Azure Blob Storage-based token store
// client: configured Azure blob container client
// containerName: the Azure container name where tokens will be stored
func NewAzureBlobTokenStore(client *azblob.ContainerClient) *AzureBlobTokenStore {
	return &AzureBlobTokenStore{
		containerName: client.ContainerName(),
		client:        client,
	}
}

// StoreToken saves a token to Azure Blob Storage with encryption at rest
func (a *AzureBlobTokenStore) StoreToken(ctx context.Context, key string, token *Token) error {
	data, err := json.Marshal(token)
	if err != nil {
		return errdefs.System(fmt.Errorf("failed to marshal token: %w", err))
	}

	blobClient := a.client.NewBlockBlobClient(key)
	_, err = blobClient.Upload(ctx, io.NopCloser(bytes.NewReader(data)), nil)
	if err != nil {
		return errdefs.System(fmt.Errorf("failed to store token in Azure: %w", err))
	}

	return nil
}

// RetrieveToken retrieves a token from Azure Blob Storage
func (a *AzureBlobTokenStore) RetrieveToken(ctx context.Context, key string) (*Token, error) {
	blobClient := a.client.NewBlockBlobClient(key)
	download, err := blobClient.Download(ctx, nil)
	if err != nil {
		return nil, errdefs.NotFound(fmt.Errorf("token not found: %w", err))
	}
	defer download.Body.Close()

	data, err := io.ReadAll(download.Body)
	if err != nil {
		return nil, errdefs.System(fmt.Errorf("failed to read token from Azure: %w", err))
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, errdefs.System(fmt.Errorf("failed to unmarshal token: %w", err))
	}

	return &token, nil
}

// DeleteToken removes a token from Azure Blob Storage
func (a *AzureBlobTokenStore) DeleteToken(ctx context.Context, key string) error {
	blobClient := a.client.NewBlockBlobClient(key)
	_, err := blobClient.Delete(ctx, nil)
	if err != nil {
		return errdefs.System(fmt.Errorf("failed to delete token from Azure: %w", err))
	}
	return nil
}

// TokenExists checks if a token exists in Azure Blob Storage
func (a *AzureBlobTokenStore) TokenExists(ctx context.Context, key string) (bool, error) {
	blobClient := a.client.NewBlockBlobClient(key)
	_, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// Close closes the Azure Blob token store
func (a *AzureBlobTokenStore) Close() error {
	return nil
}
