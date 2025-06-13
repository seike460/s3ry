package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupManager_CreateBackup(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create test source files
	sourceDir := filepath.Join(tempDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("test content"), 0644))
	
	// Create backup manager
	config := DefaultBackupConfig()
	config.BackupDir = filepath.Join(tempDir, "backups")
	config.CompressionEnabled = false
	config.EncryptionEnabled = false
	config.VerificationEnabled = false
	
	storage := NewFileBackupStorage(config.BackupDir)
	metadataStore := NewFileMetadataStore(filepath.Join(tempDir, "metadata"))
	
	manager, err := NewBackupManager(config, storage, metadataStore)
	require.NoError(t, err)
	
	// Create backup
	backup, err := manager.CreateBackup(BackupTypeConfiguration, []string{sourceDir}, "test-backup")
	assert.NoError(t, err)
	assert.NotNil(t, backup)
	assert.NotEmpty(t, backup.ID)
	assert.Equal(t, "test-backup", backup.Name)
	assert.Equal(t, BackupTypeConfiguration, backup.Type)
	assert.Equal(t, BackupStatusPending, backup.Status)
	
	// Wait for backup to complete
	time.Sleep(time.Millisecond * 100)
	
	// Check backup metadata
	savedBackup, err := metadataStore.LoadBackupMetadata(backup.ID)
	assert.NoError(t, err)
	assert.Equal(t, backup.ID, savedBackup.ID)
}

func TestBackupManager_RestoreBackup(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create test source files
	sourceDir := filepath.Join(tempDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	testContent := "test content for restore"
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte(testContent), 0644))
	
	// Create backup manager
	config := DefaultBackupConfig()
	config.BackupDir = filepath.Join(tempDir, "backups")
	config.CompressionEnabled = false
	config.EncryptionEnabled = false
	config.VerificationEnabled = false
	
	storage := NewFileBackupStorage(config.BackupDir)
	metadataStore := NewFileMetadataStore(filepath.Join(tempDir, "metadata"))
	
	manager, err := NewBackupManager(config, storage, metadataStore)
	require.NoError(t, err)
	
	// Create backup
	backup, err := manager.CreateBackup(BackupTypeConfiguration, []string{sourceDir}, "test-backup")
	require.NoError(t, err)
	
	// Wait for backup to complete
	time.Sleep(time.Millisecond * 200)
	
	// Create restore request
	restoreDir := filepath.Join(tempDir, "restore")
	restore, err := manager.RestoreBackup(backup.ID, restoreDir, RestoreTypeFull)
	assert.NoError(t, err)
	assert.NotNil(t, restore)
	assert.Equal(t, backup.ID, restore.BackupID)
	assert.Equal(t, restoreDir, restore.TargetPath)
	assert.Equal(t, RestoreTypeFull, restore.RestoreType)
	
	// Wait for restore to complete
	time.Sleep(time.Millisecond * 200)
	
	// Check restored file
	restoredFile := filepath.Join(restoreDir, "test.txt")
	content, err := os.ReadFile(restoredFile)
	assert.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

func TestBackupManager_ListBackups(t *testing.T) {
	tempDir := t.TempDir()
	
	config := DefaultBackupConfig()
	config.BackupDir = filepath.Join(tempDir, "backups")
	config.CompressionEnabled = false
	config.EncryptionEnabled = false
	config.VerificationEnabled = false
	
	storage := NewFileBackupStorage(config.BackupDir)
	metadataStore := NewFileMetadataStore(filepath.Join(tempDir, "metadata"))
	
	manager, err := NewBackupManager(config, storage, metadataStore)
	require.NoError(t, err)
	
	// Create test backups directly in metadata store for testing
	backup1 := &Backup{
		ID:        "backup-1",
		Name:      "Test Backup 1",
		Type:      BackupTypeConfiguration,
		Status:    BackupStatusCompleted,
		StartTime: time.Now(),
	}
	
	backup2 := &Backup{
		ID:        "backup-2", 
		Name:      "Test Backup 2",
		Type:      BackupTypeUserData,
		Status:    BackupStatusCompleted,
		StartTime: time.Now(),
	}
	
	require.NoError(t, metadataStore.SaveBackupMetadata(backup1))
	require.NoError(t, metadataStore.SaveBackupMetadata(backup2))
	
	// List all backups
	backups, err := manager.ListBackups(nil)
	assert.NoError(t, err)
	assert.Len(t, backups, 2)
	
	// List with filter
	filters := map[string]interface{}{
		"type": string(BackupTypeConfiguration),
	}
	configBackups, err := manager.ListBackups(filters)
	assert.NoError(t, err)
	assert.Len(t, configBackups, 1)
	assert.Equal(t, backup1.ID, configBackups[0].ID)
	
	// List with status filter
	statusFilters := map[string]interface{}{
		"status": string(BackupStatusCompleted),
	}
	completedBackups, err := manager.ListBackups(statusFilters)
	assert.NoError(t, err)
	assert.Len(t, completedBackups, 2) // Both should be completed
}

func TestBackupManager_DeleteBackup(t *testing.T) {
	tempDir := t.TempDir()
	
	config := DefaultBackupConfig()
	config.BackupDir = filepath.Join(tempDir, "backups")
	
	storage := NewFileBackupStorage(config.BackupDir)
	metadataStore := NewFileMetadataStore(filepath.Join(tempDir, "metadata"))
	
	manager, err := NewBackupManager(config, storage, metadataStore)
	require.NoError(t, err)
	
	// Create test backup
	sourceDir := filepath.Join(tempDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("test"), 0644))
	
	backup, err := manager.CreateBackup(BackupTypeConfiguration, []string{sourceDir}, "test-backup")
	require.NoError(t, err)
	
	// Wait for backup to complete
	time.Sleep(time.Millisecond * 100)
	
	// Verify backup exists
	_, err = manager.GetBackup(backup.ID)
	assert.NoError(t, err)
	
	// Delete backup
	err = manager.DeleteBackup(backup.ID)
	assert.NoError(t, err)
	
	// Verify backup is deleted
	_, err = manager.GetBackup(backup.ID)
	assert.Error(t, err)
}

func TestBackupManager_StartStop(t *testing.T) {
	tempDir := t.TempDir()
	
	config := DefaultBackupConfig()
	config.BackupDir = filepath.Join(tempDir, "backups")
	config.DisasterRecovery.Enabled = false // Disable for testing
	
	storage := NewFileBackupStorage(config.BackupDir)
	metadataStore := NewFileMetadataStore(filepath.Join(tempDir, "metadata"))
	
	manager, err := NewBackupManager(config, storage, metadataStore)
	require.NoError(t, err)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Test start
	err = manager.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, manager.running)
	
	// Test double start
	err = manager.Start(ctx)
	assert.Error(t, err)
	
	// Test stop
	err = manager.Stop()
	assert.NoError(t, err)
	assert.False(t, manager.running)
}

func TestBackupScheduler_CreateDefaultSchedules(t *testing.T) {
	config := DefaultBackupConfig()
	manager := &BackupManager{config: config}
	scheduler := NewBackupScheduler(config, manager)
	
	scheduler.createDefaultSchedules()
	
	assert.Len(t, scheduler.schedules, 2)
	assert.Contains(t, scheduler.schedules, "daily_config")
	assert.Contains(t, scheduler.schedules, "weekly_full")
	
	dailySchedule := scheduler.schedules["daily_config"]
	assert.Equal(t, "Daily Configuration Backup", dailySchedule.Name)
	assert.Equal(t, BackupTypeConfiguration, dailySchedule.Type)
	assert.Equal(t, time.Hour*24, dailySchedule.Frequency)
	assert.True(t, dailySchedule.Enabled)
}

func TestBackupScheduler_GetSourcePathsForType(t *testing.T) {
	config := DefaultBackupConfig()
	manager := &BackupManager{config: config}
	scheduler := NewBackupScheduler(config, manager)
	
	testCases := []struct {
		backupType    BackupType
		expectedPaths []string
	}{
		{BackupTypeConfiguration, []string{"config", "internal/enterprise", "internal/sla"}},
		{BackupTypeUserData, []string{"data", "uploads"}},
		{BackupTypeMetadata, []string{"metadata", "logs"}},
		{BackupTypeFull, []string{".", "config", "data", "logs"}},
	}
	
	for _, tc := range testCases {
		paths := scheduler.getSourcePathsForType(tc.backupType)
		assert.Equal(t, tc.expectedPaths, paths)
	}
}

func TestFileMetadataStore_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	store := NewFileMetadataStore(tempDir)
	
	backup := &Backup{
		ID:       "test-backup-123",
		Name:     "Test Backup",
		Type:     BackupTypeConfiguration,
		Status:   BackupStatusCompleted,
		StartTime: time.Now(),
		Size:     1024,
		Tags:     map[string]string{"test": "true"},
	}
	
	// Test save
	err := store.SaveBackupMetadata(backup)
	assert.NoError(t, err)
	
	// Test load
	loadedBackup, err := store.LoadBackupMetadata(backup.ID)
	assert.NoError(t, err)
	assert.Equal(t, backup.ID, loadedBackup.ID)
	assert.Equal(t, backup.Name, loadedBackup.Name)
	assert.Equal(t, backup.Type, loadedBackup.Type)
	assert.Equal(t, backup.Status, loadedBackup.Status)
	assert.Equal(t, backup.Size, loadedBackup.Size)
	
	// Test list
	backups, err := store.ListBackups(nil)
	assert.NoError(t, err)
	assert.Len(t, backups, 1)
	assert.Equal(t, backup.ID, backups[0].ID)
	
	// Test delete
	err = store.DeleteBackupMetadata(backup.ID)
	assert.NoError(t, err)
	
	// Verify deletion
	_, err = store.LoadBackupMetadata(backup.ID)
	assert.Error(t, err)
}

func TestFileMetadataStore_RestoreRequests(t *testing.T) {
	tempDir := t.TempDir()
	store := NewFileMetadataStore(tempDir)
	
	restore := &RestoreRequest{
		ID:          "test-restore-123",
		BackupID:    "backup-123",
		TargetPath:  "/tmp/restore",
		RestoreType: RestoreTypeFull,
		Status:      RestoreStatusCompleted,
		StartTime:   time.Now(),
		RequestedBy: "admin",
		Tags:        map[string]string{"test": "true"},
	}
	
	// Test save
	err := store.SaveRestoreRequest(restore)
	assert.NoError(t, err)
	
	// Test load
	loadedRestore, err := store.LoadRestoreRequest(restore.ID)
	assert.NoError(t, err)
	assert.Equal(t, restore.ID, loadedRestore.ID)
	assert.Equal(t, restore.BackupID, loadedRestore.BackupID)
	assert.Equal(t, restore.TargetPath, loadedRestore.TargetPath)
	assert.Equal(t, restore.RestoreType, loadedRestore.RestoreType)
	assert.Equal(t, restore.Status, loadedRestore.Status)
	
	// Test list
	restores, err := store.ListRestoreRequests(nil)
	assert.NoError(t, err)
	assert.Len(t, restores, 1)
	assert.Equal(t, restore.ID, restores[0].ID)
}

func TestEncryptionManager(t *testing.T) {
	manager := NewSimpleEncryptionManager()
	
	// Test key generation
	key, err := manager.GenerateKey()
	assert.NoError(t, err)
	assert.Len(t, key, 32)
	
	// Test key setting
	err = manager.SetKey(key)
	assert.NoError(t, err)
	
	// Test invalid key length
	err = manager.SetKey([]byte("short"))
	assert.Error(t, err)
}

func TestCompressionManager(t *testing.T) {
	manager := NewGzipCompressionManager()
	
	// Test compression ratio
	ratio := manager.GetCompressionRatio()
	assert.Greater(t, ratio, 0.0)
	assert.Less(t, ratio, 1.0)
}

func TestVerificationManager(t *testing.T) {
	manager := NewSHA256VerificationManager()
	
	// Create test data
	testData := "test data for checksum"
	
	// Test checksum generation
	checksum1, err := manager.GenerateChecksum(strings.NewReader(testData))
	assert.NoError(t, err)
	assert.NotEmpty(t, checksum1)
	
	// Test checksum consistency
	checksum2, err := manager.GenerateChecksum(strings.NewReader(testData))
	assert.NoError(t, err)
	assert.Equal(t, checksum1, checksum2)
	
	// Test checksum verification
	valid, err := manager.VerifyChecksum(strings.NewReader(testData), checksum1)
	assert.NoError(t, err)
	assert.True(t, valid)
	
	// Test checksum verification with wrong data
	valid, err = manager.VerifyChecksum(strings.NewReader("wrong data"), checksum1)
	assert.NoError(t, err)
	assert.False(t, valid)
}

func TestNotificationManager(t *testing.T) {
	manager := NewSimpleNotificationManager()
	
	backup := &Backup{
		ID:   "test-backup",
		Name: "Test Backup",
		Size: 1024,
		Duration: time.Minute,
	}
	
	// Test backup notifications
	err := manager.SendBackupNotification(backup, BackupEventStarted)
	assert.NoError(t, err)
	
	err = manager.SendBackupNotification(backup, BackupEventCompleted)
	assert.NoError(t, err)
	
	err = manager.SendBackupNotification(backup, BackupEventFailed)
	assert.NoError(t, err)
	
	// Test restore notifications
	restore := &RestoreRequest{
		ID:         "test-restore",
		BackupID:   "backup-123",
		TargetPath: "/tmp/restore",
		Duration:   time.Minute * 2,
	}
	
	err = manager.SendRestoreNotification(restore, RestoreEventStarted)
	assert.NoError(t, err)
	
	err = manager.SendRestoreNotification(restore, RestoreEventCompleted)
	assert.NoError(t, err)
	
	// Test disaster recovery notifications
	err = manager.SendDisasterRecoveryNotification(DREventFailoverStarted)
	assert.NoError(t, err)
	
	err = manager.SendDisasterRecoveryNotification(DREventHealthCheckFailed)
	assert.NoError(t, err)
}