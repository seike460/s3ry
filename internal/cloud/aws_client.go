package cloud

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// parseStorageClass converts AWS SDK StorageClass string to our StorageClass enum
func parseStorageClass(s3StorageClass string) StorageClass {
	switch strings.ToUpper(s3StorageClass) {
	case "STANDARD":
		return StorageClassStandard
	case "STANDARD_IA":
		return StorageClassStandardIA
	case "REDUCED_REDUNDANCY":
		return StorageClassReducedRedundancy
	case "GLACIER":
		return StorageClassGlacier
	case "GLACIER_IR":
		return StorageClassGlacierInstantRetrieval
	case "GLACIER_FR":
		return StorageClassGlacierFlexibleRetrieval
	case "DEEP_ARCHIVE":
		return StorageClassDeepArchive
	case "INTELLIGENT_TIERING":
		return StorageClassIntelligentTiering
	case "ONEZONE_IA":
		return StorageClassOnezone
	default:
		return StorageClassStandard
	}
}

// AWSClient implements StorageClient for AWS S3
type AWSClient struct {
	config   *CloudConfig
	logger   Logger
	region   string
	s3Client *s3.Client
}

// NewAWSClient creates a new AWS S3 client
func NewAWSClient(config *CloudConfig, logger Logger) (*AWSClient, error) {
	if config == nil {
		config = DefaultCloudConfig()
		config.Provider = ProviderAWS
	}

	// Create AWS config with region
	awsConfig := aws.Config{
		Region: config.Region,
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsConfig)

	client := &AWSClient{
		config:   config,
		logger:   logger,
		region:   config.Region,
		s3Client: s3Client,
	}

	return client, nil
}

// Provider information methods
func (c *AWSClient) GetProvider() CloudProvider {
	return ProviderAWS
}

func (c *AWSClient) GetRegion() string {
	return c.region
}

// Connection management
func (c *AWSClient) Connect(ctx context.Context) error {
	c.logger.Info("Connecting to AWS S3 in region %s", c.region)
	return c.HealthCheck(ctx)
}

func (c *AWSClient) Disconnect(ctx context.Context) error {
	c.logger.Info("Disconnecting from AWS S3")
	// TODO: Cleanup AWS SDK resources
	return nil
}

func (c *AWSClient) HealthCheck(ctx context.Context) error {
	c.logger.Debug("Performing AWS S3 health check")
	_, err := c.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	return err
}

// Bucket operations
func (c *AWSClient) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	c.logger.Debug("Listing AWS S3 buckets")

	resp, err := c.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	buckets := make([]BucketInfo, len(resp.Buckets))
	for i, bucket := range resp.Buckets {
		buckets[i] = BucketInfo{
			Name:         aws.ToString(bucket.Name),
			Region:       c.region,
			CreationDate: aws.ToTime(bucket.CreationDate),
		}
	}

	return buckets, nil
}

func (c *AWSClient) CreateBucket(ctx context.Context, bucket string, options *CreateBucketOptions) error {
	c.logger.Info("Creating AWS S3 bucket: %s", bucket)

	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}

	// Set region-specific configuration
	if c.region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(c.region),
		}
	}

	_, err := c.s3Client.CreateBucket(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
	}

	return nil
}

func (c *AWSClient) DeleteBucket(ctx context.Context, bucket string) error {
	c.logger.Info("Deleting AWS S3 bucket: %s", bucket)

	_, err := c.s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket %s: %w", bucket, err)
	}

	return nil
}

func (c *AWSClient) BucketExists(ctx context.Context, bucket string) (bool, error) {
	c.logger.Debug("Checking if AWS S3 bucket exists: %s", bucket)

	_, err := c.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		// Check if it's a "not found" error
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchBucket") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (c *AWSClient) GetBucketInfo(ctx context.Context, bucket string) (*BucketInfo, error) {
	c.logger.Debug("Getting AWS S3 bucket info: %s", bucket)

	// Get bucket location
	locationResp, err := c.s3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket location: %w", err)
	}

	region := string(locationResp.LocationConstraint)
	if region == "" {
		region = "us-east-1" // Default region for empty location constraint
	}

	// List buckets to get creation date
	buckets, err := c.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	for _, b := range buckets {
		if b.Name == bucket {
			return &BucketInfo{
				Name:         bucket,
				Region:       region,
				CreationDate: b.CreationDate,
			}, nil
		}
	}

	return nil, fmt.Errorf("bucket %s not found", bucket)
}

// Object operations
func (c *AWSClient) ListObjects(ctx context.Context, bucket string, options *ListObjectsOptions) (*ObjectList, error) {
	c.logger.Debug("Listing AWS S3 objects in bucket: %s", bucket)

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}

	if options != nil {
		if options.Prefix != "" {
			input.Prefix = aws.String(options.Prefix)
		}
		if options.MaxKeys > 0 {
			input.MaxKeys = aws.Int32(int32(options.MaxKeys))
		}
		if options.ContinuationToken != "" {
			input.ContinuationToken = aws.String(options.ContinuationToken)
		}
	}

	resp, err := c.s3Client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	objects := make([]ObjectInfo, len(resp.Contents))
	for i, obj := range resp.Contents {
		storageClass := StorageClassStandard
		if obj.StorageClass != "" {
			// Convert AWS SDK v2 StorageClass enum to our StorageClass
			storageClass = parseStorageClass(string(obj.StorageClass))
		}

		objects[i] = ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
			ETag:         aws.ToString(obj.ETag),
			StorageClass: storageClass,
		}
	}

	return &ObjectList{
		Objects:               objects,
		IsTruncated:           aws.ToBool(resp.IsTruncated),
		NextContinuationToken: aws.ToString(resp.NextContinuationToken),
	}, nil
}

func (c *AWSClient) GetObject(ctx context.Context, bucket, key string, options *GetObjectOptions) (*Object, error) {
	c.logger.Debug("Getting AWS S3 object: %s/%s", bucket, key)

	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	if options != nil {
		if options.Range != nil {
			input.Range = aws.String(fmt.Sprintf("bytes=%d-%d", options.Range.Start, options.Range.End))
		}
	}

	resp, err := c.s3Client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s/%s: %w", bucket, key, err)
	}

	return &Object{
		Info: &ObjectInfo{
			Key:          key,
			Size:         aws.ToInt64(resp.ContentLength),
			LastModified: aws.ToTime(resp.LastModified),
			ETag:         aws.ToString(resp.ETag),
			ContentType:  aws.ToString(resp.ContentType),
		},
		Body:   resp.Body,
		Length: aws.ToInt64(resp.ContentLength),
	}, nil
}

func (c *AWSClient) PutObject(ctx context.Context, bucket, key string, data io.Reader, options *PutObjectOptions) (*PutObjectResult, error) {
	c.logger.Debug("Putting AWS S3 object: %s/%s", bucket, key)

	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   data,
	}

	if options != nil {
		if options.ContentType != "" {
			input.ContentType = aws.String(options.ContentType)
		}
		if options.StorageClass != StorageClassStandard {
			input.StorageClass = types.StorageClass(options.StorageClass.String())
		}
		if len(options.Metadata) > 0 {
			input.Metadata = options.Metadata
		}
	}

	resp, err := c.s3Client.PutObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to put object %s/%s: %w", bucket, key, err)
	}

	storageClass := StorageClassStandard
	if input.StorageClass != "" {
		storageClass = parseStorageClass(string(input.StorageClass))
	}

	return &PutObjectResult{
		ETag:         aws.ToString(resp.ETag),
		Size:         0, // Size not returned in PutObject response
		UploadTime:   time.Now(),
		StorageClass: storageClass,
	}, nil
}

func (c *AWSClient) DeleteObject(ctx context.Context, bucket, key string, options *DeleteObjectOptions) error {
	c.logger.Debug("Deleting AWS S3 object: %s/%s", bucket, key)

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := c.s3Client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object %s/%s: %w", bucket, key, err)
	}

	return nil
}

func (c *AWSClient) DeleteObjects(ctx context.Context, bucket string, keys []string, options *DeleteObjectsOptions) (*BatchDeleteResult, error) {
	c.logger.Debug("Deleting %d AWS S3 objects from bucket: %s", len(keys), bucket)

	if len(keys) == 0 {
		return &BatchDeleteResult{
			Deleted: make([]DeletedObject, 0),
			Errors:  make([]DeleteError, 0),
		}, nil
	}

	// Prepare delete objects
	objects := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = types.ObjectIdentifier{
			Key: aws.String(key),
		}
	}

	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: objects,
			Quiet:   aws.Bool(false),
		},
	}

	resp, err := c.s3Client.DeleteObjects(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to delete objects: %w", err)
	}

	// Process results
	deleted := make([]DeletedObject, len(resp.Deleted))
	for i, obj := range resp.Deleted {
		deleted[i] = DeletedObject{
			Key: aws.ToString(obj.Key),
		}
	}

	errors := make([]DeleteError, len(resp.Errors))
	for i, err := range resp.Errors {
		errors[i] = DeleteError{
			Key:     aws.ToString(err.Key),
			Code:    aws.ToString(err.Code),
			Message: aws.ToString(err.Message),
		}
	}

	return &BatchDeleteResult{
		Deleted: deleted,
		Errors:  errors,
	}, nil
}

func (c *AWSClient) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, options *CopyObjectOptions) error {
	c.logger.Debug("Copying AWS S3 object: %s/%s -> %s/%s", srcBucket, srcKey, dstBucket, dstKey)

	copySource := fmt.Sprintf("%s/%s", srcBucket, srcKey)
	input := &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(copySource),
	}

	if options != nil {
		if options.StorageClass != StorageClassStandard {
			input.StorageClass = types.StorageClass(options.StorageClass.String())
		}
		if len(options.Metadata) > 0 {
			input.Metadata = options.Metadata
			input.MetadataDirective = types.MetadataDirectiveReplace
		}
	}

	_, err := c.s3Client.CopyObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to copy object %s/%s to %s/%s: %w", srcBucket, srcKey, dstBucket, dstKey, err)
	}

	return nil
}

// Object metadata and properties
func (c *AWSClient) HeadObject(ctx context.Context, bucket, key string) (*ObjectMetadata, error) {
	c.logger.Debug("Getting AWS S3 object metadata: %s/%s", bucket, key)

	resp, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata %s/%s: %w", bucket, key, err)
	}

	storageClass := StorageClassStandard
	if resp.StorageClass != "" {
		storageClass = parseStorageClass(string(resp.StorageClass))
	}

	return &ObjectMetadata{
		Key:          key,
		Size:         aws.ToInt64(resp.ContentLength),
		LastModified: aws.ToTime(resp.LastModified),
		ETag:         aws.ToString(resp.ETag),
		ContentType:  aws.ToString(resp.ContentType),
		StorageClass: storageClass,
		Metadata:     resp.Metadata,
	}, nil
}

func (c *AWSClient) SetObjectMetadata(ctx context.Context, bucket, key string, metadata map[string]string) error {
	c.logger.Debug("Setting AWS S3 object metadata: %s/%s", bucket, key)

	// For AWS S3, we need to copy the object to itself with new metadata
	copySource := fmt.Sprintf("%s/%s", bucket, key)
	_, err := c.s3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(bucket),
		Key:               aws.String(key),
		CopySource:        aws.String(copySource),
		Metadata:          metadata,
		MetadataDirective: types.MetadataDirectiveReplace,
	})
	if err != nil {
		return fmt.Errorf("failed to set object metadata %s/%s: %w", bucket, key, err)
	}

	return nil
}

func (c *AWSClient) GetObjectURL(ctx context.Context, bucket, key string, options *URLOptions) (string, error) {
	c.logger.Debug("Getting AWS S3 object URL: %s/%s", bucket, key)
	// TODO: Implement AWS S3 get object URL
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, c.region, key), nil
}

func (c *AWSClient) GeneratePresignedURL(ctx context.Context, bucket, key string, options *PresignedURLOptions) (string, error) {
	c.logger.Debug("Generating AWS S3 presigned URL: %s/%s", bucket, key)

	// Create presigner
	presigner := s3.NewPresignClient(c.s3Client)

	expiration := 15 * time.Minute // Default expiration
	if options != nil && options.Expires > 0 {
		expiration = options.Expires
	}

	// Generate presigned URL for GetObject
	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiration
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return req.URL, nil
}

// Advanced operations
func (c *AWSClient) MultipartUpload(ctx context.Context, bucket, key string, options *MultipartUploadOptions) (MultipartUploader, error) {
	c.logger.Debug("Starting AWS S3 multipart upload: %s/%s", bucket, key)
	// TODO: Implement AWS S3 multipart upload
	return &AWSMultipartUploader{
		client:   c,
		bucket:   bucket,
		key:      key,
		uploadID: "dummy-upload-id",
	}, nil
}

func (c *AWSClient) BatchOperations(ctx context.Context, operations []BatchOperation) (*BatchResult, error) {
	c.logger.Debug("Executing %d AWS S3 batch operations", len(operations))
	// TODO: Implement AWS S3 batch operations
	return &BatchResult{
		Successful:   make([]BatchOperationResult, 0),
		Failed:       make([]BatchOperationError, 0),
		TotalCount:   len(operations),
		SuccessCount: 0,
		FailureCount: 0,
	}, nil
}

// Storage class and lifecycle
func (c *AWSClient) SetStorageClass(ctx context.Context, bucket, key string, storageClass StorageClass) error {
	c.logger.Debug("Setting AWS S3 storage class for %s/%s to %s", bucket, key, storageClass.String())
	// TODO: Implement AWS S3 set storage class
	return nil
}

func (c *AWSClient) GetStorageClass(ctx context.Context, bucket, key string) (StorageClass, error) {
	c.logger.Debug("Getting AWS S3 storage class for %s/%s", bucket, key)
	// TODO: Implement AWS S3 get storage class
	return StorageClassStandard, nil
}

// Access control
func (c *AWSClient) SetBucketPolicy(ctx context.Context, bucket string, policy *BucketPolicy) error {
	c.logger.Debug("Setting AWS S3 bucket policy for: %s", bucket)
	// TODO: Implement AWS S3 set bucket policy
	return nil
}

func (c *AWSClient) GetBucketPolicy(ctx context.Context, bucket string) (*BucketPolicy, error) {
	c.logger.Debug("Getting AWS S3 bucket policy for: %s", bucket)
	// TODO: Implement AWS S3 get bucket policy
	return nil, fmt.Errorf("not implemented")
}

func (c *AWSClient) SetObjectACL(ctx context.Context, bucket, key string, acl *ObjectACL) error {
	c.logger.Debug("Setting AWS S3 object ACL for: %s/%s", bucket, key)
	// TODO: Implement AWS S3 set object ACL
	return nil
}

func (c *AWSClient) GetObjectACL(ctx context.Context, bucket, key string) (*ObjectACL, error) {
	c.logger.Debug("Getting AWS S3 object ACL for: %s/%s", bucket, key)
	// TODO: Implement AWS S3 get object ACL
	return nil, fmt.Errorf("not implemented")
}

// AWSMultipartUploader implements MultipartUploader for AWS S3
type AWSMultipartUploader struct {
	client   *AWSClient
	bucket   string
	key      string
	uploadID string
}

func (u *AWSMultipartUploader) UploadPart(ctx context.Context, partNumber int, data io.Reader) (*UploadPartResult, error) {
	u.client.logger.Debug("Uploading part %d for AWS S3 multipart upload: %s/%s", partNumber, u.bucket, u.key)
	// TODO: Implement AWS S3 upload part
	return &UploadPartResult{
		PartNumber: partNumber,
		ETag:       "dummy-etag",
		Size:       0,
	}, nil
}

func (u *AWSMultipartUploader) Complete(ctx context.Context, parts []CompletedPart) (*PutObjectResult, error) {
	u.client.logger.Debug("Completing AWS S3 multipart upload: %s/%s", u.bucket, u.key)
	// TODO: Implement AWS S3 complete multipart upload
	return &PutObjectResult{
		ETag:         "dummy-etag",
		Size:         0,
		UploadTime:   time.Now(),
		StorageClass: StorageClassStandard,
	}, nil
}

func (u *AWSMultipartUploader) Abort(ctx context.Context) error {
	u.client.logger.Debug("Aborting AWS S3 multipart upload: %s/%s", u.bucket, u.key)
	// TODO: Implement AWS S3 abort multipart upload
	return nil
}

func (u *AWSMultipartUploader) ListParts(ctx context.Context) ([]UploadPartResult, error) {
	u.client.logger.Debug("Listing parts for AWS S3 multipart upload: %s/%s", u.bucket, u.key)
	// TODO: Implement AWS S3 list parts
	return []UploadPartResult{}, nil
}

func (u *AWSMultipartUploader) GetUploadID() string {
	return u.uploadID
}
