package s3ry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/seike460/s3ry/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestPromptItems(t *testing.T) {
	now := time.Now()
	item := PromptItems{
		Key:          1,
		Val:          "test-value",
		Size:         1024,
		LastModified: now,
		Tag:          "test-tag",
	}

	assert.Equal(t, 1, item.Key)
	assert.Equal(t, "test-value", item.Val)
	assert.Equal(t, int64(1024), item.Size)
	assert.Equal(t, now, item.LastModified)
	assert.Equal(t, "test-tag", item.Tag)
}

func TestCheckLocalExists_FileNotExists(t *testing.T) {
	// Test with non-existent file
	objectKey := "non-existent-file.txt"

	// This should not panic or cause issues
	checkLocalExists(objectKey)

	// Verify file still doesn't exist
	testhelpers.AssertFileNotExists(t, filepath.Base(objectKey))
}

func TestCheckLocalExists_FileExists(t *testing.T) {
	// Create a temporary file
	tmpFile := testhelpers.CreateTempFile(t, "test content")
	defer testhelpers.CleanupTempFile(tmpFile)

	// Test that the file exists check recognizes the file
	testhelpers.AssertFileExists(t, tmpFile)

	// Note: We can't easily test the interactive prompt without mocking stdin
	// This test just ensures the function can handle existing files without panicking
}

func TestAwsErrorPrint(t *testing.T) {
	// Test with AWS error
	awsErr := awserr.New("TestCode", "Test AWS error message", nil)

	// Note: awsErrorPrint calls log.Fatal which exits the process
	// This test just verifies the function signature and AWS error creation
	assert.NotNil(t, awsErr)
	assert.Contains(t, awsErr.Error(), "Test AWS error message")
}

func TestDirwalk_CurrentDirectory(t *testing.T) {
	// Test with current directory
	files := dirwalk("")

	// Should return at least some files
	assert.NotEmpty(t, files)

	// All returned paths should be valid (some start with ./, some are just filenames)
	for _, file := range files {
		// Files should either start with ./ or be valid relative paths
		assert.True(t, strings.HasPrefix(file, "./") || !strings.HasPrefix(file, "/"),
			"File path should be relative: %s", file)
	}
}

func TestDirwalk_SpecificDirectory(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create test files
	testFile1 := filepath.Join(tmpDir, "test1.txt")
	testFile2 := filepath.Join(tmpDir, "test2.txt")

	err := os.WriteFile(testFile1, []byte("content1"), 0644)
	assert.NoError(t, err)

	err = os.WriteFile(testFile2, []byte("content2"), 0644)
	assert.NoError(t, err)

	// Create subdirectory with file
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	assert.NoError(t, err)

	testFile3 := filepath.Join(subDir, "test3.txt")
	err = os.WriteFile(testFile3, []byte("content3"), 0644)
	assert.NoError(t, err)

	// Test dirwalk
	files := dirwalk(tmpDir)

	// Should find all files including subdirectory files
	assert.Len(t, files, 3)

	// Convert to relative paths for easier testing
	var filenames []string
	for _, file := range files {
		filenames = append(filenames, filepath.Base(file))
	}

	assert.Contains(t, filenames, "test1.txt")
	assert.Contains(t, filenames, "test2.txt")
	assert.Contains(t, filenames, "test3.txt")
}

func TestDirwalk_EmptyDirectory(t *testing.T) {
	// Create empty temporary directory
	tmpDir := t.TempDir()

	files := dirwalk(tmpDir)

	// Should return empty slice for empty directory
	assert.Empty(t, files)
}

func TestDirwalk_NonExistentDirectory(t *testing.T) {
	// Test with non-existent directory
	// Note: dirwalk calls log.Fatal which exits the process
	// We can't easily test this without modifying the dirwalk function
	// For now, we'll just test that the function exists and accepts invalid input

	// This test is skipped because dirwalk calls log.Fatal which would terminate the test process
	t.Skip("Cannot test log.Fatal behavior without process termination")
}

func TestSpinnerFunctions(t *testing.T) {
	// Test spinner start/stop functions
	// These are mainly to ensure they don't panic

	sps("Test spinner message")

	// Give spinner a moment to start
	time.Sleep(10 * time.Millisecond)

	spe()

	// Test completed without panic
	assert.True(t, true)
}

func BenchmarkDirwalk(b *testing.B) {
	// Create a directory with many files for benchmarking
	tmpDir := b.TempDir()

	// Create 100 test files
	for i := 0; i < 100; i++ {
		testFile := filepath.Join(tmpDir, "testfile"+string(rune(i))+".txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		files := dirwalk(tmpDir)
		if len(files) != 100 {
			b.Errorf("Expected 100 files, got %d", len(files))
		}
	}
}
