// Package types provides common types used across the s3ry application
package types

import (
	"time"
)

// Bucket represents an S3 bucket with its metadata
type Bucket struct {
	Name         string    `json:"name"`
	CreationDate time.Time `json:"creation_date"`
	Region       string    `json:"region"`
}

// Object represents an S3 object with its metadata
type Object struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag"`
	StorageClass string    `json:"storage_class"`
}

// TransferJob represents a file transfer job between local filesystem and S3
type TransferJob struct {
	ID               string         `json:"id"`
	Type             TransferType   `json:"type"`
	Source           string         `json:"source"`
	Target           string         `json:"target"`
	Size             int64          `json:"size"`
	BytesTransferred int64          `json:"bytes_transferred"`
	Status           TransferStatus `json:"status"`
	Error            error          `json:"error,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// TransferType represents the type of transfer operation
type TransferType int

const (
	// Upload represents uploading from local to S3
	Upload TransferType = iota
	// Download represents downloading from S3 to local
	Download
	// Copy represents copying between S3 locations
	Copy
	// Delete represents deleting from S3
	Delete
)

// String returns the string representation of TransferType
func (t TransferType) String() string {
	switch t {
	case Upload:
		return "upload"
	case Download:
		return "download"
	case Copy:
		return "copy"
	case Delete:
		return "delete"
	default:
		return "unknown"
	}
}

// TransferStatus represents the status of a transfer job
type TransferStatus int

const (
	// Pending indicates the job is waiting to be processed
	Pending TransferStatus = iota
	// InProgress indicates the job is currently being processed
	InProgress
	// Completed indicates the job has finished successfully
	Completed
	// Failed indicates the job has failed
	Failed
	// Cancelled indicates the job was cancelled
	Cancelled
	// Paused indicates the job is temporarily paused
	Paused
)

// String returns the string representation of TransferStatus
func (s TransferStatus) String() string {
	switch s {
	case Pending:
		return "pending"
	case InProgress:
		return "in_progress"
	case Completed:
		return "completed"
	case Failed:
		return "failed"
	case Cancelled:
		return "cancelled"
	case Paused:
		return "paused"
	default:
		return "unknown"
	}
}

// IsTerminal returns true if the status is a terminal state
func (s TransferStatus) IsTerminal() bool {
	return s == Completed || s == Failed || s == Cancelled
}

// Progress calculates the progress percentage for a transfer job
func (tj *TransferJob) Progress() float64 {
	if tj.Size == 0 {
		return 0
	}
	return float64(tj.BytesTransferred) / float64(tj.Size) * 100
}

// IsComplete returns true if the transfer is complete
func (tj *TransferJob) IsComplete() bool {
	return tj.Status == Completed
}

// IsFailed returns true if the transfer has failed
func (tj *TransferJob) IsFailed() bool {
	return tj.Status == Failed
}

// CanRetry returns true if the transfer can be retried
func (tj *TransferJob) CanRetry() bool {
	return tj.Status == Failed || tj.Status == Cancelled
}

// TransferStats represents statistics for transfer operations
type TransferStats struct {
	TotalJobs        int           `json:"total_jobs"`
	CompletedJobs    int           `json:"completed_jobs"`
	FailedJobs       int           `json:"failed_jobs"`
	TotalBytes       int64         `json:"total_bytes"`
	TransferredBytes int64         `json:"transferred_bytes"`
	Duration         time.Duration `json:"duration"`
	Speed            int64         `json:"speed"` // bytes per second
}

// Progress calculates the overall progress percentage
func (ts *TransferStats) Progress() float64 {
	if ts.TotalBytes == 0 {
		return 0
	}
	return float64(ts.TransferredBytes) / float64(ts.TotalBytes) * 100
}

// JobProgress calculates the job completion percentage
func (ts *TransferStats) JobProgress() float64 {
	if ts.TotalJobs == 0 {
		return 0
	}
	return float64(ts.CompletedJobs) / float64(ts.TotalJobs) * 100
}

// ViewType represents different UI view types
type ViewType int

const (
	// BucketListView shows the list of S3 buckets
	BucketListView ViewType = iota
	// ObjectListView shows the list of objects in a bucket
	ObjectListView
	// TransferView shows active transfers
	TransferView
	// SettingsView shows application settings
	SettingsView
	// HelpView shows help information
	HelpView
)

// String returns the string representation of ViewType
func (v ViewType) String() string {
	switch v {
	case BucketListView:
		return "bucket_list"
	case ObjectListView:
		return "object_list"
	case TransferView:
		return "transfer"
	case SettingsView:
		return "settings"
	case HelpView:
		return "help"
	default:
		return "unknown"
	}
}

// AppConfig represents application configuration
type AppConfig struct {
	AWS struct {
		Region   string `yaml:"region"`
		Profile  string `yaml:"profile"`
		Endpoint string `yaml:"endpoint,omitempty"`
	} `yaml:"aws"`

	UI struct {
		Theme           string `yaml:"theme"`
		Language        string `yaml:"language"`
		RefreshInterval int    `yaml:"refresh_interval"`
	} `yaml:"ui"`

	Performance struct {
		Workers   int `yaml:"workers"`
		ChunkSize int `yaml:"chunk_size"`
		Timeout   int `yaml:"timeout"`
	} `yaml:"performance"`

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
		File   string `yaml:"file,omitempty"`
	} `yaml:"logging"`
}

// Worker-related types that were previously in worker package

// DownloadRequest represents a file download request
type DownloadRequest struct {
	Bucket   string
	Key      string
	FilePath string
}

// UploadRequest represents a file upload request
type UploadRequest struct {
	Bucket      string
	Key         string
	FilePath    string
	ContentType string
	Metadata    map[string]*string
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
