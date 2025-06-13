package ai

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// CostOptimizer provides AI-powered cost optimization recommendations
type CostOptimizer struct {
	config     *OptimizerConfig
	calculator *CostCalculator
	analyzer   *UsageAnalyzer
	logger     Logger
}

// OptimizerConfig configures the cost optimizer
type OptimizerConfig struct {
	Region                    string        `json:"region"`
	Currency                  string        `json:"currency"`
	AnalysisPeriodDays       int           `json:"analysis_period_days"`
	MinimumSavingsThreshold  float64       `json:"minimum_savings_threshold"`
	ConfidenceThreshold      float64       `json:"confidence_threshold"`
	EnablePredictiveAnalysis bool          `json:"enable_predictive_analysis"`
	PredictionHorizonDays    int           `json:"prediction_horizon_days"`
	UpdateInterval           time.Duration `json:"update_interval"`
}

// DefaultOptimizerConfig returns default optimizer configuration
func DefaultOptimizerConfig() *OptimizerConfig {
	return &OptimizerConfig{
		Region:                   "us-east-1",
		Currency:                 "USD",
		AnalysisPeriodDays:      30,
		MinimumSavingsThreshold: 1.0, // $1 minimum savings
		ConfidenceThreshold:     0.8,
		EnablePredictiveAnalysis: true,
		PredictionHorizonDays:   90,
		UpdateInterval:          24 * time.Hour,
	}
}

// CostOptimizationResult contains comprehensive cost analysis and recommendations
type CostOptimizationResult struct {
	AnalysisDate        time.Time               `json:"analysis_date"`
	CurrentCosts        *CostBreakdown          `json:"current_costs"`
	ProjectedCosts      *CostBreakdown          `json:"projected_costs"`
	Recommendations     []OptimizationRec       `json:"recommendations"`
	TotalSavings        float64                 `json:"total_savings"`
	ImplementationRisk  RiskLevel               `json:"implementation_risk"`
	ROI                 float64                 `json:"roi"`
	PaybackPeriodDays   int                     `json:"payback_period_days"`
	Confidence          float64                 `json:"confidence"`
	NextReviewDate      time.Time               `json:"next_review_date"`
}

// CostBreakdown provides detailed cost analysis
type CostBreakdown struct {
	StorageCosts        float64            `json:"storage_costs"`
	TransferCosts       float64            `json:"transfer_costs"`
	RequestCosts        float64            `json:"request_costs"`
	RetrievalCosts      float64            `json:"retrieval_costs"`
	TotalCosts          float64            `json:"total_costs"`
	CostsByStorageClass map[string]float64 `json:"costs_by_storage_class"`
	CostsByRegion       map[string]float64 `json:"costs_by_region"`
	TrendAnalysis       *CostTrend         `json:"trend_analysis"`
}

// CostTrend analyzes cost trends over time
type CostTrend struct {
	Direction           TrendDirection `json:"direction"`
	MonthlyGrowthRate   float64        `json:"monthly_growth_rate"`
	SeasonalPatterns    []float64      `json:"seasonal_patterns"`
	AnomaliesDetected   []CostAnomaly  `json:"anomalies_detected"`
	ProjectedNextMonth  float64        `json:"projected_next_month"`
}

// TrendDirection represents the cost trend direction
type TrendDirection int

const (
	TrendDecreasing TrendDirection = iota
	TrendStable
	TrendIncreasing
)

func (td TrendDirection) String() string {
	switch td {
	case TrendDecreasing:
		return "decreasing"
	case TrendStable:
		return "stable"
	case TrendIncreasing:
		return "increasing"
	default:
		return "unknown"
	}
}

// CostAnomaly represents an unusual cost pattern
type CostAnomaly struct {
	Date        time.Time `json:"date"`
	ExpectedCost float64  `json:"expected_cost"`
	ActualCost   float64  `json:"actual_cost"`
	Deviation    float64  `json:"deviation"`
	Explanation  string   `json:"explanation"`
}

// OptimizationRec represents a cost optimization recommendation
type OptimizationRec struct {
	ID                  string             `json:"id"`
	Type                OptimizationType   `json:"type"`
	Priority            Priority           `json:"priority"`
	Title               string             `json:"title"`
	Description         string             `json:"description"`
	EstimatedSavings    float64            `json:"estimated_savings"`
	AnnualSavings       float64            `json:"annual_savings"`
	ImplementationCost  float64            `json:"implementation_cost"`
	Effort              EffortLevel        `json:"effort"`
	Risk                RiskLevel          `json:"risk"`
	Confidence          float64            `json:"confidence"`
	AffectedResources   []string           `json:"affected_resources"`
	Steps               []string           `json:"steps"`
	Metrics             map[string]float64 `json:"metrics"`
	DeadlineDate        *time.Time         `json:"deadline_date,omitempty"`
	Dependencies        []string           `json:"dependencies,omitempty"`
	AutoImplementable   bool               `json:"auto_implementable"`
}

// OptimizationType represents the type of optimization
type OptimizationType int

const (
	StorageClassOptimization OptimizationType = iota
	LifecycleOptimization
	RegionOptimization
	DeduplicationOptimization
	CompressionOptimization
	CachingOptimization
	ArchivalOptimization
	RequestOptimization
	TransferOptimization
	ReplicationOptimization
)

func (ot OptimizationType) String() string {
	switch ot {
	case StorageClassOptimization:
		return "storage_class"
	case LifecycleOptimization:
		return "lifecycle"
	case RegionOptimization:
		return "region"
	case DeduplicationOptimization:
		return "deduplication"
	case CompressionOptimization:
		return "compression"
	case CachingOptimization:
		return "caching"
	case ArchivalOptimization:
		return "archival"
	case RequestOptimization:
		return "request"
	case TransferOptimization:
		return "transfer"
	case ReplicationOptimization:
		return "replication"
	default:
		return "unknown"
	}
}

// Priority represents recommendation priority
type Priority int

const (
	LowPriority Priority = iota
	MediumPriority
	HighPriority
	CriticalPriority
)

func (p Priority) String() string {
	switch p {
	case LowPriority:
		return "low"
	case MediumPriority:
		return "medium"
	case HighPriority:
		return "high"
	case CriticalPriority:
		return "critical"
	default:
		return "unknown"
	}
}

// EffortLevel represents implementation effort required
type EffortLevel int

const (
	LowEffort EffortLevel = iota
	MediumEffort
	HighEffort
)

func (el EffortLevel) String() string {
	switch el {
	case LowEffort:
		return "low"
	case MediumEffort:
		return "medium"
	case HighEffort:
		return "high"
	default:
		return "unknown"
	}
}

// UsageData contains storage usage statistics
type UsageData struct {
	Buckets          []BucketUsage      `json:"buckets"`
	TotalSize        int64              `json:"total_size"`
	TotalObjects     int64              `json:"total_objects"`
	AccessPatterns   []AccessPattern    `json:"access_patterns"`
	StorageClasses   map[string]int64   `json:"storage_classes"`
	RequestStats     *RequestStatistics `json:"request_stats"`
	TransferStats    *TransferStatistics `json:"transfer_stats"`
	CollectionDate   time.Time          `json:"collection_date"`
}

// BucketUsage contains usage data for a bucket
type BucketUsage struct {
	Name           string            `json:"name"`
	Region         string            `json:"region"`
	Size           int64             `json:"size"`
	ObjectCount    int64             `json:"object_count"`
	StorageClasses map[string]int64  `json:"storage_classes"`
	LastAccessed   time.Time         `json:"last_accessed"`
	AccessFrequency float64          `json:"access_frequency"`
	Tags           map[string]string `json:"tags"`
}

// AccessPattern represents file access patterns
type AccessPattern struct {
	FilePattern    string    `json:"file_pattern"`
	AccessCount    int64     `json:"access_count"`
	LastAccess     time.Time `json:"last_access"`
	AverageSize    int64     `json:"average_size"`
	Frequency      string    `json:"frequency"` // "daily", "weekly", "monthly", "rarely"
}

// RequestStatistics contains request usage statistics
type RequestStatistics struct {
	GETRequests     int64   `json:"get_requests"`
	PUTRequests     int64   `json:"put_requests"`
	DELETERequests  int64   `json:"delete_requests"`
	HEADRequests    int64   `json:"head_requests"`
	LISTRequests    int64   `json:"list_requests"`
	TotalRequests   int64   `json:"total_requests"`
	AverageCost     float64 `json:"average_cost"`
}

// TransferStatistics contains data transfer statistics
type TransferStatistics struct {
	InboundGB      float64 `json:"inbound_gb"`
	OutboundGB     float64 `json:"outbound_gb"`
	InterRegionGB  float64 `json:"inter_region_gb"`
	CloudFrontGB   float64 `json:"cloudfront_gb"`
	TotalTransferCost float64 `json:"total_transfer_cost"`
}

// NewCostOptimizer creates a new cost optimizer
func NewCostOptimizer(config *OptimizerConfig, logger Logger) *CostOptimizer {
	if config == nil {
		config = DefaultOptimizerConfig()
	}

	return &CostOptimizer{
		config:     config,
		calculator: NewCostCalculator(config.Region, config.Currency),
		analyzer:   NewUsageAnalyzer(),
		logger:     logger,
	}
}

// AnalyzeAndOptimize performs comprehensive cost analysis and generates optimization recommendations
func (co *CostOptimizer) AnalyzeAndOptimize(ctx context.Context, usageData *UsageData) (*CostOptimizationResult, error) {
	co.logger.Info("Starting cost optimization analysis")

	result := &CostOptimizationResult{
		AnalysisDate:   time.Now(),
		Recommendations: make([]OptimizationRec, 0),
	}

	// Calculate current costs
	currentCosts, err := co.calculator.CalculateCurrentCosts(usageData)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate current costs: %w", err)
	}
	result.CurrentCosts = currentCosts

	// Generate optimization recommendations
	recommendations := co.generateRecommendations(usageData, currentCosts)
	result.Recommendations = recommendations

	// Calculate total potential savings
	totalSavings := 0.0
	for _, rec := range recommendations {
		if rec.Confidence >= co.config.ConfidenceThreshold {
			totalSavings += rec.AnnualSavings
		}
	}
	result.TotalSavings = totalSavings

	// Calculate projected costs after optimization
	projectedCosts := co.calculateProjectedCosts(currentCosts, recommendations)
	result.ProjectedCosts = projectedCosts

	// Calculate ROI and implementation metrics
	result.ROI = co.calculateROI(totalSavings, recommendations)
	result.PaybackPeriodDays = co.calculatePaybackPeriod(totalSavings, recommendations)
	result.ImplementationRisk = co.assessImplementationRisk(recommendations)
	result.Confidence = co.calculateOverallConfidence(recommendations)

	// Set next review date
	result.NextReviewDate = time.Now().Add(co.config.UpdateInterval)

	// Sort recommendations by priority and savings
	sort.Slice(result.Recommendations, func(i, j int) bool {
		if result.Recommendations[i].Priority != result.Recommendations[j].Priority {
			return result.Recommendations[i].Priority > result.Recommendations[j].Priority
		}
		return result.Recommendations[i].AnnualSavings > result.Recommendations[j].AnnualSavings
	})

	co.logger.Info("Cost optimization analysis completed. Total potential savings: $%.2f", totalSavings)
	return result, nil
}

// generateRecommendations generates optimization recommendations based on usage data
func (co *CostOptimizer) generateRecommendations(usageData *UsageData, currentCosts *CostBreakdown) []OptimizationRec {
	var recommendations []OptimizationRec

	// Storage class optimization
	recommendations = append(recommendations, co.analyzeStorageClassOptimization(usageData)...)

	// Lifecycle optimization
	recommendations = append(recommendations, co.analyzeLifecycleOptimization(usageData)...)

	// Deduplication opportunities
	recommendations = append(recommendations, co.analyzeDeduplicationOpportunities(usageData)...)

	// Request optimization
	recommendations = append(recommendations, co.analyzeRequestOptimization(usageData)...)

	// Transfer optimization
	recommendations = append(recommendations, co.analyzeTransferOptimization(usageData)...)

	// Regional optimization
	recommendations = append(recommendations, co.analyzeRegionalOptimization(usageData)...)

	// Archival opportunities
	recommendations = append(recommendations, co.analyzeArchivalOpportunities(usageData)...)

	return recommendations
}

// analyzeStorageClassOptimization analyzes storage class optimization opportunities
func (co *CostOptimizer) analyzeStorageClassOptimization(usageData *UsageData) []OptimizationRec {
	var recommendations []OptimizationRec

	for _, bucket := range usageData.Buckets {
		// Analyze each storage class
		for storageClass, size := range bucket.StorageClasses {
			if storageClass == "STANDARD" && bucket.AccessFrequency < 0.1 { // Accessed less than once per 10 days
				// Recommend Standard-IA
				savings := co.calculator.CalculateStorageClassSavings(size, "STANDARD", "STANDARD_IA")
				
				if savings*12 > co.config.MinimumSavingsThreshold { // Annual savings
					recommendations = append(recommendations, OptimizationRec{
						ID:                fmt.Sprintf("storage-class-%s-%s", bucket.Name, "standard-ia"),
						Type:              StorageClassOptimization,
						Priority:          MediumPriority,
						Title:             fmt.Sprintf("Optimize storage class for %s", bucket.Name),
						Description:       fmt.Sprintf("Move %d GB to Standard-IA due to low access frequency", size/(1024*1024*1024)),
						EstimatedSavings:  savings,
						AnnualSavings:     savings * 12,
						ImplementationCost: 0,
						Effort:            LowEffort,
						Risk:              LowRisk,
						Confidence:        0.9,
						AffectedResources: []string{bucket.Name},
						Steps: []string{
							"Review access patterns for confirmation",
							"Configure lifecycle policy to transition objects to Standard-IA after 30 days",
							"Monitor access patterns after implementation",
						},
						AutoImplementable: true,
					})
				}
			}

			if storageClass == "STANDARD_IA" && bucket.AccessFrequency < 0.03 { // Less than once per month
				// Recommend Glacier
				savings := co.calculator.CalculateStorageClassSavings(size, "STANDARD_IA", "GLACIER")
				
				if savings*12 > co.config.MinimumSavingsThreshold {
					recommendations = append(recommendations, OptimizationRec{
						ID:                fmt.Sprintf("storage-class-%s-%s", bucket.Name, "glacier"),
						Type:              StorageClassOptimization,
						Priority:          HighPriority,
						Title:             fmt.Sprintf("Archive rarely accessed data in %s", bucket.Name),
						Description:       fmt.Sprintf("Move %d GB to Glacier for significant cost savings", size/(1024*1024*1024)),
						EstimatedSavings:  savings,
						AnnualSavings:     savings * 12,
						ImplementationCost: 0,
						Effort:            LowEffort,
						Risk:              MediumRisk,
						Confidence:        0.85,
						AffectedResources: []string{bucket.Name},
						Steps: []string{
							"Analyze retrieval requirements",
							"Configure lifecycle policy to transition objects to Glacier after 90 days",
							"Set up retrieval notifications if needed",
						},
						AutoImplementable: false, // Requires review of retrieval requirements
					})
				}
			}
		}
	}

	return recommendations
}

// analyzeLifecycleOptimization analyzes lifecycle policy optimization opportunities
func (co *CostOptimizer) analyzeLifecycleOptimization(usageData *UsageData) []OptimizationRec {
	var recommendations []OptimizationRec

	for _, bucket := range usageData.Buckets {
		// Check if lifecycle policies are optimally configured
		if co.needsLifecycleOptimization(bucket) {
			savings := co.estimateLifecycleSavings(bucket)
			
			recommendations = append(recommendations, OptimizationRec{
				ID:                fmt.Sprintf("lifecycle-%s", bucket.Name),
				Type:              LifecycleOptimization,
				Priority:          HighPriority,
				Title:             fmt.Sprintf("Optimize lifecycle policies for %s", bucket.Name),
				Description:       "Implement comprehensive lifecycle policies to automatically transition objects through storage classes",
				EstimatedSavings:  savings / 12, // Monthly savings
				AnnualSavings:     savings,
				ImplementationCost: 0,
				Effort:            MediumEffort,
				Risk:              LowRisk,
				Confidence:        0.8,
				AffectedResources: []string{bucket.Name},
				Steps: []string{
					"Analyze object access patterns",
					"Design optimal lifecycle transitions",
					"Implement lifecycle policies with proper filtering",
					"Monitor and adjust based on access patterns",
				},
				AutoImplementable: false,
			})
		}
	}

	return recommendations
}

// analyzeDeduplicationOpportunities analyzes deduplication opportunities
func (co *CostOptimizer) analyzeDeduplicationOpportunities(usageData *UsageData) []OptimizationRec {
	var recommendations []OptimizationRec

	// Estimate duplicate data percentage (simplified)
	estimatedDuplicatePercentage := 0.15 // Assume 15% duplicates
	totalSizeGB := float64(usageData.TotalSize) / (1024 * 1024 * 1024)
	duplicateGB := totalSizeGB * estimatedDuplicatePercentage
	
	if duplicateGB > 10 { // More than 10GB of potential duplicates
		monthlySavings := duplicateGB * co.calculator.GetStorageCostPerGB("STANDARD")
		
		recommendations = append(recommendations, OptimizationRec{
			ID:                "deduplication-analysis",
			Type:              DeduplicationOptimization,
			Priority:          MediumPriority,
			Title:             "Eliminate duplicate files",
			Description:       fmt.Sprintf("Estimated %.1f GB of duplicate data could be removed", duplicateGB),
			EstimatedSavings:  monthlySavings,
			AnnualSavings:     monthlySavings * 12,
			ImplementationCost: 0,
			Effort:            MediumEffort,
			Risk:              MediumRisk,
			Confidence:        0.7,
			AffectedResources: []string{"all-buckets"},
			Steps: []string{
				"Run comprehensive duplicate analysis",
				"Review identified duplicates manually",
				"Implement automated deduplication for new uploads",
				"Clean up existing duplicates safely",
			},
			Metrics: map[string]float64{
				"estimated_duplicate_gb": duplicateGB,
				"estimated_savings_per_gb": co.calculator.GetStorageCostPerGB("STANDARD"),
			},
			AutoImplementable: false,
		})
	}

	return recommendations
}

// analyzeRequestOptimization analyzes request optimization opportunities
func (co *CostOptimizer) analyzeRequestOptimization(usageData *UsageData) []OptimizationRec {
	var recommendations []OptimizationRec

	if usageData.RequestStats != nil {
		// Check for excessive LIST requests
		if usageData.RequestStats.LISTRequests > 100000 { // More than 100k LIST requests
			monthlySavings := float64(usageData.RequestStats.LISTRequests) * 0.0005 // $0.0005 per 1000 requests
			
			recommendations = append(recommendations, OptimizationRec{
				ID:                "optimize-list-requests",
				Type:              RequestOptimization,
				Priority:          MediumPriority,
				Title:             "Optimize LIST request patterns",
				Description:       fmt.Sprintf("High volume of LIST requests detected (%d monthly)", usageData.RequestStats.LISTRequests),
				EstimatedSavings:  monthlySavings * 0.3, // Assume 30% reduction possible
				AnnualSavings:     monthlySavings * 0.3 * 12,
				ImplementationCost: 100, // Development cost
				Effort:            MediumEffort,
				Risk:              LowRisk,
				Confidence:        0.75,
				AffectedResources: []string{"application-layer"},
				Steps: []string{
					"Analyze LIST request patterns in application",
					"Implement caching for frequently accessed object lists",
					"Use pagination efficiently",
					"Consider alternative architectures to reduce LIST operations",
				},
				AutoImplementable: false,
			})
		}
	}

	return recommendations
}

// analyzeTransferOptimization analyzes data transfer optimization opportunities
func (co *CostOptimizer) analyzeTransferOptimization(usageData *UsageData) []OptimizationRec {
	var recommendations []OptimizationRec

	if usageData.TransferStats != nil {
		// Check for high outbound transfer costs
		if usageData.TransferStats.OutboundGB > 1000 { // More than 1TB outbound
			transferCostSavings := usageData.TransferStats.OutboundGB * 0.05 // Assume $0.05 savings per GB with CloudFront
			
			recommendations = append(recommendations, OptimizationRec{
				ID:                "optimize-data-transfer",
				Type:              TransferOptimization,
				Priority:          HighPriority,
				Title:             "Optimize data transfer costs with CloudFront",
				Description:       fmt.Sprintf("High outbound transfer volume detected (%.1f GB monthly)", usageData.TransferStats.OutboundGB),
				EstimatedSavings:  transferCostSavings,
				AnnualSavings:     transferCostSavings * 12,
				ImplementationCost: 200, // Setup and configuration cost
				Effort:            MediumEffort,
				Risk:              LowRisk,
				Confidence:        0.85,
				AffectedResources: []string{"cloudfront", "s3-buckets"},
				Steps: []string{
					"Set up CloudFront distribution",
					"Configure origin access identity",
					"Update application to use CloudFront URLs",
					"Monitor transfer cost reduction",
				},
				AutoImplementable: false,
			})
		}
	}

	return recommendations
}

// analyzeRegionalOptimization analyzes regional optimization opportunities
func (co *CostOptimizer) analyzeRegionalOptimization(usageData *UsageData) []OptimizationRec {
	var recommendations []OptimizationRec

	// Group buckets by region and analyze costs
	regionCosts := make(map[string]float64)
	regionSizes := make(map[string]int64)

	for _, bucket := range usageData.Buckets {
		regionCosts[bucket.Region] += co.calculator.CalculateStorageCost(bucket.Size, "STANDARD")
		regionSizes[bucket.Region] += bucket.Size
	}

	// Check if consolidating regions would save money
	if len(regionCosts) > 2 { // Multiple regions in use
		// Find the most cost-effective region
		cheapestRegion := ""
		lowestCost := math.Inf(1)
		
		for region, cost := range regionCosts {
			costPerGB := cost / float64(regionSizes[region]/(1024*1024*1024))
			if costPerGB < lowestCost {
				lowestCost = costPerGB
				cheapestRegion = region
			}
		}

		// Calculate potential savings from consolidation
		totalCurrentCost := 0.0
		for _, cost := range regionCosts {
			totalCurrentCost += cost
		}

		totalSize := int64(0)
		for _, size := range regionSizes {
			totalSize += size
		}

		consolidatedCost := co.calculator.CalculateStorageCost(totalSize, "STANDARD")
		monthlySavings := totalCurrentCost - consolidatedCost

		if monthlySavings > co.config.MinimumSavingsThreshold {
			recommendations = append(recommendations, OptimizationRec{
				ID:                "regional-consolidation",
				Type:              RegionOptimization,
				Priority:          LowPriority,
				Title:             fmt.Sprintf("Consider consolidating to %s region", cheapestRegion),
				Description:       "Multiple regions detected with potential cost savings through consolidation",
				EstimatedSavings:  monthlySavings,
				AnnualSavings:     monthlySavings * 12,
				ImplementationCost: 500, // Migration costs
				Effort:            HighEffort,
				Risk:              HighRisk,
				Confidence:        0.6,
				AffectedResources: []string{"multiple-regions"},
				Steps: []string{
					"Analyze data transfer patterns and latency requirements",
					"Calculate comprehensive migration costs",
					"Plan migration strategy with minimal downtime",
					"Consider compliance and data sovereignty requirements",
				},
				AutoImplementable: false,
			})
		}
	}

	return recommendations
}

// analyzeArchivalOpportunities analyzes long-term archival opportunities
func (co *CostOptimizer) analyzeArchivalOpportunities(usageData *UsageData) []OptimizationRec {
	var recommendations []OptimizationRec

	for _, bucket := range usageData.Buckets {
		// Check for old data that could be archived
		daysSinceLastAccess := time.Since(bucket.LastAccessed).Hours() / 24
		
		if daysSinceLastAccess > 365 && bucket.Size > 10*1024*1024*1024 { // 1 year old, >10GB
			currentCost := co.calculator.CalculateStorageCost(bucket.Size, "STANDARD")
			deepArchiveCost := co.calculator.CalculateStorageCost(bucket.Size, "DEEP_ARCHIVE")
			monthlySavings := currentCost - deepArchiveCost
			
			recommendations = append(recommendations, OptimizationRec{
				ID:                fmt.Sprintf("deep-archive-%s", bucket.Name),
				Type:              ArchivalOptimization,
				Priority:          HighPriority,
				Title:             fmt.Sprintf("Archive old data in %s to Deep Archive", bucket.Name),
				Description:       fmt.Sprintf("Data not accessed for %.0f days - excellent candidate for Deep Archive", daysSinceLastAccess),
				EstimatedSavings:  monthlySavings,
				AnnualSavings:     monthlySavings * 12,
				ImplementationCost: 0,
				Effort:            LowEffort,
				Risk:              MediumRisk,
				Confidence:        0.9,
				AffectedResources: []string{bucket.Name},
				Steps: []string{
					"Verify data retention requirements",
					"Configure lifecycle policy for Deep Archive transition",
					"Set up retrieval process documentation",
					"Implement monitoring for any access attempts",
				},
				Metrics: map[string]float64{
					"days_since_access": daysSinceLastAccess,
					"size_gb":          float64(bucket.Size) / (1024 * 1024 * 1024),
				},
				AutoImplementable: false,
			})
		}
	}

	return recommendations
}

// Helper methods

func (co *CostOptimizer) needsLifecycleOptimization(bucket BucketUsage) bool {
	// Simplified logic - in reality would check existing lifecycle policies
	return bucket.Size > 1024*1024*1024 && bucket.AccessFrequency < 0.5 // >1GB and low access
}

func (co *CostOptimizer) estimateLifecycleSavings(bucket BucketUsage) float64 {
	// Simplified calculation
	currentCost := co.calculator.CalculateStorageCost(bucket.Size, "STANDARD") * 12
	optimizedCost := currentCost * 0.7 // Assume 30% savings with proper lifecycle
	return currentCost - optimizedCost
}

func (co *CostOptimizer) calculateProjectedCosts(current *CostBreakdown, recommendations []OptimizationRec) *CostBreakdown {
	projected := *current // Copy current costs
	
	for _, rec := range recommendations {
		if rec.Confidence >= co.config.ConfidenceThreshold {
			projected.TotalCosts -= rec.EstimatedSavings
		}
	}
	
	return &projected
}

func (co *CostOptimizer) calculateROI(totalSavings float64, recommendations []OptimizationRec) float64 {
	totalImplementationCost := 0.0
	for _, rec := range recommendations {
		totalImplementationCost += rec.ImplementationCost
	}
	
	if totalImplementationCost == 0 {
		return math.Inf(1) // Infinite ROI
	}
	
	return (totalSavings * 12) / totalImplementationCost // Annual ROI
}

func (co *CostOptimizer) calculatePaybackPeriod(totalSavings float64, recommendations []OptimizationRec) int {
	totalImplementationCost := 0.0
	for _, rec := range recommendations {
		totalImplementationCost += rec.ImplementationCost
	}
	
	if totalSavings == 0 {
		return 0
	}
	
	return int(totalImplementationCost / totalSavings * 30) // Days to payback
}

func (co *CostOptimizer) assessImplementationRisk(recommendations []OptimizationRec) RiskLevel {
	highRiskCount := 0
	totalCount := len(recommendations)
	
	for _, rec := range recommendations {
		if rec.Risk == HighRisk {
			highRiskCount++
		}
	}
	
	if float64(highRiskCount)/float64(totalCount) > 0.3 {
		return HighRisk
	} else if float64(highRiskCount)/float64(totalCount) > 0.1 {
		return MediumRisk
	}
	
	return LowRisk
}

func (co *CostOptimizer) calculateOverallConfidence(recommendations []OptimizationRec) float64 {
	if len(recommendations) == 0 {
		return 0.0
	}
	
	totalConfidence := 0.0
	for _, rec := range recommendations {
		totalConfidence += rec.Confidence
	}
	
	return totalConfidence / float64(len(recommendations))
}

// CostCalculator handles cost calculations
type CostCalculator struct {
	region   string
	currency string
	rates    map[string]float64
}

// NewCostCalculator creates a new cost calculator
func NewCostCalculator(region, currency string) *CostCalculator {
	// Simplified pricing - in reality would fetch from AWS API
	rates := map[string]float64{
		"STANDARD":      0.023,  // per GB per month
		"STANDARD_IA":   0.0125, // per GB per month
		"GLACIER":       0.004,  // per GB per month
		"DEEP_ARCHIVE":  0.00099, // per GB per month
	}

	return &CostCalculator{
		region:   region,
		currency: currency,
		rates:    rates,
	}
}

func (cc *CostCalculator) CalculateCurrentCosts(usageData *UsageData) (*CostBreakdown, error) {
	breakdown := &CostBreakdown{
		CostsByStorageClass: make(map[string]float64),
		CostsByRegion:       make(map[string]float64),
	}

	for _, bucket := range usageData.Buckets {
		for storageClass, size := range bucket.StorageClasses {
			cost := cc.CalculateStorageCost(size, storageClass)
			breakdown.StorageCosts += cost
			breakdown.CostsByStorageClass[storageClass] += cost
			breakdown.CostsByRegion[bucket.Region] += cost
		}
	}

	// Add request and transfer costs (simplified)
	if usageData.RequestStats != nil {
		breakdown.RequestCosts = float64(usageData.RequestStats.TotalRequests) * 0.0004 / 1000 // $0.0004 per 1000 requests
	}

	if usageData.TransferStats != nil {
		breakdown.TransferCosts = usageData.TransferStats.OutboundGB * 0.09 // $0.09 per GB
	}

	breakdown.TotalCosts = breakdown.StorageCosts + breakdown.RequestCosts + breakdown.TransferCosts + breakdown.RetrievalCosts

	return breakdown, nil
}

func (cc *CostCalculator) CalculateStorageCost(sizeBytes int64, storageClass string) float64 {
	sizeGB := float64(sizeBytes) / (1024 * 1024 * 1024)
	rate, exists := cc.rates[storageClass]
	if !exists {
		rate = cc.rates["STANDARD"] // Default to standard rate
	}
	return sizeGB * rate
}

func (cc *CostCalculator) CalculateStorageClassSavings(sizeBytes int64, fromClass, toClass string) float64 {
	currentCost := cc.CalculateStorageCost(sizeBytes, fromClass)
	newCost := cc.CalculateStorageCost(sizeBytes, toClass)
	return currentCost - newCost
}

func (cc *CostCalculator) GetStorageCostPerGB(storageClass string) float64 {
	rate, exists := cc.rates[storageClass]
	if !exists {
		return cc.rates["STANDARD"]
	}
	return rate
}

// UsageAnalyzer analyzes usage patterns
type UsageAnalyzer struct{}

func NewUsageAnalyzer() *UsageAnalyzer {
	return &UsageAnalyzer{}
}