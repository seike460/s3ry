package s3

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	transfertypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
)

// UploadOptions controls a single object upload.
type UploadOptions struct {
	// ContentType overrides the MIME type detected from the file.
	ContentType string
	// StorageClass selects an S3 storage class; empty uses STANDARD.
	StorageClass string
	// Progress receives transfer progress events.
	Progress ProgressFunc
	// ProgressInterval throttles intermediate progress events; zero or
	// negative uses the default cadence.
	ProgressInterval time.Duration
}

// Upload streams localPath to s3://bucket/key using the region-aware
// transfer manager. Files larger than the configured part size are uploaded
// in parallel parts.
func (s *Session) Upload(ctx context.Context, localPath, bucket, key string, o UploadOptions) (err error) {
	m := &meter{
		fn:       o.Progress,
		interval: o.ProgressInterval,
		base: Progress{
			Op:     OpUpload,
			Bucket: bucket,
			Key:    key,
			Local:  localPath,
			Total:  -1,
		},
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = finishTransfer(m, "upload", bucket, key, panicError(recovered))
			panic(recovered)
		}
		err = finishTransfer(m, "upload", bucket, key, err)
	}()

	file, size, err := uploadSource(localPath, bucket, key)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	m.setTotal(size)

	contentType := o.ContentType
	if contentType == "" {
		contentType = DetectContentType(localPath)
	}

	tm, err := s.transfer(ctx, bucket)
	if err != nil {
		return err
	}

	input := &transfermanager.UploadObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        &countingFile{File: file, m: m},
		ContentType: aws.String(contentType),
	}
	if o.StorageClass != "" {
		input.StorageClass = transfertypes.StorageClass(o.StorageClass)
	}

	_, err = tm.UploadObject(ctx, input)
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	return err
}

// uploadSource validates the inputs and opens localPath, returning the file
// and its size for the transfer.
func uploadSource(localPath, bucket, key string) (*os.File, int64, error) {
	if err := ValidateBucketName(bucket); err != nil {
		return nil, 0, err
	}
	if err := ValidateKey(key); err != nil {
		return nil, 0, err
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return nil, 0, localFileError("upload", bucket, key, localPath, err)
	}
	if info.IsDir() {
		return nil, 0, newInvalidError("upload", bucket, key, "local path is a directory")
	}

	file, err := os.Open(localPath)
	if err != nil {
		return nil, 0, localFileError("upload", bucket, key, localPath, err)
	}
	return file, info.Size(), nil
}
