package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"

	"nowdone/internal/config"
)

// S3Service uploads task/note attachments (images, video) to S3-compatible
// object storage. It supports two flows:
//
//   - Upload:     the API streams the bytes to S3 itself (legacy / fallback).
//   - PresignPut: the API only signs a short-lived PUT URL and the browser
//     uploads straight to S3, so large files never touch the API.
type S3Service struct {
	client        *s3.Client
	uploader      *manager.Uploader
	presigner     *s3.PresignClient
	bucket        string
	endpoint      string
	publicBaseURL string
	presignExpiry time.Duration
}

func NewS3Service(ctx context.Context, cfg *config.Config) (*S3Service, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
		o.UsePathStyle = true
	})

	// The presign client uses the endpoint with any explicit default port
	// (":443" / ":80") stripped: a browser sending the presigned PUT drops the
	// default port from the Host header, and SigV4 verification fails if the
	// signed host differs from the one actually sent.
	presignEndpoint := stripDefaultPort(cfg.S3Endpoint)
	presigner := s3.NewPresignClient(s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if presignEndpoint != "" {
			o.BaseEndpoint = aws.String(presignEndpoint)
		}
		o.UsePathStyle = true
	}))

	expiry := cfg.S3PresignExpiry
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	return &S3Service{
		client:        client,
		uploader:      manager.NewUploader(client),
		presigner:     presigner,
		bucket:        cfg.S3BucketName,
		endpoint:      strings.TrimRight(cfg.S3Endpoint, "/"),
		publicBaseURL: strings.TrimRight(cfg.S3PublicBaseURL, "/"),
		presignExpiry: expiry,
	}, nil
}

// HealthCheck verifies the configured bucket is reachable with the current
// credentials/endpoint. Callers may treat a failure as non-fatal (log a warning)
// so the API still starts, but a misconfiguration is visible immediately.
func (s *S3Service) HealthCheck(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err := s.client.HeadBucket(checkCtx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		return fmt.Errorf("head bucket %q at %s: %w", s.bucket, s.endpoint, err)
	}
	return nil
}

// Upload streams a file to S3 under attachments/{uuid}{ext} and returns its
// public URL. Kept as a fallback for when the browser cannot reach S3 directly
// (e.g. bucket CORS not yet configured).
func (s *S3Service) Upload(ctx context.Context, filename string, body io.Reader) (string, error) {
	key := objectKey(filename)

	uploadCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	_, err := s.uploader.Upload(uploadCtx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentTypeFor(filename, "")),
	})
	if err != nil {
		return "", fmt.Errorf("upload to s3: %w", err)
	}
	return s.publicURL(key), nil
}

// PresignedUpload is everything the client needs to upload one file straight to
// S3 and then hand the resulting URL back to us for storage.
type PresignedUpload struct {
	UploadURL   string        // PUT the file here
	FileURL     string        // browser-facing URL once the PUT succeeds
	Key         string        // S3 object key (for logging/debugging)
	ContentType string        // MUST be sent verbatim as the Content-Type header
	ExpiresIn   time.Duration // how long UploadURL stays valid
}

// PresignPut returns a short-lived presigned PUT URL so the client can upload an
// attachment directly to S3 without streaming it through the API.
//
// The signature covers the object key and the Content-Type, so the client must
// issue the PUT with exactly the ContentType returned here — any difference
// makes S3 reject the request with SignatureDoesNotMatch.
//
// The credentials in S3_ACCESS_KEY_ID / S3_SECRET_ACCESS_KEY must be allowed to
// perform s3:PutObject on the bucket (same permission Upload already relies on).
func (s *S3Service) PresignPut(ctx context.Context, filename, contentTypeHint string) (*PresignedUpload, error) {
	key := objectKey(filename)
	contentType := contentTypeFor(filename, contentTypeHint)

	req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(s.presignExpiry))
	if err != nil {
		return nil, fmt.Errorf("presign put object: %w", err)
	}

	return &PresignedUpload{
		UploadURL:   req.URL,
		FileURL:     s.publicURL(key),
		Key:         key,
		ContentType: contentType,
		ExpiresIn:   s.presignExpiry,
	}, nil
}

// DeleteObject removes a single object from the bucket. Deleting a key that no
// longer exists is treated as success: S3 delete is idempotent, and any
// provider that still surfaces NoSuchKey/NotFound means the same end state we
// want. Every other error is returned so the caller can abort its transaction.
func (s *S3Service) DeleteObject(ctx context.Context, key string) error {
	delCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err := s.client.DeleteObject(delCtx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err == nil || isNotFoundErr(err) {
		return nil
	}
	return fmt.Errorf("delete object %q: %w", key, err)
}

// isNotFoundErr reports whether err is S3's "object does not exist" response,
// in either its modeled (*types.NoSuchKey) or generic API-error form.
func isNotFoundErr(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	return false
}

// KeyFromURL reverses publicURL: it extracts the S3 object key from a stored
// attachment URL. ok is false when the URL does not belong to this bucket
// (foreign, legacy, or hand-edited) — the caller should then leave it alone.
func (s *S3Service) KeyFromURL(rawURL string) (key string, ok bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", false
	}

	// Prefixes publicURL could have produced, most specific first. The
	// stripped-port variant covers URLs signed by the presign client.
	prefixes := make([]string, 0, 3)
	if s.publicBaseURL != "" {
		prefixes = append(prefixes, s.publicBaseURL+"/")
	}
	prefixes = append(prefixes,
		s.endpoint+"/"+s.bucket+"/",
		stripDefaultPort(s.endpoint)+"/"+s.bucket+"/",
	)

	for _, p := range prefixes {
		after, found := strings.CutPrefix(rawURL, p)
		if !found || after == "" {
			continue
		}
		// Drop any query string / fragment (e.g. presigned GET params).
		after = strings.SplitN(after, "?", 2)[0]
		after = strings.SplitN(after, "#", 2)[0]
		if after == "" {
			return "", false
		}
		return after, true
	}
	return "", false
}

// objectKey builds the S3 key for an uploaded attachment, keeping the original
// file extension so content types and downloads behave correctly.
func objectKey(filename string) string {
	return fmt.Sprintf("attachments/%s%s", uuid.New().String(), filepath.Ext(filename))
}

// contentTypeFor resolves a usable MIME type: the caller's hint wins, then the
// file extension, then a binary fallback.
func contentTypeFor(filename, hint string) string {
	if hint != "" {
		return hint
	}
	if ct := mime.TypeByExtension(filepath.Ext(filename)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// publicURL builds the browser-facing URL for a stored object: an explicit
// public base URL when configured, otherwise the path-style URL against the API
// endpoint.
func (s *S3Service) publicURL(key string) string {
	if s.publicBaseURL != "" {
		return fmt.Sprintf("%s/%s", s.publicBaseURL, key)
	}
	return fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key)
}

// stripDefaultPort removes a trailing ":443" (https) or ":80" (http) from an
// endpoint URL so the signed Host header matches what a browser actually sends.
func stripDefaultPort(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")
	switch {
	case strings.HasPrefix(endpoint, "https://") && strings.HasSuffix(endpoint, ":443"):
		return strings.TrimSuffix(endpoint, ":443")
	case strings.HasPrefix(endpoint, "http://") && strings.HasSuffix(endpoint, ":80"):
		return strings.TrimSuffix(endpoint, ":80")
	default:
		return endpoint
	}
}
