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
)

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

	skipped, err := checkDownloadTarget(bucket, key, localPath, o.Overwrite, m)
	if err != nil || skipped {
		return err
	}

	tmp, err := newDownloadTemp(localPath)
	if err != nil {
		return err
	}
	defer func() { err = tmp.cleanup(err) }()

	if err := s.fetchToTemp(ctx, bucket, key, tmp, m); err != nil {
		return err
	}
	if err := tmp.close(); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return tmp.publish(localPath, bucket, key, o.Overwrite, m)
}

// checkDownloadTarget validates the inputs and resolves the overwrite
// decision for an existing localPath.
func checkDownloadTarget(bucket, key, localPath string, overwrite OverwriteMode, m *meter) (skipped bool, err error) {
	if err := ValidateBucketName(bucket); err != nil {
		return false, err
	}
	if err := ValidateKey(key); err != nil {
		return false, err
	}
	return resolveExistingTarget(localPath, bucket, key, overwrite, m)
}

// resolveExistingTarget applies the overwrite mode to an existing or missing
// localPath.
func resolveExistingTarget(localPath, bucket, key string, overwrite OverwriteMode, m *meter) (bool, error) {
	info, statErr := os.Stat(localPath)
	if statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return false, nil
		}
		return false, statErr
	}
	return existingTargetDecision(info, bucket, key, overwrite, m)
}

// existingTargetDecision applies the overwrite mode to an existing path.
func existingTargetDecision(info fs.FileInfo, bucket, key string, overwrite OverwriteMode, m *meter) (bool, error) {
	switch {
	case info.IsDir():
		return false, newInvalidError("download", bucket, key, "local path is a directory")
	case overwrite == OverwriteSkip:
		m.finishSkipped()
		return true, nil
	case overwrite != OverwriteAlways:
		return false, &Error{
			Kind:   KindExists,
			Op:     "download",
			Bucket: bucket,
			Key:    key,
		}
	default:
		return false, nil
	}
}

// downloadTemp stages a download in a sibling temporary file until the
// transfer completes and the file is published at the target path.
type downloadTemp struct {
	file    *os.File
	path    string
	closed  bool
	removed bool
}

// newDownloadTemp creates the staging file in the target directory.
func newDownloadTemp(localPath string) (*downloadTemp, error) {
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(dir, filepath.Base(localPath)+".s3ry-tmp-*")
	if err != nil {
		return nil, err
	}
	tmp := &downloadTemp{file: file, path: file.Name()}
	if err := file.Chmod(0o644); err != nil {
		return nil, tmp.cleanup(err)
	}
	return tmp, nil
}

// close seals the staged file exactly once.
func (t *downloadTemp) close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	if err := t.file.Close(); err != nil {
		return fmt.Errorf("close temporary file %q: %w", t.path, err)
	}
	return nil
}

// remove deletes the staged file exactly once.
func (t *downloadTemp) remove() error {
	if t.removed {
		return nil
	}
	if err := os.Remove(t.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove temporary file %q: %w", t.path, err)
	}
	t.removed = true
	return nil
}

// cleanup closes and removes the staging file, appending any failure to err.
func (t *downloadTemp) cleanup(err error) error {
	err = appendCleanupError(err, t.close())
	err = appendCleanupError(err, t.remove())
	return err
}

// publish installs the staged file at localPath honoring the overwrite mode.
func (t *downloadTemp) publish(localPath, bucket, key string, overwrite OverwriteMode, m *meter) error {
	exists, err := publishDownload(t.path, localPath, overwrite)
	if err != nil {
		return err
	}
	if !exists {
		t.removed = true
		return nil
	}
	return t.resolveExisting(bucket, key, overwrite, m)
}

// resolveExisting resolves the case where localPath appeared during the
// transfer: the staging file is removed and the caller learns whether the
// download was skipped or refused.
func (t *downloadTemp) resolveExisting(bucket, key string, overwrite OverwriteMode, m *meter) error {
	var existsErr error
	if overwrite != OverwriteSkip {
		existsErr = &Error{
			Kind:   KindExists,
			Op:     "download",
			Bucket: bucket,
			Key:    key,
		}
	}
	if err := t.remove(); err != nil {
		return appendCleanupError(existsErr, err)
	}
	if overwrite == OverwriteSkip {
		m.finishSkipped()
		return nil
	}
	return existsErr
}

// fetchToTemp streams the object into tmp through the transfer manager.
func (s *Session) fetchToTemp(ctx context.Context, bucket, key string, tmp *downloadTemp, m *meter) error {
	tm, err := s.transfer(ctx, bucket)
	if err != nil {
		return err
	}
	_, err = tm.DownloadObject(ctx, &transfermanager.DownloadObjectInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		WriterAt: &countingFile{File: tmp.file, m: m},
	}, func(opts *transfermanager.Options) {
		opts.ObjectProgressListeners.Register(&downloadProgressListener{m: m})
	})
	if err == nil {
		err = ctx.Err()
	}
	return err
}

func appendCleanupError(primary, cleanupErr error) error {
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
