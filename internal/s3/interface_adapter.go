package s3

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/seike460/s3ry/pkg/interfaces"
)

// Interface implementation methods for Client - MVP Basic Operations Only

// wrapError provides basic error wrapping for MVP
func (c *Client) wrapError(err error, operation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s failed: %w", operation, err)
}

// ListBuckets implements the interfaces.S3Client interface
func (c *Client) ListBuckets(ctx context.Context) ([]interfaces.BucketInfo, error) {
	if c.s3Client == nil {
		return nil, fmt.Errorf("S3 client not initialized")
	}

	result, err := c.s3Client.ListBucketsWithContext(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, c.wrapError(err, "ListBuckets")
	}

	buckets := make([]interfaces.BucketInfo, len(result.Buckets))
	for i, bucket := range result.Buckets {
		buckets[i] = interfaces.BucketInfo{
			Name:    aws.StringValue(bucket.Name),
			Created: aws.TimeValue(bucket.CreationDate),
		}
	}

	return buckets, nil
}

// ListObjects implements the interfaces.S3Client interface
func (c *Client) ListObjects(ctx context.Context, bucket, prefix, delimiter string) ([]interfaces.ObjectInfo, error) {
	if bucket == "" {
		return nil, fmt.Errorf("bucket name cannot be empty")
	}
	if c.s3Client == nil {
		return nil, fmt.Errorf("S3 client not initialized")
	}

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}

	// Add delimiter if provided
	if delimiter != "" {
		input.Delimiter = aws.String(delimiter)
	}

	result, err := c.s3Client.ListObjectsV2WithContext(ctx, input)
	if err != nil {
		return nil, c.wrapError(err, "ListObjects")
	}

	objects := make([]interfaces.ObjectInfo, len(result.Contents))
	for i, obj := range result.Contents {
		objects[i] = interfaces.ObjectInfo{
			Key:          aws.StringValue(obj.Key),
			Size:         aws.Int64Value(obj.Size),
			LastModified: aws.TimeValue(obj.LastModified),
			ETag:         strings.Trim(aws.StringValue(obj.ETag), "\""),
		}
	}

	return objects, nil
}

// UploadFile implements the interfaces.S3Client interface
func (c *Client) UploadFile(ctx context.Context, localPath, bucket, key string) error {
	if localPath == "" || bucket == "" || key == "" {
		return fmt.Errorf("localPath, bucket, and key cannot be empty")
	}
	if c.uploader == nil {
		return fmt.Errorf("S3 uploader not initialized")
	}

	file, err := os.Open(localPath)
	if err != nil {
		return c.wrapError(err, "UploadFile")
	}
	defer file.Close()

	_, err = c.uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})

	if err != nil {
		return c.wrapError(err, "UploadFile")
	}

	return nil
}

// DownloadFile implements the interfaces.S3Client interface
func (c *Client) DownloadFile(ctx context.Context, bucket, key, localPath string) error {
	if bucket == "" || key == "" || localPath == "" {
		return fmt.Errorf("bucket, key, and localPath cannot be empty")
	}
	if c.downloader == nil {
		return fmt.Errorf("S3 downloader not initialized")
	}

	file, err := os.Create(localPath)
	if err != nil {
		return c.wrapError(err, "DownloadFile")
	}
	defer file.Close()

	_, err = c.downloader.DownloadWithContext(ctx, file, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return c.wrapError(err, "DownloadFile")
	}

	return nil
}

// DeleteObject implements the interfaces.S3Client interface
func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	if bucket == "" || key == "" {
		return fmt.Errorf("bucket and key cannot be empty")
	}
	if c.s3Client == nil {
		return fmt.Errorf("S3 client not initialized")
	}

	_, err := c.s3Client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return c.wrapError(err, "DeleteObject")
	}

	return nil
}

// Extended operations - MVP dummy implementations
func (c *Client) CreateBucket(ctx context.Context, bucket string) error {
	return fmt.Errorf("CreateBucket not implemented in MVP")
}

func (c *Client) DeleteBucket(ctx context.Context, bucket string) error {
	return fmt.Errorf("DeleteBucket not implemented in MVP")
}

func (c *Client) GetBucketInfo(ctx context.Context, bucket string) (*interfaces.BucketInfo, error) {
	return nil, fmt.Errorf("GetBucketInfo not implemented in MVP")
}

func (c *Client) ListObjectsWithLimit(ctx context.Context, bucket, prefix string, maxKeys int) (*interfaces.ObjectList, error) {
	return nil, fmt.Errorf("ListObjectsWithLimit not implemented in MVP")
}

func (c *Client) UploadObject(ctx context.Context, bucket, key string, content []byte) (*interfaces.PutResult, error) {
	return nil, fmt.Errorf("UploadObject not implemented in MVP")
}

func (c *Client) GetPresignedURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	return "", fmt.Errorf("GetPresignedURL not implemented in MVP")
}

func (c *Client) StreamObject(ctx context.Context, bucket, key string) (*interfaces.Object, error) {
	return nil, fmt.Errorf("StreamObject not implemented in MVP")
}

func (c *Client) GetObjectMetadata(ctx context.Context, bucket, key string) (*interfaces.ObjectMetadata, error) {
	return nil, fmt.Errorf("GetObjectMetadata not implemented in MVP")
}
