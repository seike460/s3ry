package s3

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/seike460/s3ry/internal/worker"
)

// BatchProcessor provides batch operations for S3
type BatchProcessor struct {
	client *Client
	pool   *worker.Pool
}

// BatchConfig configures batch operation behavior
type BatchConfig struct {
	ConcurrentOperations int           // Number of concurrent operations
	BatchSize            int           // Size of each batch
	RetryCount           int           // Number of retries for failed operations
	RetryDelay           time.Duration // Delay between retries
	DryRun               bool          // Whether to perform a dry run
	OnProgress           func(completed, total int) // Progress callback
}

// DefaultBatchConfig returns default batch configuration
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		ConcurrentOperations: 10,
		BatchSize:            100,
		RetryCount:           3,
		RetryDelay:           1 * time.Second,
		DryRun:               false,
	}
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(client *Client, config BatchConfig) *BatchProcessor {
	poolConfig := worker.DefaultConfig()
	poolConfig.Workers = config.ConcurrentOperations
	pool := worker.NewPool(poolConfig)
	pool.Start()

	return &BatchProcessor{
		client: client,
		pool:   pool,
	}
}

// Close stops the batch processor and cleans up resources
func (bp *BatchProcessor) Close() {
	bp.pool.Stop()
}

// DeleteBatch deletes multiple objects concurrently
func (bp *BatchProcessor) DeleteBatch(ctx context.Context, bucket string, keys []string, config BatchConfig) []BatchOperation {
	if config.DryRun {
		// Return simulated results for dry run
		operations := make([]BatchOperation, len(keys))
		for i, key := range keys {
			operations[i] = BatchOperation{
				Operation: "delete",
				Key:       key,
				Success:   true,
				Error:     nil,
			}
		}
		return operations
	}

	// Use AWS S3 batch delete API for better performance
	return bp.deleteBatchOptimized(ctx, bucket, keys, config)
}

// deleteBatchOptimized uses AWS S3's native batch delete API
func (bp *BatchProcessor) deleteBatchOptimized(ctx context.Context, bucket string, keys []string, config BatchConfig) []BatchOperation {
	operations := make([]BatchOperation, 0, len(keys))
	var mutex sync.Mutex

	// Process keys in batches (AWS allows up to 1000 objects per delete request)
	maxBatchSize := 1000
	if config.BatchSize > 0 && config.BatchSize < maxBatchSize {
		maxBatchSize = config.BatchSize
	}

	for i := 0; i < len(keys); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(keys) {
			end = len(keys)
		}

		batchKeys := keys[i:end]
		
		// Create delete objects input
		deleteObjects := make([]*s3.ObjectIdentifier, len(batchKeys))
		for j, key := range batchKeys {
			deleteObjects[j] = &s3.ObjectIdentifier{
				Key: aws.String(key),
			}
		}

		input := &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &s3.Delete{
				Objects: deleteObjects,
				Quiet:   aws.Bool(false), // We want to know which objects were deleted
			},
		}

		// Submit batch delete job
		job := &BatchDeleteJob{
			client:    bp.client,
			input:     input,
			keys:      batchKeys,
			mutex:     &mutex,
			operations: &operations,
		}

		if err := bp.pool.Submit(job); err != nil {
			// Add error operations for this batch
			mutex.Lock()
			for _, key := range batchKeys {
				operations = append(operations, BatchOperation{
					Operation: "delete",
					Key:       key,
					Success:   false,
					Error:     err,
				})
			}
			mutex.Unlock()
		}
	}

	// Wait for all batch operations to complete
	return bp.waitForBatchCompletion(len(keys), operations, config)
}

// CopyBatch copies multiple objects concurrently
func (bp *BatchProcessor) CopyBatch(ctx context.Context, operations []CopyOperation, config BatchConfig) []BatchOperation {
	if config.DryRun {
		// Return simulated results for dry run
		results := make([]BatchOperation, len(operations))
		for i, op := range operations {
			results[i] = BatchOperation{
				Operation: "copy",
				Key:       op.DestKey,
				Success:   true,
				Error:     nil,
			}
		}
		return results
	}

	jobs := make([]worker.Job, len(operations))
	for i, op := range operations {
		jobs[i] = &BatchCopyJob{
			client:    bp.client,
			operation: op,
		}
	}

	batchProcessor := worker.NewBatchProcessor(bp.pool, config.OnProgress)
	results := batchProcessor.ProcessBatch(jobs)

	// Convert to batch operations
	batchResults := make([]BatchOperation, len(results))
	for i, result := range results {
		batchResults[i] = BatchOperation{
			Operation: "copy",
			Key:       operations[i].DestKey,
			Success:   result.Error == nil,
			Error:     result.Error,
		}
	}

	return batchResults
}

// UpdateMetadataBatch updates metadata for multiple objects concurrently
func (bp *BatchProcessor) UpdateMetadataBatch(ctx context.Context, bucket string, updates []MetadataUpdate, config BatchConfig) []BatchOperation {
	if config.DryRun {
		// Return simulated results for dry run
		results := make([]BatchOperation, len(updates))
		for i, update := range updates {
			results[i] = BatchOperation{
				Operation: "metadata_update",
				Key:       update.Key,
				Success:   true,
				Error:     nil,
			}
		}
		return results
	}

	jobs := make([]worker.Job, len(updates))
	for i, update := range updates {
		jobs[i] = &BatchMetadataUpdateJob{
			client: bp.client,
			bucket: bucket,
			update: update,
		}
	}

	batchProcessor := worker.NewBatchProcessor(bp.pool, config.OnProgress)
	results := batchProcessor.ProcessBatch(jobs)

	// Convert to batch operations
	batchResults := make([]BatchOperation, len(results))
	for i, result := range results {
		batchResults[i] = BatchOperation{
			Operation: "metadata_update",
			Key:       updates[i].Key,
			Success:   result.Error == nil,
			Error:     result.Error,
		}
	}

	return batchResults
}

// SetPermissionsBatch sets permissions for multiple objects concurrently
func (bp *BatchProcessor) SetPermissionsBatch(ctx context.Context, bucket string, permissions []PermissionUpdate, config BatchConfig) []BatchOperation {
	if config.DryRun {
		// Return simulated results for dry run
		results := make([]BatchOperation, len(permissions))
		for i, perm := range permissions {
			results[i] = BatchOperation{
				Operation: "permission_update",
				Key:       perm.Key,
				Success:   true,
				Error:     nil,
			}
		}
		return results
	}

	jobs := make([]worker.Job, len(permissions))
	for i, perm := range permissions {
		jobs[i] = &BatchPermissionUpdateJob{
			client:     bp.client,
			bucket:     bucket,
			permission: perm,
		}
	}

	batchProcessor := worker.NewBatchProcessor(bp.pool, config.OnProgress)
	results := batchProcessor.ProcessBatch(jobs)

	// Convert to batch operations
	batchResults := make([]BatchOperation, len(results))
	for i, result := range results {
		batchResults[i] = BatchOperation{
			Operation: "permission_update",
			Key:       permissions[i].Key,
			Success:   result.Error == nil,
			Error:     result.Error,
		}
	}

	return batchResults
}

// waitForBatchCompletion waits for all batch operations to complete
func (bp *BatchProcessor) waitForBatchCompletion(expectedCount int, operations []BatchOperation, config BatchConfig) []BatchOperation {
	timeout := time.After(30 * time.Second) // 30 second timeout
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			// Timeout reached, return what we have
			return operations
		case <-ticker.C:
			if len(operations) >= expectedCount {
				return operations
			}
		}
	}
}

// CopyOperation represents a copy operation
type CopyOperation struct {
	SourceBucket string
	SourceKey    string
	DestBucket   string
	DestKey      string
	Metadata     map[string]*string
}

// MetadataUpdate represents a metadata update operation
type MetadataUpdate struct {
	Key      string
	Metadata map[string]*string
}

// PermissionUpdate represents a permission update operation
type PermissionUpdate struct {
	Key string
	ACL string // "private", "public-read", etc.
}

// Batch job implementations

// BatchDeleteJob implements worker.Job for batch delete operations
type BatchDeleteJob struct {
	client     *Client
	input      *s3.DeleteObjectsInput
	keys       []string
	mutex      *sync.Mutex
	operations *[]BatchOperation
}

func (j *BatchDeleteJob) Execute(ctx context.Context) error {
	output, err := j.client.s3Client.DeleteObjectsWithContext(ctx, j.input)
	
	j.mutex.Lock()
	defer j.mutex.Unlock()

	if err != nil {
		// Add error operations for all keys in this batch
		for _, key := range j.keys {
			*j.operations = append(*j.operations, BatchOperation{
				Operation: "delete",
				Key:       key,
				Success:   false,
				Error:     err,
			})
		}
		return err
	}

	// Track successful deletions
	successMap := make(map[string]bool)
	for _, deleted := range output.Deleted {
		if deleted.Key != nil {
			successMap[*deleted.Key] = true
		}
	}

	// Track errors
	errorMap := make(map[string]error)
	for _, deleteError := range output.Errors {
		if deleteError.Key != nil && deleteError.Message != nil {
			errorMap[*deleteError.Key] = fmt.Errorf(*deleteError.Message)
		}
	}

	// Create batch operations for all keys
	for _, key := range j.keys {
		if successMap[key] {
			*j.operations = append(*j.operations, BatchOperation{
				Operation: "delete",
				Key:       key,
				Success:   true,
				Error:     nil,
			})
		} else if err, hasError := errorMap[key]; hasError {
			*j.operations = append(*j.operations, BatchOperation{
				Operation: "delete",
				Key:       key,
				Success:   false,
				Error:     err,
			})
		} else {
			// Unknown status
			*j.operations = append(*j.operations, BatchOperation{
				Operation: "delete",
				Key:       key,
				Success:   false,
				Error:     fmt.Errorf("unknown delete status"),
			})
		}
	}

	return nil
}

// BatchCopyJob implements worker.Job for copy operations
type BatchCopyJob struct {
	client    *Client
	operation CopyOperation
}

func (j *BatchCopyJob) Execute(ctx context.Context) error {
	copySource := fmt.Sprintf("%s/%s", j.operation.SourceBucket, j.operation.SourceKey)
	
	input := &s3.CopyObjectInput{
		Bucket:     aws.String(j.operation.DestBucket),
		Key:        aws.String(j.operation.DestKey),
		CopySource: aws.String(copySource),
	}

	if j.operation.Metadata != nil {
		input.Metadata = j.operation.Metadata
		input.MetadataDirective = aws.String("REPLACE")
	}

	_, err := j.client.s3Client.CopyObjectWithContext(ctx, input)
	return err
}

// BatchMetadataUpdateJob implements worker.Job for metadata updates
type BatchMetadataUpdateJob struct {
	client *Client
	bucket string
	update MetadataUpdate
}

func (j *BatchMetadataUpdateJob) Execute(ctx context.Context) error {
	copySource := fmt.Sprintf("%s/%s", j.bucket, j.update.Key)
	
	input := &s3.CopyObjectInput{
		Bucket:            aws.String(j.bucket),
		Key:               aws.String(j.update.Key),
		CopySource:        aws.String(copySource),
		Metadata:          j.update.Metadata,
		MetadataDirective: aws.String("REPLACE"),
	}

	_, err := j.client.s3Client.CopyObjectWithContext(ctx, input)
	return err
}

// BatchPermissionUpdateJob implements worker.Job for permission updates
type BatchPermissionUpdateJob struct {
	client     *Client
	bucket     string
	permission PermissionUpdate
}

func (j *BatchPermissionUpdateJob) Execute(ctx context.Context) error {
	input := &s3.PutObjectAclInput{
		Bucket: aws.String(j.bucket),
		Key:    aws.String(j.permission.Key),
		ACL:    aws.String(j.permission.ACL),
	}

	_, err := j.client.s3Client.PutObjectAclWithContext(ctx, input)
	return err
}