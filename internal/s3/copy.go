package s3

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
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

	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
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

	walkRoot, err := resolveWalkRoot(srcDir)
	if err != nil {
		return 0, err
	}

	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	parallel := bulkParallel(s, o)
	bulk := newBulkTransfers(ctx, parallel, o.ContinueOnError)

	walkErr := filepath.WalkDir(walkRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		return s.visitUploadEntry(bulk, walkRoot, bucket, prefix, path, entry, walkErr, o)
	})
	walkErr = classifyBulkWalkError("upload_dir", bucket, "", walkErr)
	return bulk.finish(walkErr)
}

// resolveWalkRoot returns the directory to walk for srcDir. A symlinked root
// itself is resolved, while symlinks inside the tree are never followed.
func resolveWalkRoot(srcDir string) (string, error) {
	info, err := os.Lstat(srcDir)
	if err != nil {
		return "", &Error{
			Kind: KindInvalid,
			Op:   "upload_dir",
			Err:  fmt.Errorf("source path %q: %w", srcDir, err),
		}
	}
	root := srcDir
	if info.Mode()&fs.ModeSymlink != 0 {
		root, err = filepath.EvalSymlinks(srcDir)
		if err != nil {
			return "", &Error{
				Kind: KindInvalid,
				Op:   "upload_dir",
				Err:  fmt.Errorf("source path %q: %w", srcDir, err),
			}
		}
		info, err = os.Stat(root)
		if err != nil {
			return "", &Error{
				Kind: KindInvalid,
				Op:   "upload_dir",
				Err:  fmt.Errorf("source path %q: %w", srcDir, err),
			}
		}
	}
	if !info.IsDir() {
		return "", &Error{
			Kind: KindInvalid,
			Op:   "upload_dir",
			Err:  fmt.Errorf("source path %q is not a directory", srcDir),
		}
	}
	return root, nil
}

// visitUploadEntry enqueues one filesystem entry for upload, skipping
// directories and symlinks.
func (s *Session) visitUploadEntry(bulk *bulkTransfers, walkRoot, bucket, prefix, path string, entry fs.DirEntry, walkErr error, o BulkOptions) error {
	if walkErr != nil {
		return walkErr
	}
	if entry.Type()&fs.ModeSymlink != 0 || entry.IsDir() {
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
}
