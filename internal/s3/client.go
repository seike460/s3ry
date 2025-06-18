package s3

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// Client wraps AWS S3 client for basic S3/MinIO operations
type Client struct {
	session    *session.Session
	s3Client   *s3.S3
	uploader   *s3manager.Uploader
	downloader *s3manager.Downloader
}

// NewClient creates a new S3 client with the given region
func NewClient(region string) *Client {
	awsConfig := &aws.Config{
		Region: aws.String(region),
	}

	sess := session.Must(session.NewSession(awsConfig))

	return &Client{
		session:    sess,
		s3Client:   s3.New(sess),
		uploader:   s3manager.NewUploader(sess),
		downloader: s3manager.NewDownloader(sess),
	}
}

// NewClientWithEndpoint creates a new S3 client with custom endpoint (for MinIO)
func NewClientWithEndpoint(region, endpoint string, forcePathStyle bool) *Client {
	awsConfig := &aws.Config{
		Region:           aws.String(region),
		Endpoint:         aws.String(endpoint),
		S3ForcePathStyle: aws.Bool(forcePathStyle),
	}

	sess := session.Must(session.NewSession(awsConfig))

	return &Client{
		session:    sess,
		s3Client:   s3.New(sess),
		uploader:   s3manager.NewUploader(sess),
		downloader: s3manager.NewDownloader(sess),
	}
}

// Session returns the AWS session
func (c *Client) Session() *session.Session {
	return c.session
}

// S3Client returns the AWS S3 client
func (c *Client) S3Client() *s3.S3 {
	return c.s3Client
}

// S3 returns the AWS S3 client (implements worker.S3Client interface)
func (c *Client) S3() *s3.S3 {
	return c.s3Client
}

// Uploader returns the S3 uploader
func (c *Client) Uploader() *s3manager.Uploader {
	return c.uploader
}

// Downloader returns the S3 downloader
func (c *Client) Downloader() *s3manager.Downloader {
	return c.downloader
}

// GetBucketRegion determines the region of a given bucket
func (c *Client) GetBucketRegion(ctx context.Context, bucket string) (string, error) {
	return s3manager.GetBucketRegion(ctx, c.session, bucket, "us-east-1")
}
