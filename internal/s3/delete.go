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

// DeletePrefix walks prefix and streams complete delete batches to one
// sequential batch consumer. prefix must be non-empty; an empty prefix is
// rejected because deleting a whole bucket is not supported. A prefix without
// a trailing slash is normalized to one, so "p" has directory semantics and
// matches "p/x" but not "p2/x". If the listing is incomplete, its final
// partial batch is not deleted.
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
	if err := ValidateKey(prefix); err != nil {
		return DeleteResult{}, &Error{
			Kind:   KindInvalid,
			Op:     "delete-prefix",
			Bucket: bucket,
			Key:    prefix,
			Err:    err,
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

	walkCtx, cancelWalk := context.WithCancel(ctx)
	defer cancelWalk()

	type prefixBatch struct {
		keys []string
		done chan struct{}
	}
	batches := make(chan prefixBatch)
	var deleted DeleteResult
	var deleteErr error
	stopDeleting := false
	var deleter sync.WaitGroup
	deleter.Add(1)
	go func() {
		defer deleter.Done()
		for item := range batches {
			batch := item.keys
			if stopDeleting {
				close(item.done)
				continue
			}

			partial, err := s.DeleteKeys(ctx, bucket, batch, o)
			mergeDeleteResult(&deleted, partial)
			if err != nil && deleteErr == nil {
				deleteErr = err
			}
			if err != nil {
				stopDeleting = true
				cancelWalk()
			}
			close(item.done)
		}
	}()

	batch := make([]string, 0, MaxDeleteBatch)
	// Walk serializes callbacks. Each complete batch waits for the consumer to
	// finish before listing can continue, so a hard delete error cancels the walk
	// before later pages are requested.
	walkErr := s.Walk(walkCtx, bucket, prefix, WalkOptions{Concurrency: s.Options().Concurrency}, func(object Object) error {
		batch = append(batch, object.Key)
		if len(batch) == MaxDeleteBatch {
			item := prefixBatch{keys: batch, done: make(chan struct{})}
			select {
			case batches <- item:
			case <-walkCtx.Done():
				return Classify("list", bucket, prefix, walkCtx.Err())
			}
			batch = make([]string, 0, MaxDeleteBatch)
			select {
			case <-item.done:
			case <-walkCtx.Done():
				return Classify("list", bucket, prefix, walkCtx.Err())
			}
		}
		return nil
	})
	// An incomplete listing is never partially deleted: only hand off the
	// remaining batch after Walk has completed successfully.
	if walkErr == nil && deleteErr == nil && len(batch) > 0 {
		item := prefixBatch{keys: batch, done: make(chan struct{})}
		select {
		case batches <- item:
			select {
			case <-item.done:
			case <-walkCtx.Done():
				walkErr = Classify("list", bucket, prefix, walkCtx.Err())
			}
		case <-walkCtx.Done():
			walkErr = Classify("list", bucket, prefix, walkCtx.Err())
		}
	}
	close(batches)
	deleter.Wait()

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
