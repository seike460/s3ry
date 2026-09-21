package s3

import (
	"context"
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"golang.org/x/sync/errgroup"
)

func (s *Session) deleteObjectsBatch(ctx context.Context, client *awss3.Client, bucket string, keys []string) (DeleteResult, error) {
	objects := make([]s3types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = s3types.ObjectIdentifier{Key: aws.String(key)}
	}

	out, err := client.DeleteObjects(ctx, &awss3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3types.Delete{
			Objects: objects,
			Quiet:   aws.Bool(false),
		},
	})
	if err != nil {
		return DeleteResult{}, err
	}

	result := DeleteResult{Deleted: make([]string, 0, len(out.Deleted))}
	seen := make(map[string]struct{}, len(out.Deleted)+len(out.Errors))
	for _, item := range out.Deleted {
		// S3 reports keys that do not exist as Deleted; this is S3 semantics and
		// is intentionally preserved here.
		key := aws.ToString(item.Key)
		seen[key] = struct{}{}
		result.Deleted = append(result.Deleted, key)
	}
	for _, item := range out.Errors {
		key := aws.ToString(item.Key)
		seen[key] = struct{}{}
		classified := Classify("delete", bucket, key, &smithy.GenericAPIError{
			Code:    aws.ToString(item.Code),
			Message: aws.ToString(item.Message),
		})
		result.Failed = append(result.Failed, KeyError{Key: key, Err: classified})
	}
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		result.Failed = append(result.Failed, KeyError{
			Key: key,
			Err: &Error{
				Kind:   KindUnknown,
				Op:     "delete",
				Bucket: bucket,
				Key:    key,
				Err:    errors.New("DeleteObjects response omitted key"),
			},
		})
	}
	return result, nil
}

func (s *Session) singleDeleteBatch(ctx context.Context, client *awss3.Client, bucket string, keys []string) (DeleteResult, error) {
	group, groupCtx := errgroup.WithContext(ctx)
	concurrency := s.Options().Concurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	group.SetLimit(concurrency)

	var mu sync.Mutex
	var result DeleteResult
	for _, key := range keys {
		key := key
		group.Go(func() error {
			if groupCtx.Err() != nil {
				return nil
			}
			_, err := client.DeleteObject(groupCtx, &awss3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			if err != nil {
				classified := Classify("delete", bucket, key, err)
				if errors.Is(classified, ErrCanceled) {
					return nil
				}
				if errors.Is(classified, ErrNotFound) {
					mu.Lock()
					result.Deleted = append(result.Deleted, key)
					mu.Unlock()
					return nil
				}
				mu.Lock()
				result.Failed = append(result.Failed, KeyError{Key: key, Err: classified})
				mu.Unlock()
				return classified
			}

			mu.Lock()
			result.Deleted = append(result.Deleted, key)
			mu.Unlock()
			return nil
		})
	}
	groupErr := group.Wait()
	if err := ctx.Err(); err != nil {
		return result, Classify("delete", bucket, "", err)
	}
	return result, groupErr
}

func emitDeleteProgress(fn ProgressFunc, bucket, key string, err error) {
	if fn == nil {
		return
	}
	fn(Progress{Op: OpDelete, Bucket: bucket, Key: key, Done: true, Err: err})
}

func emitDeleteProgressForResult(fn ProgressFunc, bucket string, keys []string, result DeleteResult) {
	failed := deleteErrorsByKey(result.Failed)
	deleted := make(map[string]struct{}, len(result.Deleted))
	for _, key := range result.Deleted {
		deleted[key] = struct{}{}
	}
	for _, key := range keys {
		if err, ok := failed[key]; ok {
			emitDeleteProgress(fn, bucket, key, err)
			continue
		}
		if _, ok := deleted[key]; ok {
			emitDeleteProgress(fn, bucket, key, nil)
		}
	}
}

func deleteErrorsByKey(failed []KeyError) map[string]error {
	if len(failed) == 0 {
		return nil
	}
	result := make(map[string]error, len(failed))
	for _, failure := range failed {
		result[failure.Key] = failure.Err
	}
	return result
}

func mergeDeleteResult(dst *DeleteResult, src DeleteResult) {
	dst.Deleted = append(dst.Deleted, src.Deleted...)
	dst.Failed = append(dst.Failed, src.Failed...)
}
