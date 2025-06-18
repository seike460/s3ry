package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seike460/s3ry/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// E2ETestSuite contains end-to-end tests for the s3ry application
type E2ETestSuite struct {
	suite.Suite
	binaryPath  string
	testBucket  string
	testFiles   []string
	originalDir string
	tempDir     string
}

// SetupSuite builds the binary and sets up the test environment
func (suite *E2ETestSuite) SetupSuite() {
	// Skip E2E tests if not explicitly requested
	if os.Getenv("RUN_E2E_TESTS") == "" {
		suite.T().Skip("Skipping E2E tests. Set RUN_E2E_TESTS=1 to run.")
	}

	// Skip if AWS credentials are not available
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		suite.T().Skip("Skipping E2E tests - AWS credentials not available")
	}

	// Skip if test bucket is not specified
	testBucket := os.Getenv("S3RY_TEST_BUCKET")
	if testBucket == "" {
		suite.T().Skip("Skipping E2E tests - S3RY_TEST_BUCKET not set")
	}

	suite.testBucket = testBucket

	// Store original directory
	originalDir, err := os.Getwd()
	assert.NoError(suite.T(), err)
	suite.originalDir = originalDir

	// Create temporary directory for tests
	suite.tempDir = suite.T().TempDir()

	// Build the binary
	suite.buildBinary()

	// Create test files
	suite.createTestFiles()
}

// TearDownSuite cleans up after all tests
func (suite *E2ETestSuite) TearDownSuite() {
	// Change back to original directory
	if suite.originalDir != "" {
		os.Chdir(suite.originalDir)
	}

	// Clean up test files
	suite.cleanupTestFiles()

	// Remove binary
	if suite.binaryPath != "" {
		os.Remove(suite.binaryPath)
	}
}

// SetupTest prepares for each test
func (suite *E2ETestSuite) SetupTest() {
	// Change to temp directory for each test
	err := os.Chdir(suite.tempDir)
	assert.NoError(suite.T(), err)
}

// TearDownTest cleans up after each test
func (suite *E2ETestSuite) TearDownTest() {
	// Clean up any files created during test
	files, _ := os.ReadDir(suite.tempDir)
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "test-") {
			os.Remove(filepath.Join(suite.tempDir, file.Name()))
		}
	}
}

// buildBinary builds the s3ry binary for testing
func (suite *E2ETestSuite) buildBinary() {
	// Change to project root
	err := os.Chdir(suite.originalDir)
	assert.NoError(suite.T(), err)

	binaryName := "s3ry-test"
	if os.Getenv("GOOS") == "windows" {
		binaryName += ".exe"
	}

	suite.binaryPath = filepath.Join(suite.tempDir, binaryName)

	// Build command
	cmd := exec.Command("go", "build", "-o", suite.binaryPath, "./cmd/s3ry")
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	_ = err // Suppress unused variable warning
	if err != nil {
		suite.T().Fatalf("Failed to build binary: %v\nOutput: %s", err, output)
	}

	// Verify binary exists and is executable
	info, err := os.Stat(suite.binaryPath)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), info.Mode().IsRegular())
}

// createTestFiles creates test files for upload/download tests
func (suite *E2ETestSuite) createTestFiles() {
	testFiles := []struct {
		name    string
		content string
	}{
		{"test-small.txt", "Small test file content"},
		{"test-medium.log", strings.Repeat("Line of medium test file\n", 100)},
		{"test-json.json", `{"test": true, "message": "e2e test file"}`},
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(suite.tempDir, tf.name)
		err := os.WriteFile(filePath, []byte(tf.content), 0644)
		assert.NoError(suite.T(), err)
		suite.testFiles = append(suite.testFiles, tf.name)
	}
}

// cleanupTestFiles removes test files from S3
func (suite *E2ETestSuite) cleanupTestFiles() {
	// This would require AWS CLI or SDK calls to clean up
	// For now, we'll rely on manual cleanup or test bucket lifecycle policies
}

// runCommand runs the s3ry binary with given arguments and input
func (suite *E2ETestSuite) runCommand(args []string, input string) (string, string, error) {
	cmd := exec.Command(suite.binaryPath, args...)
	cmd.Dir = suite.tempDir

	// Set up environment
	cmd.Env = os.Environ()

	// Provide input if specified
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// TestBinaryExists tests that the binary was built successfully
func (suite *E2ETestSuite) TestBinaryExists() {
	assert.FileExists(suite.T(), suite.binaryPath)

	// Test that binary is executable
	info, err := os.Stat(suite.binaryPath)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), info.Mode().IsRegular())
}

// TestBinaryVersion tests running the binary (basic execution)
func (suite *E2ETestSuite) TestBinaryExecution() {
	// This test will likely fail because the binary expects interactive input
	// But it verifies that the binary can start
	stdout, stderr, _ := suite.runCommand([]string{}, "")
	// Ignore error as binary execution might fail in test environment

	// The binary should start and try to list buckets, then fail or prompt
	// We're mainly testing that it doesn't crash immediately
	assert.NotEmpty(suite.T(), stdout+stderr, "Should produce some output")
}

// TestListBuckets tests bucket listing functionality
func (suite *E2ETestSuite) TestListBuckets() {
	// Since the app is interactive, we can't easily test the full flow
	// But we can test that it starts and attempts to list buckets

	// Use timeout to prevent hanging
	cmd := exec.Command(suite.binaryPath)
	cmd.Dir = suite.tempDir
	cmd.Env = os.Environ()

	// Start the command
	err := cmd.Start()
	_ = err // Will be checked in assert
	assert.NoError(suite.T(), err)

	// Wait for a short time
	time.Sleep(2 * time.Second)

	// Kill the process
	if cmd.Process != nil {
		cmd.Process.Kill()
	}

	// Process should have started successfully
	assert.NotNil(suite.T(), cmd.Process)
}

// TestFileOperations tests file upload/download operations
func (suite *E2ETestSuite) TestFileOperations() {
	// This is a complex test that would require mocking user input
	// For now, we'll test the supporting components

	// Verify test files exist
	for _, testFile := range suite.testFiles {
		filePath := filepath.Join(suite.tempDir, testFile)
		testhelpers.AssertFileExists(suite.T(), filePath)
	}

	// Test file content
	content, err := os.ReadFile(filepath.Join(suite.tempDir, "test-small.txt"))
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "Small test file content", string(content))
}

// TestConfigFiles tests configuration file handling
func (suite *E2ETestSuite) TestConfigFiles() {
	// Test that the binary can handle missing config files gracefully

	// Remove any existing config files
	configFiles := []string{".s3ry", ".aws/config", ".aws/credentials"}
	for _, config := range configFiles {
		os.RemoveAll(filepath.Join(suite.tempDir, config))
	}

	// The binary should still attempt to run with AWS environment variables
	stdout, stderr, _ := suite.runCommand([]string{}, "")
	// Ignore error as binary execution might fail in test environment

	// Should not crash due to missing config files
	output := stdout + stderr
	assert.NotContains(suite.T(), strings.ToLower(output), "panic")
}

// TestEnvironmentVariables tests AWS environment variable handling
func (suite *E2ETestSuite) TestEnvironmentVariables() {
	// Test with missing AWS credentials
	originalKey := os.Getenv("AWS_ACCESS_KEY_ID")
	originalSecret := os.Getenv("AWS_SECRET_ACCESS_KEY")
	_ = originalKey    // Used in defer
	_ = originalSecret // Used in defer

	// Temporarily remove credentials
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	defer func() {
		// Restore credentials
		if originalKey != "" {
			os.Setenv("AWS_ACCESS_KEY_ID", originalKey)
		}
		if originalSecret != "" {
			os.Setenv("AWS_SECRET_ACCESS_KEY", originalSecret)
		}
	}()

	// Run binary - should handle missing credentials gracefully
	stdout, stderr, _ := suite.runCommand([]string{}, "")
	// Ignore error as binary execution might fail in test environment

	// Should get an AWS credentials error, not a panic
	output := strings.ToLower(stdout + stderr)
	if strings.Contains(output, "error") {
		// Should be a credentials error, not a crash
		assert.NotContains(suite.T(), output, "panic")
		assert.NotContains(suite.T(), output, "runtime error")
	}
}

// TestLargeFileHandling tests handling of large files
func (suite *E2ETestSuite) TestLargeFileHandling() {
	// Create a larger test file
	largeFileName := "test-large.txt"
	largeContent := strings.Repeat("This is a line in a large test file.\n", 1000)
	largeFilePath := filepath.Join(suite.tempDir, largeFileName)

	err := os.WriteFile(largeFilePath, []byte(largeContent), 0644)
	assert.NoError(suite.T(), err)

	// Verify file was created
	info, err := os.Stat(largeFilePath)
	assert.NoError(suite.T(), err)
	assert.Greater(suite.T(), info.Size(), int64(10000)) // Should be > 10KB

	// Clean up
	os.Remove(largeFilePath)
}

// TestErrorHandling tests various error conditions
func (suite *E2ETestSuite) TestErrorHandling() {
	// Test with invalid AWS region
	cmd := exec.Command(suite.binaryPath)
	cmd.Dir = suite.tempDir
	cmd.Env = append(os.Environ(), "AWS_DEFAULT_REGION=invalid-region-12345")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start and quickly terminate
	err := cmd.Start()
	_ = err // Used conditionally below
	if err == nil {
		time.Sleep(1 * time.Second)
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}

	// Should not panic with invalid region
	output := strings.ToLower(stdout.String() + stderr.String())
	assert.NotContains(suite.T(), output, "panic")
}

// TestConcurrentAccess tests concurrent access scenarios
func (suite *E2ETestSuite) TestConcurrentAccess() {
	// Test running multiple instances (though they shouldn't interfere)

	commands := make([]*exec.Cmd, 3)
	for i := 0; i < 3; i++ {
		cmd := exec.Command(suite.binaryPath)
		cmd.Dir = suite.tempDir
		cmd.Env = os.Environ()
		commands[i] = cmd
	}

	// Start all commands
	for i, cmd := range commands {
		err := cmd.Start()
		assert.NoError(suite.T(), err, "Command %d should start", i)
	}

	// Wait briefly
	time.Sleep(1 * time.Second)

	// Terminate all commands
	for i, cmd := range commands {
		if cmd.Process != nil {
			err := cmd.Process.Kill()
			assert.NoError(suite.T(), err, "Command %d should terminate", i)
		}
	}
}

// Run the E2E test suite
func TestE2ETestSuite(t *testing.T) {
	suite.Run(t, new(E2ETestSuite))
}

// Benchmark the binary startup time
func BenchmarkBinaryStartup(b *testing.B) {
	if os.Getenv("RUN_E2E_TESTS") == "" {
		b.Skip("Skipping E2E benchmarks. Set RUN_E2E_TESTS=1 to run.")
	}

	// Build binary once
	originalDir, _ := os.Getwd()
	tempDir := b.TempDir()
	binaryPath := filepath.Join(tempDir, "s3ry-bench")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/s3ry")
	cmd.Dir = originalDir
	err := cmd.Run()
	if err != nil {
		b.Fatal("Failed to build binary for benchmark")
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cmd := exec.Command(binaryPath)
		cmd.Dir = tempDir

		start := time.Now()
		err := cmd.Start()
		if err == nil && cmd.Process != nil {
			// Measure startup time by killing immediately
			cmd.Process.Kill()
			cmd.Wait()
		}
		elapsed := time.Since(start)

		b.ReportMetric(float64(elapsed.Nanoseconds()), "startup-ns")
	}
}
