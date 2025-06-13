package s3

import (
	"testing"
	"time"

	"github.com/seike460/s3ry/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestToTypesDownloadRequest(t *testing.T) {
	s3Request := DownloadRequest{
		Bucket:   "test-bucket",
		Key:      "test-key",
		FilePath: "/local/path/file.txt",
	}
	
	typesRequest := ToTypesDownloadRequest(s3Request)
	
	assert.Equal(t, s3Request.Bucket, typesRequest.Bucket)
	assert.Equal(t, s3Request.Key, typesRequest.Key)
	assert.Equal(t, s3Request.FilePath, typesRequest.FilePath)
}

func TestToTypesUploadRequest(t *testing.T) {
	metadata := map[string]*string{
		"author": stringPtr("test-user"),
		"type":   stringPtr("document"),
	}
	
	s3Request := UploadRequest{
		Bucket:      "test-bucket",
		Key:         "test-key",
		FilePath:    "/local/path/file.txt",
		ContentType: "text/plain",
		Metadata:    metadata,
	}
	
	typesRequest := ToTypesUploadRequest(s3Request)
	
	assert.Equal(t, s3Request.Bucket, typesRequest.Bucket)
	assert.Equal(t, s3Request.Key, typesRequest.Key)
	assert.Equal(t, s3Request.FilePath, typesRequest.FilePath)
	assert.Equal(t, s3Request.ContentType, typesRequest.ContentType)
	assert.Equal(t, s3Request.Metadata, typesRequest.Metadata)
}

func TestToTypesListRequest(t *testing.T) {
	s3Request := ListRequest{
		Bucket:     "test-bucket",
		Prefix:     "documents/",
		Delimiter:  "/",
		MaxKeys:    100,
		StartAfter: "documents/file1.txt",
	}
	
	typesRequest := ToTypesListRequest(s3Request)
	
	assert.Equal(t, s3Request.Bucket, typesRequest.Bucket)
	assert.Equal(t, s3Request.Prefix, typesRequest.Prefix)
	assert.Equal(t, s3Request.Delimiter, typesRequest.Delimiter)
	assert.Equal(t, s3Request.MaxKeys, typesRequest.MaxKeys)
	assert.Equal(t, s3Request.StartAfter, typesRequest.StartAfter)
}

func TestFromTypesObject(t *testing.T) {
	now := time.Now()
	typesObject := types.Object{
		Key:          "test/file.txt",
		Size:         1024,
		LastModified: now,
		ETag:         `"d41d8cd98f00b204e9800998ecf8427e"`,
		StorageClass: "STANDARD",
	}
	
	s3Object := FromTypesObject(typesObject)
	
	assert.Equal(t, typesObject.Key, s3Object.Key)
	assert.Equal(t, typesObject.Size, s3Object.Size)
	assert.Equal(t, typesObject.LastModified, s3Object.LastModified)
	assert.Equal(t, typesObject.ETag, s3Object.ETag)
	assert.Equal(t, typesObject.StorageClass, s3Object.StorageClass)
}

func TestFromTypesObjects(t *testing.T) {
	now := time.Now()
	typesObjects := []types.Object{
		{
			Key:          "file1.txt",
			Size:         1024,
			LastModified: now,
			ETag:         `"etag1"`,
			StorageClass: "STANDARD",
		},
		{
			Key:          "file2.txt",
			Size:         2048,
			LastModified: now.Add(time.Hour),
			ETag:         `"etag2"`,
			StorageClass: "IA",
		},
		{
			Key:          "file3.txt",
			Size:         4096,
			LastModified: now.Add(2 * time.Hour),
			ETag:         `"etag3"`,
			StorageClass: "GLACIER",
		},
	}
	
	s3Objects := FromTypesObjects(typesObjects)
	
	assert.Len(t, s3Objects, 3)
	
	for i, s3Object := range s3Objects {
		assert.Equal(t, typesObjects[i].Key, s3Object.Key)
		assert.Equal(t, typesObjects[i].Size, s3Object.Size)
		assert.Equal(t, typesObjects[i].LastModified, s3Object.LastModified)
		assert.Equal(t, typesObjects[i].ETag, s3Object.ETag)
		assert.Equal(t, typesObjects[i].StorageClass, s3Object.StorageClass)
	}
}

func TestFromTypesObjects_Empty(t *testing.T) {
	typesObjects := []types.Object{}
	s3Objects := FromTypesObjects(typesObjects)
	
	assert.Len(t, s3Objects, 0)
	assert.NotNil(t, s3Objects) // Should be empty slice, not nil
}

func TestToTypesProgressCallback_Nil(t *testing.T) {
	var nilCallback ProgressCallback
	typesCallback := ToTypesProgressCallback(nilCallback)
	
	assert.Nil(t, typesCallback)
}

func TestToTypesProgressCallback_Valid(t *testing.T) {
	var capturedBytes, capturedTotal int64
	
	s3Callback := func(bytes, total int64) {
		capturedBytes = bytes
		capturedTotal = total
	}
	
	typesCallback := ToTypesProgressCallback(s3Callback)
	assert.NotNil(t, typesCallback)
	
	// Test the conversion by calling the callback
	typesCallback(512, 1024)
	assert.Equal(t, int64(512), capturedBytes)
	assert.Equal(t, int64(1024), capturedTotal)
}

func TestConversion_RoundTrip_DownloadRequest(t *testing.T) {
	original := DownloadRequest{
		Bucket:   "round-trip-bucket",
		Key:      "round-trip-key",
		FilePath: "/round/trip/path",
	}
	
	// Convert to types and back (simulating worker conversion)
	typesRequest := ToTypesDownloadRequest(original)
	
	// Verify the conversion maintains all data
	assert.Equal(t, original.Bucket, typesRequest.Bucket)
	assert.Equal(t, original.Key, typesRequest.Key)
	assert.Equal(t, original.FilePath, typesRequest.FilePath)
}

func TestConversion_RoundTrip_UploadRequest(t *testing.T) {
	metadata := map[string]*string{
		"test": stringPtr("value"),
	}
	
	original := UploadRequest{
		Bucket:      "round-trip-bucket",
		Key:         "round-trip-key",
		FilePath:    "/round/trip/path",
		ContentType: "application/octet-stream",
		Metadata:    metadata,
	}
	
	// Convert to types and back (simulating worker conversion)
	typesRequest := ToTypesUploadRequest(original)
	
	// Verify the conversion maintains all data
	assert.Equal(t, original.Bucket, typesRequest.Bucket)
	assert.Equal(t, original.Key, typesRequest.Key)
	assert.Equal(t, original.FilePath, typesRequest.FilePath)
	assert.Equal(t, original.ContentType, typesRequest.ContentType)
	assert.Equal(t, original.Metadata, typesRequest.Metadata)
}

func TestConversion_RoundTrip_ListRequest(t *testing.T) {
	original := ListRequest{
		Bucket:     "round-trip-bucket",
		Prefix:     "test/prefix/",
		Delimiter:  "/",
		MaxKeys:    1000,
		StartAfter: "test/prefix/start",
	}
	
	// Convert to types and back (simulating worker conversion)
	typesRequest := ToTypesListRequest(original)
	
	// Verify the conversion maintains all data
	assert.Equal(t, original.Bucket, typesRequest.Bucket)
	assert.Equal(t, original.Prefix, typesRequest.Prefix)
	assert.Equal(t, original.Delimiter, typesRequest.Delimiter)
	assert.Equal(t, original.MaxKeys, typesRequest.MaxKeys)
	assert.Equal(t, original.StartAfter, typesRequest.StartAfter)
}

func TestConversion_SpecialCharacters(t *testing.T) {
	// Test conversion with special characters
	s3Request := DownloadRequest{
		Bucket:   "bucket-with-特殊文字",
		Key:      "path/with spaces/file (1).txt",
		FilePath: "/local/path with spaces/file (1).txt",
	}
	
	typesRequest := ToTypesDownloadRequest(s3Request)
	
	assert.Equal(t, s3Request.Bucket, typesRequest.Bucket)
	assert.Equal(t, s3Request.Key, typesRequest.Key)
	assert.Equal(t, s3Request.FilePath, typesRequest.FilePath)
}

func TestConversion_EmptyValues(t *testing.T) {
	// Test conversion with empty values
	s3Request := ListRequest{
		Bucket:     "test-bucket",
		Prefix:     "",
		Delimiter:  "",
		MaxKeys:    0,
		StartAfter: "",
	}
	
	typesRequest := ToTypesListRequest(s3Request)
	
	assert.Equal(t, "test-bucket", typesRequest.Bucket)
	assert.Equal(t, "", typesRequest.Prefix)
	assert.Equal(t, "", typesRequest.Delimiter)
	assert.Equal(t, int64(0), typesRequest.MaxKeys)
	assert.Equal(t, "", typesRequest.StartAfter)
}

// Benchmark tests
func BenchmarkToTypesDownloadRequest(b *testing.B) {
	s3Request := DownloadRequest{
		Bucket:   "benchmark-bucket",
		Key:      "benchmark-key",
		FilePath: "/benchmark/path",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ToTypesDownloadRequest(s3Request)
	}
}

func BenchmarkToTypesUploadRequest(b *testing.B) {
	metadata := map[string]*string{
		"author": stringPtr("test-user"),
	}
	
	s3Request := UploadRequest{
		Bucket:      "benchmark-bucket",
		Key:         "benchmark-key",
		FilePath:    "/benchmark/path",
		ContentType: "text/plain",
		Metadata:    metadata,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ToTypesUploadRequest(s3Request)
	}
}

func BenchmarkFromTypesObject(b *testing.B) {
	now := time.Now()
	typesObject := types.Object{
		Key:          "benchmark/file.txt",
		Size:         1024,
		LastModified: now,
		ETag:         `"benchmark-etag"`,
		StorageClass: "STANDARD",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FromTypesObject(typesObject)
	}
}

func BenchmarkFromTypesObjects(b *testing.B) {
	now := time.Now()
	typesObjects := make([]types.Object, 1000)
	for i := range typesObjects {
		typesObjects[i] = types.Object{
			Key:          "benchmark/file.txt",
			Size:         1024,
			LastModified: now,
			ETag:         `"benchmark-etag"`,
			StorageClass: "STANDARD",
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FromTypesObjects(typesObjects)
	}
}

func BenchmarkProgressCallbackConversion(b *testing.B) {
	callback := func(bytes, total int64) {
		// Simulate some work
		_ = float64(bytes) / float64(total)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		typesCallback := ToTypesProgressCallback(callback)
		_ = typesCallback
	}
}