package s3

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

	if err := ValidateBucketName(bucket); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return localFileError("upload", bucket, key, localPath, err)
	}
	if info.IsDir() {
		return newInvalidError("upload", bucket, key, "local path is a directory")
	}
	m.setTotal(info.Size())

	contentType := o.ContentType
	if contentType == "" {
		contentType = DetectContentType(localPath)
	}

	file, err := os.Open(localPath)
	if err != nil {
		return localFileError("upload", bucket, key, localPath, err)
	}
	defer func() { _ = file.Close() }()

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

// DownloadOptions controls a single object download.
type DownloadOptions struct {
	// Overwrite selects how an existing localPath is handled.
	Overwrite OverwriteMode
	// Progress receives transfer progress events.
	Progress ProgressFunc
	// ProgressInterval throttles intermediate progress events; zero or
	// negative uses the default cadence.
	ProgressInterval time.Duration
}

// Download streams s3://bucket/key to localPath through the region-aware
// transfer manager. The object is fetched in parallel ranges when the part
// size allows it.
func (s *Session) Download(ctx context.Context, bucket, key, localPath string, o DownloadOptions) (err error) {
	m := &meter{
		fn:       o.Progress,
		interval: o.ProgressInterval,
		base: Progress{
			Op:     OpDownload,
			Bucket: bucket,
			Key:    key,
			Local:  localPath,
			Total:  -1,
		},
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = finishTransfer(m, "download", bucket, key, panicError(recovered))
			panic(recovered)
		}
		err = finishTransfer(m, "download", bucket, key, err)
	}()

	if err := ValidateBucketName(bucket); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}

	info, statErr := os.Stat(localPath)
	if statErr == nil {
		if info.IsDir() {
			return newInvalidError("download", bucket, key, "local path is a directory")
		}
		if o.Overwrite != OverwriteAlways {
			if o.Overwrite == OverwriteSkip {
				m.finishSkipped()
				return nil
			}
			return &Error{
				Kind:   KindExists,
				Op:     "download",
				Bucket: bucket,
				Key:    key,
			}
		}
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o750); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(localPath), filepath.Base(localPath)+".s3ry-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	closed := false
	tmpGone := false
	appendCleanupError := func(primary, cleanupErr error) error {
		if cleanupErr == nil || errors.Is(cleanupErr, fs.ErrNotExist) {
			return primary
		}
		if primary == nil {
			return cleanupErr
		}
		if classified, ok := primary.(*Error); ok {
			cloned := *classified
			if cloned.Err == nil {
				cloned.Err = cleanupErr
			} else {
				cloned.Err = errors.Join(cloned.Err, cleanupErr)
			}
			return &cloned
		}
		return errors.Join(primary, cleanupErr)
	}
	cleanup := func() {
		if !closed {
			closeErr := tmpFile.Close()
			closed = true
			if closeErr != nil {
				err = appendCleanupError(err, fmt.Errorf("close temporary file %q: %w", tmpPath, closeErr))
			}
		}
		if !tmpGone {
			removeErr := os.Remove(tmpPath)
			if removeErr == nil || errors.Is(removeErr, fs.ErrNotExist) {
				tmpGone = true
			} else {
				err = appendCleanupError(err, fmt.Errorf("remove temporary file %q: %w", tmpPath, removeErr))
			}
		}
	}
	defer cleanup()
	if err := tmpFile.Chmod(0o644); err != nil {
		return err
	}

	tm, err := s.transfer(ctx, bucket)
	if err != nil {
		return err
	}

	_, err = tm.DownloadObject(ctx, &transfermanager.DownloadObjectInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		WriterAt: &countingFile{File: tmpFile, m: m},
	}, func(opts *transfermanager.Options) {
		opts.ObjectProgressListeners.Register(&downloadProgressListener{m: m})
	})
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		return err
	}

	closeErr := tmpFile.Close()
	closed = true
	if closeErr != nil {
		return fmt.Errorf("close temporary file %q: %w", tmpPath, closeErr)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	exists, err := publishDownload(tmpPath, localPath, o.Overwrite)
	if err != nil {
		return err
	}
	if exists {
		var existsErr error
		if o.Overwrite != OverwriteSkip {
			existsErr = &Error{
				Kind:   KindExists,
				Op:     "download",
				Bucket: bucket,
				Key:    key,
			}
		}
		removeErr := os.Remove(tmpPath)
		if removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			cleanupErr := fmt.Errorf("remove temporary file %q: %w", tmpPath, removeErr)
			if existsErr != nil {
				return appendCleanupError(existsErr, cleanupErr)
			}
			return cleanupErr
		}
		tmpGone = true
		if o.Overwrite == OverwriteSkip {
			m.finishSkipped()
			return nil
		}
		return existsErr
	}
	tmpGone = true
	return nil
}

func publishDownload(tmpPath, localPath string, overwrite OverwriteMode) (exists bool, err error) {
	if overwrite == OverwriteAlways {
		// Rename deliberately replaces a symlink at localPath instead of its target.
		return false, os.Rename(tmpPath, localPath)
	}

	err = os.Link(tmpPath, localPath)
	if err == nil {
		return false, os.Remove(tmpPath)
	}
	if errors.Is(err, fs.ErrExist) {
		return true, nil
	}

	if _, statErr := os.Lstat(localPath); statErr == nil {
		return true, nil
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return false, statErr
	}

	// This fallback supports filesystems without hard links. A creator can
	// still win between Lstat and Rename, so that residual race remains.
	if renameErr := os.Rename(tmpPath, localPath); renameErr != nil {
		if errors.Is(renameErr, fs.ErrExist) {
			return true, nil
		}
		return false, renameErr
	}
	return false, nil
}

func localFileError(op, bucket, key, localPath string, err error) error {
	var kind Kind
	switch {
	case errors.Is(err, fs.ErrNotExist):
		kind = KindNotFound
	case errors.Is(err, fs.ErrPermission):
		kind = KindAccessDenied
	default:
		return Classify(op, bucket, key, err)
	}

	return &Error{
		Kind:   kind,
		Op:     op,
		Bucket: bucket,
		Key:    key,
		Err:    fmt.Errorf("local path %q: %w", localPath, err),
	}
}

func panicError(value any) error {
	if err, ok := value.(error); ok {
		return fmt.Errorf("transfer panic: %w", err)
	}
	return fmt.Errorf("transfer panic: %v", value)
}

func finishTransfer(m *meter, op, bucket, key string, err error) error {
	classified := Classify(op, bucket, key, err)
	m.finish(classified)
	return classified
}
