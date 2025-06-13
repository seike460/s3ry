package metrics

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Monitor provides continuous monitoring and reporting
type Monitor struct {
	metrics  *Metrics
	interval time.Duration
	cancel   context.CancelFunc
	ctx      context.Context
}

// NewMonitor creates a new performance monitor
func NewMonitor(interval time.Duration) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Monitor{
		metrics:  GetGlobalMetrics(),
		interval: interval,
		cancel:   cancel,
		ctx:      ctx,
	}
}

// Start begins continuous monitoring
func (m *Monitor) Start() {
	go m.monitorLoop()
}

// Stop stops the monitoring
func (m *Monitor) Stop() {
	m.cancel()
}

// monitorLoop runs the monitoring loop
func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateAndReport()
		case <-m.ctx.Done():
			return
		}
	}
}

// updateAndReport updates metrics and reports if needed
func (m *Monitor) updateAndReport() {
	m.metrics.UpdateMemoryMetrics()
	
	// Check for performance issues
	if m.shouldReport() {
		snapshot := m.metrics.GetSnapshot()
		m.reportPerformance(snapshot)
	}
}

// shouldReport determines if we should report current metrics
func (m *Monitor) shouldReport() bool {
	snapshot := m.metrics.GetSnapshot()
	
	// Report if failure rate is high
	if snapshot.FailureRate > 10.0 {
		return true
	}
	
	// Report if memory usage is high
	if snapshot.MemoryUsage.AllocatedBytes > 100*1024*1024 { // 100MB
		return true
	}
	
	// Report every 5 minutes regardless
	if snapshot.Uptime.Minutes() > 0 && int(snapshot.Uptime.Minutes())%5 == 0 {
		return true
	}
	
	return false
}

// reportPerformance reports performance metrics
func (m *Monitor) reportPerformance(snapshot MetricsSnapshot) {
	log.Printf("Performance Report: Ops/sec=%.2f, Failure=%.2f%%, Memory=%d MB", 
		snapshot.OperationsPerSec, 
		snapshot.FailureRate, 
		snapshot.MemoryUsage.AllocatedBytes/(1024*1024))
}

// PerformanceAlert represents a performance alert
type PerformanceAlert struct {
	Type        string
	Severity    string
	Message     string
	Timestamp   time.Time
	MetricValue interface{}
}

// AlertThresholds defines thresholds for alerts
type AlertThresholds struct {
	MaxFailureRate    float64 // percentage
	MaxMemoryUsage    uint64  // bytes
	MinOperationsRate float64 // ops per second
	MaxResponseTime   time.Duration
}

// DefaultAlertThresholds returns sensible default thresholds
func DefaultAlertThresholds() AlertThresholds {
	return AlertThresholds{
		MaxFailureRate:    15.0,                // 15% failure rate
		MaxMemoryUsage:    200 * 1024 * 1024,   // 200MB
		MinOperationsRate: 0.1,                 // 0.1 ops/sec minimum
		MaxResponseTime:   10 * time.Second,    // 10 second max response
	}
}

// AlertManager manages performance alerts
type AlertManager struct {
	thresholds AlertThresholds
	alerts     []PerformanceAlert
	callbacks  []func(PerformanceAlert)
}

// NewAlertManager creates a new alert manager
func NewAlertManager(thresholds AlertThresholds) *AlertManager {
	return &AlertManager{
		thresholds: thresholds,
		alerts:     make([]PerformanceAlert, 0),
		callbacks:  make([]func(PerformanceAlert), 0),
	}
}

// AddCallback adds an alert callback function
func (am *AlertManager) AddCallback(callback func(PerformanceAlert)) {
	am.callbacks = append(am.callbacks, callback)
}

// CheckAlerts checks current metrics against thresholds
func (am *AlertManager) CheckAlerts(snapshot MetricsSnapshot) {
	// Check failure rate
	if snapshot.FailureRate > am.thresholds.MaxFailureRate {
		alert := PerformanceAlert{
			Type:        "failure_rate",
			Severity:    "warning",
			Message:     fmt.Sprintf("High failure rate: %.2f%%", snapshot.FailureRate),
			Timestamp:   time.Now(),
			MetricValue: snapshot.FailureRate,
		}
		am.triggerAlert(alert)
	}

	// Check memory usage
	if snapshot.MemoryUsage.AllocatedBytes > am.thresholds.MaxMemoryUsage {
		alert := PerformanceAlert{
			Type:        "memory_usage",
			Severity:    "warning",
			Message:     fmt.Sprintf("High memory usage: %d MB", snapshot.MemoryUsage.AllocatedBytes/(1024*1024)),
			Timestamp:   time.Now(),
			MetricValue: snapshot.MemoryUsage.AllocatedBytes,
		}
		am.triggerAlert(alert)
	}

	// Check operations rate
	if snapshot.OperationsPerSec < am.thresholds.MinOperationsRate && snapshot.TotalOperations > 0 {
		alert := PerformanceAlert{
			Type:        "low_throughput",
			Severity:    "info",
			Message:     fmt.Sprintf("Low operation rate: %.2f ops/sec", snapshot.OperationsPerSec),
			Timestamp:   time.Now(),
			MetricValue: snapshot.OperationsPerSec,
		}
		am.triggerAlert(alert)
	}
}

// triggerAlert triggers an alert and calls callbacks
func (am *AlertManager) triggerAlert(alert PerformanceAlert) {
	am.alerts = append(am.alerts, alert)
	
	// Call all registered callbacks
	for _, callback := range am.callbacks {
		go callback(alert)
	}
}

// GetRecentAlerts returns recent alerts
func (am *AlertManager) GetRecentAlerts(since time.Duration) []PerformanceAlert {
	cutoff := time.Now().Add(-since)
	var recent []PerformanceAlert
	
	for _, alert := range am.alerts {
		if alert.Timestamp.After(cutoff) {
			recent = append(recent, alert)
		}
	}
	
	return recent
}

// ClearAlerts clears all stored alerts
func (am *AlertManager) ClearAlerts() {
	am.alerts = make([]PerformanceAlert, 0)
}

// BottleneckDetector detects performance bottlenecks
type BottleneckDetector struct {
	metrics *Metrics
}

// NewBottleneckDetector creates a new bottleneck detector
func NewBottleneckDetector() *BottleneckDetector {
	return &BottleneckDetector{
		metrics: GetGlobalMetrics(),
	}
}

// DetectBottlenecks analyzes metrics to identify bottlenecks
func (bd *BottleneckDetector) DetectBottlenecks() []string {
	snapshot := bd.metrics.GetSnapshot()
	var bottlenecks []string

	// Check for slow operations
	for operation, duration := range snapshot.PerformanceTimers {
		if duration > 5*time.Second {
			bottlenecks = append(bottlenecks, 
				fmt.Sprintf("Slow %s operation: %v", operation, duration))
		}
	}

	// Check for high failure rate
	if snapshot.FailureRate > 10.0 {
		bottlenecks = append(bottlenecks, 
			fmt.Sprintf("High failure rate: %.2f%%", snapshot.FailureRate))
	}

	// Check for memory pressure
	if snapshot.MemoryUsage.AllocatedBytes > 150*1024*1024 { // 150MB
		bottlenecks = append(bottlenecks, 
			fmt.Sprintf("High memory usage: %d MB", 
				snapshot.MemoryUsage.AllocatedBytes/(1024*1024)))
	}

	// Check for GC pressure
	if snapshot.MemoryUsage.GCRuns > 100 && snapshot.Uptime < 10*time.Minute {
		bottlenecks = append(bottlenecks, 
			fmt.Sprintf("Frequent GC: %d runs in %v", 
				snapshot.MemoryUsage.GCRuns, snapshot.Uptime))
	}

	return bottlenecks
}