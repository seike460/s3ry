package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestObject_Creation(t *testing.T) {
	now := time.Now()
	obj := Object{
		Key:          "test-object.txt",
		Size:         1024,
		LastModified: now,
		ETag:         "abc123",
		StorageClass: "STANDARD",
	}

	assert.Equal(t, "test-object.txt", obj.Key)
	assert.Equal(t, int64(1024), obj.Size)
	assert.Equal(t, now, obj.LastModified)
	assert.Equal(t, "abc123", obj.ETag)
	assert.Equal(t, "STANDARD", obj.StorageClass)
}

func TestBucket_Creation(t *testing.T) {
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

func TestUploadRequest_Creation(t *testing.T) {
	userVal := "test-user"
	envVal := "test"
	metadata := map[string]*string{
		"x-amz-meta-user": &userVal,
		"x-amz-meta-env":  &envVal,
	}

	req := UploadRequest{
		Bucket:      "test-bucket",
		Key:         "test-file.txt",
		FilePath:    "/path/to/file.txt",
		ContentType: "text/plain",
		Metadata:    metadata,
	}

	assert.Equal(t, "test-bucket", req.Bucket)
	assert.Equal(t, "test-file.txt", req.Key)
	assert.Equal(t, "/path/to/file.txt", req.FilePath)
	assert.Equal(t, "text/plain", req.ContentType)
	assert.Equal(t, metadata, req.Metadata)
	assert.Len(t, req.Metadata, 2)
}

func TestDownloadRequest_Creation(t *testing.T) {
	req := DownloadRequest{
		Bucket:   "test-bucket",
		Key:      "test-file.txt",
		FilePath: "/path/to/download.txt",
	}

	assert.Equal(t, "test-bucket", req.Bucket)
	assert.Equal(t, "test-file.txt", req.Key)
	assert.Equal(t, "/path/to/download.txt", req.FilePath)
}

func TestListRequest_Creation(t *testing.T) {
	req := ListRequest{
		Bucket:    "test-bucket",
		Prefix:    "photos/",
		Delimiter: "/",
		MaxKeys:   1000,
	}

	assert.Equal(t, "test-bucket", req.Bucket)
	assert.Equal(t, "photos/", req.Prefix)
	assert.Equal(t, "/", req.Delimiter)
	assert.Equal(t, int64(1000), req.MaxKeys)
}

func TestProgressCallback_Invocation(t *testing.T) {
	var receivedCurrent, receivedTotal int64
	callback := ProgressCallback(func(current, total int64) {
		receivedCurrent = current
		receivedTotal = total
	})

	callback(512, 1024)

	assert.Equal(t, int64(512), receivedCurrent)
	assert.Equal(t, int64(1024), receivedTotal)
}

func TestProgressCallback_Nil(t *testing.T) {
	var callback ProgressCallback
	
	// Should not panic when nil
	assert.NotPanics(t, func() {
		if callback != nil {
			callback(100, 200)
		}
	})
}

func TestObject_IsDirectory(t *testing.T) {
	tests := []struct {
		key        string
		isDir      bool
		name       string
	}{
		{"folder/", true, "folder with trailing slash"},
		{"file.txt", false, "regular file"},
		{"", false, "empty key"},
		{"folder/subfolder/", true, "nested folder"},
		{"folder/file.txt", false, "file in folder"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := Object{Key: tt.key}
			// This would require a method to be added to Object type
			// For now, we test the key format
			isDir := obj.Key != "" && obj.Key[len(obj.Key)-1] == '/'
			assert.Equal(t, tt.isDir, isDir, "Object key: %s", tt.key)
		})
	}
}

func TestRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     interface{}
		isValid bool
	}{
		{
			name: "valid upload request",
			req: UploadRequest{
				Bucket:   "valid-bucket",
				Key:      "valid-key.txt",
				FilePath: "/path/to/file.txt",
			},
			isValid: true,
		},
		{
			name: "invalid upload request - empty bucket",
			req: UploadRequest{
				Bucket:   "",
				Key:      "valid-key.txt",
				FilePath: "/path/to/file.txt",
			},
			isValid: false,
		},
		{
			name: "valid download request",
			req: DownloadRequest{
				Bucket:   "valid-bucket",
				Key:      "valid-key.txt",
				FilePath: "/path/to/download.txt",
			},
			isValid: true,
		},
		{
			name: "invalid download request - empty key",
			req: DownloadRequest{
				Bucket:   "valid-bucket",
				Key:      "",
				FilePath: "/path/to/download.txt",
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var isValid bool
			switch req := tt.req.(type) {
			case UploadRequest:
				isValid = req.Bucket != "" && req.Key != "" && req.FilePath != ""
			case DownloadRequest:
				isValid = req.Bucket != "" && req.Key != "" && req.FilePath != ""
			}
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

func TestTypes_ZeroValues(t *testing.T) {
	var obj Object
	var bucket Bucket
	var upload UploadRequest
	var download DownloadRequest
	var list ListRequest

	assert.Empty(t, obj.Key)
	assert.Equal(t, int64(0), obj.Size)
	assert.True(t, obj.LastModified.IsZero())

	assert.Empty(t, bucket.Name)
	assert.True(t, bucket.CreationDate.IsZero())

	assert.Empty(t, upload.Bucket)
	assert.Nil(t, upload.Metadata)

	assert.Empty(t, download.Bucket)

	assert.Empty(t, list.Bucket)
	assert.Equal(t, int64(0), list.MaxKeys)
}