package contribution

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"io"
	"strings"
	"time"
)

type S3Storage struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func NewS3Storage(ctx context.Context, endpoint, region, bucket, key, secret string, pathStyle bool) (*S3Storage, error) {
	if region == "" || bucket == "" || key == "" || secret == "" {
		return nil, fmt.Errorf("S3 region, bucket, access key, and secret key are required")
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(key, secret, "")))
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = pathStyle
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	return &S3Storage{client: client, presign: s3.NewPresignClient(client), bucket: bucket}, nil
}
func (s *S3Storage) Save(ctx context.Context, ext string, body io.Reader) (string, error) {
	key := "proofs/" + uuid.NewString() + strings.ToLower(ext)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: body})
	if err != nil {
		return "", fmt.Errorf("upload proof: %w", err)
	}
	return key, nil
}
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return err
}
func (s *S3Storage) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	out, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("sign proof URL: %w", err)
	}
	return out.URL, nil
}
