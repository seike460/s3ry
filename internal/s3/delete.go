package s3

import (
	"context"
	"errors"
	"strings"
	"sync"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// DeleteOptions controls a delete operation.
type DeleteOptions struct {
	DryRun bool

	// Progress receives one completion event per validated or attempted key.
	// Calls are serialized and never concurrent. DeleteKeys invokes Progress on
	// the caller's goroutine; DeletePrefix invokes it on its internal delete
	// goroutine.
	Progress ProgressFunc
}

// DeleteResult reports the keys deleted and the keys that failed.
type DeleteResult struct {
	Deleted []string
	Failed  []KeyError
}

// DeleteKeys deletes keys in S3's multi-object-delete batch size, falling back
// to individual deletes when the endpoint does not support multi-object delete.
func (s *Session) DeleteKeys(ctx context.Context, bucket string, keys []string, o DeleteOptions) (DeleteResult, error) {
	var result DeleteResult
	if err := ValidateBucketName(bucket); err != nil {
		return result, err
	}

	valid := validateDeleteKeys(bucket, keys, o, &result)

	if o.DryRun {
		result.Deleted = append(result.Deleted, valid...)
		for _, key := range valid {
			emitDeleteProgress(o.Progress, bucket, key, nil)
		}
		return result, nil
	}
	if len(valid) == 0 {
		return result, nil
	}

	client, err := s.client(ctx, bucket)
	if err != nil {
		return result, Classify("delete", bucket, "", err)
	}

	for start := 0; start < len(valid); start += MaxDeleteBatch {
		end := start + MaxDeleteBatch
		if end > len(valid) {
			end = len(valid)
		}
		if err := s.deleteBatch(ctx, client, bucket, valid[start:end], o, &result); err != nil {
			return result, err
		}
	}

	return result, nil
}

// validateDeleteKeys deduplicates keys and validates each, recording invalid
// keys in result and emitting their progress.
func validateDeleteKeys(bucket string, keys []string, o DeleteOptions, result *DeleteResult) []string {
	seen := make(map[string]struct{}, len(keys))
	valid := make([]string, 0, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := ValidateKey(key); err != nil {
			result.Failed = append(result.Failed, KeyError{Key: key, Err: err})
			emitDeleteProgress(o.Progress, bucket, key, err)
			continue
		}
		valid = append(valid, key)
	}
	return valid
}

// deleteBatch deletes one batch and merges the outcome into result. A non-nil
// return aborts the remaining batches.
func (s *Session) deleteBatch(ctx context.Context, client *awss3.Client, bucket string, batch []string, o DeleteOptions, result *DeleteResult) error {
	batchResult, single, batchErr := s.runDeleteBatch(ctx, client, bucket, batch)
	if batchErr == nil {
		mergeDeleteResult(result, batchResult)
		batchErrors := deleteErrorsByKey(batchResult.Failed)
		for _, key := range batch {
			emitDeleteProgress(o.Progress, bucket, key, batchErrors[key])
		}
		return nil
	}

	classified := Classify("delete", bucket, "", batchErr)
	if single {
		mergeDeleteResult(result, batchResult)
		emitDeleteProgressForResult(o.Progress, bucket, batch, batchResult)
	} else {
		for _, key := range batch {
			emitDeleteProgress(o.Progress, bucket, key, classified)
		}
	}
	return classified
}

// runDeleteBatch executes one batch with the multi-object delete API, or with
// single deletes once the endpoint has been downgraded. It reports whether
// single deletes were used.
func (s *Session) runDeleteBatch(ctx context.Context, client *awss3.Client, bucket string, batch []string) (DeleteResult, bool, error) {
	if s.singleDelete.Load() {
		result, err := s.singleDeleteBatch(ctx, client, bucket, batch)
		return result, true, err
	}

	result, err := s.deleteObjectsBatch(ctx, client, bucket, batch)
	if err == nil || !deleteNotImplemented(err) {
		return result, false, err
	}
	// singleDelete is session-wide: one non-AWS endpoint downgrades all buckets on this session.
	s.singleDelete.Store(true)
	result, err = s.singleDeleteBatch(ctx, client, bucket, batch)
	return result, true, err
}

// DeletePrefix walks prefix and deletes each full batch before the listing
// continues. prefix must be non-empty; an empty prefix is rejected because
// deleting a whole bucket is not supported. A prefix without a trailing slash
// is normalized to one, so "p" has directory semantics and matches "p/x" but
// not "p2/x". If the listing is incomplete, its final partial batch is not
// deleted.
func (s *Session) DeletePrefix(ctx context.Context, bucket, prefix string, o DeleteOptions) (DeleteResult, error) {
	prefix, err := validateDeletePrefix(bucket, prefix)
	if err != nil {
		return DeleteResult{}, err
	}

	d := &prefixDeleter{
		s:      s,
		o:      o,
		prefix: prefix,
		batch:  make([]string, 0, MaxDeleteBatch),
	}
	walkErr := s.Walk(ctx, bucket, prefix, WalkOptions{Concurrency: s.Options().Concurrency}, func(object Object) error {
		return d.visit(ctx, bucket, object)
	})

	// An incomplete listing is never partially deleted: only flush the
	// remaining batch after Walk has completed successfully.
	if walkErr == nil {
		d.flush(ctx, bucket)
	}

	return d.outcome(ctx, bucket, walkErr)
}

// validateDeletePrefix checks the bucket name and normalizes prefix to
// directory semantics with a trailing slash.
func validateDeletePrefix(bucket, prefix string) (string, error) {
	if err := ValidateBucketName(bucket); err != nil {
		return "", err
	}
	if prefix == "" {
		return "", &Error{
			Kind:   KindInvalid,
			Op:     "delete-prefix",
			Bucket: bucket,
			Err:    errors.New("prefix must not be empty; deleting a whole bucket is not supported"),
		}
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if err := ValidateKey(prefix); err != nil {
		return "", &Error{
			Kind:   KindInvalid,
			Op:     "delete-prefix",
			Bucket: bucket,
			Key:    prefix,
			Err:    err,
		}
	}
	return prefix, nil
}

// prefixDeleter accumulates walked keys into batches and deletes each full
// batch while the listing continues.
type prefixDeleter struct {
	s      *Session
	o      DeleteOptions
	prefix string

	mu        sync.Mutex
	deleted   DeleteResult
	deleteErr error
	batch     []string
}

// visit collects an object key and deletes each full batch. The mutex
// serializes collection and deletion across concurrent walk callbacks. A
// delete error is returned to the walker, which cancels sibling crawls before
// later pages are requested.
func (d *prefixDeleter) visit(ctx context.Context, bucket string, object Object) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.deleteErr != nil {
		return d.deleteErr
	}
	d.batch = append(d.batch, object.Key)
	if len(d.batch) < MaxDeleteBatch {
		return nil
	}
	keys := d.batch
	d.batch = make([]string, 0, MaxDeleteBatch)
	if err := ctx.Err(); err != nil {
		return Classify("list", bucket, d.prefix, err)
	}
	d.deleteBatch(ctx, bucket, keys)
	return d.deleteErr
}

// flush deletes the remaining partial batch after a successful walk.
func (d *prefixDeleter) flush(ctx context.Context, bucket string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.deleteErr != nil || len(d.batch) == 0 {
		return
	}
	d.deleteBatch(ctx, bucket, d.batch)
	d.batch = nil
}

// deleteBatch deletes keys and records the first failure.
func (d *prefixDeleter) deleteBatch(ctx context.Context, bucket string, keys []string) {
	partial, err := d.s.DeleteKeys(ctx, bucket, keys, d.o)
	mergeDeleteResult(&d.deleted, partial)
	if err != nil && d.deleteErr == nil {
		d.deleteErr = err
	}
}

// outcome combines walk, delete, and cancellation errors into the result.
func (d *prefixDeleter) outcome(ctx context.Context, bucket string, walkErr error) (DeleteResult, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		canceled := Classify("list", bucket, d.prefix, ctxErr)
		if d.deleteErr != nil {
			return d.deleted, errors.Join(canceled, d.deleteErr)
		}
		return d.deleted, canceled
	}
	if d.deleteErr != nil {
		return d.deleted, d.deleteErr
	}
	if walkErr != nil {
		return d.deleted, walkErr
	}
	return d.deleted, nil
}
