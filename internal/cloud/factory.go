package cloud

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ClientFactory creates storage clients for different providers
type ClientFactory struct {
	configs map[CloudProvider]*CloudConfig
	clients map[CloudProvider]StorageClient
	mu      sync.RWMutex
	logger  Logger
}

// NewClientFactory creates a new client factory
func NewClientFactory(logger Logger) *ClientFactory {
	return &ClientFactory{
		configs: make(map[CloudProvider]*CloudConfig),
		clients: make(map[CloudProvider]StorageClient),
		logger:  logger,
	}
}

// RegisterProvider registers a cloud provider configuration
func (cf *ClientFactory) RegisterProvider(provider CloudProvider, config *CloudConfig) error {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	
	if config == nil {
		return fmt.Errorf("config cannot be nil for provider %s", provider.String())
	}
	
	config.Provider = provider
	cf.configs[provider] = config
	
	cf.logger.Info("Registered cloud provider: %s", provider.String())
	return nil
}

// GetClient returns a storage client for the specified provider
func (cf *ClientFactory) GetClient(provider CloudProvider) (StorageClient, error) {
	cf.mu.RLock()
	if client, exists := cf.clients[provider]; exists {
		cf.mu.RUnlock()
		return client, nil
	}
	cf.mu.RUnlock()
	
	// Create new client
	cf.mu.Lock()
	defer cf.mu.Unlock()
	
	// Double-check after acquiring write lock
	if client, exists := cf.clients[provider]; exists {
		return client, nil
	}
	
	config, exists := cf.configs[provider]
	if !exists {
		return nil, fmt.Errorf("no configuration found for provider %s", provider.String())
	}
	
	client, err := cf.createClient(provider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for provider %s: %w", provider.String(), err)
	}
	
	cf.clients[provider] = client
	cf.logger.Info("Created new client for provider: %s", provider.String())
	
	return client, nil
}

// GetMultipleClients returns multiple storage clients
func (cf *ClientFactory) GetMultipleClients(providers []CloudProvider) (map[CloudProvider]StorageClient, error) {
	clients := make(map[CloudProvider]StorageClient)
	
	for _, provider := range providers {
		client, err := cf.GetClient(provider)
		if err != nil {
			return nil, fmt.Errorf("failed to get client for provider %s: %w", provider.String(), err)
		}
		clients[provider] = client
	}
	
	return clients, nil
}

// createClient creates a new storage client based on provider
func (cf *ClientFactory) createClient(provider CloudProvider, config *CloudConfig) (StorageClient, error) {
	switch provider {
	case ProviderAWS:
		return NewAWSClient(config, cf.logger)
	case ProviderAzure:
		return NewAzureClient(config, cf.logger)
	case ProviderGCS:
		return NewGCSClient(config, cf.logger)
	case ProviderMinIO:
		return NewMinIOClient(config, cf.logger)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider.String())
	}
}

// ListProviders returns all registered providers
func (cf *ClientFactory) ListProviders() []CloudProvider {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	
	providers := make([]CloudProvider, 0, len(cf.configs))
	for provider := range cf.configs {
		providers = append(providers, provider)
	}
	
	return providers
}

// RemoveProvider removes a provider and closes its client
func (cf *ClientFactory) RemoveProvider(provider CloudProvider) error {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	
	if client, exists := cf.clients[provider]; exists {
		if err := client.Disconnect(context.Background()); err != nil {
			cf.logger.Warn("Failed to disconnect client for provider %s: %v", provider.String(), err)
		}
		delete(cf.clients, provider)
	}
	
	delete(cf.configs, provider)
	cf.logger.Info("Removed provider: %s", provider.String())
	
	return nil
}

// Close closes all clients and cleans up resources
func (cf *ClientFactory) Close() error {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	
	var lastErr error
	
	for provider, client := range cf.clients {
		if err := client.Disconnect(context.Background()); err != nil {
			cf.logger.Warn("Failed to disconnect client for provider %s: %v", provider.String(), err)
			lastErr = err
		}
	}
	
	cf.clients = make(map[CloudProvider]StorageClient)
	cf.configs = make(map[CloudProvider]*CloudConfig)
	
	cf.logger.Info("Closed all cloud clients")
	return lastErr
}

// MultiCloudManager manages operations across multiple cloud providers
type MultiCloudManager struct {
	factory *ClientFactory
	logger  Logger
}

// NewMultiCloudManager creates a new multi-cloud manager
func NewMultiCloudManager(factory *ClientFactory, logger Logger) *MultiCloudManager {
	return &MultiCloudManager{
		factory: factory,
		logger:  logger,
	}
}

// CrossCloudCopy copies an object from one cloud provider to another
func (mcm *MultiCloudManager) CrossCloudCopy(ctx context.Context, req *CrossCloudCopyRequest) (*CrossCloudCopyResult, error) {
	sourceClient, err := mcm.factory.GetClient(req.SourceProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to get source client: %w", err)
	}
	
	targetClient, err := mcm.factory.GetClient(req.TargetProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to get target client: %w", err)
	}
	
	// Get object from source
	sourceObj, err := sourceClient.GetObject(ctx, req.SourceBucket, req.SourceKey, req.GetOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get source object: %w", err)
	}
	defer sourceObj.Body.Close()
	
	// Put object to target
	putResult, err := targetClient.PutObject(ctx, req.TargetBucket, req.TargetKey, sourceObj.Body, req.PutOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to put target object: %w", err)
	}
	
	result := &CrossCloudCopyResult{
		SourceProvider: req.SourceProvider,
		TargetProvider: req.TargetProvider,
		SourceBucket:   req.SourceBucket,
		SourceKey:      req.SourceKey,
		TargetBucket:   req.TargetBucket,
		TargetKey:      req.TargetKey,
		Size:           sourceObj.Info.Size,
		PutResult:      putResult,
	}
	
	mcm.logger.Info("Cross-cloud copy completed: %s:%s/%s -> %s:%s/%s", 
		req.SourceProvider.String(), req.SourceBucket, req.SourceKey,
		req.TargetProvider.String(), req.TargetBucket, req.TargetKey)
	
	return result, nil
}

// CrossCloudSync synchronizes objects between two cloud providers
func (mcm *MultiCloudManager) CrossCloudSync(ctx context.Context, req *CrossCloudSyncRequest) (*CrossCloudSyncResult, error) {
	sourceClient, err := mcm.factory.GetClient(req.SourceProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to get source client: %w", err)
	}
	
	targetClient, err := mcm.factory.GetClient(req.TargetProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to get target client: %w", err)
	}
	
	// List objects in source
	sourceObjects, err := sourceClient.ListObjects(ctx, req.SourceBucket, req.ListOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list source objects: %w", err)
	}
	
	// List objects in target
	targetObjects, err := targetClient.ListObjects(ctx, req.TargetBucket, req.ListOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list target objects: %w", err)
	}
	
	// Create maps for comparison
	sourceMap := make(map[string]*ObjectInfo)
	for i := range sourceObjects.Objects {
		obj := &sourceObjects.Objects[i]
		sourceMap[obj.Key] = obj
	}
	
	targetMap := make(map[string]*ObjectInfo)
	for i := range targetObjects.Objects {
		obj := &targetObjects.Objects[i]
		targetMap[obj.Key] = obj
	}
	
	result := &CrossCloudSyncResult{
		SourceProvider: req.SourceProvider,
		TargetProvider: req.TargetProvider,
		SourceBucket:   req.SourceBucket,
		TargetBucket:   req.TargetBucket,
		Summary: &SyncSummary{
			TotalSourceObjects: len(sourceObjects.Objects),
			TotalTargetObjects: len(targetObjects.Objects),
		},
	}
	
	// Find objects to copy (new or modified)
	for key, sourceObj := range sourceMap {
		targetObj, exists := targetMap[key]
		
		var shouldCopy bool
		if !exists {
			shouldCopy = true
			result.Summary.NewObjects++
		} else if req.SyncMode == SyncModeUpdate && sourceObj.LastModified.After(targetObj.LastModified) {
			shouldCopy = true
			result.Summary.ModifiedObjects++
		} else if req.SyncMode == SyncModeForce {
			shouldCopy = true
			result.Summary.ModifiedObjects++
		}
		
		if shouldCopy {
			copyReq := &CrossCloudCopyRequest{
				SourceProvider: req.SourceProvider,
				TargetProvider: req.TargetProvider,
				SourceBucket:   req.SourceBucket,
				SourceKey:      key,
				TargetBucket:   req.TargetBucket,
				TargetKey:      key,
				GetOptions:     req.GetOptions,
				PutOptions:     req.PutOptions,
			}
			
			copyResult, err := mcm.CrossCloudCopy(ctx, copyReq)
			if err != nil {
				result.Summary.FailedObjects++
				result.Errors = append(result.Errors, SyncError{
					Key:   key,
					Error: err.Error(),
				})
				mcm.logger.Warn("Failed to sync object %s: %v", key, err)
			} else {
				result.Summary.SyncedObjects++
				result.Summary.TotalBytes += copyResult.Size
			}
		} else {
			result.Summary.SkippedObjects++
		}
	}
	
	// Find objects to delete (if delete mode is enabled)
	if req.SyncMode == SyncModeSync || req.SyncMode == SyncModeForce {
		for key := range targetMap {
			if _, exists := sourceMap[key]; !exists {
				err := targetClient.DeleteObject(ctx, req.TargetBucket, key, nil)
				if err != nil {
					result.Summary.FailedObjects++
					result.Errors = append(result.Errors, SyncError{
						Key:   key,
						Error: err.Error(),
					})
					mcm.logger.Warn("Failed to delete object %s: %v", key, err)
				} else {
					result.Summary.DeletedObjects++
				}
			}
		}
	}
	
	mcm.logger.Info("Cross-cloud sync completed: %s:%s -> %s:%s (%d synced, %d failed)", 
		req.SourceProvider.String(), req.SourceBucket,
		req.TargetProvider.String(), req.TargetBucket,
		result.Summary.SyncedObjects, result.Summary.FailedObjects)
	
	return result, nil
}

// MultiCloudSearch searches for objects across multiple cloud providers
func (mcm *MultiCloudManager) MultiCloudSearch(ctx context.Context, req *MultiCloudSearchRequest) (*MultiCloudSearchResult, error) {
	results := make(map[CloudProvider]*SearchProviderResult)
	
	for _, provider := range req.Providers {
		client, err := mcm.factory.GetClient(provider)
		if err != nil {
			mcm.logger.Warn("Failed to get client for provider %s: %v", provider.String(), err)
			continue
		}
		
		providerResult := &SearchProviderResult{
			Provider: provider,
			Objects:  make([]ObjectInfo, 0),
		}
		
		for _, bucket := range req.Buckets {
			objects, err := client.ListObjects(ctx, bucket, req.ListOptions)
			if err != nil {
				mcm.logger.Warn("Failed to list objects in %s:%s: %v", provider.String(), bucket, err)
				providerResult.Errors = append(providerResult.Errors, fmt.Sprintf("bucket %s: %s", bucket, err.Error()))
				continue
			}
			
			// Filter objects based on search criteria
			for _, obj := range objects.Objects {
				if mcm.matchesSearchCriteria(&obj, req.Criteria) {
					providerResult.Objects = append(providerResult.Objects, obj)
				}
			}
		}
		
		results[provider] = providerResult
	}
	
	return &MultiCloudSearchResult{
		Results: results,
	}, nil
}

// matchesSearchCriteria checks if an object matches the search criteria
func (mcm *MultiCloudManager) matchesSearchCriteria(obj *ObjectInfo, criteria *SearchCriteria) bool {
	if criteria == nil {
		return true
	}
	
	// Check key pattern
	if criteria.KeyPattern != "" {
		// Simple pattern matching (could be enhanced with regex)
		if !contains(obj.Key, criteria.KeyPattern) {
			return false
		}
	}
	
	// Check size range
	if criteria.MinSize > 0 && obj.Size < criteria.MinSize {
		return false
	}
	if criteria.MaxSize > 0 && obj.Size > criteria.MaxSize {
		return false
	}
	
	// Check date range
	if criteria.ModifiedAfter != nil && obj.LastModified.Before(*criteria.ModifiedAfter) {
		return false
	}
	if criteria.ModifiedBefore != nil && obj.LastModified.After(*criteria.ModifiedBefore) {
		return false
	}
	
	// Check storage class
	if criteria.StorageClass != nil && obj.StorageClass != *criteria.StorageClass {
		return false
	}
	
	// Check tags
	if len(criteria.Tags) > 0 {
		for key, value := range criteria.Tags {
			if objValue, exists := obj.Tags[key]; !exists || objValue != value {
				return false
			}
		}
	}
	
	return true
}

// HealthCheckAll performs health checks on all registered providers
func (mcm *MultiCloudManager) HealthCheckAll(ctx context.Context) (*MultiCloudHealthResult, error) {
	providers := mcm.factory.ListProviders()
	results := make(map[CloudProvider]*HealthCheckResult)
	
	for _, provider := range providers {
		client, err := mcm.factory.GetClient(provider)
		if err != nil {
			results[provider] = &HealthCheckResult{
				Provider: provider,
				Healthy:  false,
				Error:    err.Error(),
			}
			continue
		}
		
		err = client.HealthCheck(ctx)
		results[provider] = &HealthCheckResult{
			Provider: provider,
			Healthy:  err == nil,
			Error:    func() string { if err != nil { return err.Error() }; return "" }(),
		}
	}
	
	return &MultiCloudHealthResult{
		Results: results,
	}, nil
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr || 
			 containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 1; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Request and response types for multi-cloud operations

// CrossCloudCopyRequest contains parameters for cross-cloud copy
type CrossCloudCopyRequest struct {
	SourceProvider CloudProvider        `json:"source_provider"`
	TargetProvider CloudProvider        `json:"target_provider"`
	SourceBucket   string               `json:"source_bucket"`
	SourceKey      string               `json:"source_key"`
	TargetBucket   string               `json:"target_bucket"`
	TargetKey      string               `json:"target_key"`
	GetOptions     *GetObjectOptions    `json:"get_options,omitempty"`
	PutOptions     *PutObjectOptions    `json:"put_options,omitempty"`
}

// CrossCloudCopyResult contains the result of cross-cloud copy
type CrossCloudCopyResult struct {
	SourceProvider CloudProvider    `json:"source_provider"`
	TargetProvider CloudProvider    `json:"target_provider"`
	SourceBucket   string           `json:"source_bucket"`
	SourceKey      string           `json:"source_key"`
	TargetBucket   string           `json:"target_bucket"`
	TargetKey      string           `json:"target_key"`
	Size           int64            `json:"size"`
	PutResult      *PutObjectResult `json:"put_result"`
}

// SyncMode represents different synchronization modes
type SyncMode int

const (
	SyncModeUpdate SyncMode = iota // Only copy new and modified objects
	SyncModeSync                   // Copy and delete to match source exactly
	SyncModeForce                  // Copy all objects regardless of modification time
)

// CrossCloudSyncRequest contains parameters for cross-cloud sync
type CrossCloudSyncRequest struct {
	SourceProvider CloudProvider      `json:"source_provider"`
	TargetProvider CloudProvider      `json:"target_provider"`
	SourceBucket   string             `json:"source_bucket"`
	TargetBucket   string             `json:"target_bucket"`
	SyncMode       SyncMode           `json:"sync_mode"`
	ListOptions    *ListObjectsOptions `json:"list_options,omitempty"`
	GetOptions     *GetObjectOptions   `json:"get_options,omitempty"`
	PutOptions     *PutObjectOptions   `json:"put_options,omitempty"`
}

// CrossCloudSyncResult contains the result of cross-cloud sync
type CrossCloudSyncResult struct {
	SourceProvider CloudProvider `json:"source_provider"`
	TargetProvider CloudProvider `json:"target_provider"`
	SourceBucket   string        `json:"source_bucket"`
	TargetBucket   string        `json:"target_bucket"`
	Summary        *SyncSummary  `json:"summary"`
	Errors         []SyncError   `json:"errors,omitempty"`
}

// SyncSummary contains summary statistics for sync operation
type SyncSummary struct {
	TotalSourceObjects int   `json:"total_source_objects"`
	TotalTargetObjects int   `json:"total_target_objects"`
	NewObjects         int   `json:"new_objects"`
	ModifiedObjects    int   `json:"modified_objects"`
	DeletedObjects     int   `json:"deleted_objects"`
	SkippedObjects     int   `json:"skipped_objects"`
	SyncedObjects      int   `json:"synced_objects"`
	FailedObjects      int   `json:"failed_objects"`
	TotalBytes         int64 `json:"total_bytes"`
}

// SyncError represents an error during sync operation
type SyncError struct {
	Key   string `json:"key"`
	Error string `json:"error"`
}

// MultiCloudSearchRequest contains parameters for multi-cloud search
type MultiCloudSearchRequest struct {
	Providers   []CloudProvider      `json:"providers"`
	Buckets     []string             `json:"buckets"`
	Criteria    *SearchCriteria      `json:"criteria,omitempty"`
	ListOptions *ListObjectsOptions  `json:"list_options,omitempty"`
}

// SearchCriteria contains search criteria for objects
type SearchCriteria struct {
	KeyPattern      string             `json:"key_pattern,omitempty"`
	MinSize         int64              `json:"min_size,omitempty"`
	MaxSize         int64              `json:"max_size,omitempty"`
	ModifiedAfter   *time.Time         `json:"modified_after,omitempty"`
	ModifiedBefore  *time.Time         `json:"modified_before,omitempty"`
	StorageClass    *StorageClass      `json:"storage_class,omitempty"`
	Tags            map[string]string  `json:"tags,omitempty"`
}

// MultiCloudSearchResult contains the result of multi-cloud search
type MultiCloudSearchResult struct {
	Results map[CloudProvider]*SearchProviderResult `json:"results"`
}

// SearchProviderResult contains search results for a specific provider
type SearchProviderResult struct {
	Provider CloudProvider `json:"provider"`
	Objects  []ObjectInfo  `json:"objects"`
	Errors   []string      `json:"errors,omitempty"`
}

// MultiCloudHealthResult contains health check results for all providers
type MultiCloudHealthResult struct {
	Results map[CloudProvider]*HealthCheckResult `json:"results"`
}

// HealthCheckResult contains health check result for a provider
type HealthCheckResult struct {
	Provider CloudProvider `json:"provider"`
	Healthy  bool          `json:"healthy"`
	Error    string        `json:"error,omitempty"`
}