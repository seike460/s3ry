package cloud

import (
	"context"
	"io"
	"time"
)

// CloudProvider represents different cloud storage providers
type CloudProvider int

const (
	ProviderAWS CloudProvider = iota
	ProviderAzure
	ProviderGCS
	ProviderMinIO
)

func (cp CloudProvider) String() string {
	switch cp {
	case ProviderAWS:
		return "aws"
	case ProviderAzure:
		return "azure"
	case ProviderGCS:
		return "gcs"
	case ProviderMinIO:
		return "minio"
	default:
		return "unknown"
	}
}

// StorageClient provides a unified interface for cloud storage operations
type StorageClient interface {
	// Provider information
	GetProvider() CloudProvider
	GetRegion() string
	
	// Connection management
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	HealthCheck(ctx context.Context) error
	
	// Bucket operations
	ListBuckets(ctx context.Context) ([]BucketInfo, error)
	CreateBucket(ctx context.Context, bucket string, options *CreateBucketOptions) error
	DeleteBucket(ctx context.Context, bucket string) error
	BucketExists(ctx context.Context, bucket string) (bool, error)
	GetBucketInfo(ctx context.Context, bucket string) (*BucketInfo, error)
	
	// Object operations
	ListObjects(ctx context.Context, bucket string, options *ListObjectsOptions) (*ObjectList, error)
	GetObject(ctx context.Context, bucket, key string, options *GetObjectOptions) (*Object, error)
	PutObject(ctx context.Context, bucket, key string, data io.Reader, options *PutObjectOptions) (*PutObjectResult, error)
	DeleteObject(ctx context.Context, bucket, key string, options *DeleteObjectOptions) error
	DeleteObjects(ctx context.Context, bucket string, keys []string, options *DeleteObjectsOptions) (*BatchDeleteResult, error)
	CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, options *CopyObjectOptions) error
	
	// Object metadata and properties
	HeadObject(ctx context.Context, bucket, key string) (*ObjectMetadata, error)
	SetObjectMetadata(ctx context.Context, bucket, key string, metadata map[string]string) error
	GetObjectURL(ctx context.Context, bucket, key string, options *URLOptions) (string, error)
	GeneratePresignedURL(ctx context.Context, bucket, key string, options *PresignedURLOptions) (string, error)
	
	// Advanced operations
	MultipartUpload(ctx context.Context, bucket, key string, options *MultipartUploadOptions) (MultipartUploader, error)
	BatchOperations(ctx context.Context, operations []BatchOperation) (*BatchResult, error)
	
	// Storage class and lifecycle
	SetStorageClass(ctx context.Context, bucket, key string, storageClass StorageClass) error
	GetStorageClass(ctx context.Context, bucket, key string) (StorageClass, error)
	
	// Access control
	SetBucketPolicy(ctx context.Context, bucket string, policy *BucketPolicy) error
	GetBucketPolicy(ctx context.Context, bucket string) (*BucketPolicy, error)
	SetObjectACL(ctx context.Context, bucket, key string, acl *ObjectACL) error
	GetObjectACL(ctx context.Context, bucket, key string) (*ObjectACL, error)
}

// BucketInfo contains information about a bucket
type BucketInfo struct {
	Name           string            `json:"name"`
	Region         string            `json:"region"`
	CreationDate   time.Time         `json:"creation_date"`
	Versioning     bool              `json:"versioning"`
	Encryption     *EncryptionInfo   `json:"encryption,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
	StorageClass   StorageClass      `json:"storage_class"`
	Size           int64             `json:"size"`
	ObjectCount    int64             `json:"object_count"`
	LastModified   time.Time         `json:"last_modified"`
	PublicAccess   bool              `json:"public_access"`
	Website        *WebsiteConfig    `json:"website,omitempty"`
	CORS           *CORSConfig       `json:"cors,omitempty"`
	Lifecycle      *LifecycleConfig  `json:"lifecycle,omitempty"`
}

// CreateBucketOptions contains options for creating a bucket
type CreateBucketOptions struct {
	Region         string            `json:"region,omitempty"`
	StorageClass   StorageClass      `json:"storage_class,omitempty"`
	Versioning     bool              `json:"versioning,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
	Encryption     *EncryptionInfo   `json:"encryption,omitempty"`
	PublicAccess   bool              `json:"public_access,omitempty"`
	Website        *WebsiteConfig    `json:"website,omitempty"`
	CORS           *CORSConfig       `json:"cors,omitempty"`
	Lifecycle      *LifecycleConfig  `json:"lifecycle,omitempty"`
}

// ListObjectsOptions contains options for listing objects
type ListObjectsOptions struct {
	Prefix        string `json:"prefix,omitempty"`
	Delimiter     string `json:"delimiter,omitempty"`
	MaxKeys       int    `json:"max_keys,omitempty"`
	ContinuationToken string `json:"continuation_token,omitempty"`
	StartAfter    string `json:"start_after,omitempty"`
	Recursive     bool   `json:"recursive,omitempty"`
	IncludeMetadata bool `json:"include_metadata,omitempty"`
	IncludeVersions bool `json:"include_versions,omitempty"`
}

// ObjectList contains the result of listing objects
type ObjectList struct {
	Objects           []ObjectInfo `json:"objects"`
	CommonPrefixes    []string     `json:"common_prefixes,omitempty"`
	IsTruncated       bool         `json:"is_truncated"`
	NextContinuationToken string   `json:"next_continuation_token,omitempty"`
	TotalCount        int64        `json:"total_count"`
	TotalSize         int64        `json:"total_size"`
}

// ObjectInfo contains information about an object
type ObjectInfo struct {
	Key              string            `json:"key"`
	Size             int64             `json:"size"`
	LastModified     time.Time         `json:"last_modified"`
	ETag             string            `json:"etag"`
	StorageClass     StorageClass      `json:"storage_class"`
	ContentType      string            `json:"content_type"`
	ContentEncoding  string            `json:"content_encoding,omitempty"`
	Tags             map[string]string `json:"tags,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	VersionID        string            `json:"version_id,omitempty"`
	IsLatest         bool              `json:"is_latest"`
	DeleteMarker     bool              `json:"delete_marker"`
	Checksum         string            `json:"checksum,omitempty"`
	Owner            *OwnerInfo        `json:"owner,omitempty"`
}

// Object represents a complete object with its content
type Object struct {
	Info    *ObjectInfo `json:"info"`
	Body    io.ReadCloser `json:"-"`
	Length  int64       `json:"length"`
}

// GetObjectOptions contains options for getting an object
type GetObjectOptions struct {
	Range         *Range            `json:"range,omitempty"`
	IfMatch       string            `json:"if_match,omitempty"`
	IfNoneMatch   string            `json:"if_none_match,omitempty"`
	IfModifiedSince    *time.Time   `json:"if_modified_since,omitempty"`
	IfUnmodifiedSince  *time.Time   `json:"if_unmodified_since,omitempty"`
	VersionID     string            `json:"version_id,omitempty"`
	ResponseCacheControl    string  `json:"response_cache_control,omitempty"`
	ResponseContentDisposition string `json:"response_content_disposition,omitempty"`
	ResponseContentEncoding string  `json:"response_content_encoding,omitempty"`
	ResponseContentLanguage string  `json:"response_content_language,omitempty"`
	ResponseContentType     string  `json:"response_content_type,omitempty"`
	ResponseExpires         *time.Time `json:"response_expires,omitempty"`
}

// PutObjectOptions contains options for putting an object
type PutObjectOptions struct {
	ContentType     string            `json:"content_type,omitempty"`
	ContentEncoding string            `json:"content_encoding,omitempty"`
	ContentLanguage string            `json:"content_language,omitempty"`
	ContentDisposition string         `json:"content_disposition,omitempty"`
	CacheControl    string            `json:"cache_control,omitempty"`
	Expires         *time.Time        `json:"expires,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Tags            map[string]string `json:"tags,omitempty"`
	StorageClass    StorageClass      `json:"storage_class,omitempty"`
	Encryption      *EncryptionInfo   `json:"encryption,omitempty"`
	ACL             *ObjectACL        `json:"acl,omitempty"`
	Checksum        string            `json:"checksum,omitempty"`
	ProgressCallback func(uploaded, total int64) `json:"-"`
}

// PutObjectResult contains the result of putting an object
type PutObjectResult struct {
	ETag         string    `json:"etag"`
	VersionID    string    `json:"version_id,omitempty"`
	Checksum     string    `json:"checksum,omitempty"`
	Size         int64     `json:"size"`
	UploadTime   time.Time `json:"upload_time"`
	StorageClass StorageClass `json:"storage_class"`
}

// DeleteObjectOptions contains options for deleting an object
type DeleteObjectOptions struct {
	VersionID string `json:"version_id,omitempty"`
	MFA       string `json:"mfa,omitempty"`
}

// DeleteObjectsOptions contains options for batch deleting objects
type DeleteObjectsOptions struct {
	Quiet bool   `json:"quiet,omitempty"`
	MFA   string `json:"mfa,omitempty"`
}

// BatchDeleteResult contains the result of batch delete operation
type BatchDeleteResult struct {
	Deleted []DeletedObject `json:"deleted"`
	Errors  []DeleteError   `json:"errors,omitempty"`
}

// DeletedObject represents a successfully deleted object
type DeletedObject struct {
	Key       string `json:"key"`
	VersionID string `json:"version_id,omitempty"`
}

// DeleteError represents an error during delete operation
type DeleteError struct {
	Key       string `json:"key"`
	VersionID string `json:"version_id,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// CopyObjectOptions contains options for copying an object
type CopyObjectOptions struct {
	Metadata         map[string]string `json:"metadata,omitempty"`
	MetadataDirective MetadataDirective `json:"metadata_directive,omitempty"`
	TagsDirective    TagsDirective     `json:"tags_directive,omitempty"`
	Tags             map[string]string `json:"tags,omitempty"`
	StorageClass     StorageClass      `json:"storage_class,omitempty"`
	Encryption       *EncryptionInfo   `json:"encryption,omitempty"`
	ACL              *ObjectACL        `json:"acl,omitempty"`
	IfMatch          string            `json:"if_match,omitempty"`
	IfNoneMatch      string            `json:"if_none_match,omitempty"`
	IfModifiedSince  *time.Time        `json:"if_modified_since,omitempty"`
	IfUnmodifiedSince *time.Time       `json:"if_unmodified_since,omitempty"`
	SourceVersionID  string            `json:"source_version_id,omitempty"`
}

// ObjectMetadata contains metadata about an object
type ObjectMetadata struct {
	Key              string            `json:"key"`
	Size             int64             `json:"size"`
	LastModified     time.Time         `json:"last_modified"`
	ETag             string            `json:"etag"`
	ContentType      string            `json:"content_type"`
	ContentEncoding  string            `json:"content_encoding,omitempty"`
	ContentLanguage  string            `json:"content_language,omitempty"`
	ContentDisposition string          `json:"content_disposition,omitempty"`
	CacheControl     string            `json:"cache_control,omitempty"`
	Expires          *time.Time        `json:"expires,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Tags             map[string]string `json:"tags,omitempty"`
	StorageClass     StorageClass      `json:"storage_class"`
	VersionID        string            `json:"version_id,omitempty"`
	IsLatest         bool              `json:"is_latest"`
	DeleteMarker     bool              `json:"delete_marker"`
	Checksum         string            `json:"checksum,omitempty"`
	Encryption       *EncryptionInfo   `json:"encryption,omitempty"`
	Owner            *OwnerInfo        `json:"owner,omitempty"`
}

// URLOptions contains options for generating object URLs
type URLOptions struct {
	Secure    bool              `json:"secure,omitempty"`
	CustomDomain string         `json:"custom_domain,omitempty"`
	Params    map[string]string `json:"params,omitempty"`
}

// PresignedURLOptions contains options for generating presigned URLs
type PresignedURLOptions struct {
	Method     string            `json:"method"`
	Expires    time.Duration     `json:"expires"`
	Headers    map[string]string `json:"headers,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
	VersionID  string            `json:"version_id,omitempty"`
}

// MultipartUploadOptions contains options for multipart upload
type MultipartUploadOptions struct {
	PartSize         int64             `json:"part_size,omitempty"`
	Concurrency      int               `json:"concurrency,omitempty"`
	ContentType      string            `json:"content_type,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Tags             map[string]string `json:"tags,omitempty"`
	StorageClass     StorageClass      `json:"storage_class,omitempty"`
	Encryption       *EncryptionInfo   `json:"encryption,omitempty"`
	ACL              *ObjectACL        `json:"acl,omitempty"`
	ProgressCallback func(uploaded, total int64) `json:"-"`
}

// MultipartUploader provides interface for multipart upload operations
type MultipartUploader interface {
	UploadPart(ctx context.Context, partNumber int, data io.Reader) (*UploadPartResult, error)
	Complete(ctx context.Context, parts []CompletedPart) (*PutObjectResult, error)
	Abort(ctx context.Context) error
	ListParts(ctx context.Context) ([]UploadPartResult, error)
	GetUploadID() string
}

// UploadPartResult contains the result of uploading a part
type UploadPartResult struct {
	PartNumber   int    `json:"part_number"`
	ETag         string `json:"etag"`
	Size         int64  `json:"size"`
	Checksum     string `json:"checksum,omitempty"`
}

// CompletedPart represents a completed part for multipart upload
type CompletedPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

// BatchOperation represents an operation in a batch
type BatchOperation struct {
	Type     BatchOperationType `json:"type"`
	Bucket   string             `json:"bucket"`
	Key      string             `json:"key"`
	Source   *BatchSource       `json:"source,omitempty"`
	Target   *BatchTarget       `json:"target,omitempty"`
	Options  interface{}        `json:"options,omitempty"`
}

// BatchOperationType represents the type of batch operation
type BatchOperationType int

const (
	BatchCopy BatchOperationType = iota
	BatchDelete
	BatchRestore
	BatchSetMetadata
	BatchSetTags
	BatchSetStorageClass
)

// BatchSource represents the source for batch operations
type BatchSource struct {
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	VersionID string `json:"version_id,omitempty"`
}

// BatchTarget represents the target for batch operations
type BatchTarget struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// BatchResult contains the result of batch operations
type BatchResult struct {
	Successful []BatchOperationResult `json:"successful"`
	Failed     []BatchOperationError  `json:"failed"`
	TotalCount int                    `json:"total_count"`
	SuccessCount int                  `json:"success_count"`
	FailureCount int                  `json:"failure_count"`
}

// BatchOperationResult represents a successful batch operation
type BatchOperationResult struct {
	Operation BatchOperation `json:"operation"`
	Result    interface{}    `json:"result"`
}

// BatchOperationError represents a failed batch operation
type BatchOperationError struct {
	Operation BatchOperation `json:"operation"`
	Error     string         `json:"error"`
	Code      string         `json:"code"`
}

// StorageClass represents different storage classes
type StorageClass int

const (
	StorageClassStandard StorageClass = iota
	StorageClassStandardIA
	StorageClassReducedRedundancy
	StorageClassGlacier
	StorageClassGlacierInstantRetrieval
	StorageClassGlacierFlexibleRetrieval
	StorageClassDeepArchive
	StorageClassIntelligentTiering
	StorageClassOnezone
	StorageClassColdline
	StorageClassNearline
	StorageClassArchive
	StorageClassMultiRegional
	StorageClassRegional
)

func (sc StorageClass) String() string {
	switch sc {
	case StorageClassStandard:
		return "STANDARD"
	case StorageClassStandardIA:
		return "STANDARD_IA"
	case StorageClassReducedRedundancy:
		return "REDUCED_REDUNDANCY"
	case StorageClassGlacier:
		return "GLACIER"
	case StorageClassGlacierInstantRetrieval:
		return "GLACIER_IR"
	case StorageClassGlacierFlexibleRetrieval:
		return "GLACIER_FR"
	case StorageClassDeepArchive:
		return "DEEP_ARCHIVE"
	case StorageClassIntelligentTiering:
		return "INTELLIGENT_TIERING"
	case StorageClassOnezone:
		return "ONEZONE_IA"
	case StorageClassColdline:
		return "COLDLINE"
	case StorageClassNearline:
		return "NEARLINE"
	case StorageClassArchive:
		return "ARCHIVE"
	case StorageClassMultiRegional:
		return "MULTI_REGIONAL"
	case StorageClassRegional:
		return "REGIONAL"
	default:
		return "UNKNOWN"
	}
}

// EncryptionInfo contains encryption information
type EncryptionInfo struct {
	Type      EncryptionType `json:"type"`
	Algorithm string         `json:"algorithm,omitempty"`
	KeyID     string         `json:"key_id,omitempty"`
	Key       string         `json:"key,omitempty"`
	Context   map[string]string `json:"context,omitempty"`
}

// EncryptionType represents different encryption types
type EncryptionType int

const (
	EncryptionNone EncryptionType = iota
	EncryptionSSES3
	EncryptionSSEKMS
	EncryptionSSEC
	EncryptionClientSide
)

func (et EncryptionType) String() string {
	switch et {
	case EncryptionNone:
		return "NONE"
	case EncryptionSSES3:
		return "SSE-S3"
	case EncryptionSSEKMS:
		return "SSE-KMS"
	case EncryptionSSEC:
		return "SSE-C"
	case EncryptionClientSide:
		return "CLIENT_SIDE"
	default:
		return "UNKNOWN"
	}
}

// BucketPolicy represents bucket access policy
type BucketPolicy struct {
	Version   string      `json:"version"`
	Statement []Statement `json:"statement"`
}

// Statement represents a policy statement
type Statement struct {
	Sid       string                 `json:"sid,omitempty"`
	Effect    Effect                 `json:"effect"`
	Principal interface{}            `json:"principal,omitempty"`
	Action    interface{}            `json:"action"`
	Resource  interface{}            `json:"resource"`
	Condition map[string]interface{} `json:"condition,omitempty"`
}

// Effect represents the effect of a policy statement
type Effect int

const (
	EffectAllow Effect = iota
	EffectDeny
)

func (e Effect) String() string {
	switch e {
	case EffectAllow:
		return "Allow"
	case EffectDeny:
		return "Deny"
	default:
		return "Unknown"
	}
}

// ObjectACL represents object access control list
type ObjectACL struct {
	Owner  *OwnerInfo `json:"owner"`
	Grants []Grant    `json:"grants"`
}

// OwnerInfo represents owner information
type OwnerInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
}

// Grant represents an ACL grant
type Grant struct {
	Grantee    *Grantee    `json:"grantee"`
	Permission Permission `json:"permission"`
}

// Grantee represents a grantee in an ACL
type Grantee struct {
	Type         GranteeType `json:"type"`
	ID           string      `json:"id,omitempty"`
	DisplayName  string      `json:"display_name,omitempty"`
	Email        string      `json:"email,omitempty"`
	URI          string      `json:"uri,omitempty"`
}

// GranteeType represents the type of grantee
type GranteeType int

const (
	GranteeCanonicalUser GranteeType = iota
	GranteeAmazonCustomerByEmail
	GranteeGroup
)

// Permission represents ACL permissions
type Permission int

const (
	PermissionRead Permission = iota
	PermissionWrite
	PermissionReadACP
	PermissionWriteACP
	PermissionFullControl
)

// WebsiteConfig represents website configuration
type WebsiteConfig struct {
	IndexDocument string       `json:"index_document,omitempty"`
	ErrorDocument string       `json:"error_document,omitempty"`
	RedirectRules []RedirectRule `json:"redirect_rules,omitempty"`
}

// RedirectRule represents a redirect rule
type RedirectRule struct {
	Condition RedirectCondition `json:"condition,omitempty"`
	Redirect  RedirectTarget    `json:"redirect"`
}

// RedirectCondition represents redirect condition
type RedirectCondition struct {
	KeyPrefixEquals             string `json:"key_prefix_equals,omitempty"`
	HttpErrorCodeReturnedEquals string `json:"http_error_code_returned_equals,omitempty"`
}

// RedirectTarget represents redirect target
type RedirectTarget struct {
	HostName               string `json:"host_name,omitempty"`
	HttpRedirectCode       string `json:"http_redirect_code,omitempty"`
	Protocol               string `json:"protocol,omitempty"`
	ReplaceKeyPrefixWith   string `json:"replace_key_prefix_with,omitempty"`
	ReplaceKeyWith         string `json:"replace_key_with,omitempty"`
}

// CORSConfig represents CORS configuration
type CORSConfig struct {
	Rules []CORSRule `json:"rules"`
}

// CORSRule represents a CORS rule
type CORSRule struct {
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers,omitempty"`
	ExposeHeaders  []string `json:"expose_headers,omitempty"`
	MaxAgeSeconds  int      `json:"max_age_seconds,omitempty"`
}

// LifecycleConfig represents lifecycle configuration
type LifecycleConfig struct {
	Rules []LifecycleRule `json:"rules"`
}

// LifecycleRule represents a lifecycle rule
type LifecycleRule struct {
	ID                             string                         `json:"id,omitempty"`
	Status                         string                         `json:"status"`
	Filter                         *LifecycleFilter               `json:"filter,omitempty"`
	Transitions                    []LifecycleTransition          `json:"transitions,omitempty"`
	Expiration                     *LifecycleExpiration           `json:"expiration,omitempty"`
	NoncurrentVersionTransitions   []LifecycleTransition          `json:"noncurrent_version_transitions,omitempty"`
	NoncurrentVersionExpiration    *LifecycleExpiration           `json:"noncurrent_version_expiration,omitempty"`
	AbortIncompleteMultipartUpload *AbortIncompleteMultipartUpload `json:"abort_incomplete_multipart_upload,omitempty"`
}

// LifecycleFilter represents lifecycle filter
type LifecycleFilter struct {
	Prefix string            `json:"prefix,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
	And    *LifecycleAnd     `json:"and,omitempty"`
}

// LifecycleAnd represents AND condition in lifecycle filter
type LifecycleAnd struct {
	Prefix string            `json:"prefix,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

// LifecycleTransition represents a lifecycle transition
type LifecycleTransition struct {
	Date         *time.Time   `json:"date,omitempty"`
	Days         int          `json:"days,omitempty"`
	StorageClass StorageClass `json:"storage_class"`
}

// LifecycleExpiration represents lifecycle expiration
type LifecycleExpiration struct {
	Date                      *time.Time `json:"date,omitempty"`
	Days                      int        `json:"days,omitempty"`
	ExpiredObjectDeleteMarker bool       `json:"expired_object_delete_marker,omitempty"`
}

// AbortIncompleteMultipartUpload represents abort incomplete multipart upload rule
type AbortIncompleteMultipartUpload struct {
	DaysAfterInitiation int `json:"days_after_initiation"`
}

// Range represents a byte range
type Range struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// MetadataDirective represents metadata directive for copy operations
type MetadataDirective int

const (
	MetadataDirectiveCopy MetadataDirective = iota
	MetadataDirectiveReplace
)

// TagsDirective represents tags directive for copy operations
type TagsDirective int

const (
	TagsDirectiveCopy TagsDirective = iota
	TagsDirectiveReplace
)

// CloudConfig contains configuration for cloud providers
type CloudConfig struct {
	Provider CloudProvider `json:"provider"`
	Region   string        `json:"region"`
	
	// AWS specific
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	SessionToken    string `json:"session_token,omitempty"`
	Profile         string `json:"profile,omitempty"`
	
	// Azure specific
	AccountName   string `json:"account_name,omitempty"`
	AccountKey    string `json:"account_key,omitempty"`
	SASToken      string `json:"sas_token,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"`
	ClientID      string `json:"client_id,omitempty"`
	ClientSecret  string `json:"client_secret,omitempty"`
	
	// GCS specific
	ProjectID   string `json:"project_id,omitempty"`
	KeyFile     string `json:"key_file,omitempty"`
	KeyData     []byte `json:"key_data,omitempty"`
	
	// MinIO specific
	Endpoint        string `json:"endpoint,omitempty"`
	UseSSL          bool   `json:"use_ssl,omitempty"`
	
	// Common options
	Timeout         time.Duration `json:"timeout,omitempty"`
	RetryAttempts   int           `json:"retry_attempts,omitempty"`
	RetryDelay      time.Duration `json:"retry_delay,omitempty"`
	MaxConnections  int           `json:"max_connections,omitempty"`
	EnableLogging   bool          `json:"enable_logging,omitempty"`
	LogLevel        string        `json:"log_level,omitempty"`
	UserAgent       string        `json:"user_agent,omitempty"`
}

// DefaultCloudConfig returns default configuration
func DefaultCloudConfig() *CloudConfig {
	return &CloudConfig{
		Provider:       ProviderAWS,
		Region:         "us-east-1",
		Timeout:        30 * time.Second,
		RetryAttempts:  3,
		RetryDelay:     1 * time.Second,
		MaxConnections: 100,
		EnableLogging:  true,
		LogLevel:       "INFO",
		UserAgent:      "s3ry-unified-client/1.0",
	}
}

// Logger interface for cloud operations
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}