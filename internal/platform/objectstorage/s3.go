package objectstorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

type ObjectStorage interface {
	PutObject(ctx context.Context, key string, body io.Reader, contentType string) error
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	PresignGetObject(ctx context.Context, key string, expiry time.Duration) (string, error)
	EnsureBucket(ctx context.Context) error
	Ping(ctx context.Context) error
}

var _ ObjectStorage = (*Client)(nil)

type Client struct {
	s3     *s3.Client
	bucket string
}

func New(ctx context.Context, cfg config.ObjectStorageConfig) (*Client, error) {
	resolver := s3.EndpointResolverFromURL(endpointURL(cfg.Endpoint, cfg.UseSSL))
	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.ForcePathStyle
		o.EndpointResolver = resolver
	})
	return &Client{s3: client, bucket: cfg.Bucket}, nil
}

func (c *Client) PutObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:      &c.bucket,
		Key:         &key,
		Body:        body,
		ContentType: &contentType,
	}
	_, err := c.s3.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

func (c *Client) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: &c.bucket,
		Key:    &key,
	}
	resp, err := c.s3.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}
	return resp.Body, nil
}

func (c *Client) PresignGetObject(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(c.s3)
	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &c.bucket,
		Key:    &key,
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("presign get object %s: %w", key, err)
	}
	return req.URL, nil
}

func (c *Client) createBucket(ctx context.Context) error {
	_, err := c.s3.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &c.bucket})
	if err != nil {
		return fmt.Errorf("create bucket %s: %w", c.bucket, err)
	}
	return nil
}

func (c *Client) EnsureBucket(ctx context.Context) error {
	if c.bucket == "" {
		return errors.New("object storage bucket is required")
	}
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &c.bucket})
	if err == nil {
		return nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchBucket") {
		return c.createBucket(ctx)
	}

	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == http.StatusNotFound {
		return c.createBucket(ctx)
	}

	return fmt.Errorf("head bucket %s: %w", c.bucket, err)
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &c.bucket})
	if err != nil {
		return fmt.Errorf("head bucket %s: %w", c.bucket, err)
	}
	return nil
}

func endpointURL(endpoint string, useSSL bool) string {
	if endpoint == "" {
		return endpoint
	}
	scheme := "http://"
	if useSSL {
		scheme = "https://"
	}
	return scheme + endpoint
}
