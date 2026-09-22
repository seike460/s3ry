package s3

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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

// prefixCrawler carries the shared state of a parallel delimiter crawl.
type prefixCrawler struct {
	client  *awss3.Client
	bucket  string
	maxKeys int32
	fn      func(Object) error
	fnMu    sync.Mutex
	sem     chan struct{}
	seen    map[string]struct{}
	seenMu  sync.Mutex
	g       *errgroup.Group
}

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
	c := &prefixCrawler{
		client:  client,
		bucket:  bucket,
		maxKeys: o.MaxKeys,
		fn:      fn,
		sem:     make(chan struct{}, o.Concurrency),
		seen:    map[string]struct{}{prefix: {}},
		g:       g,
	}

	// The root prefix also occupies a pool slot; Concurrency > 1 here, so the
	// send never blocks and at most Concurrency listings run in parallel.
	c.sem <- struct{}{}
	g.Go(func() error {
		defer func() { <-c.sem }()
		return c.crawl(walkCtx, prefix)
	})
	err := g.Wait()

	var recovered walkPanic
	if errors.As(err, &recovered) {
		panic(recovered.value)
	}
	return err
}

// crawl lists every page of current, delivering objects to fn and crawling
// each discovered child prefix.
func (c *prefixCrawler) crawl(ctx context.Context, current string) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = walkPanic{value: value}
		}
	}()

	token := ""
	for {
		if err := ctx.Err(); err != nil {
			return Classify("list", c.bucket, current, err)
		}

		out, err := c.client.ListObjectsV2(ctx, listObjectsInput(c.bucket, current, token, c.maxKeys, aws.String("/")))
		if err != nil {
			return Classify("list", c.bucket, current, err)
		}
		if err := c.deliver(ctx, current, out.Contents); err != nil {
			return err
		}
		if err := c.visitPrefixes(ctx, current, out.CommonPrefixes); err != nil {
			return err
		}

		token = aws.ToString(out.NextContinuationToken)
		if token == "" {
			return nil
		}
	}
}

// deliver serializes fn calls for one page of objects.
func (c *prefixCrawler) deliver(ctx context.Context, current string, items []s3types.Object) error {
	for _, item := range items {
		c.fnMu.Lock()
		if ctx.Err() != nil {
			c.fnMu.Unlock()
			return Classify("list", c.bucket, current, ctx.Err())
		}
		callbackErr := c.fn(objectFromListObject(item))
		c.fnMu.Unlock()
		if callbackErr != nil {
			return callbackErr
		}
	}
	return nil
}

// visitPrefixes crawls each unseen child prefix, in the pool when a slot is
// free and inline when the pool is full so a full semaphore cannot deadlock
// the tree walk.
func (c *prefixCrawler) visitPrefixes(ctx context.Context, current string, prefixes []s3types.CommonPrefix) error {
	for _, item := range prefixes {
		p := aws.ToString(item.Prefix)
		if p == "" || !strings.HasPrefix(p, current) || len(p) <= len(current) || !c.markSeen(p) {
			continue
		}
		select {
		case c.sem <- struct{}{}:
			c.g.Go(func() error {
				defer func() { <-c.sem }()
				return c.crawl(ctx, p)
			})
		default:
			if err := c.crawl(ctx, p); err != nil {
				return err
			}
		}
	}
	return nil
}

// markSeen records p and reports whether it was new.
func (c *prefixCrawler) markSeen(p string) bool {
	c.seenMu.Lock()
	defer c.seenMu.Unlock()
	if _, duplicate := c.seen[p]; duplicate {
		return false
	}
	c.seen[p] = struct{}{}
	return true
}
