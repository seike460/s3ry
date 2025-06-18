// Package adapters provides implementation adapters for interface compatibility
package adapters

import (
	"context"
	"io"
	"time"

	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/pkg/interfaces"
)

// S3ClientAdapter adapts the S3 client to the platform-independent interface
type S3ClientAdapter struct {
	client *s3.Client
}

// NewS3ClientAdapter creates a new S3 client adapter
func NewS3ClientAdapter(client *s3.Client) *S3ClientAdapter {
	return &S3ClientAdapter{
		client: client,
	}
}

// ListObjects implements the platform-independent ListObjects interface
func (adapter *S3ClientAdapter) ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) (*interfaces.ObjectList, error) {
	input := &awss3.ListObjectsV2Input{
		Bucket:  &bucket,
		Prefix:  &prefix,
		MaxKeys: int64ToPtr(int64(maxKeys)),
	}

	result, err := adapter.client.S3().ListObjectsV2WithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	objectList := &interfaces.ObjectList{
		Objects:     make([]interfaces.ObjectInfo, 0, len(result.Contents)),
		IsTruncated: result.IsTruncated != nil && *result.IsTruncated,
	}

	if result.NextContinuationToken != nil {
		objectList.NextContinuationToken = *result.NextContinuationToken
	}

	for _, obj := range result.Contents {
		objectInfo := interfaces.ObjectInfo{
			Key:          derefString(obj.Key),
			Size:         derefInt64(obj.Size),
			LastModified: derefTime(obj.LastModified),
			ETag:         cleanETag(derefString(obj.ETag)),
		}

		if obj.StorageClass != nil {
			objectInfo.StorageClass = *obj.StorageClass
		}

		objectList.Objects = append(objectList.Objects, objectInfo)
	}

	return objectList, nil
}

// GetObject implements the platform-independent GetObject interface
func (adapter *S3ClientAdapter) GetObject(ctx context.Context, bucket, key string) (*interfaces.Object, error) {
	input := &awss3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	result, err := adapter.client.S3().GetObjectWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	objectInfo := interfaces.ObjectInfo{
		Key:          key,
		Size:         derefInt64(result.ContentLength),
		LastModified: derefTime(result.LastModified),
		ETag:         cleanETag(derefString(result.ETag)),
	}

	if result.StorageClass != nil {
		objectInfo.StorageClass = *result.StorageClass
	}

	return &interfaces.Object{
		Info:    &objectInfo,
		Content: nil, // Would need to read body for content
	}, nil
}

// PutObject implements the platform-independent PutObject interface
func (adapter *S3ClientAdapter) PutObject(ctx context.Context, bucket, key string, data io.Reader, size int64) (*interfaces.PutResult, error) {
	input := &s3manager.UploadInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   data,
	}

	result, err := adapter.client.Uploader().UploadWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	return &interfaces.PutResult{
		ETag: result.Location,
	}, nil
}

// DeleteObject implements the platform-independent DeleteObject interface
func (adapter *S3ClientAdapter) DeleteObject(ctx context.Context, bucket, key string) error {
	input := &awss3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	_, err := adapter.client.S3().DeleteObjectWithContext(ctx, input)
	return err
}

// DeleteObjects implements the platform-independent DeleteObjects interface
func (adapter *S3ClientAdapter) DeleteObjects(ctx context.Context, bucket string, keys []string) (*interfaces.BatchDeleteResult, error) {
	if len(keys) == 0 {
		return &interfaces.BatchDeleteResult{}, nil
	}

	objects := make([]*awss3.ObjectIdentifier, 0, len(keys))
	for _, key := range keys {
		objects = append(objects, &awss3.ObjectIdentifier{
			Key: &key,
		})
	}

	input := &awss3.DeleteObjectsInput{
		Bucket: &bucket,
		Delete: &awss3.Delete{
			Objects: objects,
			Quiet:   boolPtr(false),
		},
	}

	result, err := adapter.client.S3().DeleteObjectsWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	batchResult := &interfaces.BatchDeleteResult{
		Deleted: make([]interfaces.DeletedObject, 0, len(result.Deleted)),
		Errors:  make([]interfaces.DeleteError, 0, len(result.Errors)),
	}

	for _, deleted := range result.Deleted {
		batchResult.Deleted = append(batchResult.Deleted, interfaces.DeletedObject{
			Key: derefString(deleted.Key),
		})
	}

	for _, deleteError := range result.Errors {
		batchResult.Errors = append(batchResult.Errors, interfaces.DeleteError{
			Key:     derefString(deleteError.Key),
			Code:    derefString(deleteError.Code),
			Message: derefString(deleteError.Message),
		})
	}

	return batchResult, nil
}

// CreateMultipartUpload implements the platform-independent CreateMultipartUpload interface
func (adapter *S3ClientAdapter) CreateMultipartUpload(ctx context.Context, bucket, key string) (*interfaces.MultipartUpload, error) {
	input := &awss3.CreateMultipartUploadInput{
		Bucket: &bucket,
		Key:    &key,
	}

	result, err := adapter.client.S3().CreateMultipartUploadWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	return &interfaces.MultipartUpload{
		UploadID: derefString(result.UploadId),
		Bucket:   bucket,
		Key:      key,
	}, nil
}

// UploadPart implements the platform-independent UploadPart interface
func (adapter *S3ClientAdapter) UploadPart(ctx context.Context, upload *interfaces.MultipartUpload, partNumber int, data io.ReadSeeker) (*interfaces.UploadPart, error) {
	input := &awss3.UploadPartInput{
		Bucket:     &upload.Bucket,
		Key:        &upload.Key,
		UploadId:   &upload.UploadID,
		PartNumber: int64ToPtr(int64(partNumber)),
		Body:       data,
	}

	result, err := adapter.client.S3().UploadPartWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	return &interfaces.UploadPart{
		PartNumber: partNumber,
		ETag:       cleanETag(derefString(result.ETag)),
		Size:       0, // AWS doesn't return size in UploadPart response
	}, nil
}

// CompleteMultipartUpload implements the platform-independent CompleteMultipartUpload interface
func (adapter *S3ClientAdapter) CompleteMultipartUpload(ctx context.Context, upload *interfaces.MultipartUpload, parts []interfaces.UploadPart) (*interfaces.PutResult, error) {
	completedParts := make([]*awss3.CompletedPart, 0, len(parts))

	for _, part := range parts {
		completedParts = append(completedParts, &awss3.CompletedPart{
			PartNumber: int64ToPtr(int64(part.PartNumber)),
			ETag:       &part.ETag,
		})
	}

	input := &awss3.CompleteMultipartUploadInput{
		Bucket:   &upload.Bucket,
		Key:      &upload.Key,
		UploadId: &upload.UploadID,
		MultipartUpload: &awss3.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}

	result, err := adapter.client.S3().CompleteMultipartUploadWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	var totalSize int64
	for _, part := range parts {
		totalSize += part.Size
	}

	return &interfaces.PutResult{
		ETag: cleanETag(derefString(result.ETag)),
	}, nil
}

// HeadObject implements the platform-independent HeadObject interface
func (adapter *S3ClientAdapter) HeadObject(ctx context.Context, bucket, key string) (*interfaces.ObjectMetadata, error) {
	input := &awss3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	result, err := adapter.client.S3().HeadObjectWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	metadata := &interfaces.ObjectMetadata{
		Key:          key,
		Size:         derefInt64(result.ContentLength),
		LastModified: derefTime(result.LastModified),
		ContentType:  derefString(result.ContentType),
		Metadata:     make(map[string]*string),
	}

	// Convert AWS metadata to our format
	for k, v := range result.Metadata {
		metadata.Metadata[k] = v
	}

	return metadata, nil
}

// GetBucketRegion implements the platform-independent GetBucketRegion interface
func (adapter *S3ClientAdapter) GetBucketRegion(ctx context.Context, bucket string) (string, error) {
	return adapter.client.GetBucketRegion(ctx, bucket)
}

// GetMetrics is a stub for MVP - not implemented
func (adapter *S3ClientAdapter) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"request_count": 0,
		"error_count":   0,
	}
}

// Helper functions for safe dereferencing and type conversion

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func int64ToPtr(i int64) *int64 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

// cleanETag removes quotes from ETag if present
func cleanETag(etag string) string {
	if len(etag) >= 2 && etag[0] == '"' && etag[len(etag)-1] == '"' {
		return etag[1 : len(etag)-1]
	}
	return etag
}
