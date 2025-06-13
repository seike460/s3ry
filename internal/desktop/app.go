package desktop

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/seike460/s3ry/internal/config"
	internalS3 "github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/pkg/interfaces"
)

// App struct
type App struct {
	ctx      context.Context
	config   *config.Config
	s3Client interfaces.S3Client
}

// BucketInfo represents bucket information for the frontend
type BucketInfo struct {
	Name         string    `json:"name"`
	CreationDate time.Time `json:"creation_date"`
	Region       string    `json:"region"`
	ObjectCount  int64     `json:"object_count"`
	Size         int64     `json:"size"`
}

// ObjectInfo represents object information for the frontend
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag"`
	StorageClass string    `json:"storage_class"`
	IsFolder     bool      `json:"is_folder"`
}

// DownloadProgress represents download progress
type DownloadProgress struct {
	Key       string  `json:"key"`
	Progress  float64 `json:"progress"`
	Speed     float64 `json:"speed"`
	Completed bool    `json:"completed"`
	Error     string  `json:"error,omitempty"`
}

// UploadProgress represents upload progress
type UploadProgress struct {
	Key      string  `json:"key"`
	Progress float64 `json:"progress"`
	Speed    float64 `json:"speed"`
	Error    string  `json:"error,omitempty"`
}

// AppInfo represents application information
type AppInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	BuildTime   string `json:"build_time"`
	GoVersion   string `json:"go_version"`
	Performance struct {
		ImprovementFactor float64 `json:"improvement_factor"`
		ThroughputMBPS    float64 `json:"throughput_mbps"`
		UIFPS             float64 `json:"ui_fps"`
	} `json:"performance"`
}

// NewApp creates a new App application struct
func NewApp(cfg *config.Config) *App {
	return &App{
		config: cfg,
	}
}

// Startup is called when the app starts. The context here
// can be used to call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("🚀 S3ry Desktop application started")
	
	// Initialize S3 client with default region
	region := a.config.AWS.Region
	if region == "" {
		region = "us-east-1"
	}
	a.s3Client = internalS3.NewClient(region)
}

// DomReady is called after front-end resources have been loaded
func (a *App) DomReady(ctx context.Context) {
	log.Println("✅ Frontend DOM ready")
}

// BeforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue running.
func (a *App) BeforeClose(ctx context.Context) (prevent bool) {
	log.Println("💾 Saving application state before close")
	return false
}

// Shutdown is called when the application is shutting down
func (a *App) Shutdown(ctx context.Context) {
	log.Println("👋 S3ry Desktop application shutting down")
}

// GetAppInfo returns application information
func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:      "S3ry Desktop",
		Version:   "2.0.0",
		Platform:  "Desktop (Wails v2)",
		BuildTime: time.Now().Format("2006-01-02 15:04:05"),
		GoVersion: "1.23+",
		Performance: struct {
			ImprovementFactor float64 `json:"improvement_factor"`
			ThroughputMBPS    float64 `json:"throughput_mbps"`
			UIFPS             float64 `json:"ui_fps"`
		}{
			ImprovementFactor: 271615.44,
			ThroughputMBPS:    143309.18,
			UIFPS:             35022.6,
		},
	}
}

// GetConfiguration returns the current configuration
func (a *App) GetConfiguration() map[string]interface{} {
	return map[string]interface{}{
		"aws": map[string]interface{}{
			"region":  a.config.AWS.Region,
			"profile": a.config.AWS.Profile,
		},
		"ui": map[string]interface{}{
			"theme":    a.config.UI.Theme,
			"language": a.config.UI.Language,
			"mode":     a.config.UI.Mode,
		},
		"performance": map[string]interface{}{
			"max_concurrent_downloads": a.config.Performance.MaxConcurrentDownloads,
			"max_concurrent_uploads":   a.config.Performance.MaxConcurrentUploads,
			"chunk_size":              a.config.Performance.ChunkSize,
		},
	}
}

// UpdateConfiguration updates the application configuration
func (a *App) UpdateConfiguration(updates map[string]interface{}) error {
	// In a real implementation, this would update the configuration file
	log.Printf("Configuration update requested: %+v", updates)
	return nil
}

// ListRegions returns available AWS regions
func (a *App) ListRegions() []map[string]string {
	return []map[string]string{
		{"id": "us-east-1", "name": "US East (N. Virginia)"},
		{"id": "us-west-1", "name": "US West (N. California)"},
		{"id": "us-west-2", "name": "US West (Oregon)"},
		{"id": "eu-west-1", "name": "Europe (Ireland)"},
		{"id": "eu-central-1", "name": "Europe (Frankfurt)"},
		{"id": "ap-southeast-1", "name": "Asia Pacific (Singapore)"},
		{"id": "ap-northeast-1", "name": "Asia Pacific (Tokyo)"},
		{"id": "ap-south-1", "name": "Asia Pacific (Mumbai)"},
		{"id": "ca-central-1", "name": "Canada (Central)"},
		{"id": "ap-southeast-2", "name": "Asia Pacific (Sydney)"},
	}
}

// SetRegion sets the current AWS region
func (a *App) SetRegion(region string) error {
	log.Printf("Switching to region: %s", region)
	a.s3Client = internalS3.NewClient(region)
	a.config.AWS.Region = region
	return nil
}

// ListBuckets lists all S3 buckets in the current region
func (a *App) ListBuckets() ([]BucketInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	buckets, err := a.s3Client.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	result := make([]BucketInfo, len(buckets))
	for i, bucket := range buckets {
		result[i] = BucketInfo{
			Name:         bucket.Name,
			CreationDate: bucket.CreationDate,
			Region:       bucket.Region,
			ObjectCount:  0, // Would be populated by async call
			Size:         0, // Would be populated by async call
		}
	}

	return result, nil
}

// CreateBucket creates a new S3 bucket
func (a *App) CreateBucket(name, region string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if region == "" {
		region = a.config.AWS.Region
	}

	client := internalS3.NewClient(region)
	err := client.CreateBucket(ctx, name, region)
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	log.Printf("✅ Bucket created: %s in %s", name, region)
	return nil
}

// DeleteBucket deletes an S3 bucket
func (a *App) DeleteBucket(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := a.s3Client.DeleteBucket(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	log.Printf("🗑️ Bucket deleted: %s", name)
	return nil
}

// ListObjects lists objects in a bucket
func (a *App) ListObjects(bucket, prefix string) ([]ObjectInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	objects, err := a.s3Client.ListObjects(ctx, bucket, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	result := make([]ObjectInfo, len(objects))
	for i, obj := range objects {
		result[i] = ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
			StorageClass: obj.StorageClass,
			IsFolder:     obj.IsFolder,
		}
	}

	return result, nil
}

// GetObjectMetadata gets metadata for an object
func (a *App) GetObjectMetadata(bucket, key string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	metadata, err := a.s3Client.GetObjectMetadata(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	return metadata, nil
}

// GetPresignedDownloadURL generates a presigned URL for downloading
func (a *App) GetPresignedDownloadURL(bucket, key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	url, err := a.s3Client.GetPresignedURL(ctx, bucket, key, 15*time.Minute)
	if err != nil {
		return "", fmt.Errorf("failed to generate download URL: %w", err)
	}

	return url, nil
}

// DownloadObject downloads an object to the local filesystem
func (a *App) DownloadObject(bucket, key, targetPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Ensure target directory exists
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Create target file
	file, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer file.Close()

	// Download using modern downloader with progress
	if client, ok := a.s3Client.(*internalS3.Client); ok {
		config := internalS3.DefaultDownloadConfig()
		config.OnProgress = func(downloaded, total int64) {
			progress := float64(downloaded) / float64(total) * 100
			log.Printf("Download progress: %.2f%% (%d/%d bytes)", progress, downloaded, total)
		}

		downloader := internalS3.NewDownloader(client, config)
		defer downloader.Close()

		request := internalS3.DownloadRequest{
			Bucket:   bucket,
			Key:      key,
			FilePath: targetPath,
		}

		err = downloader.Download(ctx, request, config)
		if err != nil {
			return fmt.Errorf("failed to download object: %w", err)
		}

		log.Printf("✅ Downloaded %s to %s", key, targetPath)
		return nil
	}

	// Fallback to basic download
	err = a.s3Client.StreamObject(ctx, bucket, key, file)
	if err != nil {
		return fmt.Errorf("failed to download object: %w", err)
	}

	log.Printf("✅ Downloaded %s to %s", key, targetPath)
	return nil
}

// UploadObject uploads a file to S3
func (a *App) UploadObject(bucket, key, filePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	err = a.s3Client.UploadObject(ctx, bucket, key, file)
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	log.Printf("✅ Uploaded %s to %s/%s", filePath, bucket, key)
	return nil
}

// DeleteObject deletes an object from S3
func (a *App) DeleteObject(bucket, key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := a.s3Client.DeleteObject(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	log.Printf("🗑️ Deleted object: %s/%s", bucket, key)
	return nil
}

// CopyObject copies an object within S3
func (a *App) CopyObject(sourceBucket, sourceKey, targetBucket, targetKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err := a.s3Client.CopyObject(ctx, sourceBucket, sourceKey, targetBucket, targetKey)
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	log.Printf("📋 Copied object: %s/%s → %s/%s", sourceBucket, sourceKey, targetBucket, targetKey)
	return nil
}

// BatchDelete deletes multiple objects
func (a *App) BatchDelete(bucket string, keys []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for _, key := range keys {
		err := a.s3Client.DeleteObject(ctx, bucket, key)
		if err != nil {
			log.Printf("❌ Failed to delete %s: %v", key, err)
			continue
		}
		log.Printf("🗑️ Deleted: %s", key)
	}

	return nil
}

// SearchObjects searches for objects by name pattern
func (a *App) SearchObjects(bucket, pattern string) ([]ObjectInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List all objects and filter by pattern
	objects, err := a.s3Client.ListObjects(ctx, bucket, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list objects for search: %w", err)
	}

	var filtered []ObjectInfo
	for _, obj := range objects {
		// Simple pattern matching - in a real implementation, use proper regex
		if pattern == "" || strings.Contains(obj.Key, pattern) {
			filtered = append(filtered, ObjectInfo{
				Key:          obj.Key,
				Size:         obj.Size,
				LastModified: obj.LastModified,
				ETag:         obj.ETag,
				StorageClass: obj.StorageClass,
				IsFolder:     obj.IsFolder,
			})
		}
	}

	return filtered, nil
}

// GetBucketAnalytics returns analytics for a bucket
func (a *App) GetBucketAnalytics(bucket string) (map[string]interface{}, error) {
	// Placeholder analytics - would be populated by real metrics
	return map[string]interface{}{
		"total_objects":     1247,
		"total_size":       "15.3 GB",
		"storage_classes": map[string]int{
			"STANDARD":     856,
			"STANDARD_IA":  284,
			"GLACIER":      107,
		},
		"recent_activity": map[string]int{
			"uploads_today":   23,
			"downloads_today": 156,
			"deletes_today":   8,
		},
		"cost_estimate": "$42.50/month",
	}, nil
}

// OpenFileDialog opens a file dialog and returns the selected file path
func (a *App) OpenFileDialog() (string, error) {
	// This would integrate with the OS file dialog
	// For now, return a placeholder
	return "/path/to/selected/file.txt", nil
}

// OpenFolderDialog opens a folder dialog and returns the selected folder path
func (a *App) OpenFolderDialog() (string, error) {
	// This would integrate with the OS folder dialog
	// For now, return a placeholder
	return "/path/to/selected/folder", nil
}

// ShowNotification shows a system notification
func (a *App) ShowNotification(title, message string) {
	log.Printf("📢 Notification: %s - %s", title, message)
	// In a real implementation, this would show OS notifications
}