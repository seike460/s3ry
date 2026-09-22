package s3

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
)

// defaultProgressInterval throttles intermediate progress reports when a
// transfer does not request its own cadence.
const defaultProgressInterval = 100 * time.Millisecond

// meter throttles progress callbacks from concurrent transfer workers. It
// tracks the highest transferred offset, enforces interval between
// intermediate reports, and guarantees exactly one terminal report via done.
type meter struct {
	mu       sync.Mutex
	fn       ProgressFunc
	base     Progress
	interval time.Duration
	n        atomic.Int64
	last     atomic.Int64
	done     atomic.Bool
}

func (m *meter) add(delta int64) {
	if delta <= 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.done.Load() {
		return
	}

	m.n.Add(delta)
	if m.base.Total >= 0 && m.n.Load() > m.base.Total {
		m.n.Store(m.base.Total)
	}
	m.reportLocked()
}

func (m *meter) addAt(offset, delta int64) {
	if delta <= 0 || offset < 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.done.Load() {
		return
	}

	end := offset + delta
	if end < offset {
		end = int64(^uint64(0) >> 1)
	}
	if m.base.Total >= 0 && end > m.base.Total {
		end = m.base.Total
	}
	if end <= m.n.Load() {
		return
	}

	m.n.Store(end)
	m.reportLocked()
}

func (m *meter) reportLocked() {
	if m.fn == nil {
		return
	}

	interval := m.interval
	if interval <= 0 {
		interval = defaultProgressInterval
	}
	now := time.Now().UnixNano()
	last := m.last.Load()
	if now-last < interval.Nanoseconds() {
		return
	}
	m.last.Store(now)

	progress := m.base
	progress.Transferred = m.n.Load()
	progress.Done = false
	progress.Err = nil
	m.fn(progress)
}

func (m *meter) finish(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.done.CompareAndSwap(false, true) {
		return
	}
	if m.fn == nil {
		return
	}

	progress := m.base
	progress.Transferred = m.n.Load()
	progress.Done = true
	progress.Err = err
	m.fn(progress)
}

func (m *meter) finishSkipped() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.done.CompareAndSwap(false, true) {
		return
	}
	if m.fn == nil {
		return
	}

	progress := m.base
	progress.Transferred = m.n.Load()
	progress.Done = true
	progress.Skipped = true
	m.fn(progress)
}

func (m *meter) setTotal(total int64) {
	m.mu.Lock()
	if !m.done.Load() {
		m.base.Total = total
		if total >= 0 && m.n.Load() > total {
			m.n.Store(total)
		}
	}
	m.mu.Unlock()
}

// countingFile reports reads (uploads) and writes (downloads) to the meter.
type countingFile struct {
	*os.File
	m *meter
}

func (f *countingFile) Read(p []byte) (int, error) {
	n, err := f.File.Read(p)
	if n > 0 && f.m != nil {
		f.m.add(int64(n))
	}
	return n, err
}

// downloadProgressListener seeds the meter with the object's total size when
// the transfer manager announces it.
type downloadProgressListener struct {
	m *meter
}

func (l *downloadProgressListener) OnObjectTransferStart(_ context.Context, event *transfermanager.ObjectTransferStartEvent) {
	if event != nil {
		l.m.setTotal(event.TotalBytes)
	}
}

func (f *countingFile) WriteAt(p []byte, offset int64) (int, error) {
	n, err := f.File.WriteAt(p, offset)
	if n > 0 && f.m != nil {
		f.m.addAt(offset, int64(n))
	}
	return n, err
}
