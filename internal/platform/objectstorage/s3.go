package objectstorage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

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
		_, createErr := c.s3.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &c.bucket})
		if createErr != nil {
			return fmt.Errorf("create bucket %s: %w", c.bucket, createErr)
		}
		return nil
	}

	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == http.StatusNotFound {
		_, createErr := c.s3.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &c.bucket})
		if createErr != nil {
			return fmt.Errorf("create bucket %s: %w", c.bucket, createErr)
		}
		return nil
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
