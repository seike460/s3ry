package ai

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// PatternAnalyzer provides usage pattern learning and optimization
type PatternAnalyzer struct {
	config        *PatternConfig
	patterns      map[string]*UsagePattern
	predictions   map[string]*PredictionModel
	anomalies     []AnomalyEvent
	optimizations []OptimizationSuggestion
	mu            sync.RWMutex
	logger        Logger
}

// PatternConfig configures the pattern analyzer
type PatternConfig struct {
	LearningWindow      time.Duration `json:"learning_window"`
	PredictionHorizon   time.Duration `json:"prediction_horizon"`
	AnomalyThreshold    float64       `json:"anomaly_threshold"`
	MinDataPoints       int           `json:"min_data_points"`
	SeasonalityDetection bool         `json:"seasonality_detection"`
	TrendDetection      bool          `json:"trend_detection"`
	EnableMLPrediction  bool          `json:"enable_ml_prediction"`
	UpdateInterval      time.Duration `json:"update_interval"`
}

// DefaultPatternConfig returns default pattern analyzer configuration
func DefaultPatternConfig() *PatternConfig {
	return &PatternConfig{
		LearningWindow:       30 * 24 * time.Hour, // 30 days
		PredictionHorizon:    7 * 24 * time.Hour,  // 7 days
		AnomalyThreshold:     2.0,                 // 2 standard deviations
		MinDataPoints:        10,
		SeasonalityDetection: true,
		TrendDetection:       true,
		EnableMLPrediction:   true,
		UpdateInterval:       6 * time.Hour,
	}
}

// UsagePattern represents learned usage patterns
type UsagePattern struct {
	PatternID       string                 `json:"pattern_id"`
	Resource        string                 `json:"resource"`
	PatternType     PatternType            `json:"pattern_type"`
	Confidence      float64                `json:"confidence"`
	LastUpdated     time.Time              `json:"last_updated"`
	DataPoints      []DataPoint            `json:"data_points"`
	Statistics      *PatternStatistics     `json:"statistics"`
	Seasonality     *SeasonalPattern       `json:"seasonality,omitempty"`
	Trend           *TrendPattern          `json:"trend,omitempty"`
	Correlations    []PatternCorrelation   `json:"correlations,omitempty"`
	PredictedNext   []PredictionPoint      `json:"predicted_next"`
}

// PatternType represents the type of usage pattern
type PatternType int

const (
	AccessPattern PatternType = iota
	StoragePattern
	TransferPattern
	CostPattern
	RequestPattern
	LifecyclePattern
)

func (pt PatternType) String() string {
	switch pt {
	case AccessPattern:
		return "access"
	case StoragePattern:
		return "storage"
	case TransferPattern:
		return "transfer"
	case CostPattern:
		return "cost"
	case RequestPattern:
		return "request"
	case LifecyclePattern:
		return "lifecycle"
	default:
		return "unknown"
	}
}

// DataPoint represents a single data point in the pattern
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// PatternStatistics contains statistical analysis of the pattern
type PatternStatistics struct {
	Mean              float64   `json:"mean"`
	Median            float64   `json:"median"`
	StandardDeviation float64   `json:"standard_deviation"`
	Min               float64   `json:"min"`
	Max               float64   `json:"max"`
	Range             float64   `json:"range"`
	Variance          float64   `json:"variance"`
	Skewness          float64   `json:"skewness"`
	Kurtosis          float64   `json:"kurtosis"`
	Percentiles       []float64 `json:"percentiles"` // 25th, 50th, 75th, 90th, 95th, 99th
}

// SeasonalPattern represents seasonal patterns in the data
type SeasonalPattern struct {
	Detected        bool              `json:"detected"`
	Period          time.Duration     `json:"period"`
	Amplitude       float64           `json:"amplitude"`
	Phase           float64           `json:"phase"`
	Strength        float64           `json:"strength"`
	HourlyPattern   [24]float64       `json:"hourly_pattern"`
	DailyPattern    [7]float64        `json:"daily_pattern"`
	MonthlyPattern  [12]float64       `json:"monthly_pattern"`
	Confidence      float64           `json:"confidence"`
}

// TrendPattern represents trend patterns in the data
type TrendPattern struct {
	Detected    bool    `json:"detected"`
	Direction   string  `json:"direction"` // "increasing", "decreasing", "stable"
	Slope       float64 `json:"slope"`
	Strength    float64 `json:"strength"`
	R2          float64 `json:"r2"`
	Confidence  float64 `json:"confidence"`
}

// PatternCorrelation represents correlation with other patterns
type PatternCorrelation struct {
	PatternID     string  `json:"pattern_id"`
	Correlation   float64 `json:"correlation"`
	Lag           int     `json:"lag"` // Time lag in hours
	Significance  float64 `json:"significance"`
}

// PredictionPoint represents a predicted future data point
type PredictionPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	PredictedValue  float64   `json:"predicted_value"`
	ConfidenceUpper float64   `json:"confidence_upper"`
	ConfidenceLower float64   `json:"confidence_lower"`
	Confidence      float64   `json:"confidence"`
}

// PredictionModel contains the prediction model for a pattern
type PredictionModel struct {
	ModelType      ModelType             `json:"model_type"`
	Parameters     map[string]float64    `json:"parameters"`
	Accuracy       float64               `json:"accuracy"`
	LastTrained    time.Time             `json:"last_trained"`
	TrainingData   []DataPoint           `json:"training_data"`
	ValidationData []DataPoint           `json:"validation_data"`
	Predictions    []PredictionPoint     `json:"predictions"`
}

// ModelType represents the type of prediction model
type ModelType int

const (
	LinearRegression ModelType = iota
	ExponentialSmoothing
	ARIMA
	SeasonalDecomposition
	NeuralNetwork
	EnsembleModel
)

func (mt ModelType) String() string {
	switch mt {
	case LinearRegression:
		return "linear_regression"
	case ExponentialSmoothing:
		return "exponential_smoothing"
	case ARIMA:
		return "arima"
	case SeasonalDecomposition:
		return "seasonal_decomposition"
	case NeuralNetwork:
		return "neural_network"
	case EnsembleModel:
		return "ensemble"
	default:
		return "unknown"
	}
}

// AnomalyEvent represents an detected anomaly
type AnomalyEvent struct {
	EventID      string                 `json:"event_id"`
	Timestamp    time.Time              `json:"timestamp"`
	Resource     string                 `json:"resource"`
	PatternType  PatternType            `json:"pattern_type"`
	Severity     AnomalySeverity        `json:"severity"`
	Description  string                 `json:"description"`
	ActualValue  float64                `json:"actual_value"`
	ExpectedValue float64               `json:"expected_value"`
	Deviation    float64                `json:"deviation"`
	Confidence   float64                `json:"confidence"`
	Metadata     map[string]interface{} `json:"metadata"`
	Resolved     bool                   `json:"resolved"`
}

// AnomalySeverity represents the severity of an anomaly
type AnomalySeverity int

const (
	LowSeverity AnomalySeverity = iota
	MediumSeverity
	HighSeverity
	CriticalSeverity
)

func (as AnomalySeverity) String() string {
	switch as {
	case LowSeverity:
		return "low"
	case MediumSeverity:
		return "medium"
	case HighSeverity:
		return "high"
	case CriticalSeverity:
		return "critical"
	default:
		return "unknown"
	}
}

// OptimizationSuggestion represents an optimization suggestion based on patterns
type OptimizationSuggestion struct {
	SuggestionID    string                 `json:"suggestion_id"`
	Type            OptimizationType       `json:"type"`
	Priority        Priority               `json:"priority"`
	Resource        string                 `json:"resource"`
	Description     string                 `json:"description"`
	ExpectedBenefit string                 `json:"expected_benefit"`
	Implementation  []string               `json:"implementation"`
	EstimatedSavings float64               `json:"estimated_savings"`
	Confidence      float64                `json:"confidence"`
	BasedOnPatterns []string               `json:"based_on_patterns"`
	ValidUntil      time.Time              `json:"valid_until"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// NewPatternAnalyzer creates a new pattern analyzer
func NewPatternAnalyzer(config *PatternConfig, logger Logger) *PatternAnalyzer {
	if config == nil {
		config = DefaultPatternConfig()
	}

	return &PatternAnalyzer{
		config:        config,
		patterns:      make(map[string]*UsagePattern),
		predictions:   make(map[string]*PredictionModel),
		anomalies:     make([]AnomalyEvent, 0),
		optimizations: make([]OptimizationSuggestion, 0),
		logger:        logger,
	}
}

// AnalyzeUsagePatterns analyzes usage data to identify patterns
func (pa *PatternAnalyzer) AnalyzeUsagePatterns(ctx context.Context, usageData []DataPoint, resource string, patternType PatternType) (*UsagePattern, error) {
	if len(usageData) < pa.config.MinDataPoints {
		return nil, fmt.Errorf("insufficient data points: %d < %d", len(usageData), pa.config.MinDataPoints)
	}

	pattern := &UsagePattern{
		PatternID:   fmt.Sprintf("%s_%s_%d", resource, patternType.String(), time.Now().Unix()),
		Resource:    resource,
		PatternType: patternType,
		LastUpdated: time.Now(),
		DataPoints:  usageData,
	}

	// Calculate basic statistics
	pattern.Statistics = pa.calculateStatistics(usageData)

	// Detect seasonality if enabled
	if pa.config.SeasonalityDetection {
		pattern.Seasonality = pa.detectSeasonality(usageData)
	}

	// Detect trends if enabled
	if pa.config.TrendDetection {
		pattern.Trend = pa.detectTrend(usageData)
	}

	// Calculate confidence based on data quality and pattern strength
	pattern.Confidence = pa.calculatePatternConfidence(pattern)

	// Generate predictions if ML is enabled
	if pa.config.EnableMLPrediction {
		predictions, err := pa.generatePredictions(usageData, pattern)
		if err != nil {
			pa.logger.Warn("Failed to generate predictions for pattern %s: %v", pattern.PatternID, err)
		} else {
			pattern.PredictedNext = predictions
		}
	}

	// Store pattern
	pa.mu.Lock()
	pa.patterns[pattern.PatternID] = pattern
	pa.mu.Unlock()

	pa.logger.Info("Analyzed usage pattern for %s: confidence %.2f", resource, pattern.Confidence)
	return pattern, nil
}

// DetectAnomalies detects anomalies in the usage patterns
func (pa *PatternAnalyzer) DetectAnomalies(ctx context.Context) ([]AnomalyEvent, error) {
	pa.mu.RLock()
	patterns := make(map[string]*UsagePattern)
	for k, v := range pa.patterns {
		patterns[k] = v
	}
	pa.mu.RUnlock()

	var newAnomalies []AnomalyEvent

	for _, pattern := range patterns {
		anomalies := pa.detectPatternAnomalies(pattern)
		newAnomalies = append(newAnomalies, anomalies...)
	}

	// Store new anomalies
	pa.mu.Lock()
	pa.anomalies = append(pa.anomalies, newAnomalies...)
	pa.mu.Unlock()

	pa.logger.Info("Detected %d new anomalies", len(newAnomalies))
	return newAnomalies, nil
}

// GenerateOptimizations generates optimization suggestions based on learned patterns
func (pa *PatternAnalyzer) GenerateOptimizations(ctx context.Context) ([]OptimizationSuggestion, error) {
	pa.mu.RLock()
	patterns := make(map[string]*UsagePattern)
	for k, v := range pa.patterns {
		patterns[k] = v
	}
	pa.mu.RUnlock()

	var suggestions []OptimizationSuggestion

	for _, pattern := range patterns {
		patternSuggestions := pa.generatePatternOptimizations(pattern)
		suggestions = append(suggestions, patternSuggestions...)
	}

	// Find correlation-based optimizations
	correlationSuggestions := pa.generateCorrelationOptimizations(patterns)
	suggestions = append(suggestions, correlationSuggestions...)

	// Sort by priority and expected savings
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Priority != suggestions[j].Priority {
			return suggestions[i].Priority > suggestions[j].Priority
		}
		return suggestions[i].EstimatedSavings > suggestions[j].EstimatedSavings
	})

	// Store suggestions
	pa.mu.Lock()
	pa.optimizations = suggestions
	pa.mu.Unlock()

	pa.logger.Info("Generated %d optimization suggestions", len(suggestions))
	return suggestions, nil
}

// calculateStatistics calculates statistical measures for the data
func (pa *PatternAnalyzer) calculateStatistics(data []DataPoint) *PatternStatistics {
	if len(data) == 0 {
		return &PatternStatistics{}
	}

	values := make([]float64, len(data))
	for i, dp := range data {
		values[i] = dp.Value
	}

	sort.Float64s(values)

	stats := &PatternStatistics{
		Min: values[0],
		Max: values[len(values)-1],
	}
	stats.Range = stats.Max - stats.Min

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	stats.Mean = sum / float64(len(values))

	// Calculate median
	if len(values)%2 == 0 {
		stats.Median = (values[len(values)/2-1] + values[len(values)/2]) / 2
	} else {
		stats.Median = values[len(values)/2]
	}

	// Calculate variance and standard deviation
	sumSquaredDiff := 0.0
	for _, v := range values {
		diff := v - stats.Mean
		sumSquaredDiff += diff * diff
	}
	stats.Variance = sumSquaredDiff / float64(len(values))
	stats.StandardDeviation = math.Sqrt(stats.Variance)

	// Calculate percentiles
	percentilePositions := []float64{0.25, 0.5, 0.75, 0.9, 0.95, 0.99}
	stats.Percentiles = make([]float64, len(percentilePositions))
	for i, p := range percentilePositions {
		pos := p * float64(len(values)-1)
		if pos == float64(int(pos)) {
			stats.Percentiles[i] = values[int(pos)]
		} else {
			lower := values[int(pos)]
			upper := values[int(pos)+1]
			weight := pos - float64(int(pos))
			stats.Percentiles[i] = lower + weight*(upper-lower)
		}
	}

	// Calculate skewness (simplified)
	if stats.StandardDeviation > 0 {
		sumCubedDiff := 0.0
		for _, v := range values {
			diff := (v - stats.Mean) / stats.StandardDeviation
			sumCubedDiff += diff * diff * diff
		}
		stats.Skewness = sumCubedDiff / float64(len(values))
	}

	// Calculate kurtosis (simplified)
	if stats.StandardDeviation > 0 {
		sumQuartDiff := 0.0
		for _, v := range values {
			diff := (v - stats.Mean) / stats.StandardDeviation
			sumQuartDiff += diff * diff * diff * diff
		}
		stats.Kurtosis = sumQuartDiff/float64(len(values)) - 3 // Excess kurtosis
	}

	return stats
}

// detectSeasonality detects seasonal patterns in the data
func (pa *PatternAnalyzer) detectSeasonality(data []DataPoint) *SeasonalPattern {
	seasonality := &SeasonalPattern{}

	if len(data) < 24 { // Need at least 24 hours of data
		return seasonality
	}

	// Analyze hourly patterns
	hourlyValues := make([][]float64, 24)
	for _, dp := range data {
		hour := dp.Timestamp.Hour()
		hourlyValues[hour] = append(hourlyValues[hour], dp.Value)
	}

	// Calculate hourly averages
	var hourlyMeans [24]float64
	validHours := 0
	for i, values := range hourlyValues {
		if len(values) > 0 {
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			hourlyMeans[i] = sum / float64(len(values))
			validHours++
		}
	}

	if validHours >= 12 { // At least half the hours have data
		seasonality.HourlyPattern = hourlyMeans
		
		// Calculate amplitude as range of hourly means
		minHourly := hourlyMeans[0]
		maxHourly := hourlyMeans[0]
		for _, mean := range hourlyMeans[1:] {
			if mean < minHourly {
				minHourly = mean
			}
			if mean > maxHourly {
				maxHourly = mean
			}
		}
		seasonality.Amplitude = maxHourly - minHourly

		// Simple seasonality detection: if amplitude is significant relative to overall variance
		if len(data) > 0 {
			overallMean := 0.0
			for _, dp := range data {
				overallMean += dp.Value
			}
			overallMean /= float64(len(data))

			if seasonality.Amplitude > 0.1*overallMean { // 10% threshold
				seasonality.Detected = true
				seasonality.Period = 24 * time.Hour
				seasonality.Strength = seasonality.Amplitude / overallMean
				seasonality.Confidence = math.Min(1.0, seasonality.Strength*2)
			}
		}
	}

	// Analyze daily patterns (if enough data)
	if len(data) >= 7*24 { // At least a week of hourly data
		dailyValues := make([][]float64, 7)
		for _, dp := range data {
			weekday := int(dp.Timestamp.Weekday())
			dailyValues[weekday] = append(dailyValues[weekday], dp.Value)
		}

		var dailyMeans [7]float64
		validDays := 0
		for i, values := range dailyValues {
			if len(values) > 0 {
				sum := 0.0
				for _, v := range values {
					sum += v
				}
				dailyMeans[i] = sum / float64(len(values))
				validDays++
			}
		}

		if validDays >= 5 { // At least 5 days of the week
			seasonality.DailyPattern = dailyMeans
		}
	}

	return seasonality
}

// detectTrend detects trend patterns in the data
func (pa *PatternAnalyzer) detectTrend(data []DataPoint) *TrendPattern {
	trend := &TrendPattern{}

	if len(data) < 10 {
		return trend
	}

	// Simple linear regression to detect trend
	n := float64(len(data))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, dp := range data {
		x := float64(i)
		y := dp.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Calculate slope and intercept
	denominator := n*sumX2 - sumX*sumX
	if denominator != 0 {
		slope := (n*sumXY - sumX*sumY) / denominator
		intercept := (sumY - slope*sumX) / n

		// Calculate R-squared
		meanY := sumY / n
		ssRes := 0.0
		ssTot := 0.0
		for i, dp := range data {
			predicted := intercept + slope*float64(i)
			ssRes += (dp.Value - predicted) * (dp.Value - predicted)
			ssTot += (dp.Value - meanY) * (dp.Value - meanY)
		}

		var r2 float64
		if ssTot != 0 {
			r2 = 1 - ssRes/ssTot
		}

		trend.Slope = slope
		trend.R2 = r2

		// Determine trend direction and strength
		if math.Abs(slope) > 0.01 && r2 > 0.1 { // Minimum thresholds
			trend.Detected = true
			trend.Strength = r2
			trend.Confidence = r2
			
			if slope > 0 {
				trend.Direction = "increasing"
			} else {
				trend.Direction = "decreasing"
			}
		} else {
			trend.Direction = "stable"
		}
	}

	return trend
}

// calculatePatternConfidence calculates confidence in the detected pattern
func (pa *PatternAnalyzer) calculatePatternConfidence(pattern *UsagePattern) float64 {
	confidence := 0.0
	factors := 0

	// Data quality factor
	if len(pattern.DataPoints) >= pa.config.MinDataPoints*2 {
		confidence += 0.3
	} else {
		confidence += 0.1
	}
	factors++

	// Seasonality factor
	if pattern.Seasonality != nil && pattern.Seasonality.Detected {
		confidence += pattern.Seasonality.Confidence * 0.3
	} else {
		confidence += 0.1
	}
	factors++

	// Trend factor
	if pattern.Trend != nil && pattern.Trend.Detected {
		confidence += pattern.Trend.Confidence * 0.2
	} else {
		confidence += 0.1
	}
	factors++

	// Statistical consistency factor
	if pattern.Statistics != nil {
		cv := pattern.Statistics.StandardDeviation / pattern.Statistics.Mean
		if cv < 0.5 { // Low coefficient of variation indicates consistency
			confidence += 0.2
		} else if cv < 1.0 {
			confidence += 0.1
		}
	}
	factors++

	return math.Min(1.0, confidence)
}

// generatePredictions generates future predictions based on the pattern
func (pa *PatternAnalyzer) generatePredictions(data []DataPoint, pattern *UsagePattern) ([]PredictionPoint, error) {
	if len(data) < pa.config.MinDataPoints {
		return nil, fmt.Errorf("insufficient data for predictions")
	}

	var predictions []PredictionPoint
	
	// Generate predictions for the next prediction horizon
	predictionSteps := int(pa.config.PredictionHorizon.Hours())
	baseTime := data[len(data)-1].Timestamp

	for i := 1; i <= predictionSteps; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		
		// Simple prediction based on trend and seasonality
		predicted := pattern.Statistics.Mean
		confidence := 0.5

		// Apply trend if detected
		if pattern.Trend != nil && pattern.Trend.Detected {
			predicted += pattern.Trend.Slope * float64(i)
			confidence += pattern.Trend.Confidence * 0.3
		}

		// Apply seasonality if detected
		if pattern.Seasonality != nil && pattern.Seasonality.Detected {
			hour := timestamp.Hour()
			seasonalAdjustment := pattern.Seasonality.HourlyPattern[hour] - pattern.Statistics.Mean
			predicted += seasonalAdjustment * pattern.Seasonality.Strength
			confidence += pattern.Seasonality.Confidence * 0.2
		}

		// Calculate confidence intervals
		errorMargin := pattern.Statistics.StandardDeviation * (1.0 - confidence)
		
		predictions = append(predictions, PredictionPoint{
			Timestamp:       timestamp,
			PredictedValue:  predicted,
			ConfidenceUpper: predicted + errorMargin,
			ConfidenceLower: predicted - errorMargin,
			Confidence:      math.Min(1.0, confidence),
		})
	}

	return predictions, nil
}

// detectPatternAnomalies detects anomalies within a specific pattern
func (pa *PatternAnalyzer) detectPatternAnomalies(pattern *UsagePattern) []AnomalyEvent {
	var anomalies []AnomalyEvent

	if pattern.Statistics == nil || len(pattern.DataPoints) < pa.config.MinDataPoints {
		return anomalies
	}

	threshold := pa.config.AnomalyThreshold * pattern.Statistics.StandardDeviation

	for _, dp := range pattern.DataPoints {
		deviation := math.Abs(dp.Value - pattern.Statistics.Mean)
		
		if deviation > threshold {
			severity := LowSeverity
			if deviation > threshold*2 {
				severity = MediumSeverity
			}
			if deviation > threshold*3 {
				severity = HighSeverity
			}
			if deviation > threshold*4 {
				severity = CriticalSeverity
			}

			anomaly := AnomalyEvent{
				EventID:       fmt.Sprintf("anomaly_%s_%d", pattern.PatternID, dp.Timestamp.Unix()),
				Timestamp:     dp.Timestamp,
				Resource:      pattern.Resource,
				PatternType:   pattern.PatternType,
				Severity:      severity,
				Description:   fmt.Sprintf("Value %.2f deviates significantly from expected %.2f", dp.Value, pattern.Statistics.Mean),
				ActualValue:   dp.Value,
				ExpectedValue: pattern.Statistics.Mean,
				Deviation:     deviation,
				Confidence:    math.Min(1.0, deviation/threshold),
				Metadata:      map[string]interface{}{"pattern_id": pattern.PatternID},
				Resolved:      false,
			}

			anomalies = append(anomalies, anomaly)
		}
	}

	return anomalies
}

// generatePatternOptimizations generates optimization suggestions for a pattern
func (pa *PatternAnalyzer) generatePatternOptimizations(pattern *UsagePattern) []OptimizationSuggestion {
	var suggestions []OptimizationSuggestion

	// Storage pattern optimizations
	if pattern.PatternType == StoragePattern {
		if pattern.Trend != nil && pattern.Trend.Direction == "increasing" {
			suggestions = append(suggestions, OptimizationSuggestion{
				SuggestionID:    fmt.Sprintf("storage_growth_%s", pattern.Resource),
				Type:            StorageClassOptimization,
				Priority:        HighPriority,
				Resource:        pattern.Resource,
				Description:     "Storage is growing consistently. Consider lifecycle policies to manage costs.",
				ExpectedBenefit: "20-40% cost reduction",
				Implementation:  []string{"Implement lifecycle policies", "Review storage class usage", "Enable Intelligent Tiering"},
				EstimatedSavings: pattern.Statistics.Mean * 0.3 * 12, // 30% annual savings
				Confidence:      pattern.Confidence,
				BasedOnPatterns: []string{pattern.PatternID},
				ValidUntil:      time.Now().Add(30 * 24 * time.Hour),
			})
		}
	}

	// Access pattern optimizations
	if pattern.PatternType == AccessPattern {
		if pattern.Statistics.Mean < 0.1 { // Very low access frequency
			suggestions = append(suggestions, OptimizationSuggestion{
				SuggestionID:    fmt.Sprintf("low_access_%s", pattern.Resource),
				Type:            ArchivalOptimization,
				Priority:        HighPriority,
				Resource:        pattern.Resource,
				Description:     "Very low access frequency detected. Consider archival storage.",
				ExpectedBenefit: "60-80% cost reduction",
				Implementation:  []string{"Archive to Glacier", "Set up retrieval policies", "Update access procedures"},
				EstimatedSavings: pattern.Statistics.Mean * 1000 * 0.7 * 12, // Estimate based on storage size
				Confidence:      pattern.Confidence,
				BasedOnPatterns: []string{pattern.PatternID},
				ValidUntil:      time.Now().Add(30 * 24 * time.Hour),
			})
		}

		if pattern.Seasonality != nil && pattern.Seasonality.Detected {
			suggestions = append(suggestions, OptimizationSuggestion{
				SuggestionID:    fmt.Sprintf("seasonal_cache_%s", pattern.Resource),
				Type:            CachingOptimization,
				Priority:        MediumPriority,
				Resource:        pattern.Resource,
				Description:     "Seasonal access pattern detected. Consider predictive caching.",
				ExpectedBenefit: "30-50% performance improvement",
				Implementation:  []string{"Implement predictive caching", "Scale cache based on predicted load", "Monitor cache hit rates"},
				EstimatedSavings: 100, // Fixed estimate for performance improvements
				Confidence:      pattern.Seasonality.Confidence,
				BasedOnPatterns: []string{pattern.PatternID},
				ValidUntil:      time.Now().Add(30 * 24 * time.Hour),
			})
		}
	}

	return suggestions
}

// generateCorrelationOptimizations generates optimizations based on correlations between patterns
func (pa *PatternAnalyzer) generateCorrelationOptimizations(patterns map[string]*UsagePattern) []OptimizationSuggestion {
	var suggestions []OptimizationSuggestion

	// Find patterns with strong correlations
	patternList := make([]*UsagePattern, 0, len(patterns))
	for _, pattern := range patterns {
		patternList = append(patternList, pattern)
	}

	for i := 0; i < len(patternList); i++ {
		for j := i + 1; j < len(patternList); j++ {
			correlation := pa.calculateCorrelation(patternList[i], patternList[j])
			
			if correlation > 0.8 { // Strong positive correlation
				suggestion := OptimizationSuggestion{
					SuggestionID:    fmt.Sprintf("correlation_%s_%s", patternList[i].PatternID, patternList[j].PatternID),
					Type:            StorageClassOptimization,
					Priority:        MediumPriority,
					Resource:        fmt.Sprintf("%s,%s", patternList[i].Resource, patternList[j].Resource),
					Description:     "Strong correlation detected between resources. Consider unified optimization strategy.",
					ExpectedBenefit: "Coordinated optimization benefits",
					Implementation:  []string{"Analyze correlation causes", "Apply similar optimizations", "Monitor correlation changes"},
					EstimatedSavings: 50, // Conservative estimate
					Confidence:      (patternList[i].Confidence + patternList[j].Confidence) / 2,
					BasedOnPatterns: []string{patternList[i].PatternID, patternList[j].PatternID},
					ValidUntil:      time.Now().Add(30 * 24 * time.Hour),
					Metadata: map[string]interface{}{
						"correlation": correlation,
					},
				}
				suggestions = append(suggestions, suggestion)
			}
		}
	}

	return suggestions
}

// calculateCorrelation calculates correlation between two patterns
func (pa *PatternAnalyzer) calculateCorrelation(pattern1, pattern2 *UsagePattern) float64 {
	if len(pattern1.DataPoints) != len(pattern2.DataPoints) {
		return 0.0 // Cannot calculate correlation for different length series
	}

	n := len(pattern1.DataPoints)
	if n < 2 {
		return 0.0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64

	for i := 0; i < n; i++ {
		x := pattern1.DataPoints[i].Value
		y := pattern2.DataPoints[i].Value
		
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	numerator := float64(n)*sumXY - sumX*sumY
	denominator := math.Sqrt((float64(n)*sumX2 - sumX*sumX) * (float64(n)*sumY2 - sumY*sumY))

	if denominator == 0 {
		return 0.0
	}

	return numerator / denominator
}

// GetPatterns returns all learned patterns
func (pa *PatternAnalyzer) GetPatterns() map[string]*UsagePattern {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	result := make(map[string]*UsagePattern)
	for k, v := range pa.patterns {
		result[k] = v
	}
	return result
}

// GetAnomalies returns detected anomalies
func (pa *PatternAnalyzer) GetAnomalies() []AnomalyEvent {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	result := make([]AnomalyEvent, len(pa.anomalies))
	copy(result, pa.anomalies)
	return result
}

// GetOptimizations returns generated optimization suggestions
func (pa *PatternAnalyzer) GetOptimizations() []OptimizationSuggestion {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	result := make([]OptimizationSuggestion, len(pa.optimizations))
	copy(result, pa.optimizations)
	return result
}