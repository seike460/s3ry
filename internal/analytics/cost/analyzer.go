package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// CostAnalyzer provides comprehensive cost analysis and optimization
type CostAnalyzer struct {
	config       *AnalyzerConfig
	collector    *UsageCollector
	calculator   *CostCalculator
	optimizer    *CostOptimizer
	forecaster   *CostForecaster
	storage      CostDataStorage
	alertManager CostAlertManager
	mutex        sync.RWMutex
}

// AnalyzerConfig holds cost analyzer configuration
type AnalyzerConfig struct {
	CollectionInterval time.Duration      `json:"collection_interval"`
	RetentionPeriod    time.Duration      `json:"retention_period"`
	Currency           string             `json:"currency"`
	BillingPeriod      string             `json:"billing_period"` // monthly, quarterly, annual
	CostThresholds     map[string]float64 `json:"cost_thresholds"`
	OptimizationRules  []OptimizationRule `json:"optimization_rules"`
	ForecastHorizon    time.Duration      `json:"forecast_horizon"`
	AlertEnabled       bool               `json:"alert_enabled"`
	ReportsEnabled     bool               `json:"reports_enabled"`
	ReportSchedule     []ReportSchedule   `json:"report_schedule"`
}

// UsageCollector collects usage metrics for cost calculation
type UsageCollector struct {
	config  *CollectorConfig
	metrics map[string]*UsageMetric
	mutex   sync.RWMutex
	stopCh  chan struct{}
	running bool
}

// CollectorConfig holds usage collector configuration
type CollectorConfig struct {
	S3Pricing         S3PricingConfig      `json:"s3_pricing"`
	ComputePricing    ComputePricingConfig `json:"compute_pricing"`
	NetworkPricing    NetworkPricingConfig `json:"network_pricing"`
	StoragePricing    StoragePricingConfig `json:"storage_pricing"`
	SamplingRate      float64              `json:"sampling_rate"`
	AggregationWindow time.Duration        `json:"aggregation_window"`
}

// S3PricingConfig holds S3 service pricing configuration
type S3PricingConfig struct {
	StoragePerGB    map[string]float64 `json:"storage_per_gb"`    // Storage class -> price per GB
	RequestPricing  map[string]float64 `json:"request_pricing"`   // Request type -> price per 1000
	DataTransferOut map[string]float64 `json:"data_transfer_out"` // Region -> price per GB
	DataTransferIn  float64            `json:"data_transfer_in"`  // Usually free
	Region          string             `json:"region"`
}

// ComputePricingConfig holds compute pricing configuration
type ComputePricingConfig struct {
	CPUHourly    float64            `json:"cpu_hourly"`    // Price per CPU hour
	MemoryHourly float64            `json:"memory_hourly"` // Price per GB memory hour
	Instances    map[string]float64 `json:"instances"`     // Instance type -> hourly rate
}

// NetworkPricingConfig holds network pricing configuration
type NetworkPricingConfig struct {
	InboundFree   bool               `json:"inbound_free"`
	OutboundTiers []PricingTier      `json:"outbound_tiers"`
	InterRegion   map[string]float64 `json:"inter_region"`
	CrossAZ       float64            `json:"cross_az"`
}

// StoragePricingConfig holds storage pricing configuration
type StoragePricingConfig struct {
	StandardStorage  float64 `json:"standard_storage"`  // Per GB/month
	RedundantStorage float64 `json:"redundant_storage"` // Per GB/month
	ArchivalStorage  float64 `json:"archival_storage"`  // Per GB/month
	BackupStorage    float64 `json:"backup_storage"`    // Per GB/month
}

// PricingTier represents a pricing tier with usage thresholds
type PricingTier struct {
	ThresholdGB float64 `json:"threshold_gb"`
	PricePerGB  float64 `json:"price_per_gb"`
}

// UsageMetric represents a usage measurement
type UsageMetric struct {
	Service    string                 `json:"service"`
	MetricType string                 `json:"metric_type"`
	Value      float64                `json:"value"`
	Unit       string                 `json:"unit"`
	Timestamp  time.Time              `json:"timestamp"`
	Tags       map[string]string      `json:"tags"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// CostCalculator calculates costs from usage metrics
type CostCalculator struct {
	config        *CalculatorConfig
	pricingEngine *PricingEngine
}

// CalculatorConfig holds cost calculator configuration
type CalculatorConfig struct {
	TaxRate          float64            `json:"tax_rate"`
	DiscountRules    []DiscountRule     `json:"discount_rules"`
	ReservedCapacity map[string]float64 `json:"reserved_capacity"`
	VolumeDiscounts  []VolumeDiscount   `json:"volume_discounts"`
}

// DiscountRule represents a discount rule
type DiscountRule struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Service   string            `json:"service"`
	Condition string            `json:"condition"` // Expression to evaluate
	Discount  float64           `json:"discount"`  // Percentage or fixed amount
	Type      string            `json:"type"`      // percentage, fixed
	ValidFrom time.Time         `json:"valid_from"`
	ValidTo   time.Time         `json:"valid_to"`
	Tags      map[string]string `json:"tags"`
}

// VolumeDiscount represents volume-based discounts
type VolumeDiscount struct {
	Service   string  `json:"service"`
	Threshold float64 `json:"threshold"`
	Discount  float64 `json:"discount"`
	Type      string  `json:"type"`
}

// PricingEngine calculates actual costs
type PricingEngine struct {
	config *CollectorConfig
}

// CostBreakdown represents detailed cost breakdown
type CostBreakdown struct {
	TotalCost     float64                 `json:"total_cost"`
	Currency      string                  `json:"currency"`
	Period        string                  `json:"period"`
	ServiceCosts  map[string]ServiceCost  `json:"service_costs"`
	ResourceCosts map[string]ResourceCost `json:"resource_costs"`
	TagCosts      map[string]float64      `json:"tag_costs"`
	RegionCosts   map[string]float64      `json:"region_costs"`
	Discounts     []AppliedDiscount       `json:"discounts"`
	TotalDiscount float64                 `json:"total_discount"`
	TaxAmount     float64                 `json:"tax_amount"`
	NetCost       float64                 `json:"net_cost"`
	Timestamp     time.Time               `json:"timestamp"`
	BillingPeriod BillingPeriod           `json:"billing_period"`
}

// ServiceCost represents cost breakdown by service
type ServiceCost struct {
	Service    string             `json:"service"`
	Cost       float64            `json:"cost"`
	Usage      float64            `json:"usage"`
	Unit       string             `json:"unit"`
	Components map[string]float64 `json:"components"`
	Trend      CostTrend          `json:"trend"`
}

// ResourceCost represents cost by individual resource
type ResourceCost struct {
	ResourceID   string    `json:"resource_id"`
	ResourceType string    `json:"resource_type"`
	Cost         float64   `json:"cost"`
	Owner        string    `json:"owner"`
	Department   string    `json:"department"`
	Project      string    `json:"project"`
	LastAccessed time.Time `json:"last_accessed"`
}

// AppliedDiscount represents a discount that was applied
type AppliedDiscount struct {
	RuleID  string  `json:"rule_id"`
	Name    string  `json:"name"`
	Amount  float64 `json:"amount"`
	Type    string  `json:"type"`
	Service string  `json:"service"`
}

// CostTrend represents cost trend analysis
type CostTrend struct {
	Direction     string  `json:"direction"` // increasing, decreasing, stable
	ChangePercent float64 `json:"change_percent"`
	Confidence    float64 `json:"confidence"`
	Period        string  `json:"period"`
}

// BillingPeriod represents a billing period
type BillingPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Type  string    `json:"type"`
}

// CostOptimizer provides cost optimization recommendations
type CostOptimizer struct {
	config *OptimizerConfig
	rules  []OptimizationRule
}

// OptimizerConfig holds optimizer configuration
type OptimizerConfig struct {
	MinSavingsThreshold float64       `json:"min_savings_threshold"`
	AnalysisWindow      time.Duration `json:"analysis_window"`
	ConfidenceThreshold float64       `json:"confidence_threshold"`
	EnabledCategories   []string      `json:"enabled_categories"`
}

// OptimizationRule represents a cost optimization rule
type OptimizationRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Category    string                 `json:"category"`
	Description string                 `json:"description"`
	Condition   string                 `json:"condition"`
	Action      string                 `json:"action"`
	Impact      OptimizationImpact     `json:"impact"`
	Confidence  float64                `json:"confidence"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// OptimizationImpact represents the impact of an optimization
type OptimizationImpact struct {
	CostSavings    float64 `json:"cost_savings"`
	SavingsPercent float64 `json:"savings_percent"`
	Implementation string  `json:"implementation"` // auto, manual, approval_required
	Risk           string  `json:"risk"`           // low, medium, high
	Effort         string  `json:"effort"`         // low, medium, high
}

// Recommendation represents a cost optimization recommendation
type Recommendation struct {
	ID             string             `json:"id"`
	Type           string             `json:"type"`
	Priority       string             `json:"priority"`
	Title          string             `json:"title"`
	Description    string             `json:"description"`
	ResourceID     string             `json:"resource_id"`
	Service        string             `json:"service"`
	CurrentCost    float64            `json:"current_cost"`
	OptimizedCost  float64            `json:"optimized_cost"`
	Savings        float64            `json:"savings"`
	SavingsPercent float64            `json:"savings_percent"`
	Impact         OptimizationImpact `json:"impact"`
	Actions        []string           `json:"actions"`
	Evidence       []Evidence         `json:"evidence"`
	CreatedAt      time.Time          `json:"created_at"`
	ValidUntil     time.Time          `json:"valid_until"`
	Status         string             `json:"status"`
}

// Evidence represents supporting evidence for a recommendation
type Evidence struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Value       interface{} `json:"value"`
	Confidence  float64     `json:"confidence"`
}

// CostForecaster predicts future costs
type CostForecaster struct {
	config *ForecastConfig
	models map[string]*ForecastModel
}

// ForecastConfig holds forecasting configuration
type ForecastConfig struct {
	Models          []string      `json:"models"` // linear, exponential, seasonal
	LookbackPeriod  time.Duration `json:"lookback_period"`
	ForecastHorizon time.Duration `json:"forecast_horizon"`
	ConfidenceLevel float64       `json:"confidence_level"`
	SeasonalPattern string        `json:"seasonal_pattern"` // weekly, monthly, quarterly
}

// ForecastModel represents a cost forecasting model
type ForecastModel struct {
	Name        string             `json:"name"`
	Type        string             `json:"type"`
	Parameters  map[string]float64 `json:"parameters"`
	Accuracy    float64            `json:"accuracy"`
	LastTrained time.Time          `json:"last_trained"`
}

// CostForecast represents a cost forecast
type CostForecast struct {
	Service         string           `json:"service"`
	Period          BillingPeriod    `json:"period"`
	PredictedCost   float64          `json:"predicted_cost"`
	ConfidenceRange ConfidenceRange  `json:"confidence_range"`
	Trend           ForecastTrend    `json:"trend"`
	Seasonality     []SeasonalFactor `json:"seasonality"`
	Model           string           `json:"model"`
	Accuracy        float64          `json:"accuracy"`
	CreatedAt       time.Time        `json:"created_at"`
}

// ConfidenceRange represents forecast confidence range
type ConfidenceRange struct {
	Lower      float64 `json:"lower"`
	Upper      float64 `json:"upper"`
	Confidence float64 `json:"confidence"`
}

// ForecastTrend represents forecast trend
type ForecastTrend struct {
	Direction    string  `json:"direction"`
	Growth       float64 `json:"growth"`
	Acceleration float64 `json:"acceleration"`
}

// SeasonalFactor represents seasonal cost factors
type SeasonalFactor struct {
	Period string  `json:"period"`
	Factor float64 `json:"factor"`
}

// CostDataStorage interface for storing cost data
type CostDataStorage interface {
	StoreUsage(metric *UsageMetric) error
	StoreCostBreakdown(breakdown *CostBreakdown) error
	GetUsageHistory(service string, period time.Duration) ([]*UsageMetric, error)
	GetCostHistory(period time.Duration) ([]*CostBreakdown, error)
	GetCostByService(service string, period time.Duration) (float64, error)
	Cleanup(retentionPeriod time.Duration) error
}

// CostAlertManager interface for cost alerting
type CostAlertManager interface {
	CheckThresholds(breakdown *CostBreakdown) error
	SendAlert(alert CostAlert) error
	GetActiveAlerts() ([]CostAlert, error)
}

// CostAlert represents a cost alert
type CostAlert struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Severity   string                 `json:"severity"`
	Service    string                 `json:"service"`
	Threshold  float64                `json:"threshold"`
	ActualCost float64                `json:"actual_cost"`
	Percentage float64                `json:"percentage"`
	Period     string                 `json:"period"`
	Message    string                 `json:"message"`
	Recipients []string               `json:"recipients"`
	CreatedAt  time.Time              `json:"created_at"`
	Status     string                 `json:"status"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// ReportSchedule defines when to generate cost reports
type ReportSchedule struct {
	Name       string   `json:"name"`
	Frequency  string   `json:"frequency"` // daily, weekly, monthly
	Recipients []string `json:"recipients"`
	Format     string   `json:"format"`   // json, csv, pdf
	Sections   []string `json:"sections"` // breakdown, trends, recommendations
}

// DefaultAnalyzerConfig returns default cost analyzer configuration
func DefaultAnalyzerConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		CollectionInterval: time.Minute * 15,
		RetentionPeriod:    time.Hour * 24 * 90, // 90 days
		Currency:           "USD",
		BillingPeriod:      "monthly",
		CostThresholds: map[string]float64{
			"total_monthly":   1000.0,
			"s3_monthly":      500.0,
			"compute_monthly": 300.0,
			"network_monthly": 200.0,
		},
		ForecastHorizon: time.Hour * 24 * 30, // 30 days
		AlertEnabled:    true,
		ReportsEnabled:  true,
		ReportSchedule: []ReportSchedule{
			{
				Name:       "monthly_summary",
				Frequency:  "monthly",
				Recipients: []string{"admin@example.com"},
				Format:     "json",
				Sections:   []string{"breakdown", "trends", "recommendations"},
			},
		},
	}
}

// NewCostAnalyzer creates a new cost analyzer
func NewCostAnalyzer(config *AnalyzerConfig, storage CostDataStorage, alertManager CostAlertManager) (*CostAnalyzer, error) {
	if config == nil {
		config = DefaultAnalyzerConfig()
	}

	collector := NewUsageCollector(&CollectorConfig{
		S3Pricing: S3PricingConfig{
			StoragePerGB: map[string]float64{
				"standard": 0.023,
				"ia":       0.0125,
				"glacier":  0.004,
			},
			RequestPricing: map[string]float64{
				"get":    0.0004,
				"put":    0.005,
				"list":   0.005,
				"delete": 0.0,
			},
			DataTransferOut: map[string]float64{
				"internet": 0.09,
				"region":   0.02,
			},
			DataTransferIn: 0.0,
			Region:         "us-east-1",
		},
		ComputePricing: ComputePricingConfig{
			CPUHourly:    0.05,
			MemoryHourly: 0.01,
		},
		NetworkPricing: NetworkPricingConfig{
			InboundFree: true,
			OutboundTiers: []PricingTier{
				{ThresholdGB: 10.0, PricePerGB: 0.09},
				{ThresholdGB: 50.0, PricePerGB: 0.085},
				{ThresholdGB: 150.0, PricePerGB: 0.07},
			},
		},
		SamplingRate:      1.0,
		AggregationWindow: time.Hour,
	})

	calculator := NewCostCalculator(&CalculatorConfig{
		TaxRate: 0.08, // 8% tax
		VolumeDiscounts: []VolumeDiscount{
			{Service: "s3", Threshold: 1000.0, Discount: 0.05, Type: "percentage"},
			{Service: "compute", Threshold: 500.0, Discount: 0.10, Type: "percentage"},
		},
	})

	optimizer := NewCostOptimizer(&OptimizerConfig{
		MinSavingsThreshold: 10.0, // $10 minimum savings
		AnalysisWindow:      time.Hour * 24 * 7,
		ConfidenceThreshold: 0.8,
		EnabledCategories:   []string{"storage", "compute", "network"},
	})

	forecaster := NewCostForecaster(&ForecastConfig{
		Models:          []string{"linear", "exponential"},
		LookbackPeriod:  time.Hour * 24 * 30,
		ForecastHorizon: time.Hour * 24 * 30,
		ConfidenceLevel: 0.95,
		SeasonalPattern: "monthly",
	})

	return &CostAnalyzer{
		config:       config,
		collector:    collector,
		calculator:   calculator,
		optimizer:    optimizer,
		forecaster:   forecaster,
		storage:      storage,
		alertManager: alertManager,
	}, nil
}

// NewUsageCollector creates a new usage collector
func NewUsageCollector(config *CollectorConfig) *UsageCollector {
	return &UsageCollector{
		config:  config,
		metrics: make(map[string]*UsageMetric),
		stopCh:  make(chan struct{}),
	}
}

// NewCostCalculator creates a new cost calculator
func NewCostCalculator(config *CalculatorConfig) *CostCalculator {
	return &CostCalculator{
		config:        config,
		pricingEngine: &PricingEngine{},
	}
}

// NewCostOptimizer creates a new cost optimizer
func NewCostOptimizer(config *OptimizerConfig) *CostOptimizer {
	optimizer := &CostOptimizer{
		config: config,
		rules:  make([]OptimizationRule, 0),
	}

	// Initialize default optimization rules
	optimizer.initializeDefaultRules()

	return optimizer
}

// NewCostForecaster creates a new cost forecaster
func NewCostForecaster(config *ForecastConfig) *CostForecaster {
	return &CostForecaster{
		config: config,
		models: make(map[string]*ForecastModel),
	}
}

// Start starts the cost analyzer
func (c *CostAnalyzer) Start(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Start usage collector
	if err := c.collector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start usage collector: %w", err)
	}

	// Start periodic analysis
	go c.runPeriodicAnalysis(ctx)

	return nil
}

// Stop stops the cost analyzer
func (c *CostAnalyzer) Stop() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.collector.Stop()
}

// Start starts the usage collector
func (u *UsageCollector) Start(ctx context.Context) error {
	u.mutex.Lock()
	if u.running {
		u.mutex.Unlock()
		return fmt.Errorf("usage collector already running")
	}
	u.running = true
	u.mutex.Unlock()

	go u.collectUsage(ctx)
	return nil
}

// Stop stops the usage collector
func (u *UsageCollector) Stop() error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	if u.running {
		close(u.stopCh)
		u.running = false
	}
	return nil
}

// collectUsage periodically collects usage metrics
func (u *UsageCollector) collectUsage(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 5) // Collect every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-u.stopCh:
			return
		case <-ticker.C:
			u.sampleUsage()
		}
	}
}

// sampleUsage samples current usage metrics
func (u *UsageCollector) sampleUsage() {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	// Simulate S3 usage collection
	s3Storage := &UsageMetric{
		Service:    "s3",
		MetricType: "storage",
		Value:      float64(1000 + int(time.Now().Unix())%500), // Simulate 1000-1500 GB
		Unit:       "GB",
		Timestamp:  time.Now(),
		Tags: map[string]string{
			"storage_class": "standard",
			"region":        "us-east-1",
		},
	}
	u.metrics["s3_storage"] = s3Storage

	// Simulate S3 requests
	s3Requests := &UsageMetric{
		Service:    "s3",
		MetricType: "requests",
		Value:      float64(10000 + int(time.Now().Unix())%5000), // Simulate 10K-15K requests
		Unit:       "count",
		Timestamp:  time.Now(),
		Tags: map[string]string{
			"request_type": "get",
			"region":       "us-east-1",
		},
	}
	u.metrics["s3_requests"] = s3Requests

	// Simulate compute usage
	computeHours := &UsageMetric{
		Service:    "compute",
		MetricType: "cpu_hours",
		Value:      float64(50 + int(time.Now().Unix())%20), // Simulate 50-70 CPU hours
		Unit:       "hours",
		Timestamp:  time.Now(),
		Tags: map[string]string{
			"instance_type": "t3.medium",
			"region":        "us-east-1",
		},
	}
	u.metrics["compute_hours"] = computeHours

	// Simulate network usage
	networkOut := &UsageMetric{
		Service:    "network",
		MetricType: "data_transfer_out",
		Value:      float64(100 + int(time.Now().Unix())%50), // Simulate 100-150 GB
		Unit:       "GB",
		Timestamp:  time.Now(),
		Tags: map[string]string{
			"destination": "internet",
			"region":      "us-east-1",
		},
	}
	u.metrics["network_out"] = networkOut
}

// GetCurrentUsage returns current usage metrics
func (u *UsageCollector) GetCurrentUsage() map[string]*UsageMetric {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	// Return a copy to prevent external modification
	usage := make(map[string]*UsageMetric)
	for k, v := range u.metrics {
		metricCopy := *v
		usage[k] = &metricCopy
	}

	return usage
}

// CalculateCosts calculates costs from usage metrics
func (c *CostCalculator) CalculateCosts(usage map[string]*UsageMetric) (*CostBreakdown, error) {
	breakdown := &CostBreakdown{
		Currency:      "USD",
		Period:        "current",
		ServiceCosts:  make(map[string]ServiceCost),
		ResourceCosts: make(map[string]ResourceCost),
		TagCosts:      make(map[string]float64),
		RegionCosts:   make(map[string]float64),
		Timestamp:     time.Now(),
		BillingPeriod: BillingPeriod{
			Start: time.Now().Truncate(time.Hour * 24),
			End:   time.Now(),
			Type:  "daily",
		},
	}

	var totalCost float64

	// Calculate S3 costs
	if s3Storage, exists := usage["s3_storage"]; exists {
		storageCost := s3Storage.Value * c.config.StoragePricing.StandardStorage / 30 // Daily cost
		totalCost += storageCost

		breakdown.ServiceCosts["s3"] = ServiceCost{
			Service: "s3",
			Cost:    storageCost,
			Usage:   s3Storage.Value,
			Unit:    "GB",
			Components: map[string]float64{
				"storage": storageCost,
			},
			Trend: CostTrend{
				Direction:     "stable",
				ChangePercent: 0.0,
				Confidence:    0.8,
				Period:        "daily",
			},
		}

		breakdown.RegionCosts["us-east-1"] += storageCost
	}

	// Calculate S3 request costs
	if s3Requests, exists := usage["s3_requests"]; exists {
		requestCost := s3Requests.Value / 1000 * 0.0004 // $0.0004 per 1000 requests
		totalCost += requestCost

		if serviceCost, exists := breakdown.ServiceCosts["s3"]; exists {
			serviceCost.Cost += requestCost
			serviceCost.Components["requests"] = requestCost
			breakdown.ServiceCosts["s3"] = serviceCost
		}

		breakdown.RegionCosts["us-east-1"] += requestCost
	}

	// Calculate compute costs
	if computeHours, exists := usage["compute_hours"]; exists {
		computeCost := computeHours.Value * 0.05 // $0.05 per CPU hour
		totalCost += computeCost

		breakdown.ServiceCosts["compute"] = ServiceCost{
			Service: "compute",
			Cost:    computeCost,
			Usage:   computeHours.Value,
			Unit:    "hours",
			Components: map[string]float64{
				"cpu": computeCost,
			},
			Trend: CostTrend{
				Direction:     "increasing",
				ChangePercent: 5.2,
				Confidence:    0.9,
				Period:        "daily",
			},
		}

		breakdown.RegionCosts["us-east-1"] += computeCost
	}

	// Calculate network costs
	if networkOut, exists := usage["network_out"]; exists {
		networkCost := networkOut.Value * 0.09 // $0.09 per GB
		totalCost += networkCost

		breakdown.ServiceCosts["network"] = ServiceCost{
			Service: "network",
			Cost:    networkCost,
			Usage:   networkOut.Value,
			Unit:    "GB",
			Components: map[string]float64{
				"data_transfer_out": networkCost,
			},
			Trend: CostTrend{
				Direction:     "stable",
				ChangePercent: 1.1,
				Confidence:    0.7,
				Period:        "daily",
			},
		}

		breakdown.RegionCosts["us-east-1"] += networkCost
	}

	// Apply volume discounts
	c.applyVolumeDiscounts(breakdown)

	// Calculate tax
	breakdown.TaxAmount = breakdown.TotalCost * c.config.TaxRate
	breakdown.NetCost = breakdown.TotalCost + breakdown.TaxAmount

	breakdown.TotalCost = totalCost

	return breakdown, nil
}

// applyVolumeDiscounts applies volume-based discounts
func (c *CostCalculator) applyVolumeDiscounts(breakdown *CostBreakdown) {
	for _, discount := range c.config.VolumeDiscounts {
		if serviceCost, exists := breakdown.ServiceCosts[discount.Service]; exists {
			if serviceCost.Cost >= discount.Threshold {
				discountAmount := serviceCost.Cost * discount.Discount
				breakdown.Discounts = append(breakdown.Discounts, AppliedDiscount{
					RuleID:  fmt.Sprintf("volume_%s", discount.Service),
					Name:    fmt.Sprintf("Volume discount for %s", discount.Service),
					Amount:  discountAmount,
					Type:    discount.Type,
					Service: discount.Service,
				})
				breakdown.TotalDiscount += discountAmount

				// Update service cost
				serviceCost.Cost -= discountAmount
				breakdown.ServiceCosts[discount.Service] = serviceCost
			}
		}
	}
}

// runPeriodicAnalysis runs periodic cost analysis
func (c *CostAnalyzer) runPeriodicAnalysis(ctx context.Context) {
	ticker := time.NewTicker(c.config.CollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.performAnalysis()
		}
	}
}

// performAnalysis performs comprehensive cost analysis
func (c *CostAnalyzer) performAnalysis() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Get current usage
	usage := c.collector.GetCurrentUsage()

	// Calculate costs
	breakdown, err := c.calculator.CalculateCosts(usage)
	if err != nil {
		fmt.Printf("Failed to calculate costs: %v\n", err)
		return
	}

	// Store cost breakdown
	if err := c.storage.StoreCostBreakdown(breakdown); err != nil {
		fmt.Printf("Failed to store cost breakdown: %v\n", err)
	}

	// Store usage metrics
	for _, metric := range usage {
		if err := c.storage.StoreUsage(metric); err != nil {
			fmt.Printf("Failed to store usage metric: %v\n", err)
		}
	}

	// Check cost thresholds
	if c.config.AlertEnabled {
		if err := c.alertManager.CheckThresholds(breakdown); err != nil {
			fmt.Printf("Failed to check cost thresholds: %v\n", err)
		}
	}
}

// GetCostBreakdown returns current cost breakdown
func (c *CostAnalyzer) GetCostBreakdown() (*CostBreakdown, error) {
	usage := c.collector.GetCurrentUsage()
	return c.calculator.CalculateCosts(usage)
}

// GetOptimizationRecommendations returns cost optimization recommendations
func (c *CostAnalyzer) GetOptimizationRecommendations() ([]*Recommendation, error) {
	return c.optimizer.GenerateRecommendations(c.storage)
}

// GetCostForecast returns cost forecast
func (c *CostAnalyzer) GetCostForecast(service string, horizon time.Duration) (*CostForecast, error) {
	return c.forecaster.GenerateForecast(service, horizon, c.storage)
}

// initializeDefaultRules initializes default optimization rules
func (o *CostOptimizer) initializeDefaultRules() {
	o.rules = []OptimizationRule{
		{
			ID:          "unused_storage",
			Name:        "Unused Storage Detection",
			Category:    "storage",
			Description: "Detect storage that hasn't been accessed recently",
			Condition:   "last_accessed > 30d AND cost > 10",
			Action:      "move_to_archive",
			Impact: OptimizationImpact{
				CostSavings:    50.0,
				SavingsPercent: 70.0,
				Implementation: "auto",
				Risk:           "low",
				Effort:         "low",
			},
			Confidence: 0.9,
		},
		{
			ID:          "oversized_compute",
			Name:        "Oversized Compute Resources",
			Category:    "compute",
			Description: "Detect compute resources with low utilization",
			Condition:   "cpu_utilization < 20% AND memory_utilization < 30%",
			Action:      "downsize_instance",
			Impact: OptimizationImpact{
				CostSavings:    100.0,
				SavingsPercent: 40.0,
				Implementation: "manual",
				Risk:           "medium",
				Effort:         "medium",
			},
			Confidence: 0.8,
		},
		{
			ID:          "redundant_data_transfer",
			Name:        "Redundant Data Transfer",
			Category:    "network",
			Description: "Detect unnecessary cross-region data transfers",
			Condition:   "cross_region_transfer > 100GB AND same_region_alternative_exists",
			Action:      "optimize_data_locality",
			Impact: OptimizationImpact{
				CostSavings:    75.0,
				SavingsPercent: 60.0,
				Implementation: "manual",
				Risk:           "low",
				Effort:         "high",
			},
			Confidence: 0.7,
		},
	}
}

// GenerateRecommendations generates cost optimization recommendations
func (o *CostOptimizer) GenerateRecommendations(storage CostDataStorage) ([]*Recommendation, error) {
	recommendations := make([]*Recommendation, 0)

	// Get recent cost history
	history, err := storage.GetCostHistory(o.config.AnalysisWindow)
	if err != nil {
		return nil, fmt.Errorf("failed to get cost history: %w", err)
	}

	// Analyze each optimization rule
	for _, rule := range o.rules {
		if recommendation := o.evaluateRule(rule, history); recommendation != nil {
			recommendations = append(recommendations, recommendation)
		}
	}

	// Sort by potential savings
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Savings > recommendations[j].Savings
	})

	return recommendations, nil
}

// evaluateRule evaluates an optimization rule against cost history
func (o *CostOptimizer) evaluateRule(rule OptimizationRule, history []*CostBreakdown) *Recommendation {
	// Simplified rule evaluation - in production, this would be more sophisticated
	if len(history) == 0 {
		return nil
	}

	latest := history[len(history)-1]

	switch rule.Category {
	case "storage":
		if serviceCost, exists := latest.ServiceCosts["s3"]; exists && serviceCost.Cost > 50 {
			return &Recommendation{
				ID:             fmt.Sprintf("rec_%s_%d", rule.ID, time.Now().Unix()),
				Type:           "optimization",
				Priority:       "medium",
				Title:          rule.Name,
				Description:    rule.Description,
				Service:        "s3",
				CurrentCost:    serviceCost.Cost,
				OptimizedCost:  serviceCost.Cost * (1 - rule.Impact.SavingsPercent/100),
				Savings:        serviceCost.Cost * rule.Impact.SavingsPercent / 100,
				SavingsPercent: rule.Impact.SavingsPercent,
				Impact:         rule.Impact,
				Actions:        []string{rule.Action},
				Evidence: []Evidence{
					{
						Type:        "cost_analysis",
						Description: "High storage costs detected",
						Value:       serviceCost.Cost,
						Confidence:  rule.Confidence,
					},
				},
				CreatedAt:  time.Now(),
				ValidUntil: time.Now().Add(time.Hour * 24 * 7),
				Status:     "active",
			}
		}
	case "compute":
		if serviceCost, exists := latest.ServiceCosts["compute"]; exists && serviceCost.Cost > 30 {
			return &Recommendation{
				ID:             fmt.Sprintf("rec_%s_%d", rule.ID, time.Now().Unix()),
				Type:           "optimization",
				Priority:       "high",
				Title:          rule.Name,
				Description:    rule.Description,
				Service:        "compute",
				CurrentCost:    serviceCost.Cost,
				OptimizedCost:  serviceCost.Cost * (1 - rule.Impact.SavingsPercent/100),
				Savings:        serviceCost.Cost * rule.Impact.SavingsPercent / 100,
				SavingsPercent: rule.Impact.SavingsPercent,
				Impact:         rule.Impact,
				Actions:        []string{rule.Action},
				Evidence: []Evidence{
					{
						Type:        "utilization_analysis",
						Description: "Low CPU utilization detected",
						Value:       "15%",
						Confidence:  rule.Confidence,
					},
				},
				CreatedAt:  time.Now(),
				ValidUntil: time.Now().Add(time.Hour * 24 * 7),
				Status:     "active",
			}
		}
	}

	return nil
}

// GenerateForecast generates cost forecast for a service
func (f *CostForecaster) GenerateForecast(service string, horizon time.Duration, storage CostDataStorage) (*CostForecast, error) {
	// Get historical data
	history, err := storage.GetCostHistory(f.config.LookbackPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to get cost history: %w", err)
	}

	if len(history) < 7 {
		return nil, fmt.Errorf("insufficient historical data for forecasting")
	}

	// Extract service costs over time
	var costs []float64
	for _, breakdown := range history {
		if serviceCost, exists := breakdown.ServiceCosts[service]; exists {
			costs = append(costs, serviceCost.Cost)
		} else {
			costs = append(costs, 0)
		}
	}

	// Simple linear regression forecast
	predictedCost := f.linearForecast(costs, horizon)

	// Calculate confidence range (simplified)
	variance := f.calculateVariance(costs)
	stdDev := math.Sqrt(variance)
	confidenceMargin := stdDev * 1.96 // 95% confidence

	return &CostForecast{
		Service:       service,
		Period:        BillingPeriod{Start: time.Now(), End: time.Now().Add(horizon), Type: "forecast"},
		PredictedCost: predictedCost,
		ConfidenceRange: ConfidenceRange{
			Lower:      math.Max(0, predictedCost-confidenceMargin),
			Upper:      predictedCost + confidenceMargin,
			Confidence: f.config.ConfidenceLevel,
		},
		Trend: ForecastTrend{
			Direction:    f.determineTrend(costs),
			Growth:       f.calculateGrowthRate(costs),
			Acceleration: 0.0,
		},
		Model:     "linear",
		Accuracy:  0.85,
		CreatedAt: time.Now(),
	}, nil
}

// linearForecast performs simple linear regression forecasting
func (f *CostForecaster) linearForecast(costs []float64, horizon time.Duration) float64 {
	if len(costs) == 0 {
		return 0
	}

	// Simple linear trend calculation
	n := float64(len(costs))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i, cost := range costs {
		x := float64(i)
		sumX += x
		sumY += cost
		sumXY += x * cost
		sumX2 += x * x
	}

	// Calculate slope and intercept
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	// Project forward based on horizon
	daysForward := horizon.Hours() / 24
	lastIndex := float64(len(costs) - 1)
	predictedIndex := lastIndex + daysForward

	return slope*predictedIndex + intercept
}

// calculateVariance calculates variance of cost data
func (f *CostForecaster) calculateVariance(costs []float64) float64 {
	if len(costs) <= 1 {
		return 0
	}

	mean := 0.0
	for _, cost := range costs {
		mean += cost
	}
	mean /= float64(len(costs))

	variance := 0.0
	for _, cost := range costs {
		diff := cost - mean
		variance += diff * diff
	}

	return variance / float64(len(costs)-1)
}

// determineTrend determines the overall trend direction
func (f *CostForecaster) determineTrend(costs []float64) string {
	if len(costs) < 2 {
		return "stable"
	}

	recent := costs[len(costs)-3:]
	if len(recent) < 2 {
		recent = costs
	}

	sum := 0.0
	for i := 1; i < len(recent); i++ {
		sum += recent[i] - recent[i-1]
	}

	avgChange := sum / float64(len(recent)-1)

	if avgChange > 0.01 {
		return "increasing"
	} else if avgChange < -0.01 {
		return "decreasing"
	}
	return "stable"
}

// calculateGrowthRate calculates the growth rate
func (f *CostForecaster) calculateGrowthRate(costs []float64) float64 {
	if len(costs) < 2 {
		return 0
	}

	first := costs[0]
	last := costs[len(costs)-1]

	if first == 0 {
		return 0
	}

	return ((last - first) / first) * 100
}
