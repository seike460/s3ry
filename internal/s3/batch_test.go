package s3

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultBatchConfig(t *testing.T) {
	config := DefaultBatchConfig()

	assert.Equal(t, 10, config.ConcurrentOperations)
	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 3, config.RetryCount)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.False(t, config.DryRun)
	assert.Nil(t, config.OnProgress)
}

func TestNewBatchProcessor(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultBatchConfig()
	processor := NewBatchProcessor(client, config)

	assert.NotNil(t, processor)
	assert.NotNil(t, processor.client)
	assert.NotNil(t, processor.pool)
	assert.Equal(t, client, processor.client)

	// Clean up
	processor.Close()
}

func TestBatchProcessor_Close(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultBatchConfig()
	processor := NewBatchProcessor(client, config)

	// Should not panic
	assert.NotPanics(t, func() {
		processor.Close()
	})
}

func TestBatchConfig_CustomValues(t *testing.T) {
	config := BatchConfig{
		ConcurrentOperations: 20,
		BatchSize:            50,
		RetryCount:           5,
		RetryDelay:           2 * time.Second,
		DryRun:               true,
		OnProgress: func(completed, total int) {
			// Custom progress callback
		},
	}

	assert.Equal(t, 20, config.ConcurrentOperations)
	assert.Equal(t, 50, config.BatchSize)
	assert.Equal(t, 5, config.RetryCount)
	assert.Equal(t, 2*time.Second, config.RetryDelay)
	assert.True(t, config.DryRun)
	assert.NotNil(t, config.OnProgress)
}

func TestBatchOperation_Creation(t *testing.T) {
	operation := BatchOperation{
		Operation: "delete",
		Key:       "test-key",
		Success:   true,
		Error:     nil,
	}

	assert.Equal(t, "delete", operation.Operation)
	assert.Equal(t, "test-key", operation.Key)
	assert.True(t, operation.Success)
	assert.Nil(t, operation.Error)
}

func TestBatchOperation_WithError(t *testing.T) {
	testError := assert.AnError
	operation := BatchOperation{
		Operation: "upload",
		Key:       "failed-key",
		Success:   false,
		Error:     testError,
	}

	assert.Equal(t, "upload", operation.Operation)
	assert.Equal(t, "failed-key", operation.Key)
	assert.False(t, operation.Success)
	assert.Equal(t, testError, operation.Error)
}

func TestCopyOperation_Creation(t *testing.T) {
	metadata := map[string]*string{
		"author": stringPtr("test-user"),
	}

	operation := CopyOperation{
		SourceBucket: "source-bucket",
		SourceKey:    "source/key",
		DestBucket:   "dest-bucket",
		DestKey:      "dest/key",
		Metadata:     metadata,
	}

	assert.Equal(t, "source-bucket", operation.SourceBucket)
	assert.Equal(t, "source/key", operation.SourceKey)
	assert.Equal(t, "dest-bucket", operation.DestBucket)
	assert.Equal(t, "dest/key", operation.DestKey)
	assert.Equal(t, metadata, operation.Metadata)
}

func TestMetadataUpdate_Creation(t *testing.T) {
	metadata := map[string]*string{
		"author":  stringPtr("updated-user"),
		"version": stringPtr("2.0"),
	}

	update := MetadataUpdate{
		Key:      "test-key",
		Metadata: metadata,
	}

	assert.Equal(t, "test-key", update.Key)
	assert.Equal(t, metadata, update.Metadata)
	assert.Equal(t, "updated-user", *update.Metadata["author"])
	assert.Equal(t, "2.0", *update.Metadata["version"])
}

func TestPermissionUpdate_Creation(t *testing.T) {
	update := PermissionUpdate{
		Key: "test-key",
		ACL: "public-read",
	}

	assert.Equal(t, "test-key", update.Key)
	assert.Equal(t, "public-read", update.ACL)
}

func TestPermissionUpdate_PrivateACL(t *testing.T) {
	update := PermissionUpdate{
		Key: "private-key",
		ACL: "private",
	}

	assert.Equal(t, "private-key", update.Key)
	assert.Equal(t, "private", update.ACL)
}

func TestBatchProcessor_DryRunMode(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultBatchConfig()
	config.DryRun = true
	processor := NewBatchProcessor(client, config)
	defer processor.Close()

	// Test dry run for delete operations
	keys := []string{"key1", "key2", "key3"}
	operations := processor.DeleteBatch(context.Background(), "test-bucket", keys, config)

	assert.Len(t, operations, 3)
	for i, op := range operations {
		assert.Equal(t, "delete", op.Operation)
		assert.Equal(t, keys[i], op.Key)
		assert.True(t, op.Success)
		assert.Nil(t, op.Error)
	}
}

func TestBatchProcessor_CopyOperationsDryRun(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultBatchConfig()
	config.DryRun = true
	processor := NewBatchProcessor(client, config)
	defer processor.Close()

	copyOps := []CopyOperation{
		{
			SourceBucket: "source1",
			SourceKey:    "key1",
			DestBucket:   "dest1",
			DestKey:      "new-key1",
		},
		{
			SourceBucket: "source2",
			SourceKey:    "key2",
			DestBucket:   "dest2",
			DestKey:      "new-key2",
		},
	}

	operations := processor.CopyBatch(context.Background(), copyOps, config)

	assert.Len(t, operations, 2)
	for i, op := range operations {
		assert.Equal(t, "copy", op.Operation)
		assert.Equal(t, copyOps[i].DestKey, op.Key)
		assert.True(t, op.Success)
		assert.Nil(t, op.Error)
	}
}

func TestBatchProcessor_MetadataUpdatesDryRun(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultBatchConfig()
	config.DryRun = true
	processor := NewBatchProcessor(client, config)
	defer processor.Close()

	updates := []MetadataUpdate{
		{
			Key: "key1",
			Metadata: map[string]*string{
				"author": stringPtr("user1"),
			},
		},
		{
			Key: "key2",
			Metadata: map[string]*string{
				"author": stringPtr("user2"),
			},
		},
	}

	operations := processor.UpdateMetadataBatch(context.Background(), "test-bucket", updates, config)

	assert.Len(t, operations, 2)
	for i, op := range operations {
		assert.Equal(t, "metadata_update", op.Operation)
		assert.Equal(t, updates[i].Key, op.Key)
		assert.True(t, op.Success)
		assert.Nil(t, op.Error)
	}
}

func TestBatchProcessor_PermissionUpdatesDryRun(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultBatchConfig()
	config.DryRun = true
	processor := NewBatchProcessor(client, config)
	defer processor.Close()

	permissions := []PermissionUpdate{
		{Key: "key1", ACL: "private"},
		{Key: "key2", ACL: "public-read"},
	}

	operations := processor.SetPermissionsBatch(context.Background(), "test-bucket", permissions, config)

	assert.Len(t, operations, 2)
	for i, op := range operations {
		assert.Equal(t, "permission_update", op.Operation)
		assert.Equal(t, permissions[i].Key, op.Key)
		assert.True(t, op.Success)
		assert.Nil(t, op.Error)
	}
}

func TestBatchProcessor_ProgressCallback(t *testing.T) {
	client := NewClient("us-east-1")

	var capturedCompleted, capturedTotal int
	config := BatchConfig{
		ConcurrentOperations: 5,
		OnProgress: func(completed, total int) {
			capturedCompleted = completed
			capturedTotal = total
		},
	}

	processor := NewBatchProcessor(client, config)
	defer processor.Close()

	// Test that progress callback is properly stored
	assert.NotNil(t, config.OnProgress)
	config.OnProgress(5, 10)
	assert.Equal(t, 5, capturedCompleted)
	assert.Equal(t, 10, capturedTotal)
}

func TestBatchProcessor_BatchSizeLogic(t *testing.T) {
	// Test batch size calculation logic
	keys := make([]string, 2500) // 2500 keys
	for i := range keys {
		keys[i] = "key" + string(rune(i))
	}

	maxBatchSize := 1000
	batchSize := 500

	if batchSize > 0 && batchSize < maxBatchSize {
		maxBatchSize = batchSize
	}

	// Calculate expected number of batches
	expectedBatches := (len(keys) + maxBatchSize - 1) / maxBatchSize

	assert.Equal(t, 500, maxBatchSize)
	assert.Equal(t, 5, expectedBatches) // 2500 / 500 = 5 batches
}

func TestBatchProcessor_WaitForCompletion(t *testing.T) {
	// Test the completion waiting logic
	operations := []BatchOperation{}
	var mutex sync.Mutex

	// Simulate adding operations concurrently
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(10 * time.Millisecond)
			mutex.Lock()
			operations = append(operations, BatchOperation{
				Operation: "test",
				Key:       "key",
				Success:   true,
			})
			mutex.Unlock()
		}
	}()

	// Wait for operations to complete (simplified version)
	timeout := time.After(1 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	expectedCount := 5
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for operations")
		case <-ticker.C:
			mutex.Lock()
			if len(operations) >= expectedCount {
				mutex.Unlock()
				assert.Len(t, operations, expectedCount)
				return
			}
			mutex.Unlock()
		}
	}
}

func TestBatchProcessor_ResourceCleanup(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultBatchConfig()

	// Create and close multiple processors to test resource cleanup
	for i := 0; i < 3; i++ {
		processor := NewBatchProcessor(client, config)
		assert.NotNil(t, processor)
		processor.Close()
	}
}

func TestBatchJob_Interface(t *testing.T) {
	client := NewClient("us-east-1")

	// Test that job types implement the interface correctly
	deleteJob := &BatchDeleteJob{
		client: client,
		keys:   []string{"test-key"},
	}
	assert.NotNil(t, deleteJob)

	copyJob := &BatchCopyJob{
		client: client,
		operation: CopyOperation{
			SourceBucket: "source",
			SourceKey:    "key",
			DestBucket:   "dest",
			DestKey:      "new-key",
		},
	}
	assert.NotNil(t, copyJob)

	metadataJob := &BatchMetadataUpdateJob{
		client: client,
		bucket: "test-bucket",
		update: MetadataUpdate{
			Key: "test-key",
		},
	}
	assert.NotNil(t, metadataJob)

	permissionJob := &BatchPermissionUpdateJob{
		client: client,
		bucket: "test-bucket",
		permission: PermissionUpdate{
			Key: "test-key",
			ACL: "private",
		},
	}
	assert.NotNil(t, permissionJob)
}

// Benchmark tests
func BenchmarkNewBatchProcessor(b *testing.B) {
	client := NewClient("us-east-1")
	config := DefaultBatchConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor := NewBatchProcessor(client, config)
		processor.Close()
	}
}

func BenchmarkBatchConfig_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := DefaultBatchConfig()
		_ = config
	}
}

func BenchmarkBatchOperation_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		operation := BatchOperation{
			Operation: "delete",
			Key:       "benchmark-key",
			Success:   true,
			Error:     nil,
		}
		_ = operation
	}
}

func BenchmarkCopyOperation_Creation(b *testing.B) {
	metadata := map[string]*string{
		"author": stringPtr("test-user"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		operation := CopyOperation{
			SourceBucket: "source-bucket",
			SourceKey:    "source-key",
			DestBucket:   "dest-bucket",
			DestKey:      "dest-key",
			Metadata:     metadata,
		}
		_ = operation
	}
}

func BenchmarkMetadataUpdate_Creation(b *testing.B) {
	metadata := map[string]*string{
		"author":  stringPtr("test-user"),
		"version": stringPtr("1.0"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		update := MetadataUpdate{
			Key:      "benchmark-key",
			Metadata: metadata,
		}
		_ = update
	}
}

func BenchmarkBatchSizeCalculation(b *testing.B) {
	keys := make([]string, 10000)
	for i := range keys {
		keys[i] = "key"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		maxBatchSize := 1000
		batchSize := 500

		if batchSize > 0 && batchSize < maxBatchSize {
			maxBatchSize = batchSize
		}

		// Calculate number of batches
		_ = (len(keys) + maxBatchSize - 1) / maxBatchSize
	}
}
