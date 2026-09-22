package s3

import (
	"errors"
	"strings"
	"time"
)

// Bucket describes an S3 bucket. Region is empty when the region is unknown.
type Bucket struct {
	Name         string
	CreationDate time.Time
	Region       string
}

// Object describes an S3 object returned by a listing operation.
type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	StorageClass string
}

// Page is one page of an S3 listing.
type Page struct {
	Bucket      string
	Prefix      string
	Prefixes    []string
	Objects     []Object
	NextToken   string
	IsTruncated bool
}

// ObjectInfo contains an object and its response metadata.
type ObjectInfo struct {
	Object
	ContentType string
	Metadata    map[string]string
	VersionID   string
}

// URL identifies an S3 bucket or object.
type URL struct {
	Bucket string
	Key    string
}

// ParseURL parses an S3 URI. A trailing slash after the bucket denotes an
// empty key, which is the same bucket URL as one without the slash.
func ParseURL(s string) (URL, error) {
	const scheme = "s3://"
	if !strings.HasPrefix(s, scheme) {
		return URL{}, parseInvalidError("URL must use the s3:// scheme")
	}

	rest := s[len(scheme):]
	bucket := rest
	key := ""
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		bucket = rest[:slash]
		key = rest[slash+1:]
	}

	if err := ValidateBucketName(bucket); err != nil {
		return URL{}, &Error{Kind: KindInvalid, Op: "parse", Bucket: bucket, Err: err}
	}

	if key != "" {
		if err := ValidateKey(key); err != nil {
			return URL{}, &Error{Kind: KindInvalid, Op: "parse", Bucket: bucket, Key: key, Err: err}
		}
	}

	return URL{Bucket: bucket, Key: key}, nil
}

// String returns the canonical unescaped S3 URI for the URL.
func (u URL) String() string {
	if u.Key == "" {
		return "s3://" + u.Bucket
	}
	return "s3://" + u.Bucket + "/" + u.Key
}

// IsPrefix reports whether the URL identifies a prefix rather than one
// specific object.
func (u URL) IsPrefix() bool {
	return u.Key == "" || strings.HasSuffix(u.Key, "/")
}

// MaxDeleteBatch is the S3 DeleteObjects per-request key limit.
const MaxDeleteBatch = 1000

// Op identifies which S3 operation produced a Progress event.
type Op int

const (
	// OpUpload is a local-to-S3 transfer.
	OpUpload Op = iota + 1
	// OpDownload is an S3-to-local transfer.
	OpDownload
	// OpDelete is an object deletion.
	OpDelete
)

// Progress is one progress event emitted while an operation runs. Total is
// -1 while the size is still unknown; Done marks the final event.
type Progress struct {
	Op          Op
	Bucket      string
	Key         string
	Local       string
	Transferred int64
	Total       int64
	Done        bool
	Skipped     bool
	Err         error
}

// ProgressFunc receives Progress events. It may be called from worker
// goroutines and must be safe for concurrent use.
type ProgressFunc func(Progress)

// OverwriteMode selects how Download treats an existing local file.
type OverwriteMode int

const (
	// OverwriteFail aborts when the destination exists.
	OverwriteFail OverwriteMode = iota
	// OverwriteSkip leaves the existing file and reports success.
	OverwriteSkip
	// OverwriteAlways replaces the existing file.
	OverwriteAlways
)

func parseInvalidError(reason string) *Error {
	return &Error{Kind: KindInvalid, Op: "parse", Err: errors.New(reason)}
}
