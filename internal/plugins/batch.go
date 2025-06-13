package plugins

import (
	"context"
	"fmt"
	"time"

	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/pkg/interfaces"
	"github.com/seike460/s3ry/pkg/types"
)

// HighCapacityBatchProcessor implements BatchProcessor for high-capacity batch operations (1000+ objects)
type HighCapacityBatchProcessor struct {
	client         interfaces.S3Client
	batchProcessor *s3.BatchProcessor
	logger         Logger
}

// NewHighCapacityBatchProcessor creates a new high-capacity batch processor
func NewHighCapacityBatchProcessor(client interfaces.S3Client, logger Logger) *HighCapacityBatchProcessor {
	// Create S3 client for internal batch processor
	s3Client := &s3.Client{}
	// Note: In a real implementation, we'd need to properly initialize the s3.Client
	// For now, we'll create a basic batch processor configuration
	
	batchConfig := s3.DefaultBatchConfig()
	batchConfig.ConcurrentOperations = 50  // High concurrency for large batches
	batchConfig.BatchSize = 1000           // AWS maximum batch size
	batchConfig.RetryCount = 5             // More retries for reliability
	batchConfig.RetryDelay = 2 * time.Second
	
	batchProcessor := s3.NewBatchProcessor(s3Client, batchConfig)
	
	return &HighCapacityBatchProcessor{
		client:         client,
		batchProcessor: batchProcessor,
		logger:         logger,
	}
}

// Metadata returns plugin metadata
func (p *HighCapacityBatchProcessor) Metadata() PluginMetadata {
	return PluginMetadata{
		Name:        "high-capacity-batch-processor",
		Version:     "1.0.0",
		Description: "High-capacity batch processor for 1000+ S3 object operations",
		Author:      "s3ry",
		License:     "MIT",
		Tags:        []string{"s3", "batch", "high-capacity", "performance"},
	}
}

// SupportedOperations returns supported operations
func (p *HighCapacityBatchProcessor) SupportedOperations() []S3Operation {
	return []S3Operation{OperationBatch}
}

// Initialize initializes the plugin
func (p *HighCapacityBatchProcessor) Initialize(config map[string]interface{}) error {
	p.logger.Info("High-capacity batch processor initialized")
	return nil
}

// Execute executes the batch operation
func (p *HighCapacityBatchProcessor) Execute(ctx OperationContext, args map[string]interface{}) (*OperationResult, error) {
	batchRequest, err := p.parseBatchRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid batch request: %w", err)
	}

	// Check if this is a high-capacity operation (1000+ items)
	if len(batchRequest.Items) < 1000 {
		return &OperationResult{
			Success: false,
			Message: "High-capacity batch processor requires 1000+ items",
			Error:   "insufficient batch size",
		}, nil
	}

	// Execute batch operation based on type
	var result *BatchProcessingResult
	switch batchRequest.Operation {
	case "delete":
		result, err = p.processBatchDelete(ctx.Context, batchRequest)
	case "copy":
		result, err = p.processBatchCopy(ctx.Context, batchRequest)
	case "metadata_update":
		result, err = p.processBatchMetadataUpdate(ctx.Context, batchRequest)
	case "permission_update":
		result, err = p.processBatchPermissionUpdate(ctx.Context, batchRequest)
	default:
		return nil, fmt.Errorf("unsupported batch operation: %s", batchRequest.Operation)
	}

	if err != nil {
		return &OperationResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &OperationResult{
		Success:        result.OverallSuccess,
		Message:        result.Summary,
		Data:           result,
		BytesTotal:     int64(len(batchRequest.Items)),
		BytesProcessed: int64(result.SuccessCount),
	}, nil
}

// ProcessBatch implements BatchProcessor interface
func (p *HighCapacityBatchProcessor) ProcessBatch(ctx OperationContext, items []types.BatchItem) (*OperationResult, error) {
	if len(items) < 1000 {
		return &OperationResult{
			Success: false,
			Message: "High-capacity batch processor requires 1000+ items",
			Error:   "insufficient batch size",
		}, nil
	}

	// Convert types.BatchItem to internal format
	batchItems := make([]BatchItem, len(items))
	for i, item := range items {
		batchItems[i] = BatchItem{
			Key:       item.Key,
			Operation: item.Operation,
			Metadata:  item.Metadata,
		}
	}

	batchRequest := &BatchRequest{
		Operation: "mixed", // Mixed operations
		Bucket:    ctx.Bucket,
		Items:     batchItems,
	}

	result, err := p.processMixedBatch(ctx.Context, batchRequest)
	if err != nil {
		return &OperationResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &OperationResult{
		Success:        result.OverallSuccess,
		Message:        result.Summary,
		Data:           result,
		BytesTotal:     int64(len(items)),
		BytesProcessed: int64(result.SuccessCount),
	}, nil
}

// Cleanup cleans up resources
func (p *HighCapacityBatchProcessor) Cleanup() error {
	if p.batchProcessor != nil {
		p.batchProcessor.Close()
	}
	p.logger.Info("High-capacity batch processor cleaned up")
	return nil
}

// Priority returns high priority for large batches
func (p *HighCapacityBatchProcessor) Priority() PluginPriority {
	return PriorityHigh
}

// ShouldExecute determines if this processor should handle the batch
func (p *HighCapacityBatchProcessor) ShouldExecute(ctx OperationContext, args map[string]interface{}) bool {
	// Only execute for large batches (1000+ items)
	if items, ok := args["items"].([]interface{}); ok {
		return len(items) >= 1000
	}
	return false
}

// Internal types and methods

type BatchRequest struct {
	Operation string      `json:"operation"`
	Bucket    string      `json:"bucket"`
	Items     []BatchItem `json:"items"`
	Config    BatchConfig `json:"config"`
	DryRun    bool        `json:"dry_run"`
}

type BatchItem struct {
	Key       string                 `json:"key"`
	Operation string                 `json:"operation"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type BatchConfig struct {
	ConcurrentOperations int           `json:"concurrent_operations"`
	BatchSize            int           `json:"batch_size"`
	RetryCount           int           `json:"retry_count"`
	RetryDelay           time.Duration `json:"retry_delay"`
}

type BatchProcessingResult struct {
	OverallSuccess bool                   `json:"overall_success"`
	SuccessCount   int                    `json:"success_count"`
	ErrorCount     int                    `json:"error_count"`
	TotalCount     int                    `json:"total_count"`
	Duration       time.Duration          `json:"duration"`
	Summary        string                 `json:"summary"`
	Operations     []s3.BatchOperation    `json:"operations"`
	Stats          map[string]interface{} `json:"stats"`
}

// processBatchDelete handles batch delete operations
func (p *HighCapacityBatchProcessor) processBatchDelete(ctx context.Context, request *BatchRequest) (*BatchProcessingResult, error) {
	startTime := time.Now()
	
	// Extract keys for deletion
	keys := make([]string, len(request.Items))
	for i, item := range request.Items {
		keys[i] = item.Key
	}

	// Configure batch processing
	config := s3.DefaultBatchConfig()
	config.ConcurrentOperations = 50
	config.BatchSize = 1000
	config.RetryCount = 5
	config.DryRun = request.DryRun

	// Progress tracking
	config.OnProgress = func(completed, total int) {
		if p.logger != nil {
			p.logger.Info("Batch delete progress: %d/%d (%.1f%%)", completed, total, float64(completed)/float64(total)*100)
		}
	}

	// Execute batch delete
	operations := p.batchProcessor.DeleteBatch(ctx, request.Bucket, keys, config)
	duration := time.Since(startTime)

	// Analyze results
	successCount := 0
	errorCount := 0
	for _, op := range operations {
		if op.Success {
			successCount++
		} else {
			errorCount++
		}
	}

	result := &BatchProcessingResult{
		OverallSuccess: errorCount == 0,
		SuccessCount:   successCount,
		ErrorCount:     errorCount,
		TotalCount:     len(operations),
		Duration:       duration,
		Operations:     operations,
		Stats: map[string]interface{}{
			"operations_per_second": float64(len(operations)) / duration.Seconds(),
			"success_rate":          float64(successCount) / float64(len(operations)),
			"average_batch_size":    config.BatchSize,
		},
	}

	if request.DryRun {
		result.Summary = fmt.Sprintf("DRY RUN: Would delete %d objects from %s", len(keys), request.Bucket)
	} else {
		result.Summary = fmt.Sprintf("Deleted %d/%d objects from %s in %v", successCount, len(keys), request.Bucket, duration)
	}

	return result, nil
}

// processBatchCopy handles batch copy operations
func (p *HighCapacityBatchProcessor) processBatchCopy(ctx context.Context, request *BatchRequest) (*BatchProcessingResult, error) {
	startTime := time.Now()

	// Convert items to copy operations
	copyOps := make([]s3.CopyOperation, len(request.Items))
	for i, item := range request.Items {
		// Parse copy metadata from item
		sourceBucket := request.Bucket
		if sb, ok := item.Metadata["source_bucket"].(string); ok {
			sourceBucket = sb
		}
		destBucket := request.Bucket
		if db, ok := item.Metadata["dest_bucket"].(string); ok {
			destBucket = db
		}
		sourceKey := item.Key
		if sk, ok := item.Metadata["source_key"].(string); ok {
			sourceKey = sk
		}

		copyOps[i] = s3.CopyOperation{
			SourceBucket: sourceBucket,
			SourceKey:    sourceKey,
			DestBucket:   destBucket,
			DestKey:      item.Key,
		}
	}

	// Configure batch processing
	config := s3.DefaultBatchConfig()
	config.ConcurrentOperations = 25 // Copy operations are more resource-intensive
	config.BatchSize = 500           // Smaller batches for copy operations
	config.RetryCount = 3
	config.DryRun = request.DryRun

	// Execute batch copy
	operations := p.batchProcessor.CopyBatch(ctx, copyOps, config)
	duration := time.Since(startTime)

	// Analyze results
	successCount := 0
	errorCount := 0
	for _, op := range operations {
		if op.Success {
			successCount++
		} else {
			errorCount++
		}
	}

	result := &BatchProcessingResult{
		OverallSuccess: errorCount == 0,
		SuccessCount:   successCount,
		ErrorCount:     errorCount,
		TotalCount:     len(operations),
		Duration:       duration,
		Operations:     operations,
		Stats: map[string]interface{}{
			"operations_per_second": float64(len(operations)) / duration.Seconds(),
			"success_rate":          float64(successCount) / float64(len(operations)),
		},
	}

	if request.DryRun {
		result.Summary = fmt.Sprintf("DRY RUN: Would copy %d objects", len(copyOps))
	} else {
		result.Summary = fmt.Sprintf("Copied %d/%d objects in %v", successCount, len(copyOps), duration)
	}

	return result, nil
}

// processBatchMetadataUpdate handles batch metadata update operations
func (p *HighCapacityBatchProcessor) processBatchMetadataUpdate(ctx context.Context, request *BatchRequest) (*BatchProcessingResult, error) {
	startTime := time.Now()

	// Convert items to metadata updates
	metadataUpdates := make([]s3.MetadataUpdate, len(request.Items))
	for i, item := range request.Items {
		metadata := make(map[string]*string)
		for k, v := range item.Metadata {
			if str, ok := v.(string); ok {
				metadata[k] = &str
			}
		}
		metadataUpdates[i] = s3.MetadataUpdate{
			Key:      item.Key,
			Metadata: metadata,
		}
	}

	// Configure batch processing
	config := s3.DefaultBatchConfig()
	config.ConcurrentOperations = 30
	config.BatchSize = 500
	config.RetryCount = 3
	config.DryRun = request.DryRun

	// Execute batch metadata update
	operations := p.batchProcessor.UpdateMetadataBatch(ctx, request.Bucket, metadataUpdates, config)
	duration := time.Since(startTime)

	// Analyze results
	successCount := 0
	errorCount := 0
	for _, op := range operations {
		if op.Success {
			successCount++
		} else {
			errorCount++
		}
	}

	result := &BatchProcessingResult{
		OverallSuccess: errorCount == 0,
		SuccessCount:   successCount,
		ErrorCount:     errorCount,
		TotalCount:     len(operations),
		Duration:       duration,
		Operations:     operations,
		Stats: map[string]interface{}{
			"operations_per_second": float64(len(operations)) / duration.Seconds(),
			"success_rate":          float64(successCount) / float64(len(operations)),
		},
	}

	if request.DryRun {
		result.Summary = fmt.Sprintf("DRY RUN: Would update metadata for %d objects", len(metadataUpdates))
	} else {
		result.Summary = fmt.Sprintf("Updated metadata for %d/%d objects in %v", successCount, len(metadataUpdates), duration)
	}

	return result, nil
}

// processBatchPermissionUpdate handles batch permission update operations
func (p *HighCapacityBatchProcessor) processBatchPermissionUpdate(ctx context.Context, request *BatchRequest) (*BatchProcessingResult, error) {
	startTime := time.Now()

	// Convert items to permission updates
	permissionUpdates := make([]s3.PermissionUpdate, len(request.Items))
	for i, item := range request.Items {
		acl := "private" // default
		if aclVal, ok := item.Metadata["acl"].(string); ok {
			acl = aclVal
		}
		permissionUpdates[i] = s3.PermissionUpdate{
			Key: item.Key,
			ACL: acl,
		}
	}

	// Configure batch processing
	config := s3.DefaultBatchConfig()
	config.ConcurrentOperations = 30
	config.BatchSize = 500
	config.RetryCount = 3
	config.DryRun = request.DryRun

	// Execute batch permission update
	operations := p.batchProcessor.SetPermissionsBatch(ctx, request.Bucket, permissionUpdates, config)
	duration := time.Since(startTime)

	// Analyze results
	successCount := 0
	errorCount := 0
	for _, op := range operations {
		if op.Success {
			successCount++
		} else {
			errorCount++
		}
	}

	result := &BatchProcessingResult{
		OverallSuccess: errorCount == 0,
		SuccessCount:   successCount,
		ErrorCount:     errorCount,
		TotalCount:     len(operations),
		Duration:       duration,
		Operations:     operations,
		Stats: map[string]interface{}{
			"operations_per_second": float64(len(operations)) / duration.Seconds(),
			"success_rate":          float64(successCount) / float64(len(operations)),
		},
	}

	if request.DryRun {
		result.Summary = fmt.Sprintf("DRY RUN: Would update permissions for %d objects", len(permissionUpdates))
	} else {
		result.Summary = fmt.Sprintf("Updated permissions for %d/%d objects in %v", successCount, len(permissionUpdates), duration)
	}

	return result, nil
}

// processMixedBatch handles mixed batch operations
func (p *HighCapacityBatchProcessor) processMixedBatch(ctx context.Context, request *BatchRequest) (*BatchProcessingResult, error) {
	startTime := time.Now()

	// Group operations by type
	deleteItems := make([]string, 0)
	copyItems := make([]s3.CopyOperation, 0)
	metadataItems := make([]s3.MetadataUpdate, 0)
	permissionItems := make([]s3.PermissionUpdate, 0)

	for _, item := range request.Items {
		switch item.Operation {
		case "delete":
			deleteItems = append(deleteItems, item.Key)
		case "copy":
			// Handle copy operation conversion
			sourceBucket := request.Bucket
			if sb, ok := item.Metadata["source_bucket"].(string); ok {
				sourceBucket = sb
			}
			copyItems = append(copyItems, s3.CopyOperation{
				SourceBucket: sourceBucket,
				SourceKey:    item.Key,
				DestBucket:   request.Bucket,
				DestKey:      item.Key,
			})
		case "metadata_update":
			metadata := make(map[string]*string)
			for k, v := range item.Metadata {
				if str, ok := v.(string); ok {
					metadata[k] = &str
				}
			}
			metadataItems = append(metadataItems, s3.MetadataUpdate{
				Key:      item.Key,
				Metadata: metadata,
			})
		case "permission_update":
			acl := "private"
			if aclVal, ok := item.Metadata["acl"].(string); ok {
				acl = aclVal
			}
			permissionItems = append(permissionItems, s3.PermissionUpdate{
				Key: item.Key,
				ACL: acl,
			})
		}
	}

	// Execute operations by type
	var allOperations []s3.BatchOperation
	config := s3.DefaultBatchConfig()
	config.ConcurrentOperations = 40
	config.BatchSize = 500
	config.RetryCount = 3
	config.DryRun = request.DryRun

	if len(deleteItems) > 0 {
		ops := p.batchProcessor.DeleteBatch(ctx, request.Bucket, deleteItems, config)
		allOperations = append(allOperations, ops...)
	}

	if len(copyItems) > 0 {
		ops := p.batchProcessor.CopyBatch(ctx, copyItems, config)
		allOperations = append(allOperations, ops...)
	}

	if len(metadataItems) > 0 {
		ops := p.batchProcessor.UpdateMetadataBatch(ctx, request.Bucket, metadataItems, config)
		allOperations = append(allOperations, ops...)
	}

	if len(permissionItems) > 0 {
		ops := p.batchProcessor.SetPermissionsBatch(ctx, request.Bucket, permissionItems, config)
		allOperations = append(allOperations, ops...)
	}

	duration := time.Since(startTime)

	// Analyze results
	successCount := 0
	errorCount := 0
	for _, op := range allOperations {
		if op.Success {
			successCount++
		} else {
			errorCount++
		}
	}

	result := &BatchProcessingResult{
		OverallSuccess: errorCount == 0,
		SuccessCount:   successCount,
		ErrorCount:     errorCount,
		TotalCount:     len(allOperations),
		Duration:       duration,
		Operations:     allOperations,
		Stats: map[string]interface{}{
			"operations_per_second": float64(len(allOperations)) / duration.Seconds(),
			"success_rate":          float64(successCount) / float64(len(allOperations)),
			"delete_count":          len(deleteItems),
			"copy_count":            len(copyItems),
			"metadata_count":        len(metadataItems),
			"permission_count":      len(permissionItems),
		},
	}

	if request.DryRun {
		result.Summary = fmt.Sprintf("DRY RUN: Would process %d mixed operations", len(request.Items))
	} else {
		result.Summary = fmt.Sprintf("Processed %d/%d mixed operations in %v", successCount, len(request.Items), duration)
	}

	return result, nil
}

// parseBatchRequest parses args into a BatchRequest
func (p *HighCapacityBatchProcessor) parseBatchRequest(args map[string]interface{}) (*BatchRequest, error) {
	request := &BatchRequest{}

	// Required fields
	if operation, ok := args["operation"].(string); ok {
		request.Operation = operation
	} else {
		return nil, fmt.Errorf("operation is required")
	}

	if bucket, ok := args["bucket"].(string); ok {
		request.Bucket = bucket
	} else {
		return nil, fmt.Errorf("bucket is required")
	}

	// Parse items
	if itemsInterface, ok := args["items"].([]interface{}); ok {
		items := make([]BatchItem, len(itemsInterface))
		for i, itemInterface := range itemsInterface {
			if itemMap, ok := itemInterface.(map[string]interface{}); ok {
				item := BatchItem{}
				if key, ok := itemMap["key"].(string); ok {
					item.Key = key
				}
				if op, ok := itemMap["operation"].(string); ok {
					item.Operation = op
				}
				if metadata, ok := itemMap["metadata"].(map[string]interface{}); ok {
					item.Metadata = metadata
				}
				items[i] = item
			}
		}
		request.Items = items
	} else {
		return nil, fmt.Errorf("items are required")
	}

	// Optional fields
	if dryRun, ok := args["dry_run"].(bool); ok {
		request.DryRun = dryRun
	}

	// Parse batch config
	if configInterface, ok := args["config"].(map[string]interface{}); ok {
		config := BatchConfig{}
		if concurrency, ok := configInterface["concurrent_operations"].(int); ok {
			config.ConcurrentOperations = concurrency
		}
		if batchSize, ok := configInterface["batch_size"].(int); ok {
			config.BatchSize = batchSize
		}
		if retryCount, ok := configInterface["retry_count"].(int); ok {
			config.RetryCount = retryCount
		}
		if retryDelayStr, ok := configInterface["retry_delay"].(string); ok {
			if retryDelay, err := time.ParseDuration(retryDelayStr); err == nil {
				config.RetryDelay = retryDelay
			}
		}
		request.Config = config
	}

	return request, nil
}