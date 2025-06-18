package testhelpers

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

// MockS3API implements s3iface.S3API for testing
type MockS3API struct {
	s3iface.S3API
	ListBucketsFunc      func(*s3.ListBucketsInput) (*s3.ListBucketsOutput, error)
	ListObjectsFunc      func(*s3.ListObjectsInput) (*s3.ListObjectsOutput, error)
	ListObjectsPagesFunc func(*s3.ListObjectsInput, func(*s3.ListObjectsOutput, bool) bool) error
	GetObjectFunc        func(*s3.GetObjectInput) (*s3.GetObjectOutput, error)
	PutObjectFunc        func(*s3.PutObjectInput) (*s3.PutObjectOutput, error)
	DeleteObjectFunc     func(*s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error)
}

func (m *MockS3API) ListBuckets(input *s3.ListBucketsInput) (*s3.ListBucketsOutput, error) {
	if m.ListBucketsFunc != nil {
		return m.ListBucketsFunc(input)
	}
	return &s3.ListBucketsOutput{}, nil
}

func (m *MockS3API) ListObjects(input *s3.ListObjectsInput) (*s3.ListObjectsOutput, error) {
	if m.ListObjectsFunc != nil {
		return m.ListObjectsFunc(input)
	}
	return &s3.ListObjectsOutput{}, nil
}

func (m *MockS3API) ListObjectsPages(input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool) error {
	if m.ListObjectsPagesFunc != nil {
		return m.ListObjectsPagesFunc(input, fn)
	}
	return nil
}

func (m *MockS3API) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	if m.GetObjectFunc != nil {
		return m.GetObjectFunc(input)
	}
	return &s3.GetObjectOutput{}, nil
}

func (m *MockS3API) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	if m.PutObjectFunc != nil {
		return m.PutObjectFunc(input)
	}
	return &s3.PutObjectOutput{}, nil
}

func (m *MockS3API) DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
	if m.DeleteObjectFunc != nil {
		return m.DeleteObjectFunc(input)
	}
	return &s3.DeleteObjectOutput{}, nil
}

// CreateTestBuckets creates test bucket data
func CreateTestBuckets() []*s3.Bucket {
	now := time.Now()
	return []*s3.Bucket{
		{
			Name:         aws.String("test-bucket-1"),
			CreationDate: &now,
		},
		{
			Name:         aws.String("test-bucket-2"),
			CreationDate: &now,
		},
	}
}

// CreateTestObjects creates test object data
func CreateTestObjects() []*s3.Object {
	now := time.Now()
	return []*s3.Object{
		{
			Key:          aws.String("test-file-1.txt"),
			LastModified: &now,
			Size:         aws.Int64(1024),
		},
		{
			Key:          aws.String("test-file-2.txt"),
			LastModified: &now,
			Size:         aws.Int64(2048),
		},
		{
			Key:          aws.String("folder/"),
			LastModified: &now,
			Size:         aws.Int64(0),
		},
	}
}

// PromptItems struct for testing (copy to avoid import cycle)
type PromptItems struct {
	Key          int
	Val          string
	Size         int64
	LastModified time.Time
	Tag          string
}

// CreateTestPromptItems creates test PromptItems
func CreateTestPromptItems() []PromptItems {
	now := time.Now()
	return []PromptItems{
		{Key: 0, Val: "test-item-1", LastModified: now, Size: 1024, Tag: "Test"},
		{Key: 1, Val: "test-item-2", LastModified: now, Size: 2048, Tag: "Test"},
	}
}

// CreateTempFile creates a temporary file for testing
func CreateTempFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile.Name()
}

// CleanupTempFile removes temporary file
func CleanupTempFile(filepath string) {
	os.Remove(filepath)
}

// CaptureLogOutput captures log output for testing
func CaptureLogOutput(fn func()) string {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	fn()

	return buf.String()
}

// CaptureStdout captures stdout for testing
func CaptureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()

	return buf.String()
}

// AssertContains checks if a string contains a substring
func AssertContains(t *testing.T, actual, expected string) {
	t.Helper()
	if !strings.Contains(actual, expected) {
		t.Errorf("Expected %q to contain %q", actual, expected)
	}
}

// AssertNotContains checks if a string does not contain a substring
func AssertNotContains(t *testing.T, actual, unexpected string) {
	t.Helper()
	if strings.Contains(actual, unexpected) {
		t.Errorf("Expected %q not to contain %q", actual, unexpected)
	}
}

// AssertFileExists checks if a file exists
func AssertFileExists(t *testing.T, filepath string) {
	t.Helper()
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		t.Errorf("Expected file %q to exist", filepath)
	}
}

// AssertFileNotExists checks if a file does not exist
func AssertFileNotExists(t *testing.T, filepath string) {
	t.Helper()
	if _, err := os.Stat(filepath); !os.IsNotExist(err) {
		t.Errorf("Expected file %q not to exist", filepath)
	}
}

// TestConfig holds test configuration
type TestConfig struct {
	Region     string
	BucketName string
	ObjectKey  string
}

// DefaultTestConfig returns default test configuration
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		Region:     "us-east-1",
		BucketName: "test-bucket",
		ObjectKey:  "test-object.txt",
	}
}
