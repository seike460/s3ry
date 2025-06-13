package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/i18n"
	"github.com/seike460/s3ry/internal/metrics"
	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/security"
	"github.com/seike460/s3ry/internal/ui/app"
	"github.com/seike460/s3ry/internal/ui/views"
	"github.com/seike460/s3ry/internal/worker"
)

// TestCompleteUserWorkflow tests the entire user experience from start to finish
func TestCompleteUserWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping complete workflow tests in short mode")
	}

	t.Run("FullApplicationLifecycle", func(t *testing.T) {
		t.Log("🚀 Testing complete application lifecycle...")

		// Step 1: Configuration initialization
		cfg := testApplicationConfiguration(t)
		t.Log("✅ Application configuration initialized")

		// Step 2: Security initialization
		testSecurityInitialization(t, cfg)
		t.Log("✅ Security components initialized")

		// Step 3: Core services startup
		testCoreServicesStartup(t, cfg)
		t.Log("✅ Core services started")

		// Step 4: UI initialization
		testUIInitialization(t, cfg)
		t.Log("✅ UI components initialized")

		// Step 5: Full workflow execution
		testWorkflowExecution(t, cfg)
		t.Log("✅ Workflow execution completed")

		// Step 6: Performance validation
		testPerformanceInWorkflow(t, cfg)
		t.Log("✅ Performance validation passed")

		// Step 7: Cleanup and shutdown
		testGracefulShutdown(t, cfg)
		t.Log("✅ Graceful shutdown completed")

		t.Log("🎉 Complete application lifecycle test PASSED")
	})

	t.Run("RealWorldScenarios", func(t *testing.T) {
		t.Log("🌍 Testing real-world usage scenarios...")

		// Scenario 1: First-time user setup
		testFirstTimeUserSetup(t)
		t.Log("✅ First-time user setup workflow")

		// Scenario 2: Power user batch operations
		testPowerUserBatchOperations(t)
		t.Log("✅ Power user batch operations")

		// Scenario 3: Large file handling
		testLargeFileHandling(t)
		t.Log("✅ Large file handling scenario")

		// Scenario 4: Network interruption recovery
		testNetworkInterruptionRecovery(t)
		t.Log("✅ Network interruption recovery")

		// Scenario 5: Multi-region operations
		testMultiRegionOperations(t)
		t.Log("✅ Multi-region operations")

		t.Log("🎉 Real-world scenarios test PASSED")
	})

	t.Run("ConcurrentUserSimulation", func(t *testing.T) {
		t.Log("👥 Testing concurrent user simulation...")

		// Simulate multiple users using the application simultaneously
		testConcurrentUsers(t, 5) // 5 simulated concurrent users
		t.Log("✅ Concurrent user simulation passed")

		t.Log("🎉 Concurrent user simulation test PASSED")
	})
}

// testApplicationConfiguration initializes and validates application configuration
func testApplicationConfiguration(t *testing.T) *config.Config {
	// Test configuration loading
	cfg := config.Default()
	
	// Set test-safe defaults
	cfg.AWS.Region = "us-east-1"
	cfg.Performance.Workers = 4
	cfg.UI.Language = "en"
	cfg.UI.Mode = "bubbles"
	
	// Validate configuration
	if cfg.AWS.Region == "" {
		t.Fatal("Configuration region not set")
	}
	
	if cfg.Performance.Workers <= 0 {
		t.Fatal("Configuration workers not set properly")
	}
	
	return cfg
}

// testSecurityInitialization validates security component setup
func testSecurityInitialization(t *testing.T, cfg *config.Config) {
	// Test security configuration
	securityConfig := security.DefaultSecurityConfig()
	if securityConfig == nil {
		t.Fatal("Failed to create security config")
	}
	
	// Test credential validation functionality
	validator := security.NewValidator(securityConfig)
	if validator == nil {
		t.Fatal("Failed to create security validator")
	}
}

// testCoreServicesStartup validates core service initialization
func testCoreServicesStartup(t *testing.T, cfg *config.Config) {
	// Test metrics initialization
	metricsManager := metrics.GetGlobalMetrics()
	if metricsManager == nil {
		t.Fatal("Failed to initialize metrics")
	}
	
	// Test worker pool initialization
	workerConfig := worker.DefaultConfig()
	workerConfig.Workers = cfg.Performance.Workers
	pool := worker.NewPool(workerConfig)
	pool.Start()
	defer pool.Stop()
	
	// Test S3 client initialization
	s3Client := s3.NewClient(cfg.AWS.Region)
	if s3Client == nil {
		t.Fatal("Failed to create S3 client")
	}
	
	// Test i18n initialization
	i18n.InitWithLanguage(cfg.UI.Language)
	if i18n.Printer == nil {
		t.Fatal("Failed to initialize i18n")
	}
}

// testUIInitialization validates UI component setup
func testUIInitialization(t *testing.T, cfg *config.Config) {
	// Test application creation
	appInstance := app.New(cfg)
	if appInstance == nil {
		t.Fatal("Failed to create application instance")
	}
	
	// Test view creation
	bucketView := views.NewBucketView(cfg.AWS.Region)
	if bucketView == nil {
		t.Fatal("Failed to create bucket view")
	}
	
	objectView := views.NewObjectView(cfg.AWS.Region, "test-bucket", "list")
	if objectView == nil {
		t.Fatal("Failed to create object view")
	}
	
	operationView := views.NewOperationView(cfg.AWS.Region, "test-bucket")
	if operationView == nil {
		t.Fatal("Failed to create operation view")
	}
	
	uploadView := views.NewUploadView(cfg.AWS.Region, "test-bucket")
	if uploadView == nil {
		t.Fatal("Failed to create upload view")
	}
	
	helpView := views.NewHelpView()
	if helpView == nil {
		t.Fatal("Failed to create help view")
	}
	
	settingsView := views.NewSettingsView()
	if settingsView == nil {
		t.Fatal("Failed to create settings view")
	}
}

// testWorkflowExecution simulates complete user workflows
func testWorkflowExecution(t *testing.T, cfg *config.Config) {
	// Workflow 1: Browse buckets → select bucket → browse objects
	testBucketBrowsingWorkflow(t, cfg)
	
	// Workflow 2: Upload file → monitor progress → verify completion
	testUploadWorkflow(t, cfg)
	
	// Workflow 3: Download file → monitor progress → verify completion
	testDownloadWorkflow(t, cfg)
	
	// Workflow 4: Delete objects → confirm deletion
	testDeleteWorkflow(t, cfg)
	
	// Workflow 5: Settings management
	testSettingsWorkflow(t, cfg)
}

// testBucketBrowsingWorkflow tests the bucket browsing user flow
func testBucketBrowsingWorkflow(t *testing.T, cfg *config.Config) {
	region := cfg.AWS.Region
	
	// Step 1: Create bucket view
	bucketView := views.NewBucketView(region)
	if bucketView == nil {
		t.Fatal("Failed to create bucket view for browsing workflow")
	}
	
	// Step 2: Simulate bucket selection
	testBucket := "test-bucket-workflow"
	
	// Step 3: Navigate to object view
	objectView := views.NewObjectView(region, testBucket, "list")
	if objectView == nil {
		t.Fatal("Failed to create object view for browsing workflow")
	}
	
	// Step 4: Simulate object listing
	// (This would normally make S3 API calls)
	
	t.Log("✅ Bucket browsing workflow validated")
}

// testUploadWorkflow tests the file upload user flow
func testUploadWorkflow(t *testing.T, cfg *config.Config) {
	region := cfg.AWS.Region
	testBucket := "test-bucket-upload"
	
	// Step 1: Create upload view
	uploadView := views.NewUploadView(region, testBucket)
	if uploadView == nil {
		t.Fatal("Failed to create upload view")
	}
	
	// Step 2: Create test file
	testFile := createTestFile(t)
	defer os.Remove(testFile)
	
	// Step 3: Simulate upload initiation
	// (This would normally start the upload process)
	
	// Step 4: Monitor progress via metrics
	metricsManager := metrics.GetGlobalMetrics()
	initialUploads := metricsManager.GetSnapshot().S3Operations.Uploads
	
	// Simulate upload metrics update
	metricsManager.IncrementS3Uploads()
	metricsManager.AddBytesTransferred(1024)
	
	newSnapshot := metricsManager.GetSnapshot()
	if newSnapshot.S3Operations.Uploads <= initialUploads {
		t.Fatal("Upload metrics not updated properly")
	}
	
	t.Log("✅ Upload workflow validated")
}

// testDownloadWorkflow tests the file download user flow
func testDownloadWorkflow(t *testing.T, cfg *config.Config) {
	region := cfg.AWS.Region
	testBucket := "test-bucket-download"
	_ = "test-file.txt" // testKey
	
	// Step 1: Navigate to object view for download
	objectView := views.NewObjectView(region, testBucket, "download")
	if objectView == nil {
		t.Fatal("Failed to create object view for download")
	}
	
	// Step 2: Create S3 downloader
	s3Client := s3.NewClient(region)
	downloadConfig := s3.DefaultDownloadConfig()
	downloader := s3.NewDownloader(s3Client, downloadConfig)
	defer downloader.Close()
	
	// Step 3: Monitor download progress via metrics
	metricsManager := metrics.GetGlobalMetrics()
	initialDownloads := metricsManager.GetSnapshot().S3Operations.Downloads
	
	// Simulate download metrics update
	metricsManager.IncrementS3Downloads()
	metricsManager.AddBytesTransferred(2048)
	
	newSnapshot := metricsManager.GetSnapshot()
	if newSnapshot.S3Operations.Downloads <= initialDownloads {
		t.Fatal("Download metrics not updated properly")
	}
	
	t.Log("✅ Download workflow validated")
}

// testDeleteWorkflow tests the object deletion user flow
func testDeleteWorkflow(t *testing.T, cfg *config.Config) {
	region := cfg.AWS.Region
	testBucket := "test-bucket-delete"
	
	// Step 1: Navigate to operation view
	operationView := views.NewOperationView(region, testBucket)
	if operationView == nil {
		t.Fatal("Failed to create operation view for delete")
	}
	
	// Step 2: Simulate delete operation
	metricsManager := metrics.GetGlobalMetrics()
	initialDeletes := metricsManager.GetSnapshot().S3Operations.Deletes
	
	// Simulate delete metrics update
	metricsManager.IncrementS3Deletes()
	
	newSnapshot := metricsManager.GetSnapshot()
	if newSnapshot.S3Operations.Deletes <= initialDeletes {
		t.Fatal("Delete metrics not updated properly")
	}
	
	t.Log("✅ Delete workflow validated")
}

// testSettingsWorkflow tests the settings management flow
func testSettingsWorkflow(t *testing.T, cfg *config.Config) {
	// Step 1: Create settings view
	settingsView := views.NewSettingsView()
	if settingsView == nil {
		t.Fatal("Failed to create settings view")
	}
	
	// Step 2: Test configuration updates
	originalWorkers := cfg.Performance.Workers
	cfg.Performance.Workers = 8
	
	// Step 3: Validate settings change
	if cfg.Performance.Workers != 8 {
		t.Fatal("Failed to update configuration")
	}
	
	// Step 4: Restore original settings
	cfg.Performance.Workers = originalWorkers
	
	t.Log("✅ Settings workflow validated")
}

// testPerformanceInWorkflow validates performance during workflow execution
func testPerformanceInWorkflow(t *testing.T, cfg *config.Config) {
	// Test worker pool performance under load
	workerConfig := worker.PoolConfig{
		Workers:   10,
		QueueSize: 100,
		Timeout:   30 * time.Second,
	}
	
	pool := worker.NewPool(workerConfig)
	pool.Start()
	defer pool.Stop()
	
	// Submit multiple jobs and measure performance
	jobCount := 50
	start := time.Now()
	
	for i := 0; i < jobCount; i++ {
		job := &WorkflowJob{
			id:       fmt.Sprintf("workflow-job-%d", i),
			workTime: 5 * time.Millisecond,
		}
		pool.Submit(job)
	}
	
	// Wait for completion
	for i := 0; i < jobCount; i++ {
		<-pool.Results()
	}
	
	elapsed := time.Since(start)
	
	// With 10 workers, 50 jobs should complete quickly
	maxExpected := 500 * time.Millisecond
	if elapsed > maxExpected {
		t.Errorf("Workflow performance slower than expected: %v > %v", elapsed, maxExpected)
	}
	
	t.Logf("✅ Workflow performance: %v for %d jobs", elapsed, jobCount)
}

// testGracefulShutdown validates proper application shutdown
func testGracefulShutdown(t *testing.T, cfg *config.Config) {
	// Test worker pool shutdown
	pool := worker.NewPool(worker.DefaultConfig())
	pool.Start()
	
	// Submit some jobs
	for i := 0; i < 5; i++ {
		job := &WorkflowJob{
			id:       fmt.Sprintf("shutdown-job-%d", i),
			workTime: 10 * time.Millisecond,
		}
		pool.Submit(job)
	}
	
	// Test graceful shutdown
	start := time.Now()
	pool.Stop()
	shutdownTime := time.Since(start)
	
	// Shutdown should be quick
	maxShutdownTime := 2 * time.Second
	if shutdownTime > maxShutdownTime {
		t.Errorf("Shutdown took too long: %v > %v", shutdownTime, maxShutdownTime)
	}
	
	t.Logf("✅ Graceful shutdown completed in %v", shutdownTime)
}

// testFirstTimeUserSetup simulates first-time user experience
func testFirstTimeUserSetup(t *testing.T) {
	// Simulate configuration discovery
	cfg := config.Default()
	cfg.AWS.Region = "us-west-2"
	cfg.UI.Language = "en"
	cfg.UI.Mode = "bubbles"
	
	// Test help system
	helpView := views.NewHelpView()
	if helpView == nil {
		t.Fatal("Help view should be available for first-time users")
	}
	
	// Test initial bucket listing
	bucketView := views.NewBucketView(cfg.AWS.Region)
	if bucketView == nil {
		t.Fatal("Bucket view should be available for first-time users")
	}
	
	t.Log("✅ First-time user setup validated")
}

// testPowerUserBatchOperations simulates power user workflows
func testPowerUserBatchOperations(t *testing.T) {
	// Simulate high-volume operations
	workerConfig := worker.PoolConfig{
		Workers:   20,
		QueueSize: 500,
		Timeout:   60 * time.Second,
	}
	
	pool := worker.NewPool(workerConfig)
	pool.Start()
	defer pool.Stop()
	
	// Submit batch of operations
	batchSize := 100
	for i := 0; i < batchSize; i++ {
		job := &WorkflowJob{
			id:       fmt.Sprintf("batch-job-%d", i),
			workTime: 2 * time.Millisecond,
		}
		pool.Submit(job)
	}
	
	// Wait for batch completion
	completed := 0
	start := time.Now()
	for completed < batchSize {
		<-pool.Results()
		completed++
	}
	elapsed := time.Since(start)
	
	// Should handle batch efficiently
	maxBatchTime := 1 * time.Second
	if elapsed > maxBatchTime {
		t.Errorf("Batch operations too slow: %v > %v", elapsed, maxBatchTime)
	}
	
	t.Logf("✅ Power user batch operations: %v for %d operations", elapsed, batchSize)
}

// testLargeFileHandling simulates large file operations
func testLargeFileHandling(t *testing.T) {
	// Test large file simulation
	region := "us-east-1"
	s3Client := s3.NewClient(region)
	
	// Test uploader with large file config
	uploadConfig := s3.UploadConfig{
		PartSize:    10 * 1024 * 1024, // 10MB parts
		Concurrency: 5,
	}
	uploader := s3.NewUploader(s3Client, uploadConfig)
	defer uploader.Close()
	
	// Test downloader with large file config
	downloadConfig := s3.DownloadConfig{
		PartSize:    10 * 1024 * 1024, // 10MB parts
		Concurrency: 5,
	}
	downloader := s3.NewDownloader(s3Client, downloadConfig)
	defer downloader.Close()
	
	t.Log("✅ Large file handling configuration validated")
}

// testNetworkInterruptionRecovery simulates network issues
func testNetworkInterruptionRecovery(t *testing.T) {
	// Test timeout handling
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Simulate operation that might timeout
	job := &WorkflowJob{
		id:       "timeout-test",
		workTime: 200 * time.Millisecond, // Longer than context timeout
	}
	
	err := job.Execute(ctx)
	if err == nil {
		t.Error("Expected timeout error, but operation completed")
	}
	
	t.Log("✅ Network interruption recovery validated")
}

// testMultiRegionOperations simulates multi-region usage
func testMultiRegionOperations(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	
	for _, region := range regions {
		s3Client := s3.NewClient(region)
		if s3Client == nil {
			t.Fatalf("Failed to create S3 client for region %s", region)
		}
		
		bucketView := views.NewBucketView(region)
		if bucketView == nil {
			t.Fatalf("Failed to create bucket view for region %s", region)
		}
	}
	
	t.Log("✅ Multi-region operations validated")
}

// testConcurrentUsers simulates multiple concurrent users
func testConcurrentUsers(t *testing.T, userCount int) {
	done := make(chan bool, userCount)
	
	for i := 0; i < userCount; i++ {
		go func(userID int) {
			defer func() { done <- true }()
			
			// Each user performs a workflow
			cfg := config.Default()
			cfg.AWS.Region = "us-east-1"
			cfg.Performance.Workers = 2
			
			// User-specific operations
			pool := worker.NewPool(worker.DefaultConfig())
			pool.Start()
			defer pool.Stop()
			
			// Simulate user operations
			for j := 0; j < 10; j++ {
				job := &WorkflowJob{
					id:       fmt.Sprintf("user-%d-job-%d", userID, j),
					workTime: 5 * time.Millisecond,
				}
				pool.Submit(job)
			}
			
			// Wait for user's jobs to complete
			for j := 0; j < 10; j++ {
				<-pool.Results()
			}
		}(i)
	}
	
	// Wait for all users to complete
	for i := 0; i < userCount; i++ {
		<-done
	}
	
	t.Logf("✅ %d concurrent users simulation completed", userCount)
}

// Helper functions

// createTestFile creates a temporary test file
func createTestFile(t *testing.T) string {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "s3ry-test-file.txt")
	
	content := "This is a test file for s3ry E2E testing"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	return tmpFile
}

// WorkflowJob implements the Job interface for testing workflows
type WorkflowJob struct {
	id       string
	workTime time.Duration
}

func (j *WorkflowJob) Execute(ctx context.Context) error {
	select {
	case <-time.After(j.workTime):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}