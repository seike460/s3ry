// Package interfaces defines common interfaces to avoid circular dependencies
package interfaces

import (
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// S3Client interface for S3 operations
// This interface is used to break circular dependencies between internal packages
type S3Client interface {
	S3() *s3.S3
	Uploader() *s3manager.Uploader
	Downloader() *s3manager.Downloader
}