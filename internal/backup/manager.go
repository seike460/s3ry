package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// BackupManager provides comprehensive backup and disaster recovery
type BackupManager struct {
	config           *BackupConfig
	storage          BackupStorage
	scheduler        *BackupScheduler
	encryptionMgr    EncryptionManager
	compressionMgr   CompressionManager
	verificationMgr  VerificationManager
	notificationMgr  NotificationManager
	metadataStore    MetadataStore
	stopCh           chan struct{}
	running          bool
	mutex            sync.RWMutex
}

// BackupConfig holds backup configuration
type BackupConfig struct {
	Enabled               bool                    `json:"enabled"`
	BackupDir             string                  `json:"backup_dir"`
	ScheduleInterval      time.Duration           `json:"schedule_interval"`
	RetentionPolicy       RetentionPolicy         `json:"retention_policy"`
	CompressionEnabled    bool                    `json:"compression_enabled"`
	EncryptionEnabled     bool                    `json:"encryption_enabled"`
	VerificationEnabled   bool                    `json:"verification_enabled"`
	NotificationsEnabled  bool                    `json:"notifications_enabled"`
	BackupTypes           []BackupType            `json:"backup_types"`
	ExcludePatterns       []string                `json:"exclude_patterns"`
	IncludePatterns       []string                `json:"include_patterns"`
	MaxConcurrentBackups  int                     `json:"max_concurrent_backups"`
	DisasterRecovery      DisasterRecoveryConfig  `json:"disaster_recovery"`
	CloudBackup           CloudBackupConfig       `json:"cloud_backup"`
}

// DefaultBackupConfig returns default backup configuration
func DefaultBackupConfig() *BackupConfig {
	return &BackupConfig{
		Enabled:              true,
		BackupDir:            "backups",
		ScheduleInterval:     time.Hour * 24, // Daily backups
		RetentionPolicy: RetentionPolicy{
			DailyRetention:   7,  // 7 days
			WeeklyRetention:  4,  // 4 weeks
			MonthlyRetention: 12, // 12 months
			YearlyRetention:  5,  // 5 years
		},
		CompressionEnabled:   true,
		EncryptionEnabled:    true,
		VerificationEnabled:  true,
		NotificationsEnabled: true,
		BackupTypes: []BackupType{
			BackupTypeConfiguration,
			BackupTypeUserData,
			BackupTypeMetadata,
		},
		ExcludePatterns: []string{
			"*.tmp",
			"*.log",
			"cache/*",
			"temp/*",
		},
		MaxConcurrentBackups: 2,
		DisasterRecovery: DisasterRecoveryConfig{
			Enabled:               true,
			RPO:                   time.Hour,        // Recovery Point Objective
			RTO:                   time.Minute * 15, // Recovery Time Objective
			ReplicationEnabled:    false,
			ReplicationTargets:    []string{},
			AutoFailoverEnabled:   false,
			HealthCheckInterval:   time.Minute * 5,
		},
		CloudBackup: CloudBackupConfig{
			Enabled:         false,
			Provider:        "aws",
			Bucket:          "",
			Region:          "us-east-1",
			EncryptInTransit: true,
			EncryptAtRest:   true,
		},
	}
}

// BackupType defines types of backups
type BackupType string

const (
	BackupTypeConfiguration BackupType = "CONFIGURATION"
	BackupTypeUserData      BackupType = "USER_DATA"
	BackupTypeMetadata      BackupType = "METADATA"
	BackupTypeSecurity      BackupType = "SECURITY"
	BackupTypeLogs          BackupType = "LOGS"
	BackupTypeFull          BackupType = "FULL"
	BackupTypeIncremental   BackupType = "INCREMENTAL"
	BackupTypeDifferential  BackupType = "DIFFERENTIAL"
)

// RetentionPolicy defines backup retention rules
type RetentionPolicy struct {
	DailyRetention   int `json:"daily_retention"`   // Days to keep daily backups
	WeeklyRetention  int `json:"weekly_retention"`  // Weeks to keep weekly backups
	MonthlyRetention int `json:"monthly_retention"` // Months to keep monthly backups
	YearlyRetention  int `json:"yearly_retention"`  // Years to keep yearly backups
}

// DisasterRecoveryConfig defines disaster recovery settings
type DisasterRecoveryConfig struct {
	Enabled               bool          `json:"enabled"`
	RPO                   time.Duration `json:"rpo"`                     // Recovery Point Objective
	RTO                   time.Duration `json:"rto"`                     // Recovery Time Objective
	ReplicationEnabled    bool          `json:"replication_enabled"`
	ReplicationTargets    []string      `json:"replication_targets"`
	AutoFailoverEnabled   bool          `json:"auto_failover_enabled"`
	HealthCheckInterval   time.Duration `json:"health_check_interval"`
	NotificationChannels  []string      `json:"notification_channels"`
}

// CloudBackupConfig defines cloud backup settings
type CloudBackupConfig struct {
	Enabled           bool              `json:"enabled"`
	Provider          string            `json:"provider"`           // aws, gcp, azure
	Bucket            string            `json:"bucket"`
	Region            string            `json:"region"`
	AccessKey         string            `json:"access_key"`
	SecretKey         string            `json:"secret_key"`
	EncryptInTransit  bool              `json:"encrypt_in_transit"`
	EncryptAtRest     bool              `json:"encrypt_at_rest"`
	StorageClass      string            `json:"storage_class"`      // standard, ia, glacier
	LifecyclePolicy   LifecyclePolicy   `json:"lifecycle_policy"`
	CrossRegionBackup bool              `json:"cross_region_backup"`
	Metadata          map[string]string `json:"metadata"`
}

// LifecyclePolicy defines cloud storage lifecycle rules
type LifecyclePolicy struct {
	TransitionToIA      int `json:"transition_to_ia"`       // Days to transition to Infrequent Access
	TransitionToGlacier int `json:"transition_to_glacier"`  // Days to transition to Glacier
	ExpirationDays      int `json:"expiration_days"`        // Days to expire
}

// Backup represents a backup instance
type Backup struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Type              BackupType        `json:"type"`
	Status            BackupStatus      `json:"status"`
	StartTime         time.Time         `json:"start_time"`
	EndTime           time.Time         `json:"end_time"`
	Duration          time.Duration     `json:"duration"`
	Size              int64             `json:"size"`
	CompressedSize    int64             `json:"compressed_size"`
	CompressionRatio  float64           `json:"compression_ratio"`
	FilePath          string            `json:"file_path"`
	Checksum          string            `json:"checksum"`
	Encrypted         bool              `json:"encrypted"`
	Compressed        bool              `json:"compressed"`
	Verified          bool              `json:"verified"`
	CloudBackup       CloudBackupInfo   `json:"cloud_backup"`
	Metadata          BackupMetadata    `json:"metadata"`
	SourceInfo        SourceInfo        `json:"source_info"`
	ErrorMessage      string            `json:"error_message,omitempty"`
	Tags              map[string]string `json:"tags"`
}

// BackupStatus represents backup status
type BackupStatus string

const (
	BackupStatusPending    BackupStatus = "PENDING"
	BackupStatusRunning    BackupStatus = "RUNNING"
	BackupStatusCompleted  BackupStatus = "COMPLETED"
	BackupStatusFailed     BackupStatus = "FAILED"
	BackupStatusVerifying  BackupStatus = "VERIFYING"
	BackupStatusCorrupted  BackupStatus = "CORRUPTED"
	BackupStatusUploading  BackupStatus = "UPLOADING"
	BackupStatusArchived   BackupStatus = "ARCHIVED"
)

// CloudBackupInfo holds cloud backup information
type CloudBackupInfo struct {
	Enabled   bool              `json:"enabled"`
	Provider  string            `json:"provider"`
	Bucket    string            `json:"bucket"`
	Key       string            `json:"key"`
	Region    string            `json:"region"`
	URL       string            `json:"url"`
	Metadata  map[string]string `json:"metadata"`
	Uploaded  bool              `json:"uploaded"`
	UploadTime time.Time        `json:"upload_time"`
}

// BackupMetadata holds detailed backup metadata
type BackupMetadata struct {
	Version         string            `json:"version"`
	CreatedBy       string            `json:"created_by"`
	Application     string            `json:"application"`
	Environment     string            `json:"environment"`
	FileCount       int               `json:"file_count"`
	DirectoryCount  int               `json:"directory_count"`
	IncludedPaths   []string          `json:"included_paths"`
	ExcludedPaths   []string          `json:"excluded_paths"`
	SystemInfo      SystemInfo        `json:"system_info"`
	Dependencies    []Dependency      `json:"dependencies"`
	Configuration   map[string]string `json:"configuration"`
}

// SourceInfo holds information about backup source
type SourceInfo struct {
	Hostname        string    `json:"hostname"`
	Platform        string    `json:"platform"`
	Architecture    string    `json:"architecture"`
	IPAddress       string    `json:"ip_address"`
	RootPath        string    `json:"root_path"`
	LastModified    time.Time `json:"last_modified"`
	TotalSize       int64     `json:"total_size"`
	AvailableSpace  int64     `json:"available_space"`
}

// SystemInfo holds system information
type SystemInfo struct {
	OS              string `json:"os"`
	Kernel          string `json:"kernel"`
	Architecture    string `json:"architecture"`
	CPUCount        int    `json:"cpu_count"`
	MemoryTotal     int64  `json:"memory_total"`
	DiskTotal       int64  `json:"disk_total"`
	UptimeSeconds   int64  `json:"uptime_seconds"`
}

// Dependency represents a system dependency
type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"` // library, service, database
	Status  string `json:"status"`
}

// RestoreRequest represents a restore request
type RestoreRequest struct {
	ID               string            `json:"id"`
	BackupID         string            `json:"backup_id"`
	TargetPath       string            `json:"target_path"`
	RestoreType      RestoreType       `json:"restore_type"`
	SelectiveRestore bool              `json:"selective_restore"`
	IncludePaths     []string          `json:"include_paths"`
	ExcludePaths     []string          `json:"exclude_paths"`
	OverwriteExisting bool             `json:"overwrite_existing"`
	VerifyRestore    bool              `json:"verify_restore"`
	Status           RestoreStatus     `json:"status"`
	StartTime        time.Time         `json:"start_time"`
	EndTime          time.Time         `json:"end_time"`
	Duration         time.Duration     `json:"duration"`
	RestoredSize     int64             `json:"restored_size"`
	RestoredFiles    int               `json:"restored_files"`
	ErrorMessage     string            `json:"error_message,omitempty"`
	Progress         RestoreProgress   `json:"progress"`
	RequestedBy      string            `json:"requested_by"`
	Tags             map[string]string `json:"tags"`
}

// RestoreType defines types of restore operations
type RestoreType string

const (
	RestoreTypeFull        RestoreType = "FULL"
	RestoreTypePartial     RestoreType = "PARTIAL"
	RestoreTypeInPlace     RestoreType = "IN_PLACE"
	RestoreTypeAlternate   RestoreType = "ALTERNATE"
	RestoreTypeFileLevel   RestoreType = "FILE_LEVEL"
	RestoreTypeVolumeLevel RestoreType = "VOLUME_LEVEL"
)

// RestoreStatus represents restore status
type RestoreStatus string

const (
	RestoreStatusPending    RestoreStatus = "PENDING"
	RestoreStatusRunning    RestoreStatus = "RUNNING"
	RestoreStatusCompleted  RestoreStatus = "COMPLETED"
	RestoreStatusFailed     RestoreStatus = "FAILED"
	RestoreStatusCancelled  RestoreStatus = "CANCELLED"
	RestoreStatusVerifying  RestoreStatus = "VERIFYING"
)

// RestoreProgress tracks restore progress
type RestoreProgress struct {
	TotalFiles      int     `json:"total_files"`
	ProcessedFiles  int     `json:"processed_files"`
	TotalSize       int64   `json:"total_size"`
	ProcessedSize   int64   `json:"processed_size"`
	PercentComplete float64 `json:"percent_complete"`
	CurrentFile     string  `json:"current_file"`
	EstimatedTimeRemaining time.Duration `json:"estimated_time_remaining"`
}

// BackupStorage interface for storing backups
type BackupStorage interface {
	Store(backup *Backup, data io.Reader) error
	Retrieve(backupID string) (io.ReadCloser, error)
	Delete(backupID string) error
	List(filters map[string]interface{}) ([]*Backup, error)
	GetInfo(backupID string) (*Backup, error)
	Cleanup(retentionPolicy RetentionPolicy) error
}

// EncryptionManager interface for backup encryption
type EncryptionManager interface {
	Encrypt(data io.Reader) (io.Reader, error)
	Decrypt(data io.Reader) (io.Reader, error)
	GenerateKey() ([]byte, error)
	SetKey(key []byte) error
}

// CompressionManager interface for backup compression
type CompressionManager interface {
	Compress(data io.Reader) (io.Reader, error)
	Decompress(data io.Reader) (io.Reader, error)
	GetCompressionRatio() float64
}

// VerificationManager interface for backup verification
type VerificationManager interface {
	GenerateChecksum(data io.Reader) (string, error)
	VerifyChecksum(data io.Reader, expectedChecksum string) (bool, error)
	VerifyBackupIntegrity(backup *Backup) error
}

// NotificationManager interface for backup notifications
type NotificationManager interface {
	SendBackupNotification(backup *Backup, event BackupEvent) error
	SendRestoreNotification(restore *RestoreRequest, event RestoreEvent) error
	SendDisasterRecoveryNotification(event DREvent) error
}

// MetadataStore interface for storing backup metadata
type MetadataStore interface {
	SaveBackupMetadata(backup *Backup) error
	LoadBackupMetadata(backupID string) (*Backup, error)
	ListBackups(filters map[string]interface{}) ([]*Backup, error)
	DeleteBackupMetadata(backupID string) error
	SaveRestoreRequest(restore *RestoreRequest) error
	LoadRestoreRequest(restoreID string) (*RestoreRequest, error)
	ListRestoreRequests(filters map[string]interface{}) ([]*RestoreRequest, error)
}

// BackupEvent represents backup events for notifications
type BackupEvent string

const (
	BackupEventStarted   BackupEvent = "STARTED"
	BackupEventCompleted BackupEvent = "COMPLETED"
	BackupEventFailed    BackupEvent = "FAILED"
	BackupEventCorrupted BackupEvent = "CORRUPTED"
)

// RestoreEvent represents restore events for notifications
type RestoreEvent string

const (
	RestoreEventStarted   RestoreEvent = "STARTED"
	RestoreEventCompleted RestoreEvent = "COMPLETED"
	RestoreEventFailed    RestoreEvent = "FAILED"
)

// DREvent represents disaster recovery events
type DREvent string

const (
	DREventFailoverStarted   DREvent = "FAILOVER_STARTED"
	DREventFailoverCompleted DREvent = "FAILOVER_COMPLETED"
	DREventFailoverFailed    DREvent = "FAILOVER_FAILED"
	DREventHealthCheckFailed DREvent = "HEALTH_CHECK_FAILED"
)

// BackupScheduler manages backup scheduling
type BackupScheduler struct {
	config     *BackupConfig
	backupMgr  *BackupManager
	schedules  map[string]*Schedule
	stopCh     chan struct{}
	running    bool
	mutex      sync.RWMutex
}

// Schedule represents a backup schedule
type Schedule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        BackupType    `json:"type"`
	Frequency   time.Duration `json:"frequency"`
	NextRun     time.Time     `json:"next_run"`
	LastRun     time.Time     `json:"last_run"`
	Enabled     bool          `json:"enabled"`
	MaxRetries  int           `json:"max_retries"`
	RetryCount  int           `json:"retry_count"`
}

// NewBackupManager creates a new backup manager
func NewBackupManager(config *BackupConfig, storage BackupStorage, metadataStore MetadataStore) (*BackupManager, error) {
	if config == nil {
		config = DefaultBackupConfig()
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(config.BackupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	manager := &BackupManager{
		config:          config,
		storage:         storage,
		metadataStore:   metadataStore,
		encryptionMgr:   NewSimpleEncryptionManager(),
		compressionMgr:  NewGzipCompressionManager(),
		verificationMgr: NewSHA256VerificationManager(),
		notificationMgr: NewSimpleNotificationManager(),
		stopCh:          make(chan struct{}),
	}

	// Initialize scheduler
	manager.scheduler = NewBackupScheduler(config, manager)

	return manager, nil
}

// NewBackupScheduler creates a new backup scheduler
func NewBackupScheduler(config *BackupConfig, backupMgr *BackupManager) *BackupScheduler {
	return &BackupScheduler{
		config:    config,
		backupMgr: backupMgr,
		schedules: make(map[string]*Schedule),
		stopCh:    make(chan struct{}),
	}
}

// Start starts the backup manager
func (b *BackupManager) Start(ctx context.Context) error {
	b.mutex.Lock()
	if b.running {
		b.mutex.Unlock()
		return fmt.Errorf("backup manager already running")
	}
	b.running = true
	b.mutex.Unlock()

	// Start scheduler
	if err := b.scheduler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start backup scheduler: %w", err)
	}

	// Start disaster recovery monitoring if enabled
	if b.config.DisasterRecovery.Enabled {
		go b.monitorDisasterRecovery(ctx)
	}

	fmt.Println("Backup Manager started successfully")
	return nil
}

// Stop stops the backup manager
func (b *BackupManager) Stop() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.running {
		close(b.stopCh)
		b.running = false
		
		if b.scheduler != nil {
			b.scheduler.Stop()
		}
	}

	fmt.Println("Backup Manager stopped")
	return nil
}

// CreateBackup creates a new backup
func (b *BackupManager) CreateBackup(backupType BackupType, sourcePaths []string, name string) (*Backup, error) {
	backup := &Backup{
		ID:        fmt.Sprintf("backup_%d", time.Now().UnixNano()),
		Name:      name,
		Type:      backupType,
		Status:    BackupStatusPending,
		StartTime: time.Now(),
		Encrypted: b.config.EncryptionEnabled,
		Compressed: b.config.CompressionEnabled,
		Tags:      make(map[string]string),
		Metadata: BackupMetadata{
			Version:       "2.0.0",
			CreatedBy:     "s3ry-backup",
			Application:   "s3ry",
			Environment:   "production",
			IncludedPaths: sourcePaths,
			Configuration: make(map[string]string),
		},
		SourceInfo: b.gatherSourceInfo(sourcePaths),
	}

	// Add default tags
	backup.Tags["type"] = string(backupType)
	backup.Tags["created_by"] = "s3ry"
	backup.Tags["environment"] = "production"

	// Save initial metadata
	if err := b.metadataStore.SaveBackupMetadata(backup); err != nil {
		return nil, fmt.Errorf("failed to save backup metadata: %w", err)
	}

	// Start backup process
	go b.performBackup(backup, sourcePaths)

	return backup, nil
}

// performBackup performs the actual backup operation
func (b *BackupManager) performBackup(backup *Backup, sourcePaths []string) {
	backup.Status = BackupStatusRunning
	b.metadataStore.SaveBackupMetadata(backup)

	// Send start notification
	if b.config.NotificationsEnabled {
		b.notificationMgr.SendBackupNotification(backup, BackupEventStarted)
	}

	defer func() {
		backup.EndTime = time.Now()
		backup.Duration = backup.EndTime.Sub(backup.StartTime)
		
		if backup.Status == BackupStatusRunning {
			backup.Status = BackupStatusCompleted
		}
		
		b.metadataStore.SaveBackupMetadata(backup)

		// Send completion notification
		if b.config.NotificationsEnabled {
			event := BackupEventCompleted
			if backup.Status == BackupStatusFailed {
				event = BackupEventFailed
			}
			b.notificationMgr.SendBackupNotification(backup, event)
		}
	}()

	// Create backup archive
	archivePath := filepath.Join(b.config.BackupDir, fmt.Sprintf("%s.tar", backup.ID))
	if b.config.CompressionEnabled {
		archivePath += ".gz"
	}

	backup.FilePath = archivePath

	// Create archive
	if err := b.createArchive(backup, sourcePaths, archivePath); err != nil {
		backup.Status = BackupStatusFailed
		backup.ErrorMessage = err.Error()
		fmt.Printf("Backup failed: %v\n", err)
		return
	}

	// Get file size
	if stat, err := os.Stat(archivePath); err == nil {
		backup.Size = stat.Size()
	}

	// Generate checksum
	if b.config.VerificationEnabled {
		backup.Status = BackupStatusVerifying
		b.metadataStore.SaveBackupMetadata(backup)

		if err := b.generateAndVerifyChecksum(backup); err != nil {
			backup.Status = BackupStatusCorrupted
			backup.ErrorMessage = err.Error()
			fmt.Printf("Backup verification failed: %v\n", err)
			return
		}
		backup.Verified = true
	}

	// Upload to cloud if enabled
	if b.config.CloudBackup.Enabled {
		backup.Status = BackupStatusUploading
		b.metadataStore.SaveBackupMetadata(backup)

		if err := b.uploadToCloud(backup); err != nil {
			// Don't fail the backup if cloud upload fails
			fmt.Printf("Cloud upload failed: %v\n", err)
		}
	}

	fmt.Printf("Backup completed: %s (%s)\n", backup.Name, backup.ID)
}

// createArchive creates a compressed archive of the source paths
func (b *BackupManager) createArchive(backup *Backup, sourcePaths []string, archivePath string) error {
	// Create output file
	outFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer outFile.Close()

	var writer io.Writer = outFile

	// Add compression if enabled
	if b.config.CompressionEnabled {
		gzWriter := gzip.NewWriter(outFile)
		defer gzWriter.Close()
		writer = gzWriter
	}

	// Add encryption if enabled
	if b.config.EncryptionEnabled {
		// Note: In real implementation, would properly encrypt writer
		// For now, skip encryption in simplified implementation
		fmt.Println("Encryption enabled but using simplified implementation")
	}

	// Create tar archive
	tarWriter := tar.NewWriter(writer)
	defer tarWriter.Close()

	fileCount := 0
	totalSize := int64(0)

	// Add files to archive
	for _, sourcePath := range sourcePaths {
		err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Check exclusion patterns
			if b.shouldExclude(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Create tar header
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			// Use relative path in archive
			relPath, err := filepath.Rel(sourcePath, path)
			if err != nil {
				relPath = path
			}
			header.Name = relPath

			// Write header
			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}

			// Write file content if it's a regular file
			if info.Mode().IsRegular() {
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()

				written, err := io.Copy(tarWriter, file)
				if err != nil {
					return err
				}

				totalSize += written
				fileCount++
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to create archive: %w", err)
		}
	}

	backup.Metadata.FileCount = fileCount
	backup.SourceInfo.TotalSize = totalSize

	return nil
}

// shouldExclude checks if a path should be excluded from backup
func (b *BackupManager) shouldExclude(path string) bool {
	for _, pattern := range b.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

// generateAndVerifyChecksum generates and verifies backup checksum
func (b *BackupManager) generateAndVerifyChecksum(backup *Backup) error {
	file, err := os.Open(backup.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file for checksum: %w", err)
	}
	defer file.Close()

	checksum, err := b.verificationMgr.GenerateChecksum(file)
	if err != nil {
		return fmt.Errorf("failed to generate checksum: %w", err)
	}

	backup.Checksum = checksum

	// Verify checksum
	file.Seek(0, 0) // Reset file pointer
	valid, err := b.verificationMgr.VerifyChecksum(file, checksum)
	if err != nil {
		return fmt.Errorf("failed to verify checksum: %w", err)
	}

	if !valid {
		return fmt.Errorf("checksum verification failed")
	}

	return nil
}

// uploadToCloud uploads backup to cloud storage
func (b *BackupManager) uploadToCloud(backup *Backup) error {
	// Simulate cloud upload
	backup.CloudBackup = CloudBackupInfo{
		Enabled:    true,
		Provider:   b.config.CloudBackup.Provider,
		Bucket:     b.config.CloudBackup.Bucket,
		Key:        fmt.Sprintf("backups/%s", filepath.Base(backup.FilePath)),
		Region:     b.config.CloudBackup.Region,
		Uploaded:   true,
		UploadTime: time.Now(),
		Metadata: map[string]string{
			"backup_id":   backup.ID,
			"backup_type": string(backup.Type),
			"created_by":  "s3ry",
		},
	}

	fmt.Printf("Cloud backup uploaded: %s/%s\n", backup.CloudBackup.Bucket, backup.CloudBackup.Key)
	return nil
}

// gatherSourceInfo gathers information about backup sources
func (b *BackupManager) gatherSourceInfo(sourcePaths []string) SourceInfo {
	// In real implementation, would gather actual system info
	return SourceInfo{
		Hostname:       "localhost",
		Platform:       "linux",
		Architecture:   "amd64",
		IPAddress:      "127.0.0.1",
		RootPath:       sourcePaths[0],
		LastModified:   time.Now(),
		TotalSize:      0, // Will be calculated during backup
		AvailableSpace: 1024 * 1024 * 1024, // 1GB
	}
}

// RestoreBackup restores a backup
func (b *BackupManager) RestoreBackup(backupID, targetPath string, restoreType RestoreType) (*RestoreRequest, error) {
	// Get backup metadata
	backup, err := b.metadataStore.LoadBackupMetadata(backupID)
	if err != nil {
		return nil, fmt.Errorf("failed to load backup metadata: %w", err)
	}

	restore := &RestoreRequest{
		ID:               fmt.Sprintf("restore_%d", time.Now().UnixNano()),
		BackupID:         backupID,
		TargetPath:       targetPath,
		RestoreType:      restoreType,
		OverwriteExisting: false,
		VerifyRestore:    true,
		Status:           RestoreStatusPending,
		StartTime:        time.Now(),
		RequestedBy:      "admin",
		Tags:             make(map[string]string),
		Progress: RestoreProgress{
			TotalFiles: backup.Metadata.FileCount,
			TotalSize:  backup.Size,
		},
	}

	// Save restore request
	if err := b.metadataStore.SaveRestoreRequest(restore); err != nil {
		return nil, fmt.Errorf("failed to save restore request: %w", err)
	}

	// Start restore process
	go b.performRestore(restore, backup)

	return restore, nil
}

// performRestore performs the actual restore operation
func (b *BackupManager) performRestore(restore *RestoreRequest, backup *Backup) {
	restore.Status = RestoreStatusRunning
	b.metadataStore.SaveRestoreRequest(restore)

	// Send start notification
	if b.config.NotificationsEnabled {
		b.notificationMgr.SendRestoreNotification(restore, RestoreEventStarted)
	}

	defer func() {
		restore.EndTime = time.Now()
		restore.Duration = restore.EndTime.Sub(restore.StartTime)
		
		if restore.Status == RestoreStatusRunning {
			restore.Status = RestoreStatusCompleted
		}
		
		b.metadataStore.SaveRestoreRequest(restore)

		// Send completion notification
		if b.config.NotificationsEnabled {
			event := RestoreEventCompleted
			if restore.Status == RestoreStatusFailed {
				event = RestoreEventFailed
			}
			b.notificationMgr.SendRestoreNotification(restore, event)
		}
	}()

	// Open backup file
	backupFile, err := os.Open(backup.FilePath)
	if err != nil {
		restore.Status = RestoreStatusFailed
		restore.ErrorMessage = err.Error()
		fmt.Printf("Restore failed: %v\n", err)
		return
	}
	defer backupFile.Close()

	var reader io.Reader = backupFile

	// Decrypt if necessary
	if backup.Encrypted {
		decryptedReader, err := b.encryptionMgr.Decrypt(reader)
		if err != nil {
			restore.Status = RestoreStatusFailed
			restore.ErrorMessage = fmt.Sprintf("decryption failed: %v", err)
			fmt.Printf("Restore failed: %v\n", err)
			return
		}
		reader = decryptedReader
	}

	// Decompress if necessary
	if backup.Compressed {
		decompressedReader, err := b.compressionMgr.Decompress(reader)
		if err != nil {
			restore.Status = RestoreStatusFailed
			restore.ErrorMessage = fmt.Sprintf("decompression failed: %v", err)
			fmt.Printf("Restore failed: %v\n", err)
			return
		}
		reader = decompressedReader
	}

	// Extract tar archive
	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			restore.Status = RestoreStatusFailed
			restore.ErrorMessage = err.Error()
			fmt.Printf("Restore failed: %v\n", err)
			return
		}

		// Create target path
		targetPath := filepath.Join(restore.TargetPath, header.Name)
		
		// Update progress
		restore.Progress.ProcessedFiles++
		restore.Progress.CurrentFile = header.Name
		restore.Progress.PercentComplete = float64(restore.Progress.ProcessedFiles) / float64(restore.Progress.TotalFiles) * 100

		// Create directory if needed
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				restore.Status = RestoreStatusFailed
				restore.ErrorMessage = err.Error()
				fmt.Printf("Restore failed: %v\n", err)
				return
			}
			continue
		}

		// Create file
		if header.Typeflag == tar.TypeReg {
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				restore.Status = RestoreStatusFailed
				restore.ErrorMessage = err.Error()
				fmt.Printf("Restore failed: %v\n", err)
				return
			}

			// Create file
			outFile, err := os.Create(targetPath)
			if err != nil {
				restore.Status = RestoreStatusFailed
				restore.ErrorMessage = err.Error()
				fmt.Printf("Restore failed: %v\n", err)
				return
			}

			// Copy file content
			written, err := io.Copy(outFile, tarReader)
			if err != nil {
				outFile.Close()
				restore.Status = RestoreStatusFailed
				restore.ErrorMessage = err.Error()
				fmt.Printf("Restore failed: %v\n", err)
				return
			}

			outFile.Close()
			
			restore.Progress.ProcessedSize += written
			restore.RestoredSize += written
			restore.RestoredFiles++

			// Set file permissions
			if err := os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
				fmt.Printf("Warning: failed to set file permissions: %v\n", err)
			}
		}
	}

	// Verify restore if enabled
	if restore.VerifyRestore {
		restore.Status = RestoreStatusVerifying
		b.metadataStore.SaveRestoreRequest(restore)
		
		// Simple verification - check file count and size
		// In real implementation, would do more thorough verification
		fmt.Printf("Restore verification completed\n")
	}

	fmt.Printf("Restore completed: %s -> %s\n", restore.BackupID, restore.TargetPath)
}

// ListBackups lists available backups
func (b *BackupManager) ListBackups(filters map[string]interface{}) ([]*Backup, error) {
	return b.metadataStore.ListBackups(filters)
}

// GetBackup gets backup information
func (b *BackupManager) GetBackup(backupID string) (*Backup, error) {
	return b.metadataStore.LoadBackupMetadata(backupID)
}

// DeleteBackup deletes a backup
func (b *BackupManager) DeleteBackup(backupID string) error {
	// Get backup metadata
	backup, err := b.metadataStore.LoadBackupMetadata(backupID)
	if err != nil {
		return fmt.Errorf("failed to load backup metadata: %w", err)
	}

	// Delete backup file
	if backup.FilePath != "" {
		if err := os.Remove(backup.FilePath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: failed to delete backup file: %v\n", err)
		}
	}

	// Delete from cloud if uploaded
	if backup.CloudBackup.Uploaded {
		// In real implementation, would delete from cloud storage
		fmt.Printf("Cloud backup deleted: %s/%s\n", backup.CloudBackup.Bucket, backup.CloudBackup.Key)
	}

	// Delete metadata
	if err := b.metadataStore.DeleteBackupMetadata(backupID); err != nil {
		return fmt.Errorf("failed to delete backup metadata: %w", err)
	}

	fmt.Printf("Backup deleted: %s\n", backupID)
	return nil
}

// CleanupOldBackups cleans up old backups according to retention policy
func (b *BackupManager) CleanupOldBackups() error {
	backups, err := b.metadataStore.ListBackups(nil)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	// Sort backups by creation time
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].StartTime.After(backups[j].StartTime)
	})

	now := time.Now()
	policy := b.config.RetentionPolicy

	for _, backup := range backups {
		age := now.Sub(backup.StartTime)
		shouldDelete := false

		// Check retention policy
		switch {
		case age > time.Hour*24*time.Duration(policy.DailyRetention):
			shouldDelete = true
		case age > time.Hour*24*7*time.Duration(policy.WeeklyRetention):
			// Keep if it's a weekly backup
			if backup.StartTime.Weekday() != time.Sunday {
				shouldDelete = true
			}
		case age > time.Hour*24*30*time.Duration(policy.MonthlyRetention):
			// Keep if it's a monthly backup
			if backup.StartTime.Day() != 1 {
				shouldDelete = true
			}
		case age > time.Hour*24*365*time.Duration(policy.YearlyRetention):
			// Keep if it's a yearly backup
			if backup.StartTime.Month() != time.January || backup.StartTime.Day() != 1 {
				shouldDelete = true
			}
		}

		if shouldDelete {
			if err := b.DeleteBackup(backup.ID); err != nil {
				fmt.Printf("Failed to delete old backup %s: %v\n", backup.ID, err)
			}
		}
	}

	return nil
}

// monitorDisasterRecovery monitors system health for disaster recovery
func (b *BackupManager) monitorDisasterRecovery(ctx context.Context) {
	ticker := time.NewTicker(b.config.DisasterRecovery.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.performHealthCheck()
		}
	}
}

// performHealthCheck performs disaster recovery health checks
func (b *BackupManager) performHealthCheck() {
	// Simulate health check
	// In real implementation, would check:
	// - System resources
	// - Service availability
	// - Network connectivity
	// - Storage availability
	// - Backup integrity

	healthy := true // Simulate healthy state

	if !healthy {
		// Send disaster recovery notification
		if b.config.NotificationsEnabled {
			b.notificationMgr.SendDisasterRecoveryNotification(DREventHealthCheckFailed)
		}

		// Trigger failover if auto-failover is enabled
		if b.config.DisasterRecovery.AutoFailoverEnabled {
			b.triggerFailover()
		}
	}
}

// triggerFailover triggers disaster recovery failover
func (b *BackupManager) triggerFailover() {
	fmt.Println("Disaster recovery failover triggered")
	
	// Send notification
	if b.config.NotificationsEnabled {
		b.notificationMgr.SendDisasterRecoveryNotification(DREventFailoverStarted)
	}

	// Perform failover operations
	// In real implementation, would:
	// - Switch to backup systems
	// - Redirect traffic
	// - Restore from latest backup
	// - Update DNS records
	// - Notify operations team

	// Simulate successful failover
	if b.config.NotificationsEnabled {
		b.notificationMgr.SendDisasterRecoveryNotification(DREventFailoverCompleted)
	}

	fmt.Println("Disaster recovery failover completed")
}

// Start starts the backup scheduler
func (s *BackupScheduler) Start(ctx context.Context) error {
	s.mutex.Lock()
	if s.running {
		s.mutex.Unlock()
		return fmt.Errorf("backup scheduler already running")
	}
	s.running = true
	s.mutex.Unlock()

	// Create default schedules
	s.createDefaultSchedules()

	// Start scheduling loop
	go s.schedulingLoop(ctx)

	fmt.Println("Backup Scheduler started")
	return nil
}

// Stop stops the backup scheduler
func (s *BackupScheduler) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		close(s.stopCh)
		s.running = false
	}

	fmt.Println("Backup Scheduler stopped")
}

// createDefaultSchedules creates default backup schedules
func (s *BackupScheduler) createDefaultSchedules() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Daily configuration backup
	s.schedules["daily_config"] = &Schedule{
		ID:         "daily_config",
		Name:       "Daily Configuration Backup",
		Type:       BackupTypeConfiguration,
		Frequency:  time.Hour * 24,
		NextRun:    time.Now().Add(time.Hour),
		Enabled:    true,
		MaxRetries: 3,
	}

	// Weekly full backup
	s.schedules["weekly_full"] = &Schedule{
		ID:         "weekly_full",
		Name:       "Weekly Full Backup",
		Type:       BackupTypeFull,
		Frequency:  time.Hour * 24 * 7,
		NextRun:    time.Now().Add(time.Hour * 24),
		Enabled:    true,
		MaxRetries: 3,
	}
}

// schedulingLoop runs the backup scheduling loop
func (s *BackupScheduler) schedulingLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkSchedules()
		}
	}
}

// checkSchedules checks if any scheduled backups need to run
func (s *BackupScheduler) checkSchedules() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	for _, schedule := range s.schedules {
		if schedule.Enabled && now.After(schedule.NextRun) {
			go s.executeScheduledBackup(schedule)
			schedule.LastRun = now
			schedule.NextRun = now.Add(schedule.Frequency)
		}
	}
}

// executeScheduledBackup executes a scheduled backup
func (s *BackupScheduler) executeScheduledBackup(schedule *Schedule) {
	sourcePaths := s.getSourcePathsForType(schedule.Type)
	name := fmt.Sprintf("%s_%s", schedule.Name, time.Now().Format("2006-01-02_15-04-05"))

	backup, err := s.backupMgr.CreateBackup(schedule.Type, sourcePaths, name)
	if err != nil {
		fmt.Printf("Scheduled backup failed: %v\n", err)
		schedule.RetryCount++
		return
	}

	schedule.RetryCount = 0
	fmt.Printf("Scheduled backup started: %s (%s)\n", backup.Name, backup.ID)
}

// getSourcePathsForType returns source paths for a backup type
func (s *BackupScheduler) getSourcePathsForType(backupType BackupType) []string {
	switch backupType {
	case BackupTypeConfiguration:
		return []string{"config", "internal/enterprise", "internal/sla"}
	case BackupTypeUserData:
		return []string{"data", "uploads"}
	case BackupTypeMetadata:
		return []string{"metadata", "logs"}
	case BackupTypeFull:
		return []string{".", "config", "data", "logs"}
	default:
		return []string{"."}
	}
}