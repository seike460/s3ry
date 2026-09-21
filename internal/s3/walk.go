package s3

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
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

type prefixCrawler struct {
	mu          sync.Mutex
	cond        *sync.Cond
	queue       []string
	seen        map[string]struct{}
	outstanding int
	firstErr    error
	panicValue  any
	panicSet    bool
}

func (w *prefixCrawler) take() (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for len(w.queue) == 0 && w.outstanding > 0 && w.firstErr == nil {
		w.cond.Wait()
	}
	if w.firstErr != nil || (len(w.queue) == 0 && w.outstanding == 0) {
		return "", false
	}

	prefix := w.queue[0]
	w.queue[0] = ""
	w.queue = w.queue[1:]
	return prefix, true
}

func (w *prefixCrawler) add(prefix string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.firstErr != nil {
		return false
	}
	if _, ok := w.seen[prefix]; ok {
		return true
	}
	w.seen[prefix] = struct{}{}
	w.queue = append(w.queue, prefix)
	w.outstanding++
	w.cond.Signal()
	return true
}

func (w *prefixCrawler) done() {
	w.mu.Lock()
	w.outstanding--
	if w.outstanding == 0 {
		w.cond.Broadcast()
	}
	w.mu.Unlock()
}

func (w *prefixCrawler) setErr(err error, cancel context.CancelFunc) {
	if err == nil {
		return
	}

	w.mu.Lock()
	if w.firstErr != nil {
		w.mu.Unlock()
		return
	}
	w.firstErr = err
	w.cond.Broadcast()
	w.mu.Unlock()
	cancel()
}

func (w *prefixCrawler) err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.firstErr
}

func (w *prefixCrawler) complete() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.firstErr == nil && w.outstanding == 0
}

func (w *prefixCrawler) setErrIfIncomplete(err error, cancel context.CancelFunc) {
	if err == nil {
		return
	}

	w.mu.Lock()
	if w.firstErr != nil || w.outstanding == 0 {
		w.mu.Unlock()
		return
	}
	w.firstErr = err
	w.cond.Broadcast()
	w.mu.Unlock()
	cancel()
}

func (w *prefixCrawler) recordPanic(value any) {
	w.mu.Lock()
	if !w.panicSet {
		w.panicValue = value
		w.panicSet = true
	}
	w.mu.Unlock()
}

func (w *prefixCrawler) panicInfo() (any, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.panicValue, w.panicSet
}

func (s *Session) walkParallel(ctx context.Context, client *awss3.Client, bucket, prefix string, o WalkOptions, fn func(Object) error) error {
	if o.Concurrency > 64 {
		o.Concurrency = 64
	}

	walkCtx, cancel := context.WithCancel(ctx)
	crawler := &prefixCrawler{
		queue:       []string{prefix},
		seen:        map[string]struct{}{prefix: {}},
		outstanding: 1,
	}
	crawler.cond = sync.NewCond(&crawler.mu)

	var fnMu sync.Mutex
	var workers sync.WaitGroup
	workers.Add(o.Concurrency)
	for i := 0; i < o.Concurrency; i++ {
		go func() {
			defer workers.Done()
			defer func() {
				if value := recover(); value != nil {
					crawler.recordPanic(value)
					crawler.setErr(fmt.Errorf("panic in Walk callback: %v", value), cancel)
				}
			}()
			for {
				currentPrefix, ok := crawler.take()
				if !ok {
					return
				}
				s.walkPrefix(ctx, walkCtx, cancel, client, bucket, currentPrefix, crawler, &fnMu, o.MaxKeys, fn)
			}
		}()
	}

	watchDone := make(chan struct{})
	var watcher sync.WaitGroup
	watcher.Add(1)
	go func() {
		defer watcher.Done()
		select {
		case <-ctx.Done():
			if !crawler.complete() {
				if err := ctx.Err(); err != nil {
					crawler.setErrIfIncomplete(Classify("list", bucket, prefix, err), cancel)
				}
			}
		case <-watchDone:
		}
	}()

	workers.Wait()
	close(watchDone)
	watcher.Wait()
	cancel()
	if value, ok := crawler.panicInfo(); ok {
		panic(value)
	}
	return crawler.err()
}

func (s *Session) walkPrefix(ctx context.Context, walkCtx context.Context, cancel context.CancelFunc, client *awss3.Client, bucket, prefix string, crawler *prefixCrawler, fnMu *sync.Mutex, maxKeys int32, fn func(Object) error) {
	token := ""
	defer crawler.done()

	for {
		if crawler.err() != nil {
			return
		}
		if err := ctx.Err(); err != nil {
			crawler.setErr(Classify("list", bucket, prefix, err), cancel)
			return
		}

		out, err := client.ListObjectsV2(walkCtx, listObjectsInput(bucket, prefix, token, maxKeys, aws.String("/")))
		if err != nil {
			crawler.setErr(Classify("list", bucket, prefix, err), cancel)
			return
		}

		for _, item := range out.Contents {
			var callbackErr error
			var callbackCalled bool
			fnMu.Lock()
			func() {
				defer fnMu.Unlock()
				if crawler.err() != nil {
					return
				}
				if err := ctx.Err(); err != nil {
					callbackErr = Classify("list", bucket, prefix, err)
					crawler.setErr(callbackErr, cancel)
					return
				}
				callbackCalled = true
				callbackErr = fn(objectFromListObject(item))
				if callbackErr != nil {
					crawler.setErr(callbackErr, cancel)
					return
				}
				if err := ctx.Err(); err != nil {
					callbackErr = Classify("list", bucket, prefix, err)
					crawler.setErr(callbackErr, cancel)
				}
			}()
			if !callbackCalled || callbackErr != nil {
				return
			}
		}

		for _, item := range out.CommonPrefixes {
			p := aws.ToString(item.Prefix)
			if p == "" || !strings.HasPrefix(p, prefix) || len(p) <= len(prefix) {
				continue
			}
			if !crawler.add(p) {
				return
			}
		}

		nextToken := aws.ToString(out.NextContinuationToken)
		if nextToken == "" {
			return
		}
		token = nextToken
	}
}
