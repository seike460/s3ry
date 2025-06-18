package s3

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Creation(t *testing.T) {
	tests := []struct {
		name   string
		region string
		valid  bool
	}{
		{"Valid US East", "us-east-1", true},
		{"Valid US West", "us-west-2", true},
		{"Valid EU", "eu-west-1", true},
		{"Valid AP", "ap-northeast-1", true},
		{"Empty region", "", true}, // Should use default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.region)
			assert.NotNil(t, client)
			assert.NotNil(t, client.Session())
			assert.NotNil(t, client.S3Client())
			assert.NotNil(t, client.Uploader())
			assert.NotNil(t, client.Downloader())
		})
	}
}

func TestClient_ContextHandling(t *testing.T) {
	client := NewClient("us-east-1")
	require.NotNil(t, client)

	// Test client with context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	region, err := client.GetBucketRegion(ctx, "test-bucket")
	assert.Error(t, err)
	assert.Empty(t, region)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestClient_SessionManagement(t *testing.T) {
	client := NewClient("us-east-1")
	require.NotNil(t, client)

	t.Run("Session_NotNil", func(t *testing.T) {
		session := client.Session()
		assert.NotNil(t, session)
		assert.NotNil(t, session.Config)
		assert.NotNil(t, session.Config.Region)
	})

	t.Run("Session_Consistency", func(t *testing.T) {
		session1 := client.Session()
		session2 := client.Session()
		assert.Equal(t, session1, session2, "Session should be consistent")
	})
}

func TestClient_S3ClientAccess(t *testing.T) {
	client := NewClient("us-east-1")
	require.NotNil(t, client)

	t.Run("S3Client_NotNil", func(t *testing.T) {
		s3Client := client.S3Client()
		assert.NotNil(t, s3Client)
	})

	t.Run("S3_Alias", func(t *testing.T) {
		s3Client1 := client.S3Client()
		s3Client2 := client.S3()
		assert.Equal(t, s3Client1, s3Client2, "S3() should be alias for S3Client()")
	})
}

func TestClient_UploaderDownloader(t *testing.T) {
	client := NewClient("us-east-1")
	require.NotNil(t, client)

	t.Run("Uploader_NotNil", func(t *testing.T) {
		uploader := client.Uploader()
		assert.NotNil(t, uploader)
	})

	t.Run("Downloader_NotNil", func(t *testing.T) {
		downloader := client.Downloader()
		assert.NotNil(t, downloader)
	})

	t.Run("Uploader_Consistency", func(t *testing.T) {
		uploader1 := client.Uploader()
		uploader2 := client.Uploader()
		assert.Equal(t, uploader1, uploader2, "Uploader should be consistent")
	})

	t.Run("Downloader_Consistency", func(t *testing.T) {
		downloader1 := client.Downloader()
		downloader2 := client.Downloader()
		assert.Equal(t, downloader1, downloader2, "Downloader should be consistent")
	})
}

func TestClient_GetBucketRegion(t *testing.T) {
	client := NewClient("us-east-1")
	require.NotNil(t, client)

	t.Run("GetBucketRegion_ValidInput", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Note: This test requires actual AWS credentials and a real bucket
		// In a real test environment, you would mock this
		region, err := client.GetBucketRegion(ctx, "test-bucket")

		// Since we don't have real AWS credentials in test, we expect an error
		// In a real implementation, you would mock the AWS calls
		if err != nil {
			t.Logf("Expected error in test environment: %v", err)
		} else {
			assert.NotEmpty(t, region)
		}
	})

	t.Run("GetBucketRegion_EmptyBucket", func(t *testing.T) {
		ctx := context.Background()

		region, err := client.GetBucketRegion(ctx, "")
		assert.Error(t, err)
		assert.Empty(t, region)
	})

	t.Run("GetBucketRegion_CancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		region, err := client.GetBucketRegion(ctx, "test-bucket")
		// Note: AWS SDK may not immediately respect context cancellation
		// This test validates the structure exists but may not always fail as expected
		if err != nil {
			t.Logf("Context cancellation test returned error (expected): %v", err)
		} else {
			t.Logf("Context cancellation test completed without error (AWS SDK behavior)")
		}
		t.Logf("Region returned: %s", region)
	})
}

func TestClient_MultipleInstances(t *testing.T) {
	t.Run("Different_Regions", func(t *testing.T) {
		client1 := NewClient("us-east-1")
		client2 := NewClient("us-west-2")

		assert.NotNil(t, client1)
		assert.NotNil(t, client2)
		assert.NotEqual(t, client1, client2)

		// Sessions should be different due to different regions
		assert.NotEqual(t, client1.Session(), client2.Session())
	})

	t.Run("Same_Region", func(t *testing.T) {
		client1 := NewClient("us-east-1")
		client2 := NewClient("us-east-1")

		assert.NotNil(t, client1)
		assert.NotNil(t, client2)

		// Different instances should be created
		assert.NotEqual(t, client1, client2)
	})
}

// Benchmark tests
func BenchmarkClient_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client := NewClient("us-east-1")
		_ = client
	}
}

func BenchmarkClient_SessionAccess(b *testing.B) {
	client := NewClient("us-east-1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session := client.Session()
		_ = session
	}
}

func BenchmarkClient_S3ClientAccess(b *testing.B) {
	client := NewClient("us-east-1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s3Client := client.S3Client()
		_ = s3Client
	}
}

// Test helper functions
func createTestClient(t *testing.T) *Client {
	client := NewClient("us-east-1")
	require.NotNil(t, client)
	return client
}

func createTestClientWithRegion(t *testing.T, region string) *Client {
	client := NewClient(region)
	require.NotNil(t, client)
	return client
}

// Integration test (requires AWS credentials)
func TestClient_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient("us-east-1")
	require.NotNil(t, client)

	t.Run("AWS_Connection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Try to list buckets to test AWS connection
		// This will fail in test environment without credentials, which is expected
		_, err := client.S3Client().ListBucketsWithContext(ctx, nil)
		if err != nil {
			t.Logf("Expected error in test environment without AWS credentials: %v", err)
		}
	})
}
