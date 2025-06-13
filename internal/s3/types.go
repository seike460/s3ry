package s3

import "time"

// Object represents an S3 object with metadata
type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	StorageClass string
}

// Bucket represents an S3 bucket with metadata
type Bucket struct {
	Name         string
	CreationDate time.Time
	Region       string
}

// UploadRequest represents a file upload request
type UploadRequest struct {
	Bucket      string
	Key         string
	FilePath    string
	ContentType string
	Metadata    map[string]*string
}

// DownloadRequest represents a file download request
type DownloadRequest struct {
	Bucket   string
	Key      string
	FilePath string
}

// ListRequest represents a request to list objects
type ListRequest struct {
	Bucket     string
	Prefix     string
	Delimiter  string
	MaxKeys    int64
	StartAfter string
}

// ProgressCallback is called during upload/download to report progress
type ProgressCallback func(bytesTransferred, totalBytes int64)

// BatchOperation represents a batch operation result
type BatchOperation struct {
	Operation string // "upload", "download", "delete"
	Key       string
	Success   bool
	Error     error
}