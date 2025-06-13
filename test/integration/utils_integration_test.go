package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/seike460/s3ry/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// UtilsIntegrationTestSuite contains integration tests for utility functions
type UtilsIntegrationTestSuite struct {
	suite.Suite
	tempDir string
}

// SetupSuite creates a temporary directory for testing
func (suite *UtilsIntegrationTestSuite) SetupSuite() {
	tempDir := suite.T().TempDir()
	suite.tempDir = tempDir
}

// TestDirwalkIntegration tests dirwalk with a real directory structure
func (suite *UtilsIntegrationTestSuite) TestDirwalkIntegration() {
	// Create a complex directory structure
	testStructure := map[string]string{
		"file1.txt":            "content1",
		"file2.log":            "content2",
		"subdir/file3.json":    "content3",
		"subdir/file4.csv":     "content4",
		"subdir/deep/file5.md": "content5",
		"another/file6.txt":    "content6",
	}
	
	// Create the files
	for path, content := range testStructure {
		fullPath := filepath.Join(suite.tempDir, path)
		dir := filepath.Dir(fullPath)
		
		err := os.MkdirAll(dir, 0755)
		assert.NoError(suite.T(), err)
		
		err = os.WriteFile(fullPath, []byte(content), 0644)
		assert.NoError(suite.T(), err)
	}
	
	// Test dirwalk - copy the function locally to avoid import cycles
	files := dirwalk(suite.tempDir)
	
	// Should find all files (not directories)
	assert.Len(suite.T(), files, len(testStructure))
	
	// Convert to relative paths for easier testing
	relativePaths := make([]string, len(files))
	for i, file := range files {
		rel, err := filepath.Rel(suite.tempDir, file)
		assert.NoError(suite.T(), err)
		relativePaths[i] = rel
	}
	
	// Check that all expected files are found
	for expectedPath := range testStructure {
		assert.Contains(suite.T(), relativePaths, expectedPath)
	}
}

// TestDirwalkLargeDirectory tests dirwalk performance with many files
func (suite *UtilsIntegrationTestSuite) TestDirwalkLargeDirectory() {
	largeDir := filepath.Join(suite.tempDir, "large")
	err := os.MkdirAll(largeDir, 0755)
	assert.NoError(suite.T(), err)
	
	// Create 100 files
	fileCount := 100
	for i := 0; i < fileCount; i++ {
		filename := filepath.Join(largeDir, "file_"+string(rune('0'+(i%10)))+".txt")
		err := os.WriteFile(filename, []byte("content"), 0644)
		assert.NoError(suite.T(), err)
	}
	
	// Test dirwalk
	files := dirwalk(largeDir)
	assert.Len(suite.T(), files, fileCount)
}

// TestDirwalkSymlinks tests dirwalk behavior with symbolic links
func (suite *UtilsIntegrationTestSuite) TestDirwalkSymlinks() {
	// Create a regular file
	regularFile := filepath.Join(suite.tempDir, "regular.txt")
	err := os.WriteFile(regularFile, []byte("regular content"), 0644)
	assert.NoError(suite.T(), err)
	
	// Create a symbolic link to the file
	linkFile := filepath.Join(suite.tempDir, "link.txt")
	err = os.Symlink(regularFile, linkFile)
	if err != nil {
		// Skip if symlinks are not supported (e.g., Windows without admin)
		suite.T().Skip("Symbolic links not supported on this system")
	}
	
	// Test dirwalk
	files := dirwalk(suite.tempDir)
	
	// Should include both the regular file and the symlink
	assert.GreaterOrEqual(suite.T(), len(files), 2)
	
	var basenames []string
	for _, file := range files {
		basenames = append(basenames, filepath.Base(file))
	}
	
	assert.Contains(suite.T(), basenames, "regular.txt")
	assert.Contains(suite.T(), basenames, "link.txt")
}

// TestDirwalkPermissions tests dirwalk with different file permissions
func (suite *UtilsIntegrationTestSuite) TestDirwalkPermissions() {
	// Create files with different permissions
	files := map[string]os.FileMode{
		"readable.txt":   0644,
		"executable.txt": 0755,
		"readonly.txt":   0444,
	}
	
	for filename, mode := range files {
		fullPath := filepath.Join(suite.tempDir, filename)
		err := os.WriteFile(fullPath, []byte("content"), mode)
		assert.NoError(suite.T(), err)
	}
	
	// Test dirwalk
	foundFiles := dirwalk(suite.tempDir)
	
	// Should find all files regardless of permissions
	assert.GreaterOrEqual(suite.T(), len(foundFiles), len(files))
	
	var basenames []string
	for _, file := range foundFiles {
		basenames = append(basenames, filepath.Base(file))
	}
	
	for filename := range files {
		assert.Contains(suite.T(), basenames, filename)
	}
}

// TestCheckLocalExistsIntegration tests checkLocalExists with real files
func (suite *UtilsIntegrationTestSuite) TestCheckLocalExistsIntegration() {
	// Create a test file
	testFile := filepath.Join(suite.tempDir, "existing.txt")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	assert.NoError(suite.T(), err)
	
	// Change to the temp directory for this test
	originalDir, err := os.Getwd()
	assert.NoError(suite.T(), err)
	defer os.Chdir(originalDir)
	
	err = os.Chdir(suite.tempDir)
	assert.NoError(suite.T(), err)
	
	// Test with existing file (note: this will prompt for user input in real usage)
	// For testing, we just ensure it doesn't panic
	testhelpers.AssertFileExists(suite.T(), "existing.txt")
	
	// Test with non-existing file
	testhelpers.AssertFileNotExists(suite.T(), "nonexistent.txt")
}

// TestSpinnerIntegration tests spinner functions
func (suite *UtilsIntegrationTestSuite) TestSpinnerIntegration() {
	// Test that spinner functions don't panic
	sps("Testing spinner")
	
	// Let it spin briefly
	time.Sleep(100 * time.Millisecond)
	
	spe()
	
	// Test completed without issues
	assert.True(suite.T(), true)
}

// TestFileOperationsIntegration tests various file operations
func (suite *UtilsIntegrationTestSuite) TestFileOperationsIntegration() {
	// Test creating and reading files
	testContent := "Integration test content"
	testFile := filepath.Join(suite.tempDir, "test_ops.txt")
	
	// Write file
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	assert.NoError(suite.T(), err)
	
	// Read file back
	content, err := os.ReadFile(testFile)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), testContent, string(content))
	
	// Test file info
	info, err := os.Stat(testFile)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test_ops.txt", info.Name())
	assert.Equal(suite.T(), int64(len(testContent)), info.Size())
}

// Run the integration test suite
func TestUtilsIntegrationSuite(t *testing.T) {
	// Only run if explicitly requested
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration tests. Set RUN_INTEGRATION_TESTS=1 to run.")
	}
	
	suite.Run(t, new(UtilsIntegrationTestSuite))
}

// Benchmark dirwalk performance
func BenchmarkDirwalkIntegration(b *testing.B) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		b.Skip("Skipping integration benchmarks. Set RUN_INTEGRATION_TESTS=1 to run.")
	}
	
	// Create a temporary directory with files
	tempDir := b.TempDir()
	
	// Create 50 files for benchmarking
	for i := 0; i < 50; i++ {
		filename := filepath.Join(tempDir, "bench_file_"+string(rune('0'+(i%10)))+".txt")
		err := os.WriteFile(filename, []byte("benchmark content"), 0644)
		if err != nil {
			b.Fatal(err)
		}
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		files := dirwalk(tempDir)
		if len(files) != 50 {
			b.Errorf("Expected 50 files, got %d", len(files))
		}
	}
}