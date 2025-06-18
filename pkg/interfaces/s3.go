// Package interfaces defines common interfaces to avoid circular dependencies
package interfaces

import (
	"context"
	"time"
)

// S3Client interface for basic S3/MinIO operations
type S3Client interface {
	// Basic operations
	ListBuckets(ctx context.Context) ([]BucketInfo, error)
	ListObjects(ctx context.Context, bucket, prefix, delimiter string) ([]ObjectInfo, error)
	UploadFile(ctx context.Context, localPath, bucket, key string) error
	DownloadFile(ctx context.Context, bucket, key, localPath string) error
	DeleteObject(ctx context.Context, bucket, key string) error

	// Extended operations for API compatibility
	CreateBucket(ctx context.Context, bucket string) error
	DeleteBucket(ctx context.Context, bucket string) error
	GetBucketInfo(ctx context.Context, bucket string) (*BucketInfo, error)
	ListObjectsWithLimit(ctx context.Context, bucket, prefix string, maxKeys int) (*ObjectList, error)
	UploadObject(ctx context.Context, bucket, key string, content []byte) (*PutResult, error)
	GetPresignedURL(ctx context.Context, bucket, key string, expires time.Duration) (string, error)
	StreamObject(ctx context.Context, bucket, key string) (*Object, error)
	GetObjectMetadata(ctx context.Context, bucket, key string) (*ObjectMetadata, error)

	// Utility operations
	GetBucketRegion(ctx context.Context, bucket string) (string, error)
}

// BucketInfo represents basic bucket information
type BucketInfo struct {
	Name    string    `json:"name"`
	Region  string    `json:"region"`
	Created time.Time `json:"created"`
}

// ObjectInfo contains basic object information
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag,omitempty"`
	IsPrefix     bool      `json:"is_prefix"`
	StorageClass string    `json:"storage_class,omitempty"`
}

// OperationType represents different S3 operations
type OperationType string

const (
	OperationTypeList     OperationType = "list"
	OperationTypeUpload   OperationType = "upload"
	OperationTypeDownload OperationType = "download"
	OperationTypeDelete   OperationType = "delete"
)

// Operation represents a generic operation
type Operation struct {
	ID       string                 `json:"id"`
	Type     OperationType          `json:"type"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// OperationResult represents the result of an operation
type OperationResult struct {
	OperationID string                 `json:"operation_id"`
	Success     bool                   `json:"success"`
	Message     string                 `json:"message"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ObjectList represents a paginated list of objects
type ObjectList struct {
	Objects               []ObjectInfo `json:"objects"`
	IsTruncated           bool         `json:"is_truncated"`
	NextContinuationToken string       `json:"next_continuation_token,omitempty"`
}

// Object represents an object with its content
type Object struct {
	Info    *ObjectInfo `json:"info"`
	Content []byte      `json:"content,omitempty"`
}

// PutResult represents the result of a put operation
type PutResult struct {
	ETag     string `json:"etag"`
	Location string `json:"location,omitempty"`
}

// ObjectMetadata represents object metadata
type ObjectMetadata struct {
	Key          string             `json:"key"`
	Size         int64              `json:"size"`
	LastModified time.Time          `json:"last_modified"`
	ContentType  string             `json:"content_type,omitempty"`
	Metadata     map[string]*string `json:"metadata,omitempty"`
}

// BatchDeleteResult represents the result of batch delete operation
type BatchDeleteResult struct {
	Deleted []DeletedObject `json:"deleted"`
	Errors  []DeleteError   `json:"errors"`
}

// DeletedObject represents a successfully deleted object
type DeletedObject struct {
	Key string `json:"key"`
}

// DeleteError represents an error during delete operation
type DeleteError struct {
	Key     string `json:"key"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// MultipartUpload represents a multipart upload session
type MultipartUpload struct {
	UploadID string `json:"upload_id"`
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
}

// UploadPart represents a part of a multipart upload
type UploadPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}
