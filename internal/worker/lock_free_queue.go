package worker

import (
	"sync/atomic"
	"unsafe"
)

// LockFreeQueue implements a high-performance lock-free queue for job processing
// Based on Michael & Scott's lock-free queue algorithm optimized for S3ry
type LockFreeQueue struct {
	head unsafe.Pointer // Points to node
	tail unsafe.Pointer // Points to node
	size int64          // Atomic counter for queue size
}

// QueueNode represents a node in the lock-free queue
type QueueNode struct {
	data unsafe.Pointer // Points to Job interface
	next unsafe.Pointer // Points to next QueueNode
}

// NewLockFreeQueue creates a new lock-free queue optimized for high throughput
func NewLockFreeQueue() *LockFreeQueue {
	// Create dummy node to simplify queue operations
	dummy := &QueueNode{}
	queue := &LockFreeQueue{
		head: unsafe.Pointer(dummy),
		tail: unsafe.Pointer(dummy),
	}
	return queue
}

// Enqueue adds a job to the queue in a lock-free manner
func (q *LockFreeQueue) Enqueue(job Job) bool {
	// Create new node for the job
	newNode := &QueueNode{
		data: unsafe.Pointer(&job),
	}

	for {
		// Read current tail
		tail := (*QueueNode)(atomic.LoadPointer(&q.tail))
		next := (*QueueNode)(atomic.LoadPointer(&tail.next))

		// Check if tail is still the same (ABA problem prevention)
		if tail == (*QueueNode)(atomic.LoadPointer(&q.tail)) {
			if next == nil {
				// Tail is pointing to the last node, try to link new node
				if atomic.CompareAndSwapPointer(&tail.next, nil, unsafe.Pointer(newNode)) {
					// Successfully linked, now advance tail
					atomic.CompareAndSwapPointer(&q.tail, unsafe.Pointer(tail), unsafe.Pointer(newNode))
					atomic.AddInt64(&q.size, 1)
					return true
				}
			} else {
				// Tail is not pointing to the last node, try to advance it
				atomic.CompareAndSwapPointer(&q.tail, unsafe.Pointer(tail), unsafe.Pointer(next))
			}
		}
	}
}

// Dequeue removes and returns a job from the queue in a lock-free manner
func (q *LockFreeQueue) Dequeue() (Job, bool) {
	for {
		// Read current head and tail
		head := (*QueueNode)(atomic.LoadPointer(&q.head))
		tail := (*QueueNode)(atomic.LoadPointer(&q.tail))
		next := (*QueueNode)(atomic.LoadPointer(&head.next))

		// Check if head is still the same (ABA problem prevention)
		if head == (*QueueNode)(atomic.LoadPointer(&q.head)) {
			if head == tail {
				if next == nil {
					// Queue is empty
					return nil, false
				}
				// Tail is falling behind, try to advance it
				atomic.CompareAndSwapPointer(&q.tail, unsafe.Pointer(tail), unsafe.Pointer(next))
			} else {
				if next == nil {
					// Inconsistent state, continue
					continue
				}

				// Read data before advancing head
				data := atomic.LoadPointer(&next.data)

				// Try to advance head
				if atomic.CompareAndSwapPointer(&q.head, unsafe.Pointer(head), unsafe.Pointer(next)) {
					// Successfully dequeued
					atomic.AddInt64(&q.size, -1)
					if data != nil {
						return *(*Job)(data), true
					}
					// Data was nil, continue to next iteration
					continue
				}
			}
		}
	}
}

// Size returns the approximate size of the queue
func (q *LockFreeQueue) Size() int64 {
	return atomic.LoadInt64(&q.size)
}

// IsEmpty checks if the queue is empty
func (q *LockFreeQueue) IsEmpty() bool {
	head := (*QueueNode)(atomic.LoadPointer(&q.head))
	tail := (*QueueNode)(atomic.LoadPointer(&q.tail))
	return head == tail && atomic.LoadPointer(&head.next) == nil
}

// LockFreeRingBuffer implements a high-performance ring buffer for job queuing
type LockFreeRingBuffer struct {
	buffer []unsafe.Pointer // Ring buffer for jobs
	mask   uint64           // Size mask (size must be power of 2)
	head   uint64           // Producer position
	tail   uint64           // Consumer position
	size   uint64           // Buffer size
}

// NewLockFreeRingBuffer creates a new lock-free ring buffer
func NewLockFreeRingBuffer(size uint64) *LockFreeRingBuffer {
	// Ensure size is power of 2
	if size&(size-1) != 0 {
		// Round up to next power of 2
		size = nextPowerOf2(size)
	}

	return &LockFreeRingBuffer{
		buffer: make([]unsafe.Pointer, size),
		mask:   size - 1,
		size:   size,
	}
}

// Put adds a job to the ring buffer
func (rb *LockFreeRingBuffer) Put(job Job) bool {
	for {
		head := atomic.LoadUint64(&rb.head)
		tail := atomic.LoadUint64(&rb.tail)

		// Check if buffer is full
		if head-tail >= rb.size {
			return false
		}

		// Try to claim a slot
		if atomic.CompareAndSwapUint64(&rb.head, head, head+1) {
			// Successfully claimed slot, store job
			index := head & rb.mask
			atomic.StorePointer(&rb.buffer[index], unsafe.Pointer(&job))
			return true
		}
	}
}

// Get retrieves a job from the ring buffer
func (rb *LockFreeRingBuffer) Get() (Job, bool) {
	for {
		head := atomic.LoadUint64(&rb.head)
		tail := atomic.LoadUint64(&rb.tail)

		// Check if buffer is empty
		if tail >= head {
			return nil, false
		}

		// Try to claim a slot
		if atomic.CompareAndSwapUint64(&rb.tail, tail, tail+1) {
			// Successfully claimed slot, load job
			index := tail & rb.mask
			jobPtr := atomic.LoadPointer(&rb.buffer[index])
			if jobPtr != nil {
				job := *(*Job)(jobPtr)
				// Clear the slot for reuse
				atomic.StorePointer(&rb.buffer[index], nil)
				return job, true
			}
		}
	}
}

// Available returns the number of available slots
func (rb *LockFreeRingBuffer) Available() uint64 {
	head := atomic.LoadUint64(&rb.head)
	tail := atomic.LoadUint64(&rb.tail)
	return rb.size - (head - tail)
}

// Count returns the number of items in the buffer
func (rb *LockFreeRingBuffer) Count() uint64 {
	head := atomic.LoadUint64(&rb.head)
	tail := atomic.LoadUint64(&rb.tail)
	return head - tail
}

// nextPowerOf2 returns the next power of 2 greater than or equal to n
func nextPowerOf2(n uint64) uint64 {
	if n == 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	n++
	return n
}

// ParallelWorkStealer implements work-stealing for optimal load distribution
type ParallelWorkStealer struct {
	queues      []*LockFreeRingBuffer // Per-worker queues
	workerCount int
	stealIndex  uint64 // Atomic counter for stealing strategy
}

// NewParallelWorkStealer creates a work-stealing scheduler
func NewParallelWorkStealer(workerCount int, queueSize uint64) *ParallelWorkStealer {
	queues := make([]*LockFreeRingBuffer, workerCount)
	for i := 0; i < workerCount; i++ {
		queues[i] = NewLockFreeRingBuffer(queueSize)
	}

	return &ParallelWorkStealer{
		queues:      queues,
		workerCount: workerCount,
	}
}

// SubmitJob submits a job using work-stealing load balancing
func (pws *ParallelWorkStealer) SubmitJob(job Job, workerID int) bool {
	// Try to put in worker's own queue first
	if workerID < pws.workerCount && pws.queues[workerID].Put(job) {
		return true
	}

	// If own queue is full, try other queues
	for i := 0; i < pws.workerCount; i++ {
		queueIndex := (workerID + i) % pws.workerCount
		if pws.queues[queueIndex].Put(job) {
			return true
		}
	}

	return false // All queues are full
}

// GetJob retrieves a job using work-stealing algorithm
func (pws *ParallelWorkStealer) GetJob(workerID int) (Job, bool) {
	// Try own queue first
	if workerID < pws.workerCount {
		if job, ok := pws.queues[workerID].Get(); ok {
			return job, true
		}
	}

	// Steal from other workers' queues
	stealStart := atomic.AddUint64(&pws.stealIndex, 1) % uint64(pws.workerCount)

	for i := 0; i < pws.workerCount; i++ {
		victimIndex := (stealStart + uint64(i)) % uint64(pws.workerCount)
		if int(victimIndex) != workerID {
			if job, ok := pws.queues[victimIndex].Get(); ok {
				return job, true
			}
		}
	}

	return nil, false
}

// GetLoadStats returns load statistics for all queues
func (pws *ParallelWorkStealer) GetLoadStats() []QueueStats {
	stats := make([]QueueStats, pws.workerCount)

	for i := 0; i < pws.workerCount; i++ {
		queue := pws.queues[i]
		stats[i] = QueueStats{
			QueueID:   i,
			Count:     queue.Count(),
			Available: queue.Available(),
			LoadRatio: float64(queue.Count()) / float64(queue.size),
		}
	}

	return stats
}

// QueueStats provides queue performance statistics
type QueueStats struct {
	QueueID   int
	Count     uint64
	Available uint64
	LoadRatio float64
}
