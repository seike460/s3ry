package s3

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"golang.org/x/sync/errgroup"
)

// WalkOptions controls how Walk traverses a bucket.
//
// MaxKeys values less than or equal to zero use 1000. With Concurrency greater
// than one, callback order is unspecified. Walk serializes calls to fn, so fn
// need not be goroutine-safe, but a slow fn serializes all workers. The
// delimiter crawl issues one ListObjectsV2 request per discovered prefix, so
// deep, narrow trees require more requests; use Concurrency 1 for flat
// buckets.
type WalkOptions struct {
	Concurrency int
	MaxKeys     int32
}

// Walk calls fn for every object under prefix. A concurrency of one or less
// uses S3's lexical listing order. Larger values crawl delimiter prefixes in
// parallel, with unspecified callback order. Calls to fn are serialized; a
// slow callback therefore serializes all workers. MaxKeys values less than or
// equal to zero use 1000. Delimiter crawling issues one ListObjectsV2 request
// per discovered prefix, which can mean more requests on deep, narrow trees;
// use Concurrency 1 for flat buckets.
func (s *Session) Walk(ctx context.Context, bucket, prefix string, o WalkOptions, fn func(Object) error) error {
	if err := ValidateBucketName(bucket); err != nil {
		return err
	}

	client, err := s.client(ctx, bucket)
	if err != nil {
		return Classify("list", bucket, prefix, err)
	}
	if err := ctx.Err(); err != nil {
		return Classify("list", bucket, prefix, err)
	}

	if o.Concurrency <= 1 {
		return s.walkSequential(ctx, client, bucket, prefix, o.MaxKeys, fn)
	}
	return s.walkParallel(ctx, client, bucket, prefix, o, fn)
}

func (s *Session) walkSequential(ctx context.Context, client *awss3.Client, bucket, prefix string, maxKeys int32, fn func(Object) error) error {
	token := ""
	for {
		if err := ctx.Err(); err != nil {
			return Classify("list", bucket, prefix, err)
		}

		out, err := client.ListObjectsV2(ctx, listObjectsInput(bucket, prefix, token, maxKeys, nil))
		if err != nil {
			return Classify("list", bucket, prefix, err)
		}

		for _, item := range out.Contents {
			if err := fn(objectFromListObject(item)); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return Classify("list", bucket, prefix, err)
		}

		nextToken := aws.ToString(out.NextContinuationToken)
		if nextToken == "" {
			return nil
		}
		token = nextToken
	}
}

// walkPanic carries a recovered Walk callback panic through the errgroup so
// it can be repanicked on the calling goroutine after Wait returns.
type walkPanic struct{ value any }

func (e walkPanic) Error() string {
	return fmt.Sprintf("panic in Walk callback: %v", e.value)
}

// maxWalkConcurrency bounds the prefix crawler's goroutine pool.
const maxWalkConcurrency = 64

// walkParallel crawls delimiter prefixes with a bounded goroutine pool. Each
// discovered child prefix either acquires a semaphore slot and runs in the
// errgroup or, when the pool is full, is crawled inline by the current
// goroutine — this keeps the tree walk deadlock-free. errgroup.WithContext
// cancels sibling crawls on the first error, and callback panics are
// repanicked on the caller's goroutine.
func (s *Session) walkParallel(ctx context.Context, client *awss3.Client, bucket, prefix string, o WalkOptions, fn func(Object) error) error {
	if o.Concurrency > maxWalkConcurrency {
		o.Concurrency = maxWalkConcurrency
	}

	g, walkCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, o.Concurrency)
	var fnMu sync.Mutex
	var seenMu sync.Mutex
	seen := map[string]struct{}{prefix: {}}

	var crawl func(context.Context, string) error
	crawl = func(callCtx context.Context, current string) (err error) {
		defer func() {
			if value := recover(); value != nil {
				err = walkPanic{value: value}
			}
		}()

		token := ""
		for {
			if err := callCtx.Err(); err != nil {
				return Classify("list", bucket, current, err)
			}

			out, err := client.ListObjectsV2(callCtx, listObjectsInput(bucket, current, token, o.MaxKeys, aws.String("/")))
			if err != nil {
				return Classify("list", bucket, current, err)
			}

			for _, item := range out.Contents {
				fnMu.Lock()
				if callCtx.Err() != nil {
					fnMu.Unlock()
					return Classify("list", bucket, current, callCtx.Err())
				}
				callbackErr := fn(objectFromListObject(item))
				fnMu.Unlock()
				if callbackErr != nil {
					return callbackErr
				}
			}

			for _, item := range out.CommonPrefixes {
				p := aws.ToString(item.Prefix)
				if p == "" || !strings.HasPrefix(p, current) || len(p) <= len(current) {
					continue
				}

				seenMu.Lock()
				if _, duplicate := seen[p]; duplicate {
					seenMu.Unlock()
					continue
				}
				seen[p] = struct{}{}
				seenMu.Unlock()

				select {
				case sem <- struct{}{}:
					g.Go(func() error {
						defer func() { <-sem }()
						return crawl(callCtx, p)
					})
				default:
					// Pool is full: crawl inline so a full semaphore cannot
					// deadlock the tree walk.
					if err := crawl(callCtx, p); err != nil {
						return err
					}
				}
			}

			token = aws.ToString(out.NextContinuationToken)
			if token == "" {
				return nil
			}
		}
	}

	// The root prefix also occupies a pool slot; Concurrency > 1 here, so the
	// send never blocks and at most Concurrency listings run in parallel.
	sem <- struct{}{}
	g.Go(func() error {
		defer func() { <-sem }()
		return crawl(walkCtx, prefix)
	})
	err := g.Wait()

	var recovered walkPanic
	if errors.As(err, &recovered) {
		panic(recovered.value)
	}
	return err
}
