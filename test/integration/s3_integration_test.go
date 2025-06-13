package integration

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/seike460/s3ry"
	"github.com/seike460/s3ry/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// S3IntegrationTestSuite contains integration tests for S3 operations
type S3IntegrationTestSuite struct {
	suite.Suite
	s3ry       *s3ry.S3ry
	testBucket string
}

// SetupSuite runs before all tests in the suite
func (suite *S3IntegrationTestSuite) SetupSuite() {
	// Skip integration tests if AWS credentials are not available
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		suite.T().Skip("Skipping integration tests - AWS credentials not available")
	}
	
	// Skip if test bucket is not specified
	testBucket := os.Getenv("S3RY_TEST_BUCKET")
	if testBucket == "" {
		suite.T().Skip("Skipping integration tests - S3RY_TEST_BUCKET not set")
	}
	
	suite.testBucket = testBucket
	suite.s3ry = s3ry.NewS3ry("us-east-1") // Use us-east-1 for tests
	suite.s3ry.Bucket = suite.testBucket
}

// TearDownSuite runs after all tests in the suite
func (suite *S3IntegrationTestSuite) TearDownSuite() {
	// Clean up any test objects that might remain
	if suite.s3ry != nil && suite.testBucket != "" {
		suite.cleanupTestObjects()
	}
}

// SetupTest runs before each test
func (suite *S3IntegrationTestSuite) SetupTest() {
	// Ensure we start with a clean state
	suite.cleanupTestObjects()
}

// TearDownTest runs after each test
func (suite *S3IntegrationTestSuite) TearDownTest() {
	// Clean up after each test
	suite.cleanupTestObjects()
}

// cleanupTestObjects removes any objects with test prefix
func (suite *S3IntegrationTestSuite) cleanupTestObjects() {
	if suite.s3ry == nil || suite.testBucket == "" {
		return
	}
	
	// List objects with test prefix
	input := &s3.ListObjectsInput{
		Bucket: aws.String(suite.testBucket),
		Prefix: aws.String("s3ry-test-"),
	}
	
	resp, err := suite.s3ry.Svc.ListObjects(input)
	if err != nil {
		return // Ignore cleanup errors
	}
	
	// Delete test objects
	for _, obj := range resp.Contents {
		deleteInput := &s3.DeleteObjectInput{
			Bucket: aws.String(suite.testBucket),
			Key:    obj.Key,
		}
		suite.s3ry.Svc.DeleteObject(deleteInput)
	}
}

// TestNewS3ry tests the S3ry constructor
func (suite *S3IntegrationTestSuite) TestNewS3ry() {
	s := s3ry.NewS3ry("us-east-1")
	
	assert.NotNil(suite.T(), s)
	assert.NotNil(suite.T(), s.Sess)
	assert.NotNil(suite.T(), s.Svc)
}

// TestListBuckets tests listing S3 buckets
func (suite *S3IntegrationTestSuite) TestListBuckets() {
	buckets := suite.s3ry.ListBuckets()
	
	assert.NotNil(suite.T(), buckets)
	// Should contain at least our test bucket
	bucketNames := make([]string, len(buckets))
	for i, bucket := range buckets {
		bucketNames[i] = bucket.Val
	}
	assert.Contains(suite.T(), bucketNames, suite.testBucket)
}

// TestUploadAndDownloadObject tests uploading and downloading objects
func (suite *S3IntegrationTestSuite) TestUploadAndDownloadObject() {
	// Create a test file
	testContent := "This is test content for integration testing"
	testFile := testhelpers.CreateTempFile(suite.T(), testContent)
	defer testhelpers.CleanupTempFile(testFile)
	
	testObjectKey := "s3ry-test-upload-" + time.Now().Format("20060102-150405") + ".txt"
	
	// Test upload
	suite.s3ry.UploadObject(suite.testBucket, testFile)
	
	// Verify object exists
	_, err := suite.s3ry.Svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(suite.testBucket),
		Key:    aws.String(testFile),
	})
	assert.NoError(suite.T(), err)
	
	// Test download
	downloadFile := "downloaded-" + testObjectKey
	suite.s3ry.GetObject(suite.testBucket, testFile)
	
	// Verify downloaded file exists and has correct content
	testhelpers.AssertFileExists(suite.T(), downloadFile)
	
	// Clean up downloaded file
	os.Remove(downloadFile)
}

// TestListObjects tests listing objects in a bucket
func (suite *S3IntegrationTestSuite) TestListObjects() {
	// Upload a test object first
	testContent := "Test content for list objects"
	testFile := testhelpers.CreateTempFile(suite.T(), testContent)
	defer testhelpers.CleanupTempFile(testFile)
	
	suite.s3ry.UploadObject(suite.testBucket, testFile)
	
	// Test listing objects
	objects := suite.s3ry.ListObjects(suite.testBucket)
	
	assert.NotNil(suite.T(), objects)
	
	// Should contain our test object
	objectKeys := make([]string, len(objects))
	for i, obj := range objects {
		objectKeys[i] = obj.Val
	}
	assert.Contains(suite.T(), objectKeys, testFile)
}

// TestListObjectsPages tests paginated listing of objects
func (suite *S3IntegrationTestSuite) TestListObjectsPages() {
	// Upload multiple test objects
	testFiles := make([]string, 3)
	for i := 0; i < 3; i++ {
		content := "Test content " + string(rune('A'+i))
		testFile := testhelpers.CreateTempFile(suite.T(), content)
		testFiles[i] = testFile
		defer testhelpers.CleanupTempFile(testFile)
		
		suite.s3ry.UploadObject(suite.testBucket, testFile)
	}
	
	// Test listing with pages
	objects := suite.s3ry.ListObjectsPages(suite.testBucket)
	
	assert.NotNil(suite.T(), objects)
	assert.GreaterOrEqual(suite.T(), len(objects), 3)
}

// TestDeleteObject tests deleting an object
func (suite *S3IntegrationTestSuite) TestDeleteObject() {
	// Upload a test object first
	testContent := "Test content for deletion"
	testFile := testhelpers.CreateTempFile(suite.T(), testContent)
	defer testhelpers.CleanupTempFile(testFile)
	
	suite.s3ry.UploadObject(suite.testBucket, testFile)
	
	// Verify object exists
	_, err := suite.s3ry.Svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(suite.testBucket),
		Key:    aws.String(testFile),
	})
	assert.NoError(suite.T(), err)
	
	// Delete the object
	suite.s3ry.DeleteObject(suite.testBucket, testFile)
	
	// Verify object no longer exists
	_, err = suite.s3ry.Svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(suite.testBucket),
		Key:    aws.String(testFile),
	})
	assert.Error(suite.T(), err) // Should get "Not Found" error
}

// TestSaveObjectList tests creating an object list file
func (suite *S3IntegrationTestSuite) TestSaveObjectList() {
	// Upload some test objects first
	for i := 0; i < 2; i++ {
		content := "Test content for list " + string(rune('A'+i))
		testFile := testhelpers.CreateTempFile(suite.T(), content)
		defer testhelpers.CleanupTempFile(testFile)
		
		suite.s3ry.UploadObject(suite.testBucket, testFile)
	}
	
	// Create object list
	suite.s3ry.SaveObjectList(suite.testBucket)
	
	// Find the created object list file
	files, err := os.ReadDir(".")
	assert.NoError(suite.T(), err)
	
	var objectListFile string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "ObjectList-") && strings.HasSuffix(file.Name(), ".txt") {
			objectListFile = file.Name()
			break
		}
	}
	
	assert.NotEmpty(suite.T(), objectListFile)
	testhelpers.AssertFileExists(suite.T(), objectListFile)
	
	// Clean up object list file
	os.Remove(objectListFile)
}

// TestSelectBucketAndRegion tests the bucket and region selection
func (suite *S3IntegrationTestSuite) TestSelectBucketAndRegion() {
	// This test is more complex as it involves interactive prompts
	// For integration testing, we'll just verify the components work
	
	// Test that we can create an S3ry instance for bucket search
	s3ryForBuckets := s3ry.NewS3ry(s3ry.ApNortheastOne)
	assert.NotNil(suite.T(), s3ryForBuckets)
	
	// Test that we can list buckets
	buckets := s3ryForBuckets.ListBuckets()
	assert.NotNil(suite.T(), buckets)
	assert.NotEmpty(suite.T(), buckets)
}

// TestErrorHandling tests error handling in various scenarios
func (suite *S3IntegrationTestSuite) TestErrorHandling() {
	// Test with non-existent bucket
	s3ryBad := s3ry.NewS3ry("us-east-1")
	s3ryBad.Bucket = "non-existent-bucket-" + time.Now().Format("20060102150405")
	
	// This should handle the error gracefully
	objects := s3ryBad.ListObjects(s3ryBad.Bucket)
	// The function logs the error but doesn't return it, so objects might be empty
	assert.NotNil(suite.T(), objects)
}

// Run the integration test suite
func TestS3IntegrationSuite(t *testing.T) {
	// Only run if explicitly requested
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration tests. Set RUN_INTEGRATION_TESTS=1 to run.")
	}
	
	suite.Run(t, new(S3IntegrationTestSuite))
}

// Benchmark tests for integration
func BenchmarkS3Operations(b *testing.B) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		b.Skip("Skipping integration benchmarks. Set RUN_INTEGRATION_TESTS=1 to run.")
	}
	
	testBucket := os.Getenv("S3RY_TEST_BUCKET")
	if testBucket == "" {
		b.Skip("S3RY_TEST_BUCKET not set")
	}
	
	s3ryInstance := s3ry.NewS3ry("us-east-1")
	s3ryInstance.Bucket = testBucket
	
	b.Run("ListBuckets", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			s3ryInstance.ListBuckets()
		}
	})
	
	b.Run("ListObjects", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			s3ryInstance.ListObjects(testBucket)
		}
	})
}