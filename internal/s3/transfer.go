package s3

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	transfertypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
)

var progressInterval = 100 * time.Millisecond

type meter struct {
	mu   sync.Mutex
	fn   ProgressFunc
	base Progress
	n    atomic.Int64
	last atomic.Int64
	done atomic.Bool
}

func (m *meter) add(delta int64) {
	if delta <= 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.done.Load() {
		return
	}

	n := m.n.Add(delta)
	if m.fn == nil {
		return
	}

	now := time.Now().UnixNano()
	last := m.last.Load()
	if now-last < progressInterval.Nanoseconds() {
		return
	}
	m.last.Store(now)

	progress := m.base
	progress.Transferred = n
	progress.Done = false
	progress.Err = nil
	m.fn(progress)
}

func (m *meter) finish(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.done.CompareAndSwap(false, true) {
		return
	}
	if m.fn == nil {
		return
	}

	progress := m.base
	progress.Transferred = m.n.Load()
	progress.Done = true
	progress.Err = err
	m.fn(progress)
}

func (m *meter) finishSkipped() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.done.CompareAndSwap(false, true) {
		return
	}
	if m.fn == nil {
		return
	}

	progress := m.base
	progress.Done = true
	progress.Skipped = true
	m.fn(progress)
}

func (m *meter) setTotal(total int64) {
	m.mu.Lock()
	if !m.done.Load() {
		m.base.Total = total
	}
	m.mu.Unlock()
}

type countingFile struct {
	*os.File
	m *meter
}

func (f *countingFile) Read(p []byte) (int, error) {
	n, err := f.File.Read(p)
	if n > 0 && f.m != nil {
		f.m.add(int64(n))
	}
	return n, err
}

func (f *countingFile) WriteAt(p []byte, offset int64) (int, error) {
	n, err := f.File.WriteAt(p, offset)
	if n > 0 && f.m != nil {
		f.m.add(int64(n))
	}
	return n, err
}

type UploadOptions struct {
	ContentType  string
	StorageClass string
	Progress     ProgressFunc
}

func (s *Session) Upload(ctx context.Context, localPath, bucket, key string, o UploadOptions) error {
	m := &meter{
		fn: o.Progress,
		base: Progress{
			Op:     OpUpload,
			Bucket: bucket,
			Key:    key,
			Local:  localPath,
			Total:  -1,
		},
	}

	if err := ValidateBucketName(bucket); err != nil {
		return finishTransfer(m, "upload", bucket, key, err)
	}
	if err := ValidateKey(key); err != nil {
		return finishTransfer(m, "upload", bucket, key, err)
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return finishTransfer(m, "upload", bucket, key, err)
	}
	if info.IsDir() {
		return finishTransfer(m, "upload", bucket, key, newInvalidError("upload", bucket, key, "local path is a directory"))
	}
	m.setTotal(info.Size())

	contentType := o.ContentType
	if contentType == "" {
		contentType = DetectContentType(localPath)
	}

	file, err := os.Open(localPath)
	if err != nil {
		return finishTransfer(m, "upload", bucket, key, err)
	}
	defer file.Close()

	tm, err := s.transfer(ctx, bucket)
	if err != nil {
		return finishTransfer(m, "upload", bucket, key, err)
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
	return finishTransfer(m, "upload", bucket, key, err)
}

type DownloadOptions struct {
	Overwrite OverwriteMode
	Progress  ProgressFunc
}

func (s *Session) Download(ctx context.Context, bucket, key, localPath string, o DownloadOptions) error {
	m := &meter{
		fn: o.Progress,
		base: Progress{
			Op:     OpDownload,
			Bucket: bucket,
			Key:    key,
			Local:  localPath,
			Total:  -1,
		},
	}

	if err := ValidateBucketName(bucket); err != nil {
		return finishTransfer(m, "download", bucket, key, err)
	}
	if err := ValidateKey(key); err != nil {
		return finishTransfer(m, "download", bucket, key, err)
	}

	_, statErr := os.Stat(localPath)
	if statErr == nil {
		if o.Overwrite != OverwriteAlways {
			if o.Overwrite == OverwriteSkip {
				m.finishSkipped()
				return nil
			}
			return finishTransfer(m, "download", bucket, key, &Error{
				Kind:   KindExists,
				Op:     "download",
				Bucket: bucket,
				Key:    key,
			})
		}
	} else if !os.IsNotExist(statErr) {
		return finishTransfer(m, "download", bucket, key, statErr)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return finishTransfer(m, "download", bucket, key, err)
	}

	tmpPath := localPath + ".s3ry-tmp"
	_ = os.Remove(tmpPath)
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return finishTransfer(m, "download", bucket, key, err)
	}

	closed := false
	renamed := false
	cleanup := func() {
		if !closed {
			_ = tmpFile.Close()
			closed = true
		}
		if !renamed {
			_ = os.Remove(tmpPath)
		}
	}
	defer cleanup()

	tm, err := s.transfer(ctx, bucket)
	if err != nil {
		cleanup()
		return finishTransfer(m, "download", bucket, key, err)
	}

	out, err := tm.DownloadObject(ctx, &transfermanager.DownloadObjectInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		WriterAt: &countingFile{File: tmpFile, m: m},
	})
	if out != nil && out.ContentLength != nil {
		m.setTotal(aws.ToInt64(out.ContentLength))
	}
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		cleanup()
		return finishTransfer(m, "download", bucket, key, err)
	}

	if err := tmpFile.Close(); err != nil {
		closed = true
		cleanup()
		return finishTransfer(m, "download", bucket, key, err)
	}
	closed = true
	if ctx.Err() != nil {
		cleanup()
		return finishTransfer(m, "download", bucket, key, ctx.Err())
	}
	if err := os.Rename(tmpPath, localPath); err != nil {
		cleanup()
		return finishTransfer(m, "download", bucket, key, err)
	}
	renamed = true
	m.finish(nil)
	return nil
}

func finishTransfer(m *meter, op, bucket, key string, err error) error {
	classified := Classify(op, bucket, key, err)
	m.finish(classified)
	return classified
}
