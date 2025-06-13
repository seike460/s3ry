package s3

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/seike460/s3ry/internal/worker"
	"github.com/seike460/s3ry/pkg/types"
)

// Lister provides concurrent S3 object listing capabilities
type Lister struct {
	client *Client
	pool   *worker.Pool
}

// NewLister creates a new concurrent S3 lister
func NewLister(client *Client) *Lister {
	config := worker.DefaultConfig()
	config.Workers = 10 // Use more workers for listing operations
	pool := worker.NewPool(config)
	pool.Start()

	return &Lister{
		client: client,
		pool:   pool,
	}
}

// Close stops the lister and cleans up resources
func (l *Lister) Close() {
	l.pool.Stop()
}

// ListBuckets lists all S3 buckets with concurrent region detection
func (l *Lister) ListBuckets(ctx context.Context) ([]Bucket, error) {
	input := &s3.ListBucketsInput{}
	output, err := l.client.s3Client.ListBucketsWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	buckets := make([]Bucket, 0, len(output.Buckets))
	var mutex sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrent region checks
	semaphore := make(chan struct{}, 5)

	for _, bucket := range output.Buckets {
		if bucket.Name == nil || bucket.CreationDate == nil {
			continue
		}

		wg.Add(1)
		go func(bucketName string, creationDate time.Time) {
			defer wg.Done()
			
			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release

			// Get bucket region (this can be slow, so we do it concurrently)
			region, err := l.client.GetBucketRegion(ctx, bucketName)
			if err != nil {
				region = "unknown"
			}

			mutex.Lock()
			buckets = append(buckets, Bucket{
				Name:         bucketName,
				CreationDate: creationDate,
				Region:       region,
			})
			mutex.Unlock()
		}(*bucket.Name, *bucket.CreationDate)
	}

	wg.Wait()

	// Sort buckets by name
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Name < buckets[j].Name
	})

	return buckets, nil
}

// ListObjects lists objects in a bucket with optional prefix filtering
func (l *Lister) ListObjects(ctx context.Context, request ListRequest) ([]Object, error) {
	resultChan := make(chan []types.Object, 1)
	defer close(resultChan)

	job := &worker.S3ListJob{
		Client:  l.client,
		Request: ToTypesListRequest(request),
		Results: resultChan,
	}

	if err := l.pool.Submit(job); err != nil {
		return nil, fmt.Errorf("failed to submit list job: %w", err)
	}

	select {
	case workerObjects := <-resultChan:
		return FromTypesObjects(workerObjects), nil
	case result := <-l.pool.Results():
		if result.Error != nil {
			return nil, result.Error
		}
		// Wait for results
		select {
		case workerObjects := <-resultChan:
			return FromTypesObjects(workerObjects), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ListObjectsConcurrent lists objects from multiple prefixes concurrently
func (l *Lister) ListObjectsConcurrent(ctx context.Context, bucket string, prefixes []string) ([]Object, error) {
	if len(prefixes) == 0 {
		// If no prefixes specified, list all objects
		return l.ListObjects(ctx, ListRequest{Bucket: bucket})
	}

	resultChan := make(chan []types.Object, len(prefixes))
	var wg sync.WaitGroup

	// Submit jobs for each prefix
	for _, prefix := range prefixes {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			
			job := &worker.S3ListJob{
				Client: l.client,
				Request: ToTypesListRequest(ListRequest{
					Bucket: bucket,
					Prefix: p,
				}),
				Results: resultChan,
			}

			if err := l.pool.Submit(job); err != nil {
				// Send empty results on error
				select {
				case resultChan <- []types.Object{}:
				case <-ctx.Done():
				}
				return
			}
		}(prefix)
	}

	// Wait for all jobs to complete and collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allObjects []Object
	var mutex sync.Mutex

	// Collect results from worker pool
	for i := 0; i < len(prefixes); i++ {
		select {
		case result := <-l.pool.Results():
			if result.Error == nil {
				// Wait for the actual objects from the result channel
				select {
				case workerObjects := <-resultChan:
					mutex.Lock()
					allObjects = append(allObjects, FromTypesObjects(workerObjects)...)
					mutex.Unlock()
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Sort by last modified date (newest first)
	sort.Slice(allObjects, func(i, j int) bool {
		return allObjects[i].LastModified.After(allObjects[j].LastModified)
	})

	return allObjects, nil
}

// ListObjectsWithPagination lists objects with automatic pagination handling
func (l *Lister) ListObjectsWithPagination(ctx context.Context, bucket string, maxObjects int) ([]Object, error) {
	var allObjects []Object
	var continuationToken *string
	pageSize := int64(1000) // AWS max per request

	if maxObjects > 0 && int64(maxObjects) < pageSize {
		pageSize = int64(maxObjects)
	}

	for {
		input := &s3.ListObjectsV2Input{
			Bucket:  aws.String(bucket),
			MaxKeys: aws.Int64(pageSize),
		}

		if continuationToken != nil {
			input.ContinuationToken = continuationToken
		}

		output, err := l.client.s3Client.ListObjectsV2WithContext(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Convert AWS objects to our Object type
		for _, obj := range output.Contents {
			if obj.Key != nil && *obj.Key != "" {
				allObjects = append(allObjects, Object{
					Key:          *obj.Key,
					Size:         *obj.Size,
					LastModified: *obj.LastModified,
					ETag:         *obj.ETag,
					StorageClass: *obj.StorageClass,
				})

				// Check if we've reached the maximum
				if maxObjects > 0 && len(allObjects) >= maxObjects {
					return allObjects[:maxObjects], nil
				}
			}
		}

		// Check if there are more objects
		if output.IsTruncated == nil || !*output.IsTruncated {
			break
		}

		continuationToken = output.NextContinuationToken
	}

	return allObjects, nil
}

// StreamObjects streams objects from a bucket using a channel
func (l *Lister) StreamObjects(ctx context.Context, bucket string, objectChan chan<- Object) error {
	defer close(objectChan)

	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket:  aws.String(bucket),
			MaxKeys: aws.Int64(1000),
		}

		if continuationToken != nil {
			input.ContinuationToken = continuationToken
		}

		output, err := l.client.s3Client.ListObjectsV2WithContext(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		// Send objects to channel
		for _, obj := range output.Contents {
			if obj.Key != nil && *obj.Key != "" {
				object := Object{
					Key:          *obj.Key,
					Size:         *obj.Size,
					LastModified: *obj.LastModified,
					ETag:         *obj.ETag,
					StorageClass: *obj.StorageClass,
				}

				select {
				case objectChan <- object:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		// Check if there are more objects
		if output.IsTruncated == nil || !*output.IsTruncated {
			break
		}

		continuationToken = output.NextContinuationToken
	}

	return nil
}