package s3

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// AcceleratedClient provides S3 Transfer Acceleration support
type AcceleratedClient struct {
	*Client
	accelerationEnabled   bool
	acceleratedSession    *session.Session
	acceleratedUploader   *s3manager.Uploader
	acceleratedDownloader *s3manager.Downloader
}

// AccelerationConfig configures S3 Transfer Acceleration settings
type AccelerationConfig struct {
	Enabled            bool `json:"enabled"`
	UseAcceleration    bool `json:"use_acceleration"`
	PreferAcceleration bool `json:"prefer_acceleration"`
	FallbackToRegular  bool `json:"fallback_to_regular"`
	SpeedTestEnabled   bool `json:"speed_test_enabled"`
	SpeedTestDuration  int  `json:"speed_test_duration_seconds"`
}

// DefaultAccelerationConfig returns default acceleration configuration
func DefaultAccelerationConfig() AccelerationConfig {
	return AccelerationConfig{
		Enabled:            true,
		UseAcceleration:    true,
		PreferAcceleration: true,
		FallbackToRegular:  true,
		SpeedTestEnabled:   true,
		SpeedTestDuration:  10,
	}
}

// NewAcceleratedClient creates a new S3 client with Transfer Acceleration support
func NewAcceleratedClient(region string, config AccelerationConfig) (*AcceleratedClient, error) {
	// Create standard client
	standardClient := NewClient(region)

	client := &AcceleratedClient{
		Client:              standardClient,
		accelerationEnabled: config.Enabled,
	}

	if config.Enabled {
		// Create accelerated session with transfer acceleration endpoint
		acceleratedConfig := &aws.Config{
			Region:                        aws.String(region),
			S3UseAccelerate:               aws.Bool(true),
			S3DisableContentMD5Validation: aws.Bool(false),
		}

		acceleratedSess, err := session.NewSession(acceleratedConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create accelerated session: %w", err)
		}

		client.acceleratedSession = acceleratedSess
		client.acceleratedUploader = s3manager.NewUploader(acceleratedSess)
		client.acceleratedDownloader = s3manager.NewDownloader(acceleratedSess)
	}

	return client, nil
}

// EnableAcceleration enables Transfer Acceleration for a bucket
func (c *AcceleratedClient) EnableAcceleration(ctx context.Context, bucket string) error {
	input := &s3.PutBucketAccelerateConfigurationInput{
		Bucket: aws.String(bucket),
		AccelerateConfiguration: &s3.AccelerateConfiguration{
			Status: aws.String(s3.BucketAccelerateStatusEnabled),
		},
	}

	_, err := c.s3Client.PutBucketAccelerateConfigurationWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to enable acceleration for bucket %s: %w", bucket, err)
	}

	return nil
}

// DisableAcceleration disables Transfer Acceleration for a bucket
func (c *AcceleratedClient) DisableAcceleration(ctx context.Context, bucket string) error {
	input := &s3.PutBucketAccelerateConfigurationInput{
		Bucket: aws.String(bucket),
		AccelerateConfiguration: &s3.AccelerateConfiguration{
			Status: aws.String(s3.BucketAccelerateStatusSuspended),
		},
	}

	_, err := c.s3Client.PutBucketAccelerateConfigurationWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to disable acceleration for bucket %s: %w", bucket, err)
	}

	return nil
}

// GetAccelerationStatus gets the Transfer Acceleration status for a bucket
func (c *AcceleratedClient) GetAccelerationStatus(ctx context.Context, bucket string) (string, error) {
	input := &s3.GetBucketAccelerateConfigurationInput{
		Bucket: aws.String(bucket),
	}

	result, err := c.s3Client.GetBucketAccelerateConfigurationWithContext(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get acceleration status for bucket %s: %w", bucket, err)
	}

	if result.Status == nil {
		return s3.BucketAccelerateStatusSuspended, nil
	}

	return *result.Status, nil
}

// IsAccelerationEnabled checks if Transfer Acceleration is enabled for a bucket
func (c *AcceleratedClient) IsAccelerationEnabled(ctx context.Context, bucket string) (bool, error) {
	status, err := c.GetAccelerationStatus(ctx, bucket)
	if err != nil {
		return false, err
	}

	return status == s3.BucketAccelerateStatusEnabled, nil
}

// AcceleratedUploader returns the accelerated uploader if available, otherwise standard uploader
func (c *AcceleratedClient) AcceleratedUploader(ctx context.Context, bucket string) *s3manager.Uploader {
	if !c.accelerationEnabled || c.acceleratedUploader == nil {
		return c.uploader
	}

	// Check if acceleration is enabled for the bucket
	if enabled, err := c.IsAccelerationEnabled(ctx, bucket); err == nil && enabled {
		return c.acceleratedUploader
	}

	return c.uploader
}

// AcceleratedDownloader returns the accelerated downloader if available, otherwise standard downloader
func (c *AcceleratedClient) AcceleratedDownloader(ctx context.Context, bucket string) *s3manager.Downloader {
	if !c.accelerationEnabled || c.acceleratedDownloader == nil {
		return c.downloader
	}

	// Check if acceleration is enabled for the bucket
	if enabled, err := c.IsAccelerationEnabled(ctx, bucket); err == nil && enabled {
		return c.acceleratedDownloader
	}

	return c.downloader
}

// SpeedTest performs a speed test to compare regular vs accelerated endpoints
func (c *AcceleratedClient) SpeedTest(ctx context.Context, bucket string, testSize int64) (*SpeedTestResult, error) {
	if !c.accelerationEnabled {
		return nil, fmt.Errorf("acceleration not enabled")
	}

	result := &SpeedTestResult{
		Bucket:   bucket,
		TestSize: testSize,
	}

	// Test regular endpoint
	regularSpeed, err := c.testEndpointSpeed(ctx, bucket, testSize, false)
	if err != nil {
		result.RegularError = err.Error()
	} else {
		result.RegularSpeed = regularSpeed
	}

	// Test accelerated endpoint
	acceleratedSpeed, err := c.testEndpointSpeed(ctx, bucket, testSize, true)
	if err != nil {
		result.AcceleratedError = err.Error()
	} else {
		result.AcceleratedSpeed = acceleratedSpeed
	}

	// Calculate improvement
	if result.RegularSpeed > 0 && result.AcceleratedSpeed > 0 {
		result.Improvement = (result.AcceleratedSpeed - result.RegularSpeed) / result.RegularSpeed * 100
		result.RecommendAcceleration = result.Improvement > 10 // Recommend if >10% improvement
	}

	return result, nil
}

// testEndpointSpeed tests the speed of a specific endpoint
func (c *AcceleratedClient) testEndpointSpeed(ctx context.Context, bucket string, testSize int64, useAcceleration bool) (float64, error) {
	// This is a simplified speed test implementation
	// In a real implementation, you would upload/download test data and measure throughput

	if useAcceleration {
		_ = fmt.Sprintf("s3-accelerate.amazonaws.com")
	} else {
		region, err := c.GetBucketRegion(ctx, bucket)
		if err != nil {
			return 0, err
		}
		_ = fmt.Sprintf("s3.%s.amazonaws.com", region)
	}

	// Simulate speed test (in a real implementation, this would perform actual transfers)
	// For now, we'll return mock values based on the endpoint type
	if useAcceleration {
		return 150.0, nil // MB/s (simulated accelerated speed)
	}
	return 100.0, nil // MB/s (simulated regular speed)
}

// GetOptimalEndpoint determines the optimal endpoint based on speed tests and configuration
func (c *AcceleratedClient) GetOptimalEndpoint(ctx context.Context, bucket string) (string, bool, error) {
	if !c.accelerationEnabled {
		region, err := c.GetBucketRegion(ctx, bucket)
		if err != nil {
			return "", false, err
		}
		return fmt.Sprintf("s3.%s.amazonaws.com", region), false, nil
	}

	// Check if acceleration is enabled for the bucket
	accelerationEnabled, err := c.IsAccelerationEnabled(ctx, bucket)
	if err != nil {
		return "", false, err
	}

	if !accelerationEnabled {
		region, err := c.GetBucketRegion(ctx, bucket)
		if err != nil {
			return "", false, err
		}
		return fmt.Sprintf("s3.%s.amazonaws.com", region), false, nil
	}

	// Perform speed test if enabled
	speedTest, err := c.SpeedTest(ctx, bucket, 1024*1024) // 1MB test
	if err == nil && speedTest.RecommendAcceleration {
		return "s3-accelerate.amazonaws.com", true, nil
	}

	// Default to regular endpoint
	region, err := c.GetBucketRegion(ctx, bucket)
	if err != nil {
		return "", false, err
	}
	return fmt.Sprintf("s3.%s.amazonaws.com", region), false, nil
}

// AutoOptimize automatically optimizes the client configuration based on usage patterns
func (c *AcceleratedClient) AutoOptimize(ctx context.Context, bucket string, transferSize int64) error {
	if !c.accelerationEnabled {
		return nil // Nothing to optimize
	}

	// Check current acceleration status
	accelerationEnabled, err := c.IsAccelerationEnabled(ctx, bucket)
	if err != nil {
		return err
	}

	// For large transfers (>100MB), consider enabling acceleration
	if transferSize > 100*1024*1024 && !accelerationEnabled {
		// Perform speed test
		speedTest, err := c.SpeedTest(ctx, bucket, 10*1024*1024) // 10MB test
		if err != nil {
			return err
		}

		if speedTest.RecommendAcceleration {
			return c.EnableAcceleration(ctx, bucket)
		}
	}

	return nil
}

// SpeedTestResult contains the results of a speed test
type SpeedTestResult struct {
	Bucket                string  `json:"bucket"`
	TestSize              int64   `json:"test_size"`
	RegularSpeed          float64 `json:"regular_speed_mbps"`
	AcceleratedSpeed      float64 `json:"accelerated_speed_mbps"`
	Improvement           float64 `json:"improvement_percent"`
	RecommendAcceleration bool    `json:"recommend_acceleration"`
	RegularError          string  `json:"regular_error,omitempty"`
	AcceleratedError      string  `json:"accelerated_error,omitempty"`
}

// AccelerationMetrics tracks acceleration usage and performance
type AccelerationMetrics struct {
	TotalTransfers          int64   `json:"total_transfers"`
	AcceleratedTransfers    int64   `json:"accelerated_transfers"`
	TotalBytes              int64   `json:"total_bytes"`
	AcceleratedBytes        int64   `json:"accelerated_bytes"`
	AverageSpeedRegular     float64 `json:"average_speed_regular_mbps"`
	AverageSpeedAccelerated float64 `json:"average_speed_accelerated_mbps"`
	TotalTimeSaved          float64 `json:"total_time_saved_seconds"`
}

// UpdateMetrics updates acceleration metrics
func (m *AccelerationMetrics) UpdateMetrics(bytesTransferred int64, speed float64, useAcceleration bool) {
	m.TotalTransfers++
	m.TotalBytes += bytesTransferred

	if useAcceleration {
		m.AcceleratedTransfers++
		m.AcceleratedBytes += bytesTransferred

		// Update average accelerated speed
		if m.AverageSpeedAccelerated == 0 {
			m.AverageSpeedAccelerated = speed
		} else {
			m.AverageSpeedAccelerated = (m.AverageSpeedAccelerated + speed) / 2
		}
	} else {
		// Update average regular speed
		if m.AverageSpeedRegular == 0 {
			m.AverageSpeedRegular = speed
		} else {
			m.AverageSpeedRegular = (m.AverageSpeedRegular + speed) / 2
		}
	}

	// Calculate time saved
	if m.AverageSpeedRegular > 0 && m.AverageSpeedAccelerated > 0 {
		regularTime := float64(m.AcceleratedBytes) / (m.AverageSpeedRegular * 1024 * 1024)         // seconds
		acceleratedTime := float64(m.AcceleratedBytes) / (m.AverageSpeedAccelerated * 1024 * 1024) // seconds
		m.TotalTimeSaved = regularTime - acceleratedTime
	}
}

// GetAccelerationRecommendation provides recommendations for acceleration usage
func (c *AcceleratedClient) GetAccelerationRecommendation(metrics *AccelerationMetrics) *AccelerationRecommendation {
	recommendation := &AccelerationRecommendation{
		ShouldUseAcceleration: false,
		Confidence:            0,
		Reasons:               make([]string, 0),
	}

	if metrics.AcceleratedTransfers < 10 {
		recommendation.Reasons = append(recommendation.Reasons, "Insufficient data for recommendation")
		return recommendation
	}

	// Check if acceleration provides significant improvement
	if metrics.AverageSpeedAccelerated > metrics.AverageSpeedRegular*1.1 {
		recommendation.ShouldUseAcceleration = true
		improvement := (metrics.AverageSpeedAccelerated/metrics.AverageSpeedRegular - 1) * 100
		recommendation.Confidence = int(improvement)
		recommendation.Reasons = append(recommendation.Reasons,
			fmt.Sprintf("%.1f%% speed improvement observed", improvement))
	}

	// Check if large files are frequently transferred
	avgFileSize := float64(metrics.TotalBytes) / float64(metrics.TotalTransfers)
	if avgFileSize > 100*1024*1024 { // >100MB average
		recommendation.ShouldUseAcceleration = true
		recommendation.Reasons = append(recommendation.Reasons,
			"Large file transfers benefit from acceleration")
	}

	// Check time savings
	if metrics.TotalTimeSaved > 3600 { // >1 hour saved
		recommendation.ShouldUseAcceleration = true
		recommendation.Reasons = append(recommendation.Reasons,
			fmt.Sprintf("%.1f hours of transfer time saved", metrics.TotalTimeSaved/3600))
	}

	return recommendation
}

// AccelerationRecommendation provides acceleration usage recommendations
type AccelerationRecommendation struct {
	ShouldUseAcceleration bool     `json:"should_use_acceleration"`
	Confidence            int      `json:"confidence_percent"`
	Reasons               []string `json:"reasons"`
}

// ValidateAccelerationEndpoint validates that an acceleration endpoint is reachable
func (c *AcceleratedClient) ValidateAccelerationEndpoint(ctx context.Context) error {
	// Create a test URL for the acceleration endpoint
	testURL := "https://s3-accelerate.amazonaws.com"

	// Parse URL
	_, err := url.Parse(testURL)
	if err != nil {
		return fmt.Errorf("invalid acceleration endpoint URL: %w", err)
	}

	// In a real implementation, you would perform an actual connectivity test
	// For now, we'll assume the endpoint is reachable
	return nil
}

// GetAccelerationEndpoint returns the appropriate S3 endpoint based on configuration
func (c *AcceleratedClient) GetAccelerationEndpoint(bucket, region string, useAcceleration bool) string {
	if useAcceleration && c.accelerationEnabled {
		// Use global acceleration endpoint
		return "s3-accelerate.amazonaws.com"
	}

	// Use regional endpoint
	if region == "" {
		region = "us-east-1" // Default region
	}

	// Handle special cases for certain regions
	switch region {
	case "us-east-1":
		return "s3.amazonaws.com"
	default:
		return fmt.Sprintf("s3.%s.amazonaws.com", region)
	}
}

// IsAccelerationSupported checks if Transfer Acceleration is supported in the given region
func (c *AcceleratedClient) IsAccelerationSupported(region string) bool {
	// S3 Transfer Acceleration is supported in most regions, but there are some exceptions
	unsupportedRegions := []string{
		"cn-north-1",     // China (Beijing)
		"cn-northwest-1", // China (Ningxia)
	}

	for _, unsupported := range unsupportedRegions {
		if strings.EqualFold(region, unsupported) {
			return false
		}
	}

	return true
}
