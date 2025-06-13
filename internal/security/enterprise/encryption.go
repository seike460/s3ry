package enterprise

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"golang.org/x/crypto/scrypt"
)

// EncryptionProvider defines the interface for encryption operations
type EncryptionProvider interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
	EncryptStream(reader io.Reader, writer io.Writer) error
	DecryptStream(reader io.Reader, writer io.Writer) error
}

// ClientSideEncryption handles client-side encryption/decryption
type ClientSideEncryption struct {
	key []byte
}

// NewClientSideEncryption creates a new client-side encryption provider
func NewClientSideEncryption(password string, salt []byte) (*ClientSideEncryption, error) {
	// Derive key using scrypt
	key, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	return &ClientSideEncryption{
		key: key,
	}, nil
}

// GenerateSalt generates a random salt for key derivation
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// Encrypt encrypts data using AES-GCM
func (c *ClientSideEncryption) Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-GCM
func (c *ClientSideEncryption) Decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := data[:gcm.NonceSize()]
	ciphertext := data[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// EncryptStream encrypts data from reader to writer
func (c *ClientSideEncryption) EncryptStream(reader io.Reader, writer io.Writer) error {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Generate IV
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return fmt.Errorf("failed to generate IV: %w", err)
	}

	// Write IV to output
	if _, err := writer.Write(iv); err != nil {
		return fmt.Errorf("failed to write IV: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	streamWriter := &cipher.StreamWriter{S: stream, W: writer}

	if _, err := io.Copy(streamWriter, reader); err != nil {
		return fmt.Errorf("failed to encrypt stream: %w", err)
	}

	return nil
}

// DecryptStream decrypts data from reader to writer
func (c *ClientSideEncryption) DecryptStream(reader io.Reader, writer io.Writer) error {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Read IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(reader, iv); err != nil {
		return fmt.Errorf("failed to read IV: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	streamReader := &cipher.StreamReader{S: stream, R: reader}

	if _, err := io.Copy(writer, streamReader); err != nil {
		return fmt.Errorf("failed to decrypt stream: %w", err)
	}

	return nil
}

// KeyManager manages encryption keys
type KeyManager struct {
	keys map[string][]byte
}

// NewKeyManager creates a new key manager
func NewKeyManager() *KeyManager {
	return &KeyManager{
		keys: make(map[string][]byte),
	}
}

// GenerateKey generates a new encryption key
func (k *KeyManager) GenerateKey(keyID string) ([]byte, error) {
	key := make([]byte, 32) // 256-bit key
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	k.keys[keyID] = key
	return key, nil
}

// GetKey retrieves a key by ID
func (k *KeyManager) GetKey(keyID string) ([]byte, bool) {
	key, exists := k.keys[keyID]
	return key, exists
}

// DeleteKey deletes a key
func (k *KeyManager) DeleteKey(keyID string) {
	delete(k.keys, keyID)
}

// RotateKey rotates a key (generates new key with same ID)
func (k *KeyManager) RotateKey(keyID string) ([]byte, error) {
	return k.GenerateKey(keyID)
}

// DeriveKeyFromPassword derives an encryption key from a password
func DeriveKeyFromPassword(password string, salt []byte) ([]byte, error) {
	return scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
}

// HashPassword creates a secure hash of a password
func HashPassword(password string, salt []byte) []byte {
	hash := sha256.New()
	hash.Write([]byte(password))
	hash.Write(salt)
	return hash.Sum(nil)
}

// EncryptionConfig holds encryption configuration
type EncryptionConfig struct {
	Enabled         bool   `json:"enabled"`
	Algorithm       string `json:"algorithm"`        // AES-256-GCM, AES-256-CTR
	KeySize         int    `json:"key_size"`         // Key size in bits
	UseClientSide   bool   `json:"use_client_side"`  // Enable client-side encryption
	KeyRotationDays int    `json:"key_rotation_days"`// Days between key rotations
}

// DefaultEncryptionConfig returns default encryption configuration
func DefaultEncryptionConfig() *EncryptionConfig {
	return &EncryptionConfig{
		Enabled:         true,
		Algorithm:       "AES-256-GCM",
		KeySize:         256,
		UseClientSide:   true,
		KeyRotationDays: 90,
	}
}

// EncryptionManager manages encryption operations
type EncryptionManager struct {
	config     *EncryptionConfig
	keyManager *KeyManager
	provider   EncryptionProvider
}

// NewEncryptionManager creates a new encryption manager
func NewEncryptionManager(config *EncryptionConfig, password string) (*EncryptionManager, error) {
	if config == nil {
		config = DefaultEncryptionConfig()
	}

	keyManager := NewKeyManager()
	
	var provider EncryptionProvider
	if config.UseClientSide && password != "" {
		salt, err := GenerateSalt()
		if err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}
		
		cse, err := NewClientSideEncryption(password, salt)
		if err != nil {
			return nil, fmt.Errorf("failed to create client-side encryption: %w", err)
		}
		provider = cse
	}

	return &EncryptionManager{
		config:     config,
		keyManager: keyManager,
		provider:   provider,
	}, nil
}

// Encrypt encrypts data
func (e *EncryptionManager) Encrypt(data []byte) ([]byte, error) {
	if !e.config.Enabled || e.provider == nil {
		return data, nil
	}
	return e.provider.Encrypt(data)
}

// Decrypt decrypts data
func (e *EncryptionManager) Decrypt(data []byte) ([]byte, error) {
	if !e.config.Enabled || e.provider == nil {
		return data, nil
	}
	return e.provider.Decrypt(data)
}

// EncryptStream encrypts a stream
func (e *EncryptionManager) EncryptStream(reader io.Reader, writer io.Writer) error {
	if !e.config.Enabled || e.provider == nil {
		_, err := io.Copy(writer, reader)
		return err
	}
	return e.provider.EncryptStream(reader, writer)
}

// DecryptStream decrypts a stream
func (e *EncryptionManager) DecryptStream(reader io.Reader, writer io.Writer) error {
	if !e.config.Enabled || e.provider == nil {
		_, err := io.Copy(writer, reader)
		return err
	}
	return e.provider.DecryptStream(reader, writer)
}

// IsEnabled returns whether encryption is enabled
func (e *EncryptionManager) IsEnabled() bool {
	return e.config.Enabled && e.provider != nil
}