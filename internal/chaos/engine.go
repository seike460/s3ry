package chaos

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// ChaosEngine implements chaos engineering experiments
type ChaosEngine struct {
	config      *ChaosConfig
	experiments map[string]ChaosExperiment
	metrics     *ChaosMetrics
	scheduler   *ExperimentScheduler
	mutex       sync.RWMutex
	running     bool
}

// ChaosConfig holds chaos engineering configuration
type ChaosConfig struct {
	Enabled           bool          `json:"enabled"`
	SafeMode          bool          `json:"safe_mode"`           // Enables automatic rollback
	MaxConcurrent     int           `json:"max_concurrent"`      // Max concurrent experiments
	DefaultDuration   time.Duration `json:"default_duration"`    // Default experiment duration
	CooldownPeriod    time.Duration `json:"cooldown_period"`     // Time between experiments
	TargetServices    []string      `json:"target_services"`     // Services to target
	ExcludeProduction bool          `json:"exclude_production"`  // Skip production environments
	NotificationURL   string        `json:"notification_url"`    // Webhook for notifications
	MetricsRetention  time.Duration `json:"metrics_retention"`   // How long to keep metrics
}

// DefaultChaosConfig returns default chaos engineering configuration
func DefaultChaosConfig() *ChaosConfig {
	return &ChaosConfig{
		Enabled:           false, // Disabled by default for safety
		SafeMode:          true,
		MaxConcurrent:     3,
		DefaultDuration:   time.Minute * 5,
		CooldownPeriod:    time.Minute * 15,
		TargetServices:    []string{"s3-client", "worker-pool", "metrics"},
		ExcludeProduction: true,
		MetricsRetention:  time.Hour * 24 * 7, // 7 days
	}
}

// ChaosExperiment interface for all chaos experiments
type ChaosExperiment interface {
	Name() string
	Description() string
	Execute(ctx context.Context, target string) (*ExperimentResult, error)
	Rollback(ctx context.Context, target string) error
	IsReversible() bool
	GetSeverity() SeverityLevel
	GetDuration() time.Duration
}

// SeverityLevel represents experiment severity
type SeverityLevel string

const (
	SeverityLow    SeverityLevel = "LOW"
	SeverityMedium SeverityLevel = "MEDIUM"
	SeverityHigh   SeverityLevel = "HIGH"
)

// ExperimentResult holds the result of a chaos experiment
type ExperimentResult struct {
	ExperimentName string            `json:"experiment_name"`
	Target         string            `json:"target"`
	StartTime      time.Time         `json:"start_time"`
	EndTime        time.Time         `json:"end_time"`
	Duration       time.Duration     `json:"duration"`
	Success        bool              `json:"success"`
	ErrorMessage   string            `json:"error_message,omitempty"`
	Metrics        map[string]float64 `json:"metrics"`
	Observations   []string          `json:"observations"`
	Impact         ImpactLevel       `json:"impact"`
}

// ImpactLevel represents the impact level of an experiment
type ImpactLevel string

const (
	ImpactNone     ImpactLevel = "NONE"
	ImpactMinor    ImpactLevel = "MINOR"
	ImpactModerate ImpactLevel = "MODERATE"
	ImpactMajor    ImpactLevel = "MAJOR"
	ImpactCritical ImpactLevel = "CRITICAL"
)

// ChaosMetrics tracks chaos engineering metrics
type ChaosMetrics struct {
	TotalExperiments    int64             `json:"total_experiments"`
	SuccessfulTests     int64             `json:"successful_tests"`
	FailedTests         int64             `json:"failed_tests"`
	SystemResilience    float64           `json:"system_resilience"`    // 0-100%
	MeanTimeToRecovery  time.Duration     `json:"mean_time_to_recovery"`
	ExperimentsByType   map[string]int64  `json:"experiments_by_type"`
	ImpactDistribution  map[string]int64  `json:"impact_distribution"`
	LastExperimentTime  time.Time         `json:"last_experiment_time"`
	mutex               sync.RWMutex
}

// NewChaosMetrics creates new chaos metrics
func NewChaosMetrics() *ChaosMetrics {
	return &ChaosMetrics{
		ExperimentsByType:  make(map[string]int64),
		ImpactDistribution: make(map[string]int64),
	}
}

// ExperimentScheduler manages experiment scheduling
type ExperimentScheduler struct {
	config    *ChaosConfig
	queue     []*ScheduledExperiment
	mutex     sync.Mutex
	stopCh    chan struct{}
	ticker    *time.Ticker
}

// ScheduledExperiment represents a scheduled chaos experiment
type ScheduledExperiment struct {
	Experiment   ChaosExperiment `json:"experiment"`
	Target       string          `json:"target"`
	ScheduledAt  time.Time       `json:"scheduled_at"`
	Priority     int             `json:"priority"`
	Recurring    bool            `json:"recurring"`
	Interval     time.Duration   `json:"interval,omitempty"`
}

// NetworkLatencyExperiment adds network latency
type NetworkLatencyExperiment struct {
	latencyMs int
	duration  time.Duration
}

// NewNetworkLatencyExperiment creates a network latency experiment
func NewNetworkLatencyExperiment(latencyMs int, duration time.Duration) *NetworkLatencyExperiment {
	return &NetworkLatencyExperiment{
		latencyMs: latencyMs,
		duration:  duration,
	}
}

func (n *NetworkLatencyExperiment) Name() string {
	return "network-latency"
}

func (n *NetworkLatencyExperiment) Description() string {
	return fmt.Sprintf("Introduces %dms network latency for %v", n.latencyMs, n.duration)
}

func (n *NetworkLatencyExperiment) Execute(ctx context.Context, target string) (*ExperimentResult, error) {
	result := &ExperimentResult{
		ExperimentName: n.Name(),
		Target:         target,
		StartTime:      time.Now(),
		Metrics:        make(map[string]float64),
		Observations:   []string{},
	}

	// Simulate network latency injection
	// In a real implementation, this would use tools like tc (traffic control)
	result.Observations = append(result.Observations, 
		fmt.Sprintf("Injected %dms latency on %s", n.latencyMs, target))

	// Simulate the experiment duration
	select {
	case <-ctx.Done():
		result.Success = false
		result.ErrorMessage = "Experiment cancelled"
	case <-time.After(n.duration):
		result.Success = true
		result.Impact = n.calculateImpact()
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Metrics["latency_ms"] = float64(n.latencyMs)
	result.Metrics["duration_seconds"] = result.Duration.Seconds()

	return result, nil
}

func (n *NetworkLatencyExperiment) Rollback(ctx context.Context, target string) error {
	// Remove network latency
	// In real implementation: remove tc rules, reset network config
	return nil
}

func (n *NetworkLatencyExperiment) IsReversible() bool {
	return true
}

func (n *NetworkLatencyExperiment) GetSeverity() SeverityLevel {
	if n.latencyMs > 1000 {
		return SeverityHigh
	} else if n.latencyMs > 500 {
		return SeverityMedium
	}
	return SeverityLow
}

func (n *NetworkLatencyExperiment) GetDuration() time.Duration {
	return n.duration
}

func (n *NetworkLatencyExperiment) calculateImpact() ImpactLevel {
	if n.latencyMs > 2000 {
		return ImpactMajor
	} else if n.latencyMs > 1000 {
		return ImpactModerate
	} else if n.latencyMs > 500 {
		return ImpactMinor
	}
	return ImpactNone
}

// MemoryPressureExperiment creates memory pressure
type MemoryPressureExperiment struct {
	pressureMB int
	duration   time.Duration
}

// NewMemoryPressureExperiment creates a memory pressure experiment
func NewMemoryPressureExperiment(pressureMB int, duration time.Duration) *MemoryPressureExperiment {
	return &MemoryPressureExperiment{
		pressureMB: pressureMB,
		duration:   duration,
	}
}

func (m *MemoryPressureExperiment) Name() string {
	return "memory-pressure"
}

func (m *MemoryPressureExperiment) Description() string {
	return fmt.Sprintf("Creates %dMB memory pressure for %v", m.pressureMB, m.duration)
}

func (m *MemoryPressureExperiment) Execute(ctx context.Context, target string) (*ExperimentResult, error) {
	result := &ExperimentResult{
		ExperimentName: m.Name(),
		Target:         target,
		StartTime:      time.Now(),
		Metrics:        make(map[string]float64),
		Observations:   []string{},
	}

	// Allocate memory to create pressure
	memoryBallast := make([]byte, m.pressureMB*1024*1024)
	_ = memoryBallast // Prevent optimization

	result.Observations = append(result.Observations, 
		fmt.Sprintf("Allocated %dMB memory on %s", m.pressureMB, target))

	// Hold memory for duration
	select {
	case <-ctx.Done():
		result.Success = false
		result.ErrorMessage = "Experiment cancelled"
	case <-time.After(m.duration):
		result.Success = true
		result.Impact = m.calculateImpact()
	}

	// Release memory (automatic garbage collection)
	memoryBallast = nil

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Metrics["memory_mb"] = float64(m.pressureMB)
	result.Metrics["duration_seconds"] = result.Duration.Seconds()

	return result, nil
}

func (m *MemoryPressureExperiment) Rollback(ctx context.Context, target string) error {
	// Memory is automatically released by GC
	return nil
}

func (m *MemoryPressureExperiment) IsReversible() bool {
	return true
}

func (m *MemoryPressureExperiment) GetSeverity() SeverityLevel {
	if m.pressureMB > 500 {
		return SeverityHigh
	} else if m.pressureMB > 100 {
		return SeverityMedium
	}
	return SeverityLow
}

func (m *MemoryPressureExperiment) GetDuration() time.Duration {
	return m.duration
}

func (m *MemoryPressureExperiment) calculateImpact() ImpactLevel {
	if m.pressureMB > 1000 {
		return ImpactMajor
	} else if m.pressureMB > 500 {
		return ImpactModerate
	} else if m.pressureMB > 100 {
		return ImpactMinor
	}
	return ImpactNone
}

// CPUStressExperiment creates CPU stress
type CPUStressExperiment struct {
	workers  int
	duration time.Duration
}

// NewCPUStressExperiment creates a CPU stress experiment
func NewCPUStressExperiment(workers int, duration time.Duration) *CPUStressExperiment {
	return &CPUStressExperiment{
		workers:  workers,
		duration: duration,
	}
}

func (c *CPUStressExperiment) Name() string {
	return "cpu-stress"
}

func (c *CPUStressExperiment) Description() string {
	return fmt.Sprintf("Creates CPU stress with %d workers for %v", c.workers, c.duration)
}

func (c *CPUStressExperiment) Execute(ctx context.Context, target string) (*ExperimentResult, error) {
	result := &ExperimentResult{
		ExperimentName: c.Name(),
		Target:         target,
		StartTime:      time.Now(),
		Metrics:        make(map[string]float64),
		Observations:   []string{},
	}

	// Create CPU stress workers
	stopCh := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < c.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					// Busy work to consume CPU
					for j := 0; j < 1000000; j++ {
						_ = j * j
					}
				}
			}
		}(i)
	}

	result.Observations = append(result.Observations, 
		fmt.Sprintf("Started %d CPU stress workers on %s", c.workers, target))

	// Run for specified duration
	select {
	case <-ctx.Done():
		result.Success = false
		result.ErrorMessage = "Experiment cancelled"
	case <-time.After(c.duration):
		result.Success = true
		result.Impact = c.calculateImpact()
	}

	// Stop workers
	close(stopCh)
	wg.Wait()

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Metrics["cpu_workers"] = float64(c.workers)
	result.Metrics["duration_seconds"] = result.Duration.Seconds()

	return result, nil
}

func (c *CPUStressExperiment) Rollback(ctx context.Context, target string) error {
	// Workers are automatically stopped when experiment ends
	return nil
}

func (c *CPUStressExperiment) IsReversible() bool {
	return true
}

func (c *CPUStressExperiment) GetSeverity() SeverityLevel {
	if c.workers > 8 {
		return SeverityHigh
	} else if c.workers > 4 {
		return SeverityMedium
	}
	return SeverityLow
}

func (c *CPUStressExperiment) GetDuration() time.Duration {
	return c.duration
}

func (c *CPUStressExperiment) calculateImpact() ImpactLevel {
	if c.workers > 16 {
		return ImpactMajor
	} else if c.workers > 8 {
		return ImpactModerate
	} else if c.workers > 4 {
		return ImpactMinor
	}
	return ImpactNone
}

// NewChaosEngine creates a new chaos engineering engine
func NewChaosEngine(config *ChaosConfig) *ChaosEngine {
	if config == nil {
		config = DefaultChaosConfig()
	}

	engine := &ChaosEngine{
		config:      config,
		experiments: make(map[string]ChaosExperiment),
		metrics:     NewChaosMetrics(),
		scheduler:   NewExperimentScheduler(config),
	}

	// Register default experiments
	engine.RegisterExperiment("network-latency-low", NewNetworkLatencyExperiment(100, config.DefaultDuration))
	engine.RegisterExperiment("network-latency-medium", NewNetworkLatencyExperiment(500, config.DefaultDuration))
	engine.RegisterExperiment("network-latency-high", NewNetworkLatencyExperiment(1000, config.DefaultDuration))
	engine.RegisterExperiment("memory-pressure-low", NewMemoryPressureExperiment(50, config.DefaultDuration))
	engine.RegisterExperiment("memory-pressure-medium", NewMemoryPressureExperiment(200, config.DefaultDuration))
	engine.RegisterExperiment("cpu-stress-low", NewCPUStressExperiment(2, config.DefaultDuration))
	engine.RegisterExperiment("cpu-stress-medium", NewCPUStressExperiment(4, config.DefaultDuration))

	return engine
}

// NewExperimentScheduler creates a new experiment scheduler
func NewExperimentScheduler(config *ChaosConfig) *ExperimentScheduler {
	return &ExperimentScheduler{
		config: config,
		queue:  make([]*ScheduledExperiment, 0),
		stopCh: make(chan struct{}),
	}
}

// RegisterExperiment registers a new chaos experiment
func (e *ChaosEngine) RegisterExperiment(name string, experiment ChaosExperiment) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.experiments[name] = experiment
}

// RunExperiment executes a specific chaos experiment
func (e *ChaosEngine) RunExperiment(ctx context.Context, experimentName, target string) (*ExperimentResult, error) {
	if !e.config.Enabled {
		return nil, fmt.Errorf("chaos engineering is disabled")
	}

	e.mutex.RLock()
	experiment, exists := e.experiments[experimentName]
	e.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("experiment %s not found", experimentName)
	}

	// Check if target is in allowed services
	if !e.isTargetAllowed(target) {
		return nil, fmt.Errorf("target %s is not in allowed services", target)
	}

	// Execute experiment
	result, err := experiment.Execute(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("failed to execute experiment: %w", err)
	}

	// Update metrics
	e.updateMetrics(result)

	// Auto-rollback if safe mode is enabled and experiment failed
	if e.config.SafeMode && !result.Success && experiment.IsReversible() {
		if rollbackErr := experiment.Rollback(ctx, target); rollbackErr != nil {
			result.Observations = append(result.Observations, 
				fmt.Sprintf("Rollback failed: %v", rollbackErr))
		} else {
			result.Observations = append(result.Observations, "Auto-rollback successful")
		}
	}

	return result, nil
}

// ScheduleExperiment schedules an experiment for later execution
func (e *ChaosEngine) ScheduleExperiment(experimentName, target string, scheduledAt time.Time, recurring bool, interval time.Duration) error {
	if !e.config.Enabled {
		return fmt.Errorf("chaos engineering is disabled")
	}

	e.mutex.RLock()
	experiment, exists := e.experiments[experimentName]
	e.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("experiment %s not found", experimentName)
	}

	scheduled := &ScheduledExperiment{
		Experiment:  experiment,
		Target:      target,
		ScheduledAt: scheduledAt,
		Priority:    int(experiment.GetSeverity()),
		Recurring:   recurring,
		Interval:    interval,
	}

	return e.scheduler.AddExperiment(scheduled)
}

// Start starts the chaos engine
func (e *ChaosEngine) Start(ctx context.Context) error {
	if !e.config.Enabled {
		return fmt.Errorf("chaos engineering is disabled")
	}

	e.mutex.Lock()
	if e.running {
		e.mutex.Unlock()
		return fmt.Errorf("chaos engine is already running")
	}
	e.running = true
	e.mutex.Unlock()

	return e.scheduler.Start(ctx, e)
}

// Stop stops the chaos engine
func (e *ChaosEngine) Stop() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if !e.running {
		return fmt.Errorf("chaos engine is not running")
	}

	e.scheduler.Stop()
	e.running = false
	return nil
}

// GetMetrics returns current chaos engineering metrics
func (e *ChaosEngine) GetMetrics() *ChaosMetrics {
	e.metrics.mutex.RLock()
	defer e.metrics.mutex.RUnlock()

	// Create a copy to avoid race conditions
	metrics := &ChaosMetrics{
		TotalExperiments:   e.metrics.TotalExperiments,
		SuccessfulTests:    e.metrics.SuccessfulTests,
		FailedTests:        e.metrics.FailedTests,
		SystemResilience:   e.metrics.SystemResilience,
		MeanTimeToRecovery: e.metrics.MeanTimeToRecovery,
		LastExperimentTime: e.metrics.LastExperimentTime,
		ExperimentsByType:  make(map[string]int64),
		ImpactDistribution: make(map[string]int64),
	}

	for k, v := range e.metrics.ExperimentsByType {
		metrics.ExperimentsByType[k] = v
	}
	for k, v := range e.metrics.ImpactDistribution {
		metrics.ImpactDistribution[k] = v
	}

	return metrics
}

// ListExperiments returns all registered experiments
func (e *ChaosEngine) ListExperiments() map[string]string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	experiments := make(map[string]string)
	for name, experiment := range e.experiments {
		experiments[name] = experiment.Description()
	}

	return experiments
}

// isTargetAllowed checks if a target is in the allowed services
func (e *ChaosEngine) isTargetAllowed(target string) bool {
	if len(e.config.TargetServices) == 0 {
		return true // No restrictions
	}

	for _, allowed := range e.config.TargetServices {
		if target == allowed {
			return true
		}
	}

	return false
}

// updateMetrics updates chaos engineering metrics
func (e *ChaosEngine) updateMetrics(result *ExperimentResult) {
	e.metrics.mutex.Lock()
	defer e.metrics.mutex.Unlock()

	e.metrics.TotalExperiments++
	e.metrics.LastExperimentTime = result.EndTime

	if result.Success {
		e.metrics.SuccessfulTests++
	} else {
		e.metrics.FailedTests++
	}

	// Update experiment type counter
	e.metrics.ExperimentsByType[result.ExperimentName]++

	// Update impact distribution
	e.metrics.ImpactDistribution[string(result.Impact)]++

	// Calculate system resilience (% of successful experiments)
	if e.metrics.TotalExperiments > 0 {
		e.metrics.SystemResilience = (float64(e.metrics.SuccessfulTests) / float64(e.metrics.TotalExperiments)) * 100
	}

	// Update MTTR (simplified calculation)
	if result.Success {
		e.metrics.MeanTimeToRecovery = (e.metrics.MeanTimeToRecovery + result.Duration) / 2
	}
}

// AddExperiment adds an experiment to the scheduler queue
func (s *ExperimentScheduler) AddExperiment(experiment *ScheduledExperiment) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.queue = append(s.queue, experiment)
	return nil
}

// Start starts the experiment scheduler
func (s *ExperimentScheduler) Start(ctx context.Context, engine *ChaosEngine) error {
	s.ticker = time.NewTicker(time.Minute) // Check every minute

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-s.ticker.C:
				s.processQueue(ctx, engine)
			}
		}
	}()

	return nil
}

// Stop stops the experiment scheduler
func (s *ExperimentScheduler) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.stopCh)
}

// processQueue processes scheduled experiments
func (s *ExperimentScheduler) processQueue(ctx context.Context, engine *ChaosEngine) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	var remainingQueue []*ScheduledExperiment

	for _, scheduled := range s.queue {
		if now.After(scheduled.ScheduledAt) {
			// Execute experiment
			go func(exp *ScheduledExperiment) {
				_, err := engine.RunExperiment(ctx, exp.Experiment.Name(), exp.Target)
				if err != nil {
					fmt.Printf("Scheduled experiment failed: %v\n", err)
				}

				// Reschedule if recurring
				if exp.Recurring && exp.Interval > 0 {
					exp.ScheduledAt = now.Add(exp.Interval)
					s.mutex.Lock()
					s.queue = append(s.queue, exp)
					s.mutex.Unlock()
				}
			}(scheduled)
		} else {
			remainingQueue = append(remainingQueue, scheduled)
		}
	}

	s.queue = remainingQueue
}

// RunRandomExperiment selects and runs a random experiment
func (e *ChaosEngine) RunRandomExperiment(ctx context.Context) (*ExperimentResult, error) {
	if !e.config.Enabled {
		return nil, fmt.Errorf("chaos engineering is disabled")
	}

	// Get available experiments
	e.mutex.RLock()
	var experiments []string
	for name := range e.experiments {
		experiments = append(experiments, name)
	}
	e.mutex.RUnlock()

	if len(experiments) == 0 {
		return nil, fmt.Errorf("no experiments available")
	}

	// Select random experiment
	experimentName := experiments[rand.Intn(len(experiments))]

	// Select random target
	if len(e.config.TargetServices) == 0 {
		return nil, fmt.Errorf("no target services configured")
	}
	target := e.config.TargetServices[rand.Intn(len(e.config.TargetServices))]

	return e.RunExperiment(ctx, experimentName, target)
}