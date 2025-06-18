package worker

import "time"

// Default configuration constants
const (
	// DefaultWorkerCount represents the default number of workers
	// 0 means use the number of CPU cores
	DefaultWorkerCount = 0

	// DefaultQueueSize represents the default size of the job queue
	DefaultQueueSize = 100

	// DefaultRetryCount represents the default number of retries for failed jobs
	DefaultRetryCount = 3

	// DefaultTimeout represents the default timeout for individual jobs
	DefaultTimeout = 30 * time.Second

	// DefaultRetryDelay represents the default delay between retries
	DefaultRetryDelay = 1 * time.Second

	// MaxWorkerCount represents the maximum number of workers allowed
	MaxWorkerCount = 1000

	// MaxQueueSize represents the maximum queue size allowed
	MaxQueueSize = 10000

	// MaxRetryCount represents the maximum number of retries allowed
	MaxRetryCount = 10

	// MinTimeout represents the minimum timeout allowed
	MinTimeout = 1 * time.Second

	// MaxTimeout represents the maximum timeout allowed
	MaxTimeout = 10 * time.Minute
)

// Job execution states
const (
	// JobStatePending indicates the job is waiting to be executed
	JobStatePending = "pending"

	// JobStateRunning indicates the job is currently being executed
	JobStateRunning = "running"

	// JobStateCompleted indicates the job completed successfully
	JobStateCompleted = "completed"

	// JobStateFailed indicates the job failed
	JobStateFailed = "failed"

	// JobStateCancelled indicates the job was cancelled
	JobStateCancelled = "cancelled"

	// JobStateRetrying indicates the job is being retried
	JobStateRetrying = "retrying"
)

// Pool states
const (
	// PoolStateIdle indicates the pool is idle
	PoolStateIdle = "idle"

	// PoolStateRunning indicates the pool is running
	PoolStateRunning = "running"

	// PoolStateStopping indicates the pool is stopping
	PoolStateStopping = "stopping"

	// PoolStateStopped indicates the pool is stopped
	PoolStateStopped = "stopped"
)

// Metrics constants
const (
	// MetricsBufferSize represents the size of the metrics buffer
	MetricsBufferSize = 1000

	// MetricsFlushInterval represents how often metrics are flushed
	MetricsFlushInterval = 10 * time.Second

	// MetricsRetentionPeriod represents how long metrics are retained
	MetricsRetentionPeriod = 24 * time.Hour
)

// Error messages
const (
	// ErrPoolNotStarted indicates the pool has not been started
	ErrPoolNotStarted = "worker pool not started"

	// ErrPoolStopped indicates the pool has been stopped
	ErrPoolStopped = "worker pool stopped"

	// ErrJobQueueFull indicates the job queue is full
	ErrJobQueueFull = "job queue is full"

	// ErrInvalidWorkerCount indicates an invalid worker count
	ErrInvalidWorkerCount = "invalid worker count"

	// ErrInvalidQueueSize indicates an invalid queue size
	ErrInvalidQueueSize = "invalid queue size"

	// ErrInvalidTimeout indicates an invalid timeout
	ErrInvalidTimeout = "invalid timeout"

	// ErrJobTimeout indicates a job timed out
	ErrJobTimeout = "job execution timed out"

	// ErrJobCancelled indicates a job was cancelled
	ErrJobCancelled = "job was cancelled"
)

// Performance tuning constants
const (
	// OptimalWorkerRatio represents the optimal ratio of workers to CPU cores
	OptimalWorkerRatio = 2.0

	// HighLoadWorkerRatio represents the worker ratio for high load scenarios
	HighLoadWorkerRatio = 4.0

	// LowLoadWorkerRatio represents the worker ratio for low load scenarios
	LowLoadWorkerRatio = 1.0

	// QueueSizeMultiplier represents the multiplier for queue size based on worker count
	QueueSizeMultiplier = 10

	// BatchSizeOptimal represents the optimal batch size for batch processing
	BatchSizeOptimal = 50

	// BatchSizeMax represents the maximum batch size
	BatchSizeMax = 1000
)

// Resource limits
const (
	// MaxMemoryUsageBytes represents the maximum memory usage in bytes
	MaxMemoryUsageBytes = 1024 * 1024 * 1024 // 1GB

	// MaxCPUUsagePercent represents the maximum CPU usage percentage
	MaxCPUUsagePercent = 80.0

	// MaxGoroutines represents the maximum number of goroutines
	MaxGoroutines = 10000

	// MemoryCheckInterval represents how often memory usage is checked
	MemoryCheckInterval = 30 * time.Second

	// CPUCheckInterval represents how often CPU usage is checked
	CPUCheckInterval = 10 * time.Second
)

// Monitoring constants
const (
	// HealthCheckInterval represents how often health checks are performed
	HealthCheckInterval = 5 * time.Second

	// HealthCheckTimeout represents the timeout for health checks
	HealthCheckTimeout = 2 * time.Second

	// AlertThresholdErrorRate represents the error rate threshold for alerts
	AlertThresholdErrorRate = 0.1 // 10%

	// AlertThresholdLatency represents the latency threshold for alerts
	AlertThresholdLatency = 5 * time.Second

	// AlertCooldownPeriod represents the cooldown period between alerts
	AlertCooldownPeriod = 5 * time.Minute
)

// Configuration validation constants
const (
	// MinWorkerCount represents the minimum number of workers
	MinWorkerCount = 1

	// MinQueueSize represents the minimum queue size
	MinQueueSize = 1

	// MinRetryDelay represents the minimum retry delay
	MinRetryDelay = 100 * time.Millisecond

	// MaxRetryDelay represents the maximum retry delay
	MaxRetryDelay = 1 * time.Minute
)
