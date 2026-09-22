package s3

import (
	"context"
	"errors"
	"strings"
	"sync"
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

	unique := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, key)
	}

	valid := make([]string, 0, len(unique))
	for _, key := range unique {
		if err := ValidateKey(key); err != nil {
			result.Failed = append(result.Failed, KeyError{Key: key, Err: err})
			emitDeleteProgress(o.Progress, bucket, key, err)
			continue
		}
		valid = append(valid, key)
	}

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
		batch := valid[start:end]

		var batchResult DeleteResult
		var batchErr error
		useSingleDelete := s.singleDelete.Load()
		if useSingleDelete {
			batchResult, batchErr = s.singleDeleteBatch(ctx, client, bucket, batch)
		} else {
			batchResult, batchErr = s.deleteObjectsBatch(ctx, client, bucket, batch)
			if batchErr != nil {
				classified := Classify("delete", bucket, "", batchErr)
				if deleteNotImplemented(batchErr) {
					// singleDelete is session-wide: one non-AWS endpoint downgrades all buckets on this session.
					s.singleDelete.Store(true)
					useSingleDelete = true
					batchResult, batchErr = s.singleDeleteBatch(ctx, client, bucket, batch)
				} else {
					for _, key := range batch {
						emitDeleteProgress(o.Progress, bucket, key, classified)
					}
					return result, classified
				}
			}
		}

		mergeDeleteResult(&result, batchResult)
		if batchErr != nil {
			classified := Classify("delete", bucket, "", batchErr)
			if useSingleDelete {
				emitDeleteProgressForResult(o.Progress, bucket, batch, batchResult)
			} else {
				for _, key := range batch {
					emitDeleteProgress(o.Progress, bucket, key, classified)
				}
			}
			return result, classified
		}

		batchErrors := deleteErrorsByKey(batchResult.Failed)
		for _, key := range batch {
			emitDeleteProgress(o.Progress, bucket, key, batchErrors[key])
		}
	}

	return result, nil
}

// DeletePrefix walks prefix and deletes each full batch before the listing
// continues. prefix must be non-empty; an empty prefix is rejected because
// deleting a whole bucket is not supported. A prefix without a trailing slash
// is normalized to one, so "p" has directory semantics and matches "p/x" but
// not "p2/x". If the listing is incomplete, its final partial batch is not
// deleted.
func (s *Session) DeletePrefix(ctx context.Context, bucket, prefix string, o DeleteOptions) (DeleteResult, error) {
	if err := ValidateBucketName(bucket); err != nil {
		return DeleteResult{}, err
	}
	if prefix == "" {
		return DeleteResult{}, &Error{
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
		return DeleteResult{}, &Error{
			Kind:   KindInvalid,
			Op:     "delete-prefix",
			Bucket: bucket,
			Key:    prefix,
			Err:    err,
		}
	}

	var (
		mu        sync.Mutex
		deleted   DeleteResult
		deleteErr error
		batch     = make([]string, 0, MaxDeleteBatch)
	)
	deleteBatch := func(keys []string) {
		partial, err := s.DeleteKeys(ctx, bucket, keys, o)
		mergeDeleteResult(&deleted, partial)
		if err != nil && deleteErr == nil {
			deleteErr = err
		}
	}

	// The mutex serializes batch collection and deletion across concurrent
	// walk callbacks. A delete error is returned to the walker, which cancels
	// sibling crawls before later pages are requested.
	walkErr := s.Walk(ctx, bucket, prefix, WalkOptions{Concurrency: s.Options().Concurrency}, func(object Object) error {
		mu.Lock()
		defer mu.Unlock()
		if deleteErr != nil {
			return deleteErr
		}
		batch = append(batch, object.Key)
		if len(batch) < MaxDeleteBatch {
			return nil
		}
		keys := batch
		batch = make([]string, 0, MaxDeleteBatch)
		if err := ctx.Err(); err != nil {
			return Classify("list", bucket, prefix, err)
		}
		deleteBatch(keys)
		return deleteErr
	})

	// An incomplete listing is never partially deleted: only flush the
	// remaining batch after Walk has completed successfully.
	if walkErr == nil && deleteErr == nil && len(batch) > 0 {
		deleteBatch(batch)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		canceled := Classify("list", bucket, prefix, ctxErr)
		if deleteErr != nil {
			return deleted, errors.Join(canceled, deleteErr)
		}
		return deleted, canceled
	}
	if deleteErr != nil {
		return deleted, deleteErr
	}
	if walkErr != nil {
		return deleted, walkErr
	}
	return deleted, nil
}
