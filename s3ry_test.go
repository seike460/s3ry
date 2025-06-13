package s3ry

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/seike460/s3ry/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestNewS3ry(t *testing.T) {
	s := NewS3ry(ApNortheastOne)
	
	assert.NotNil(t, s)
	assert.NotNil(t, s.Sess)
	assert.NotNil(t, s.Svc)
	assert.Equal(t, "", s.Bucket) // Should be empty initially
}

func TestApNortheastOneConstant(t *testing.T) {
	assert.Equal(t, "ap-northeast-1", ApNortheastOne)
}

func TestListOperation(t *testing.T) {
	s := NewS3ry(ApNortheastOne)
	operations := s.ListOperation()
	
	assert.NotNil(t, operations)
	assert.Len(t, operations, 4)
	
	// Check if all expected operations are present
	operationNames := make([]string, len(operations))
	for i, op := range operations {
		operationNames[i] = op.Val
	}
	
	assert.Contains(t, operationNames, "ダウンロード")
	assert.Contains(t, operationNames, "アップロード")
	assert.Contains(t, operationNames, "オブジェクトを削除")
	assert.Contains(t, operationNames, "オブジェクトリストを作成")
}

func TestS3ry_SelectItem(t *testing.T) {
	_ = NewS3ry(ApNortheastOne)
	
	// Create test items
	items := testhelpers.CreateTestPromptItems()
	
	// Note: This test can't easily test the interactive prompt selection
	// We're mainly testing that the function is properly structured
	assert.NotNil(t, items)
	assert.Len(t, items, 2)
}

func TestSelectBucketAndRegion_Integration(t *testing.T) {
	// Skip this test if AWS credentials are not available
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("Skipping integration test - AWS credentials not available")
	}
	
	// This is an integration test that requires actual AWS credentials
	// In a real environment, you might want to use localstack or minio for testing
	t.Skip("Integration test - requires AWS setup")
}

// Unit tests with mocked S3 service would go here
func TestS3ry_WithMockService(t *testing.T) {
	// This demonstrates how we could test with a mock service
	// For now, we're focusing on the structure and basic functionality
	
	config := testhelpers.DefaultTestConfig()
	assert.Equal(t, "us-east-1", config.Region)
	assert.Equal(t, "test-bucket", config.BucketName)
	assert.Equal(t, "test-object.txt", config.ObjectKey)
}

func TestPromptItemsCreation(t *testing.T) {
	now := time.Now()
	item := PromptItems{
		Key:          1,
		Val:          "test-file.txt",
		Size:         1024,
		LastModified: now,
		Tag:          "Object",
	}
	
	assert.Equal(t, 1, item.Key)
	assert.Equal(t, "test-file.txt", item.Val)
	assert.Equal(t, int64(1024), item.Size)
	assert.Equal(t, now, item.LastModified)
	assert.Equal(t, "Object", item.Tag)
}

// Test for Operations function behavior (unit test level)
func TestOperations_Structure(t *testing.T) {
	// This tests the structure without actual AWS calls
	config := testhelpers.DefaultTestConfig()
	
	// Test that we can create a new S3ry instance
	s := NewS3ry(config.Region)
	assert.NotNil(t, s)
	
	// Test setting bucket
	s.Bucket = config.BucketName
	assert.Equal(t, config.BucketName, s.Bucket)
	
	// Test operations list
	operations := s.ListOperation()
	assert.Len(t, operations, 4)
}

// Benchmark tests
func BenchmarkNewS3ry(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s := NewS3ry(ApNortheastOne)
		if s == nil {
			b.Fatal("NewS3ry returned nil")
		}
	}
}

func BenchmarkListOperation(b *testing.B) {
	s := NewS3ry(ApNortheastOne)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		operations := s.ListOperation()
		if len(operations) != 4 {
			b.Fatalf("Expected 4 operations, got %d", len(operations))
		}
	}
}

// Example test showing how to test with mock data
func TestS3ry_ListBuckets_Mock(t *testing.T) {
	// This shows the pattern for testing with mock data
	mockBuckets := testhelpers.CreateTestBuckets()
	
	assert.Len(t, mockBuckets, 2)
	assert.Equal(t, "test-bucket-1", *mockBuckets[0].Name)
	assert.Equal(t, "test-bucket-2", *mockBuckets[1].Name)
}

func TestS3ry_ListObjects_Mock(t *testing.T) {
	// This shows the pattern for testing with mock data
	mockObjects := testhelpers.CreateTestObjects()
	
	assert.Len(t, mockObjects, 3)
	
	// Should filter out directories (keys ending with "/")
	var fileObjects []*s3.Object
	for _, obj := range mockObjects {
		if !strings.HasSuffix(*obj.Key, "/") {
			fileObjects = append(fileObjects, obj)
		}
	}
	
	assert.Len(t, fileObjects, 2) // Should have 2 files, 1 directory filtered out
}
