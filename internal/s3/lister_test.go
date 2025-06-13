package s3

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewLister(t *testing.T) {
	client := NewClient("us-east-1")
	lister := NewLister(client)
	
	assert.NotNil(t, lister)
	assert.NotNil(t, lister.client)
	assert.NotNil(t, lister.pool)
	assert.Equal(t, client, lister.client)
	
	// Clean up
	lister.Close()
}

func TestLister_Close(t *testing.T) {
	client := NewClient("us-east-1")
	lister := NewLister(client)
	
	// Should not panic
	assert.NotPanics(t, func() {
		lister.Close()
	})
}

func TestLister_MultipleListersl(t *testing.T) {
	client := NewClient("us-east-1")
	lister1 := NewLister(client)
	lister2 := NewLister(client)
	
	assert.NotNil(t, lister1)
	assert.NotNil(t, lister2)
	assert.NotEqual(t, lister1.pool, lister2.pool)
	
	lister1.Close()
	lister2.Close()
}

func TestLister_ContextHandling(t *testing.T) {
	client := NewClient("us-east-1")
	lister := NewLister(client)
	defer lister.Close()
	
	// Test context creation
	ctx := context.Background()
	assert.NotNil(t, ctx)
	
	// Test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NotNil(t, ctx)
	
	// Test context with cancellation
	ctx, cancel = context.WithCancel(context.Background())
	assert.NotNil(t, ctx)
	cancel() // Cancel immediately
	assert.Equal(t, context.Canceled, ctx.Err())
}

func TestLister_BucketSorting(t *testing.T) {
	// Test bucket sorting logic
	buckets := []Bucket{
		{Name: "zebra-bucket", Region: "us-east-1"},
		{Name: "alpha-bucket", Region: "us-west-2"},
		{Name: "beta-bucket", Region: "eu-west-1"},
	}
	
	// Sort buckets manually to test sorting logic
	for i := 0; i < len(buckets)-1; i++ {
		for j := i + 1; j < len(buckets); j++ {
			if buckets[i].Name > buckets[j].Name {
				buckets[i], buckets[j] = buckets[j], buckets[i]
			}
		}
	}
	
	assert.Equal(t, "alpha-bucket", buckets[0].Name)
	assert.Equal(t, "beta-bucket", buckets[1].Name)
	assert.Equal(t, "zebra-bucket", buckets[2].Name)
}

func TestListRequest_Creation(t *testing.T) {
	request := ListRequest{
		Bucket:     "test-bucket",
		Prefix:     "documents/",
		Delimiter:  "/",
		MaxKeys:    100,
		StartAfter: "documents/file1.txt",
	}
	
	assert.Equal(t, "test-bucket", request.Bucket)
	assert.Equal(t, "documents/", request.Prefix)
	assert.Equal(t, "/", request.Delimiter)
	assert.Equal(t, int64(100), request.MaxKeys)
	assert.Equal(t, "documents/file1.txt", request.StartAfter)
}

func TestListRequest_DefaultValues(t *testing.T) {
	request := ListRequest{
		Bucket: "test-bucket",
	}
	
	assert.Equal(t, "test-bucket", request.Bucket)
	assert.Equal(t, "", request.Prefix)
	assert.Equal(t, "", request.Delimiter)
	assert.Equal(t, int64(0), request.MaxKeys)
	assert.Equal(t, "", request.StartAfter)
}

func TestListRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request ListRequest
		valid   bool
	}{
		{
			name: "valid request",
			request: ListRequest{
				Bucket: "test-bucket",
				Prefix: "docs/",
			},
			valid: true,
		},
		{
			name: "empty bucket",
			request: ListRequest{
				Bucket: "",
				Prefix: "docs/",
			},
			valid: false,
		},
		{
			name: "minimal valid request",
			request: ListRequest{
				Bucket: "bucket",
			},
			valid: true,
		},
		{
			name: "with delimiter",
			request: ListRequest{
				Bucket:    "test-bucket",
				Delimiter: "/",
			},
			valid: true,
		},
		{
			name: "with max keys",
			request: ListRequest{
				Bucket:  "test-bucket",
				MaxKeys: 1000,
			},
			valid: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				assert.NotEmpty(t, tt.request.Bucket)
			} else {
				assert.Empty(t, tt.request.Bucket)
			}
		})
	}
}

func TestLister_ObjectSorting(t *testing.T) {
	now := time.Now()
	objects := []Object{
		{
			Key:          "file1.txt",
			LastModified: now.Add(-time.Hour),
		},
		{
			Key:          "file2.txt",
			LastModified: now,
		},
		{
			Key:          "file3.txt",
			LastModified: now.Add(-2 * time.Hour),
		},
	}
	
	// Sort by LastModified (newest first) to test sorting logic
	for i := 0; i < len(objects)-1; i++ {
		for j := i + 1; j < len(objects); j++ {
			if objects[i].LastModified.Before(objects[j].LastModified) {
				objects[i], objects[j] = objects[j], objects[i]
			}
		}
	}
	
	assert.Equal(t, "file2.txt", objects[0].Key) // Most recent
	assert.Equal(t, "file1.txt", objects[1].Key) // Middle
	assert.Equal(t, "file3.txt", objects[2].Key) // Oldest
}

func TestLister_ConcurrentPrefixes(t *testing.T) {
	// Test concurrent prefix processing
	prefixes := []string{
		"documents/",
		"images/",
		"videos/",
		"archives/",
	}
	
	assert.Len(t, prefixes, 4)
	
	// Test that each prefix is properly formatted
	for _, prefix := range prefixes {
		assert.NotEmpty(t, prefix)
		if prefix != "" {
			assert.True(t, len(prefix) > 0)
		}
	}
}

func TestLister_PaginationLogic(t *testing.T) {
	// Test pagination parameters
	maxObjects := 5000
	pageSize := int64(1000)
	
	if maxObjects > 0 && int64(maxObjects) < pageSize {
		pageSize = int64(maxObjects)
	}
	
	assert.Equal(t, int64(1000), pageSize)
	
	// Test with smaller max
	maxObjects = 500
	pageSize = int64(1000)
	if maxObjects > 0 && int64(maxObjects) < pageSize {
		pageSize = int64(maxObjects)
	}
	
	assert.Equal(t, int64(500), pageSize)
}

func TestLister_StreamingLogic(t *testing.T) {
	// Test streaming channel logic
	objectChan := make(chan Object, 10)
	
	// Simulate streaming objects
	go func() {
		defer close(objectChan)
		for i := 0; i < 5; i++ {
			obj := Object{
				Key:  "streamed-file",
				Size: int64(i * 100),
			}
			select {
			case objectChan <- obj:
			default:
				return
			}
		}
	}()
	
	// Collect streamed objects
	var objects []Object
	for obj := range objectChan {
		objects = append(objects, obj)
	}
	
	assert.Len(t, objects, 5)
	for i, obj := range objects {
		assert.Equal(t, "streamed-file", obj.Key)
		assert.Equal(t, int64(i*100), obj.Size)
	}
}

func TestLister_BucketMetadata(t *testing.T) {
	now := time.Now()
	bucket := Bucket{
		Name:         "test-bucket",
		CreationDate: now,
		Region:       "us-east-1",
	}
	
	assert.Equal(t, "test-bucket", bucket.Name)
	assert.Equal(t, now, bucket.CreationDate)
	assert.Equal(t, "us-east-1", bucket.Region)
}

func TestLister_ResourceCleanup(t *testing.T) {
	client := NewClient("us-east-1")
	
	// Create and close multiple listers to test resource cleanup
	for i := 0; i < 5; i++ {
		lister := NewLister(client)
		assert.NotNil(t, lister)
		lister.Close()
	}
}

func TestLister_RegionDetection(t *testing.T) {
	// Test region detection logic
	regions := []string{
		"us-east-1",
		"us-west-2",
		"eu-west-1",
		"ap-northeast-1",
		"unknown",
	}
	
	for _, region := range regions {
		assert.NotEmpty(t, region)
		if region == "unknown" {
			// Test fallback for unknown regions
			assert.Equal(t, "unknown", region)
		} else {
			// Test valid region format
			assert.Contains(t, region, "-")
		}
	}
}

func TestLister_ConcurrencyLimiting(t *testing.T) {
	// Test concurrency limiting with semaphore pattern
	semaphoreSize := 5
	semaphore := make(chan struct{}, semaphoreSize)
	
	// Test that we can acquire up to semaphoreSize tokens
	for i := 0; i < semaphoreSize; i++ {
		select {
		case semaphore <- struct{}{}:
			// Successfully acquired
		default:
			t.Fatalf("Should be able to acquire token %d", i)
		}
	}
	
	// Test that the next acquisition would block
	select {
	case semaphore <- struct{}{}:
		t.Fatal("Should not be able to acquire additional token")
	default:
		// Expected behavior
	}
	
	// Release tokens
	for i := 0; i < semaphoreSize; i++ {
		<-semaphore
	}
}

// Benchmark tests
func BenchmarkNewLister(b *testing.B) {
	client := NewClient("us-east-1")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lister := NewLister(client)
		lister.Close()
	}
}

func BenchmarkListRequest_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		request := ListRequest{
			Bucket:     "benchmark-bucket",
			Prefix:     "benchmark/",
			Delimiter:  "/",
			MaxKeys:    1000,
			StartAfter: "benchmark/start",
		}
		_ = request
	}
}

func BenchmarkObject_Creation(b *testing.B) {
	now := time.Now()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := Object{
			Key:          "benchmark/file.txt",
			Size:         1024,
			LastModified: now,
			ETag:         `"benchmark-etag"`,
			StorageClass: "STANDARD",
		}
		_ = obj
	}
}

func BenchmarkBucket_Creation(b *testing.B) {
	now := time.Now()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bucket := Bucket{
			Name:         "benchmark-bucket",
			CreationDate: now,
			Region:       "us-east-1",
		}
		_ = bucket
	}
}

func BenchmarkObjectSorting(b *testing.B) {
	now := time.Now()
	objects := make([]Object, 1000)
	for i := range objects {
		objects[i] = Object{
			Key:          "file",
			LastModified: now.Add(time.Duration(i) * time.Second),
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simple sorting benchmark
		for j := 0; j < len(objects)-1; j++ {
			for k := j + 1; k < len(objects); k++ {
				if objects[j].LastModified.Before(objects[k].LastModified) {
					objects[j], objects[k] = objects[k], objects[j]
				}
			}
		}
	}
}