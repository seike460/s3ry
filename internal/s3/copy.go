package s3

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// BulkOptions controls a recursive transfer.
type BulkOptions struct {
	Parallel  int
	Overwrite OverwriteMode
	// Progress is invoked concurrently from up to Parallel transfers (or the
	// session/default concurrency when Parallel is non-positive) and must be
	// goroutine-safe.
	Progress        ProgressFunc
	ContinueOnError bool
	// ProgressInterval throttles intermediate progress events for every
	// transfer in the batch; zero or negative uses the default cadence.
	ProgressInterval time.Duration
}

type bulkTransfers struct {
	g               *errgroup.Group
	ctx             context.Context
	cancel          context.CancelFunc
	continueOnError bool

	mu       sync.Mutex
	firstErr error
	errors   []KeyError
	success  atomic.Int64
}

func newBulkTransfers(ctx context.Context, parallel int, continueOnError bool) *bulkTransfers {
	workCtx, cancel := context.WithCancel(ctx)
	g, gctx := errgroup.WithContext(workCtx)
	g.SetLimit(parallel)
	return &bulkTransfers{
		g:               g,
		ctx:             gctx,
		cancel:          cancel,
		continueOnError: continueOnError,
	}
}

func (b *bulkTransfers) goTransfer(key string, transfer func(context.Context) (bool, error)) {
	b.g.Go(func() error {
		count, err := transfer(b.ctx)
		if err != nil {
			b.recordError(key, err)
			if b.continueOnError {
				return nil
			}
			return err
		}
		if count {
			b.success.Add(1)
		}
		return nil
	})
}

func (b *bulkTransfers) recordError(key string, err error) {
	if err == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.continueOnError {
		b.errors = append(b.errors, KeyError{Key: key, Err: err})
		return
	}
	if b.firstErr == nil {
		b.firstErr = err
	}
}

// callbackError records an error found before a transfer can be started. In
// continue mode the walk proceeds; otherwise it cancels in-flight transfers
// and returns the error to the walker.
func (b *bulkTransfers) callbackError(key string, err error) error {
	if err == nil {
		return nil
	}
	b.recordError(key, err)
	if b.continueOnError {
		return nil
	}
	b.cancel()
	return err
}

func (b *bulkTransfers) recordWalkError(err error) {
	if err == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.continueOnError {
		b.errors = append(b.errors, KeyError{Err: err})
		return
	}
	if b.firstErr == nil {
		b.firstErr = err
	}
	b.cancel()
}

func (b *bulkTransfers) finish(walkErr error) (int, error) {
	if walkErr != nil {
		b.recordWalkError(walkErr)
	}
	waitErr := b.g.Wait()
	b.cancel()

	n := int(b.success.Load())
	b.mu.Lock()
	firstErr := b.firstErr
	errorsCopy := append([]KeyError(nil), b.errors...)
	b.mu.Unlock()

	if !b.continueOnError {
		if firstErr != nil {
			return n, firstErr
		}
		if waitErr != nil {
			return n, waitErr
		}
		if walkErr != nil {
			return n, walkErr
		}
		return n, nil
	}
	if len(errorsCopy) > 0 {
		return n, &BulkError{Errors: errorsCopy}
	}
	return n, nil
}

func bulkParallel(s *Session, o BulkOptions) int {
	parallel := o.Parallel
	if parallel <= 0 {
		parallel = s.Options().Concurrency
	}
	if parallel <= 0 {
		parallel = defaultConcurrency
	}
	return parallel
}

func classifyBulkWalkError(op, bucket, key string, err error) error {
	if err == nil {
		return nil
	}
	return Classify(op, bucket, key, err)
}

// DownloadPrefix downloads every object below prefix into destDir. Folder
// markers, including markers that carry bytes, are skipped when their keys end
// in "/". Object keys are mapped with LocalPath, which rejects traversal
// segments before any local directories or files are created.
func (s *Session) DownloadPrefix(ctx context.Context, bucket, prefix, destDir string, o BulkOptions) (int, error) {
	if info, err := os.Stat(destDir); err == nil {
		if !info.IsDir() {
			return 0, &Error{
				Kind: KindInvalid,
				Op:   "download-prefix",
				Err:  fmt.Errorf("destination path %q is not a directory", destDir),
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return 0, &Error{
			Kind: KindInvalid,
			Op:   "download-prefix",
			Err:  fmt.Errorf("destination path %q: %w", destDir, err),
		}
	}

	parallel := bulkParallel(s, o)
	bulk := newBulkTransfers(ctx, parallel, o.ContinueOnError)

	walkErr := s.Walk(ctx, bucket, prefix, WalkOptions{Concurrency: parallel}, func(object Object) error {
		if strings.HasSuffix(object.Key, "/") {
			return nil
		}
		if err := bulk.ctx.Err(); err != nil {
			return err
		}

		localPath, err := LocalPath(destDir, prefix, object.Key)
		if err != nil {
			return bulk.callbackError(object.Key, err)
		}

		bulk.goTransfer(object.Key, func(transferCtx context.Context) (bool, error) {
			var skipped atomic.Bool
			progress := func(event Progress) {
				if event.Done && event.Skipped {
					skipped.Store(true)
				}
				if o.Progress != nil {
					o.Progress(event)
				}
			}
			err := s.Download(transferCtx, bucket, object.Key, localPath, DownloadOptions{
				Overwrite:        o.Overwrite,
				Progress:         progress,
				ProgressInterval: o.ProgressInterval,
			})
			return !skipped.Load(), err
		})
		return nil
	})
	walkErr = classifyBulkWalkError("list", bucket, prefix, walkErr)
	return bulk.finish(walkErr)
}

// UploadDir uploads every non-directory, non-symlink entry below srcDir.
// If srcDir itself is a symlink, UploadDir resolves that root symlink before
// walking it. Symlinks encountered inside the resolved tree are never
// followed. Empty local directories produce no keys, and dotfiles are
// uploaded. The relative filesystem path is converted to an S3 key using
// slash separators, regardless of the host filesystem.
func (s *Session) UploadDir(ctx context.Context, srcDir, bucket, prefix string, o BulkOptions) (int, error) {
	if err := ValidateBucketName(bucket); err != nil {
		return 0, err
	}

	info, err := os.Lstat(srcDir)
	if err != nil {
		return 0, &Error{
			Kind: KindInvalid,
			Op:   "upload_dir",
			Err:  fmt.Errorf("source path %q: %w", srcDir, err),
		}
	}
	walkRoot := srcDir
	if info.Mode()&fs.ModeSymlink != 0 {
		walkRoot, err = filepath.EvalSymlinks(srcDir)
		if err != nil {
			return 0, &Error{
				Kind: KindInvalid,
				Op:   "upload_dir",
				Err:  fmt.Errorf("source path %q: %w", srcDir, err),
			}
		}
		info, err = os.Stat(walkRoot)
		if err != nil {
			return 0, &Error{
				Kind: KindInvalid,
				Op:   "upload_dir",
				Err:  fmt.Errorf("source path %q: %w", srcDir, err),
			}
		}
	}
	if !info.IsDir() {
		return 0, &Error{
			Kind: KindInvalid,
			Op:   "upload_dir",
			Err:  fmt.Errorf("source path %q is not a directory", srcDir),
		}
	}

	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	parallel := bulkParallel(s, o)
	bulk := newBulkTransfers(ctx, parallel, o.ContinueOnError)

	walkErr := filepath.WalkDir(walkRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if err := bulk.ctx.Err(); err != nil {
			return err
		}

		rel, err := filepath.Rel(walkRoot, path)
		if err != nil {
			return err
		}
		key := prefix + filepath.ToSlash(rel)
		if err := ValidateKey(key); err != nil {
			return bulk.callbackError(key, err)
		}

		bulk.goTransfer(key, func(transferCtx context.Context) (bool, error) {
			return true, s.Upload(transferCtx, path, bucket, key, UploadOptions{
				Progress:         o.Progress,
				ProgressInterval: o.ProgressInterval,
			})
		})
		return nil
	})
	walkErr = classifyBulkWalkError("upload_dir", bucket, "", walkErr)
	return bulk.finish(walkErr)
}
