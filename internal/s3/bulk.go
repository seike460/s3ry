package s3

import (
	"context"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// bulkTransfers runs per-key transfers on a bounded errgroup and aggregates
// per-key failures. In fail-fast mode the first error cancels the rest; in
// continue mode every failure is collected into a BulkError.
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
