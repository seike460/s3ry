package s3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	region := "us-east-1"
	client := NewClient(region)
	
	assert.NotNil(t, client)
	assert.NotNil(t, client.session)
	assert.NotNil(t, client.s3Client)
	assert.NotNil(t, client.uploader)
	assert.NotNil(t, client.downloader)
}

func TestClient_Session(t *testing.T) {
	client := NewClient("us-east-1")
	session := client.Session()
	
	assert.NotNil(t, session)
	assert.Equal(t, client.session, session)
}

func TestClient_S3Client(t *testing.T) {
	client := NewClient("us-east-1")
	s3Client := client.S3Client()
	
	assert.NotNil(t, s3Client)
	assert.Equal(t, client.s3Client, s3Client)
}

func TestClient_S3(t *testing.T) {
	client := NewClient("us-east-1")
	s3Client := client.S3()
	
	assert.NotNil(t, s3Client)
	assert.Equal(t, client.s3Client, s3Client)
}

func TestClient_Uploader(t *testing.T) {
	client := NewClient("us-east-1")
	uploader := client.Uploader()
	
	assert.NotNil(t, uploader)
	assert.Equal(t, client.uploader, uploader)
}

func TestClient_Downloader(t *testing.T) {
	client := NewClient("us-east-1")
	downloader := client.Downloader()
	
	assert.NotNil(t, downloader)
	assert.Equal(t, client.downloader, downloader)
}

func TestClient_DifferentRegions(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-northeast-1"}
	
	for _, region := range regions {
		client := NewClient(region)
		assert.NotNil(t, client, "Client should be created for region: %s", region)
		assert.NotNil(t, client.Session(), "Session should be available for region: %s", region)
	}
}

// Benchmark tests
func BenchmarkNewClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		client := NewClient("us-east-1")
		_ = client
	}
}

func BenchmarkClient_AccessMethods(b *testing.B) {
	client := NewClient("us-east-1")
	
	b.Run("Session", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = client.Session()
		}
	})
	
	b.Run("S3Client", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = client.S3Client()
		}
	})
	
	b.Run("S3", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = client.S3()
		}
	})
	
	b.Run("Uploader", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = client.Uploader()
		}
	})
	
	b.Run("Downloader", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = client.Downloader()
		}
	})
}