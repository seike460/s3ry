package regression

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RegressionDetector detects performance regressions automatically
type RegressionDetector struct {
	config    *DetectorConfig
	storage   MetricsStorage
	analyzer  *StatisticalAnalyzer
	alerter   AlertManager
}

// DetectorConfig holds configuration for regression detection
type DetectorConfig struct {
	MetricsDir          string        `json:"metrics_dir"`
	ThresholdPercent    float64       `json:"threshold_percent"`     // % degradation to trigger alert
	SampleSize          int           `json:"sample_size"`           // Number of historical samples
	WindowDuration      time.Duration `json:"window_duration"`       // Time window for comparison
	MinDataPoints       int           `json:"min_data_points"`       // Minimum data points required
	SensitivityLevel    string        `json:"sensitivity_level"`     // low, medium, high
	EnableBaseline      bool          `json:"enable_baseline"`       // Enable baseline comparison
	BaselineWindow      time.Duration `json:"baseline_window"`       // Baseline comparison window
	AlertCooldown       time.Duration `json:"alert_cooldown"`        // Time between duplicate alerts
	MetricsRetention    time.Duration `json:"metrics_retention"`     // How long to keep metrics
}

// DefaultDetectorConfig returns default configuration
func DefaultDetectorConfig() *DetectorConfig {
	return &DetectorConfig{
		MetricsDir:          "metrics/performance",
		ThresholdPercent:    15.0, // 15% degradation triggers alert
		SampleSize:          50,
		WindowDuration:      time.Hour * 24,     // 24 hours
		MinDataPoints:       10,
		SensitivityLevel:    "medium",
		EnableBaseline:      true,
		BaselineWindow:      time.Hour * 24 * 7, // 7 days
		AlertCooldown:       time.Hour * 2,      // 2 hours
		MetricsRetention:    time.Hour * 24 * 30, // 30 days
	}
}

// PerformanceMetric represents a performance measurement
type PerformanceMetric struct {
	ID          string            `json:"id"`
	Timestamp   time.Time         `json:"timestamp"`
	MetricName  string            `json:"metric_name"`
	Value       float64           `json:"value"`
	Unit        string            `json:"unit"`
	Labels      map[string]string `json:"labels"`
	Environment string            `json:"environment"`
	Version     string            `json:"version"`
	TestCase    string            `json:"test_case"`
}

// RegressionAlert represents a detected performance regression
type RegressionAlert struct {
	ID               string                 `json:"id"`
	Timestamp        time.Time              `json:"timestamp"`
	MetricName       string                 `json:"metric_name"`
	CurrentValue     float64                `json:"current_value"`
	BaselineValue    float64                `json:"baseline_value"`
	DegradationPct   float64                `json:"degradation_percent"`
	Severity         AlertSeverity          `json:"severity"`
	Details          string                 `json:"details"`
	AffectedTests    []string               `json:"affected_tests"`
	StatisticalData  *StatisticalAnalysis   `json:"statistical_data"`
	Recommendations  []string               `json:"recommendations"`
}

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	SeverityLow      AlertSeverity = "LOW"
	SeverityMedium   AlertSeverity = "MEDIUM"
	SeverityHigh     AlertSeverity = "HIGH"
	SeverityCritical AlertSeverity = "CRITICAL"
)

// StatisticalAnalysis holds statistical analysis results
type StatisticalAnalysis struct {
	Mean            float64 `json:"mean"`
	Median          float64 `json:"median"`
	StandardDev     float64 `json:"standard_deviation"`
	ConfidenceLevel float64 `json:"confidence_level"`
	PValue          float64 `json:"p_value"`
	TrendDirection  string  `json:"trend_direction"` // improving, degrading, stable
	OutlierCount    int     `json:"outlier_count"`
}

// MetricsStorage interface for storing and retrieving metrics
type MetricsStorage interface {
	Store(metric *PerformanceMetric) error
	GetMetrics(metricName string, since time.Time) ([]*PerformanceMetric, error)
	GetBaseline(metricName string, window time.Duration) (*PerformanceMetric, error)
	Cleanup(retentionPeriod time.Duration) error
}

// AlertManager interface for sending alerts
type AlertManager interface {
	SendAlert(alert *RegressionAlert) error
	ShouldAlert(metricName string) bool
	RecordAlert(metricName string, timestamp time.Time) error
}

// FileMetricsStorage implements MetricsStorage using local files
type FileMetricsStorage struct {
	baseDir string
}

// NewFileMetricsStorage creates a file-based metrics storage
func NewFileMetricsStorage(baseDir string) *FileMetricsStorage {
	os.MkdirAll(baseDir, 0755)
	return &FileMetricsStorage{baseDir: baseDir}
}

// Store saves a metric to file
func (f *FileMetricsStorage) Store(metric *PerformanceMetric) error {
	filename := filepath.Join(f.baseDir, fmt.Sprintf("%s.json", metric.MetricName))
	
	var metrics []*PerformanceMetric
	
	// Load existing metrics
	if data, err := os.ReadFile(filename); err == nil {
		json.Unmarshal(data, &metrics)
	}
	
	// Add new metric
	metrics = append(metrics, metric)
	
	// Sort by timestamp
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Timestamp.Before(metrics[j].Timestamp)
	})
	
	// Save back to file
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}
	
	return os.WriteFile(filename, data, 0644)
}

// GetMetrics retrieves metrics since a specific time
func (f *FileMetricsStorage) GetMetrics(metricName string, since time.Time) ([]*PerformanceMetric, error) {
	filename := filepath.Join(f.baseDir, fmt.Sprintf("%s.json", metricName))
	
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []*PerformanceMetric{}, nil
		}
		return nil, fmt.Errorf("failed to read metrics file: %w", err)
	}
	
	var allMetrics []*PerformanceMetric
	if err := json.Unmarshal(data, &allMetrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}
	
	var filteredMetrics []*PerformanceMetric
	for _, metric := range allMetrics {
		if metric.Timestamp.After(since) {
			filteredMetrics = append(filteredMetrics, metric)
		}
	}
	
	return filteredMetrics, nil
}

// GetBaseline calculates baseline metric from historical data
func (f *FileMetricsStorage) GetBaseline(metricName string, window time.Duration) (*PerformanceMetric, error) {
	since := time.Now().Add(-window)
	metrics, err := f.GetMetrics(metricName, since)
	if err != nil {
		return nil, err
	}
	
	if len(metrics) == 0 {
		return nil, fmt.Errorf("no metrics found for baseline calculation")
	}
	
	// Calculate median as baseline
	values := make([]float64, len(metrics))
	for i, metric := range metrics {
		values[i] = metric.Value
	}
	
	sort.Float64s(values)
	median := values[len(values)/2]
	
	return &PerformanceMetric{
		MetricName: metricName,
		Value:      median,
		Timestamp:  time.Now(),
		Unit:       metrics[0].Unit,
	}, nil
}

// Cleanup removes old metrics beyond retention period
func (f *FileMetricsStorage) Cleanup(retentionPeriod time.Duration) error {
	cutoff := time.Now().Add(-retentionPeriod)
	
	return filepath.Walk(f.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".json") {
			return err
		}
		
		// Load metrics file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		
		var metrics []*PerformanceMetric
		if err := json.Unmarshal(data, &metrics); err != nil {
			return err
		}
		
		// Filter out old metrics
		var filteredMetrics []*PerformanceMetric
		for _, metric := range metrics {
			if metric.Timestamp.After(cutoff) {
				filteredMetrics = append(filteredMetrics, metric)
			}
		}
		
		// Save filtered metrics back
		if len(filteredMetrics) != len(metrics) {
			newData, err := json.MarshalIndent(filteredMetrics, "", "  ")
			if err != nil {
				return err
			}
			return os.WriteFile(path, newData, 0644)
		}
		
		return nil
	})
}

// StatisticalAnalyzer performs statistical analysis on performance data
type StatisticalAnalyzer struct{}

// NewStatisticalAnalyzer creates a new statistical analyzer
func NewStatisticalAnalyzer() *StatisticalAnalyzer {
	return &StatisticalAnalyzer{}
}

// AnalyzeMetrics performs statistical analysis on a set of metrics
func (s *StatisticalAnalyzer) AnalyzeMetrics(metrics []*PerformanceMetric) *StatisticalAnalysis {
	if len(metrics) == 0 {
		return &StatisticalAnalysis{}
	}
	
	values := make([]float64, len(metrics))
	for i, metric := range metrics {
		values[i] = metric.Value
	}
	
	sort.Float64s(values)
	
	analysis := &StatisticalAnalysis{
		Mean:            calculateMean(values),
		Median:          calculateMedian(values),
		StandardDev:     calculateStandardDeviation(values),
		ConfidenceLevel: 95.0, // Default 95% confidence
		OutlierCount:    countOutliers(values),
		TrendDirection:  calculateTrend(metrics),
	}
	
	// Calculate p-value (simplified)
	analysis.PValue = s.calculatePValue(values)
	
	return analysis
}

// calculateMean calculates the arithmetic mean
func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

// calculateMedian calculates the median value
func calculateMedian(sortedValues []float64) float64 {
	n := len(sortedValues)
	if n == 0 {
		return 0
	}
	
	if n%2 == 0 {
		return (sortedValues[n/2-1] + sortedValues[n/2]) / 2
	}
	return sortedValues[n/2]
}

// calculateStandardDeviation calculates the standard deviation
func calculateStandardDeviation(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	
	mean := calculateMean(values)
	sumSquares := 0.0
	
	for _, value := range values {
		diff := value - mean
		sumSquares += diff * diff
	}
	
	variance := sumSquares / float64(len(values)-1)
	return math.Sqrt(variance)
}

// countOutliers counts values that are outside 2 standard deviations
func countOutliers(values []float64) int {
	if len(values) < 3 {
		return 0
	}
	
	mean := calculateMean(values)
	stdDev := calculateStandardDeviation(values)
	threshold := 2 * stdDev
	
	count := 0
	for _, value := range values {
		if math.Abs(value-mean) > threshold {
			count++
		}
	}
	
	return count
}

// calculateTrend determines if metrics are improving, degrading, or stable
func calculateTrend(metrics []*PerformanceMetric) string {
	if len(metrics) < 3 {
		return "insufficient_data"
	}
	
	// Simple linear regression slope calculation
	n := float64(len(metrics))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	
	for i, metric := range metrics {
		x := float64(i)
		y := metric.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	
	if slope > 0.1 {
		return "degrading"
	} else if slope < -0.1 {
		return "improving"
	}
	return "stable"
}

// calculatePValue calculates a simplified p-value
func (s *StatisticalAnalyzer) calculatePValue(values []float64) float64 {
	// This is a simplified p-value calculation
	// In a real implementation, you'd use proper statistical tests
	if len(values) < 10 {
		return 1.0 // Not enough data for significance
	}
	
	// Simulate a t-test like calculation
	mean := calculateMean(values)
	stdDev := calculateStandardDeviation(values)
	
	if stdDev == 0 {
		return 0.0 // Perfect consistency
	}
	
	// Simplified calculation based on coefficient of variation
	cv := stdDev / mean
	if cv < 0.1 {
		return 0.01 // Very significant
	} else if cv < 0.2 {
		return 0.05 // Significant
	} else if cv < 0.3 {
		return 0.1 // Marginally significant
	}
	
	return 0.5 // Not significant
}

// SimpleAlertManager implements AlertManager interface
type SimpleAlertManager struct {
	cooldowns map[string]time.Time
	config    *DetectorConfig
}

// NewSimpleAlertManager creates a simple alert manager
func NewSimpleAlertManager(config *DetectorConfig) *SimpleAlertManager {
	return &SimpleAlertManager{
		cooldowns: make(map[string]time.Time),
		config:    config,
	}
}

// SendAlert sends an alert (implementation depends on notification system)
func (s *SimpleAlertManager) SendAlert(alert *RegressionAlert) error {
	// In a real implementation, this would send to Slack, email, etc.
	fmt.Printf("🚨 PERFORMANCE REGRESSION ALERT 🚨\n")
	fmt.Printf("Metric: %s\n", alert.MetricName)
	fmt.Printf("Degradation: %.2f%%\n", alert.DegradationPct)
	fmt.Printf("Current: %.2f, Baseline: %.2f\n", alert.CurrentValue, alert.BaselineValue)
	fmt.Printf("Severity: %s\n", alert.Severity)
	fmt.Printf("Details: %s\n", alert.Details)
	
	return s.RecordAlert(alert.MetricName, alert.Timestamp)
}

// ShouldAlert checks if an alert should be sent (considering cooldown)
func (s *SimpleAlertManager) ShouldAlert(metricName string) bool {
	lastAlert, exists := s.cooldowns[metricName]
	if !exists {
		return true
	}
	
	return time.Since(lastAlert) > s.config.AlertCooldown
}

// RecordAlert records that an alert was sent
func (s *SimpleAlertManager) RecordAlert(metricName string, timestamp time.Time) error {
	s.cooldowns[metricName] = timestamp
	return nil
}

// NewRegressionDetector creates a new regression detector
func NewRegressionDetector(config *DetectorConfig) (*RegressionDetector, error) {
	if config == nil {
		config = DefaultDetectorConfig()
	}
	
	storage := NewFileMetricsStorage(config.MetricsDir)
	analyzer := NewStatisticalAnalyzer()
	alerter := NewSimpleAlertManager(config)
	
	return &RegressionDetector{
		config:   config,
		storage:  storage,
		analyzer: analyzer,
		alerter:  alerter,
	}, nil
}

// RecordMetric records a new performance metric
func (r *RegressionDetector) RecordMetric(metric *PerformanceMetric) error {
	// Set ID and timestamp if not provided
	if metric.ID == "" {
		metric.ID = fmt.Sprintf("%s_%d", metric.MetricName, time.Now().UnixNano())
	}
	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now()
	}
	
	// Store the metric
	if err := r.storage.Store(metric); err != nil {
		return fmt.Errorf("failed to store metric: %w", err)
	}
	
	// Check for regression
	return r.checkForRegression(metric)
}

// checkForRegression analyzes if a metric indicates a performance regression
func (r *RegressionDetector) checkForRegression(newMetric *PerformanceMetric) error {
	// Get historical data
	since := time.Now().Add(-r.config.WindowDuration)
	historical, err := r.storage.GetMetrics(newMetric.MetricName, since)
	if err != nil {
		return fmt.Errorf("failed to get historical metrics: %w", err)
	}
	
	// Need minimum data points for analysis
	if len(historical) < r.config.MinDataPoints {
		return nil // Not enough data for regression detection
	}
	
	// Get baseline
	var baseline *PerformanceMetric
	if r.config.EnableBaseline {
		baseline, err = r.storage.GetBaseline(newMetric.MetricName, r.config.BaselineWindow)
		if err != nil {
			return fmt.Errorf("failed to get baseline: %w", err)
		}
	} else {
		// Use recent average as baseline
		recentMetrics := historical
		if len(historical) > r.config.SampleSize {
			recentMetrics = historical[len(historical)-r.config.SampleSize:]
		}
		
		values := make([]float64, len(recentMetrics))
		for i, metric := range recentMetrics {
			values[i] = metric.Value
		}
		
		baseline = &PerformanceMetric{
			MetricName: newMetric.MetricName,
			Value:      calculateMean(values),
			Timestamp:  time.Now(),
			Unit:       newMetric.Unit,
		}
	}
	
	// Calculate degradation percentage
	degradationPct := ((newMetric.Value - baseline.Value) / baseline.Value) * 100
	
	// Check if degradation exceeds threshold
	if degradationPct > r.config.ThresholdPercent {
		// Perform statistical analysis
		analysis := r.analyzer.AnalyzeMetrics(historical)
		
		// Determine severity
		severity := r.calculateSeverity(degradationPct)
		
		// Create alert
		alert := &RegressionAlert{
			ID:               fmt.Sprintf("reg_%s_%d", newMetric.MetricName, time.Now().UnixNano()),
			Timestamp:        time.Now(),
			MetricName:       newMetric.MetricName,
			CurrentValue:     newMetric.Value,
			BaselineValue:    baseline.Value,
			DegradationPct:   degradationPct,
			Severity:         severity,
			Details:          fmt.Sprintf("Performance degraded by %.2f%% from baseline", degradationPct),
			StatisticalData:  analysis,
			Recommendations:  r.generateRecommendations(newMetric, analysis),
		}
		
		// Send alert if cooldown allows
		if r.alerter.ShouldAlert(newMetric.MetricName) {
			return r.alerter.SendAlert(alert)
		}
	}
	
	return nil
}

// calculateSeverity determines alert severity based on degradation percentage
func (r *RegressionDetector) calculateSeverity(degradationPct float64) AlertSeverity {
	switch r.config.SensitivityLevel {
	case "high":
		if degradationPct > 50 {
			return SeverityCritical
		} else if degradationPct > 25 {
			return SeverityHigh
		} else if degradationPct > 15 {
			return SeverityMedium
		}
		return SeverityLow
	case "low":
		if degradationPct > 100 {
			return SeverityCritical
		} else if degradationPct > 75 {
			return SeverityHigh
		} else if degradationPct > 50 {
			return SeverityMedium
		}
		return SeverityLow
	default: // medium
		if degradationPct > 75 {
			return SeverityCritical
		} else if degradationPct > 40 {
			return SeverityHigh
		} else if degradationPct > 20 {
			return SeverityMedium
		}
		return SeverityLow
	}
}

// generateRecommendations generates actionable recommendations based on analysis
func (r *RegressionDetector) generateRecommendations(metric *PerformanceMetric, analysis *StatisticalAnalysis) []string {
	var recommendations []string
	
	// Generic recommendations based on trend
	switch analysis.TrendDirection {
	case "degrading":
		recommendations = append(recommendations, "Performance has been consistently degrading. Consider investigating recent changes.")
	case "improving":
		recommendations = append(recommendations, "Performance was improving but has now regressed. Check for recent deployments or configuration changes.")
	}
	
	// Recommendations based on outliers
	if analysis.OutlierCount > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Detected %d outliers in recent data. Check for intermittent issues.", analysis.OutlierCount))
	}
	
	// Metric-specific recommendations
	switch metric.MetricName {
	case "response_time", "latency":
		recommendations = append(recommendations, "For latency issues: check database queries, API calls, and network connectivity.")
	case "throughput", "requests_per_second":
		recommendations = append(recommendations, "For throughput issues: check resource utilization, connection pools, and load balancer configuration.")
	case "memory_usage":
		recommendations = append(recommendations, "For memory issues: check for memory leaks, cache size, and garbage collection.")
	case "cpu_usage":
		recommendations = append(recommendations, "For CPU issues: check algorithm efficiency, concurrent operations, and system load.")
	}
	
	// Statistical significance recommendations
	if analysis.PValue > 0.1 {
		recommendations = append(recommendations, "Low statistical significance. Consider collecting more data before taking action.")
	}
	
	return recommendations
}

// Cleanup removes old metrics and alerts
func (r *RegressionDetector) Cleanup(ctx context.Context) error {
	return r.storage.Cleanup(r.config.MetricsRetention)
}

// GetMetricSummary returns summary statistics for a metric
func (r *RegressionDetector) GetMetricSummary(metricName string, window time.Duration) (*MetricSummary, error) {
	since := time.Now().Add(-window)
	metrics, err := r.storage.GetMetrics(metricName, since)
	if err != nil {
		return nil, err
	}
	
	if len(metrics) == 0 {
		return &MetricSummary{
			MetricName: metricName,
			DataPoints: 0,
		}, nil
	}
	
	analysis := r.analyzer.AnalyzeMetrics(metrics)
	
	values := make([]float64, len(metrics))
	for i, metric := range metrics {
		values[i] = metric.Value
	}
	sort.Float64s(values)
	
	return &MetricSummary{
		MetricName:      metricName,
		DataPoints:      len(metrics),
		Mean:            analysis.Mean,
		Median:          analysis.Median,
		StandardDev:     analysis.StandardDev,
		Min:             values[0],
		Max:             values[len(values)-1],
		TrendDirection:  analysis.TrendDirection,
		OutlierCount:    analysis.OutlierCount,
		LastValue:       metrics[len(metrics)-1].Value,
		LastTimestamp:   metrics[len(metrics)-1].Timestamp,
	}, nil
}

// MetricSummary holds summary statistics for a metric
type MetricSummary struct {
	MetricName      string    `json:"metric_name"`
	DataPoints      int       `json:"data_points"`
	Mean            float64   `json:"mean"`
	Median          float64   `json:"median"`
	StandardDev     float64   `json:"standard_deviation"`
	Min             float64   `json:"min"`
	Max             float64   `json:"max"`
	TrendDirection  string    `json:"trend_direction"`
	OutlierCount    int       `json:"outlier_count"`
	LastValue       float64   `json:"last_value"`
	LastTimestamp   time.Time `json:"last_timestamp"`
}