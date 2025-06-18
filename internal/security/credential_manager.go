package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// CredentialManager manages secure storage and retrieval of credentials
type CredentialManager struct {
	config        *CredentialConfig
	encryptionKey []byte
	store         CredentialStore
	mutex         sync.RWMutex
}

// CredentialConfig holds credential management configuration
type CredentialConfig struct {
	Enabled           bool          `json:"enabled"`
	EncryptionEnabled bool          `json:"encryption_enabled"`
	StorePath         string        `json:"store_path"`
	MasterPassword    string        `json:"-"` // Never serialize
	KeyDerivationSalt []byte        `json:"-"` // Never serialize
	RotationInterval  time.Duration `json:"rotation_interval"`
	MaxCredentialAge  time.Duration `json:"max_credential_age"`
	BackupEnabled     bool          `json:"backup_enabled"`
	BackupPath        string        `json:"backup_path"`
}

// Credential represents a stored credential
type Credential struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         CredentialType         `json:"type"`
	Data         map[string]string      `json:"data"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	ExpiresAt    *time.Time             `json:"expires_at,omitempty"`
	LastAccessed time.Time              `json:"last_accessed"`
	Metadata     map[string]interface{} `json:"metadata"`
	Tags         []string               `json:"tags"`
}

// CredentialType represents different types of credentials
type CredentialType string

const (
	CredentialTypeAWS      CredentialType = "aws"
	CredentialTypeGCP      CredentialType = "gcp"
	CredentialTypeAzure    CredentialType = "azure"
	CredentialTypeMinio    CredentialType = "minio"
	CredentialTypeAPI      CredentialType = "api"
	CredentialTypeDatabase CredentialType = "database"
	CredentialTypeGeneric  CredentialType = "generic"
)

// CredentialStore interface for credential storage
type CredentialStore interface {
	Store(credential *Credential) error
	Retrieve(id string) (*Credential, error)
	List() ([]*Credential, error)
	Delete(id string) error
	Update(credential *Credential) error
	Backup() error
	Restore(backupPath string) error
}

// FileCredentialStore implements file-based credential storage
type FileCredentialStore struct {
	storePath  string
	backupPath string
	encryption bool
	mutex      sync.RWMutex
}

// DefaultCredentialConfig returns default credential management configuration
func DefaultCredentialConfig() *CredentialConfig {
	homeDir, _ := os.UserHomeDir()
	return &CredentialConfig{
		Enabled:           true,
		EncryptionEnabled: true,
		StorePath:         filepath.Join(homeDir, ".s3ry", "credentials.enc"),
		RotationInterval:  90 * 24 * time.Hour,  // 90 days
		MaxCredentialAge:  365 * 24 * time.Hour, // 1 year
		BackupEnabled:     true,
		BackupPath:        filepath.Join(homeDir, ".s3ry", "credentials_backup.enc"),
	}
}

// NewCredentialManager creates a new credential manager
func NewCredentialManager(config *CredentialConfig, masterPassword string) (*CredentialManager, error) {
	if config == nil {
		config = DefaultCredentialConfig()
	}

	cm := &CredentialManager{
		config: config,
	}

	// Generate encryption key from master password
	if config.EncryptionEnabled && masterPassword != "" {
		salt := config.KeyDerivationSalt
		if salt == nil {
			salt = make([]byte, 32)
			if _, err := rand.Read(salt); err != nil {
				return nil, fmt.Errorf("failed to generate salt: %w", err)
			}
			config.KeyDerivationSalt = salt
		}

		cm.encryptionKey = pbkdf2.Key([]byte(masterPassword), salt, 100000, 32, sha256.New)
	}

	// Initialize store
	store := &FileCredentialStore{
		storePath:  config.StorePath,
		backupPath: config.BackupPath,
		encryption: config.EncryptionEnabled,
	}
	cm.store = store

	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(config.StorePath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create credential store directory: %w", err)
	}

	return cm, nil
}

// StoreCredential securely stores a credential
func (cm *CredentialManager) StoreCredential(ctx context.Context, cred *Credential) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.config.Enabled {
		return fmt.Errorf("credential management is disabled")
	}

	// Validate credential
	if err := cm.validateCredential(cred); err != nil {
		return fmt.Errorf("credential validation failed: %w", err)
	}

	// Set timestamps
	now := time.Now()
	if cred.CreatedAt.IsZero() {
		cred.CreatedAt = now
	}
	cred.UpdatedAt = now
	cred.LastAccessed = now

	// Encrypt sensitive data if encryption is enabled
	if cm.config.EncryptionEnabled {
		if err := cm.encryptCredentialData(cred); err != nil {
			return fmt.Errorf("failed to encrypt credential data: %w", err)
		}
	}

	// Store credential
	if err := cm.store.Store(cred); err != nil {
		return fmt.Errorf("failed to store credential: %w", err)
	}

	// Create backup if enabled
	if cm.config.BackupEnabled {
		if err := cm.store.Backup(); err != nil {
			// Log backup failure but don't fail the operation
			fmt.Printf("Warning: Failed to create credential backup: %v\n", err)
		}
	}

	return nil
}

// RetrieveCredential securely retrieves a credential by ID
func (cm *CredentialManager) RetrieveCredential(ctx context.Context, id string) (*Credential, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if !cm.config.Enabled {
		return nil, fmt.Errorf("credential management is disabled")
	}

	// Retrieve credential
	cred, err := cm.store.Retrieve(id)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credential: %w", err)
	}

	// Decrypt sensitive data if encryption is enabled
	if cm.config.EncryptionEnabled {
		if err := cm.decryptCredentialData(cred); err != nil {
			return nil, fmt.Errorf("failed to decrypt credential data: %w", err)
		}
	}

	// Update last accessed time
	cred.LastAccessed = time.Now()
	if err := cm.store.Update(cred); err != nil {
		// Log update failure but don't fail the retrieval
		fmt.Printf("Warning: Failed to update credential access time: %v\n", err)
	}

	return cred, nil
}

// ListCredentials returns a list of all stored credentials (without sensitive data)
func (cm *CredentialManager) ListCredentials(ctx context.Context) ([]*Credential, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if !cm.config.Enabled {
		return nil, fmt.Errorf("credential management is disabled")
	}

	credentials, err := cm.store.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	// Return credentials with sensitive data masked
	maskedCredentials := make([]*Credential, len(credentials))
	for i, cred := range credentials {
		maskedCredentials[i] = cm.maskSensitiveData(cred)
	}

	return maskedCredentials, nil
}

// DeleteCredential securely deletes a credential
func (cm *CredentialManager) DeleteCredential(ctx context.Context, id string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.config.Enabled {
		return fmt.Errorf("credential management is disabled")
	}

	return cm.store.Delete(id)
}

// RotateCredential rotates a credential (useful for API keys, passwords)
func (cm *CredentialManager) RotateCredential(ctx context.Context, id string, newData map[string]string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.config.Enabled {
		return fmt.Errorf("credential management is disabled")
	}

	// Retrieve existing credential
	cred, err := cm.store.Retrieve(id)
	if err != nil {
		return fmt.Errorf("failed to retrieve credential for rotation: %w", err)
	}

	// Update with new data
	cred.Data = newData
	cred.UpdatedAt = time.Now()

	// Encrypt and store
	if cm.config.EncryptionEnabled {
		if err := cm.encryptCredentialData(cred); err != nil {
			return fmt.Errorf("failed to encrypt rotated credential data: %w", err)
		}
	}

	return cm.store.Update(cred)
}

// GetAWSCredentials retrieves AWS credentials in the expected format
func (cm *CredentialManager) GetAWSCredentials(ctx context.Context, profileName string) (*AWSCredentials, error) {
	cred, err := cm.RetrieveCredential(ctx, profileName)
	if err != nil {
		return nil, err
	}

	if cred.Type != CredentialTypeAWS {
		return nil, fmt.Errorf("credential is not AWS type")
	}

	return &AWSCredentials{
		AccessKeyID:     cred.Data["access_key_id"],
		SecretAccessKey: cred.Data["secret_access_key"],
		SessionToken:    cred.Data["session_token"],
		Region:          cred.Data["region"],
		Profile:         cred.Data["profile"],
	}, nil
}

// AWSCredentials represents AWS credential structure
type AWSCredentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token,omitempty"`
	Region          string `json:"region"`
	Profile         string `json:"profile,omitempty"`
}

// validateCredential validates a credential before storage
func (cm *CredentialManager) validateCredential(cred *Credential) error {
	if cred.ID == "" {
		return fmt.Errorf("credential ID is required")
	}

	if cred.Name == "" {
		return fmt.Errorf("credential name is required")
	}

	if cred.Type == "" {
		return fmt.Errorf("credential type is required")
	}

	if len(cred.Data) == 0 {
		return fmt.Errorf("credential data is required")
	}

	// Type-specific validation
	switch cred.Type {
	case CredentialTypeAWS:
		if cred.Data["access_key_id"] == "" || cred.Data["secret_access_key"] == "" {
			return fmt.Errorf("AWS credentials require access_key_id and secret_access_key")
		}
	case CredentialTypeAPI:
		if cred.Data["api_key"] == "" && cred.Data["token"] == "" {
			return fmt.Errorf("API credentials require api_key or token")
		}
	}

	return nil
}

// encryptCredentialData encrypts sensitive data in a credential
func (cm *CredentialManager) encryptCredentialData(cred *Credential) error {
	if cm.encryptionKey == nil {
		return fmt.Errorf("encryption key not available")
	}

	sensitiveFields := []string{"access_key_id", "secret_access_key", "session_token", "api_key", "token", "password"}

	for _, field := range sensitiveFields {
		if value, exists := cred.Data[field]; exists && value != "" {
			encrypted, err := cm.encrypt([]byte(value))
			if err != nil {
				return fmt.Errorf("failed to encrypt field %s: %w", field, err)
			}
			cred.Data[field] = base64.StdEncoding.EncodeToString(encrypted)
		}
	}

	return nil
}

// decryptCredentialData decrypts sensitive data in a credential
func (cm *CredentialManager) decryptCredentialData(cred *Credential) error {
	if cm.encryptionKey == nil {
		return fmt.Errorf("encryption key not available")
	}

	sensitiveFields := []string{"access_key_id", "secret_access_key", "session_token", "api_key", "token", "password"}

	for _, field := range sensitiveFields {
		if value, exists := cred.Data[field]; exists && value != "" {
			encrypted, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				continue // Field might not be encrypted
			}

			decrypted, err := cm.decrypt(encrypted)
			if err != nil {
				return fmt.Errorf("failed to decrypt field %s: %w", field, err)
			}
			cred.Data[field] = string(decrypted)
		}
	}

	return nil
}

// encrypt encrypts data using AES-GCM
func (cm *CredentialManager) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(cm.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// decrypt decrypts data using AES-GCM
func (cm *CredentialManager) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(cm.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := data[:gcm.NonceSize()]
	ciphertext := data[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// maskSensitiveData creates a copy of credential with sensitive data masked
func (cm *CredentialManager) maskSensitiveData(cred *Credential) *Credential {
	masked := *cred
	masked.Data = make(map[string]string)

	sensitiveFields := []string{"access_key_id", "secret_access_key", "session_token", "api_key", "token", "password"}

	for key, value := range cred.Data {
		isSensitive := false
		for _, sensitive := range sensitiveFields {
			if key == sensitive {
				isSensitive = true
				break
			}
		}

		if isSensitive && value != "" {
			masked.Data[key] = "***MASKED***"
		} else {
			masked.Data[key] = value
		}
	}

	return &masked
}

// Store implements CredentialStore for FileCredentialStore
func (fs *FileCredentialStore) Store(credential *Credential) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	credentials, err := fs.loadCredentials()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if credentials == nil {
		credentials = make(map[string]*Credential)
	}

	credentials[credential.ID] = credential
	return fs.saveCredentials(credentials)
}

// Retrieve implements CredentialStore for FileCredentialStore
func (fs *FileCredentialStore) Retrieve(id string) (*Credential, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	credentials, err := fs.loadCredentials()
	if err != nil {
		return nil, err
	}

	cred, exists := credentials[id]
	if !exists {
		return nil, fmt.Errorf("credential not found: %s", id)
	}

	return cred, nil
}

// List implements CredentialStore for FileCredentialStore
func (fs *FileCredentialStore) List() ([]*Credential, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	credentials, err := fs.loadCredentials()
	if err != nil {
		return nil, err
	}

	result := make([]*Credential, 0, len(credentials))
	for _, cred := range credentials {
		result = append(result, cred)
	}

	return result, nil
}

// Delete implements CredentialStore for FileCredentialStore
func (fs *FileCredentialStore) Delete(id string) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	credentials, err := fs.loadCredentials()
	if err != nil {
		return err
	}

	if _, exists := credentials[id]; !exists {
		return fmt.Errorf("credential not found: %s", id)
	}

	delete(credentials, id)
	return fs.saveCredentials(credentials)
}

// Update implements CredentialStore for FileCredentialStore
func (fs *FileCredentialStore) Update(credential *Credential) error {
	return fs.Store(credential) // Same as store for file-based implementation
}

// Backup implements CredentialStore for FileCredentialStore
func (fs *FileCredentialStore) Backup() error {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	if fs.backupPath == "" {
		return fmt.Errorf("backup path not configured")
	}

	// Create backup directory
	if err := os.MkdirAll(filepath.Dir(fs.backupPath), 0700); err != nil {
		return err
	}

	// Copy current store to backup location
	sourceData, err := os.ReadFile(fs.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to backup
		}
		return err
	}

	return os.WriteFile(fs.backupPath, sourceData, 0600)
}

// Restore implements CredentialStore for FileCredentialStore
func (fs *FileCredentialStore) Restore(backupPath string) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}

	return os.WriteFile(fs.storePath, backupData, 0600)
}

// loadCredentials loads credentials from the store file
func (fs *FileCredentialStore) loadCredentials() (map[string]*Credential, error) {
	data, err := os.ReadFile(fs.storePath)
	if err != nil {
		return nil, err
	}

	var credentials map[string]*Credential
	if err := json.Unmarshal(data, &credentials); err != nil {
		return nil, err
	}

	return credentials, nil
}

// saveCredentials saves credentials to the store file
func (fs *FileCredentialStore) saveCredentials(credentials map[string]*Credential) error {
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fs.storePath, data, 0600)
}
