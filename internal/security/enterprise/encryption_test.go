package enterprise

import (
	"bytes"
	"testing"
)

func TestGenerateSalt(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}
	
	if len(salt) != 32 {
		t.Errorf("Expected salt length 32, got %d", len(salt))
	}
	
	// Generate another salt to ensure they're different
	salt2, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}
	
	if bytes.Equal(salt, salt2) {
		t.Error("Generated salts should be different")
	}
}

func TestNewClientSideEncryption(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}
	
	cse, err := NewClientSideEncryption("testpassword", salt)
	if err != nil {
		t.Fatalf("NewClientSideEncryption failed: %v", err)
	}
	
	if cse == nil {
		t.Fatal("NewClientSideEncryption returned nil")
	}
	
	if len(cse.key) != 32 {
		t.Errorf("Expected key length 32, got %d", len(cse.key))
	}
}

func TestEncryptDecrypt(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}
	
	cse, err := NewClientSideEncryption("testpassword", salt)
	if err != nil {
		t.Fatalf("NewClientSideEncryption failed: %v", err)
	}
	
	originalData := []byte("Hello, World! This is test data for encryption.")
	
	// Encrypt
	encryptedData, err := cse.Encrypt(originalData)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	
	if bytes.Equal(originalData, encryptedData) {
		t.Error("Encrypted data should be different from original")
	}
	
	// Decrypt
	decryptedData, err := cse.Decrypt(encryptedData)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	
	if !bytes.Equal(originalData, decryptedData) {
		t.Error("Decrypted data should match original")
	}
}

func TestEncryptDecryptStream(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}
	
	cse, err := NewClientSideEncryption("testpassword", salt)
	if err != nil {
		t.Fatalf("NewClientSideEncryption failed: %v", err)
	}
	
	originalData := []byte("This is streaming test data for encryption and decryption.")
	
	// Encrypt stream
	var encryptedBuffer bytes.Buffer
	reader := bytes.NewReader(originalData)
	
	err = cse.EncryptStream(reader, &encryptedBuffer)
	if err != nil {
		t.Fatalf("EncryptStream failed: %v", err)
	}
	
	encryptedData := encryptedBuffer.Bytes()
	if bytes.Equal(originalData, encryptedData) {
		t.Error("Encrypted stream data should be different from original")
	}
	
	// Decrypt stream
	var decryptedBuffer bytes.Buffer
	encryptedReader := bytes.NewReader(encryptedData)
	
	err = cse.DecryptStream(encryptedReader, &decryptedBuffer)
	if err != nil {
		t.Fatalf("DecryptStream failed: %v", err)
	}
	
	decryptedData := decryptedBuffer.Bytes()
	if !bytes.Equal(originalData, decryptedData) {
		t.Error("Decrypted stream data should match original")
	}
}

func TestDecryptInvalidData(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}
	
	cse, err := NewClientSideEncryption("testpassword", salt)
	if err != nil {
		t.Fatalf("NewClientSideEncryption failed: %v", err)
	}
	
	// Try to decrypt invalid data
	invalidData := []byte("this is not encrypted data")
	_, err = cse.Decrypt(invalidData)
	if err == nil {
		t.Error("Decrypt should fail with invalid data")
	}
	
	// Try to decrypt data that's too short
	shortData := []byte("short")
	_, err = cse.Decrypt(shortData)
	if err == nil {
		t.Error("Decrypt should fail with data that's too short")
	}
}

func TestNewKeyManager(t *testing.T) {
	km := NewKeyManager()
	if km == nil {
		t.Fatal("NewKeyManager returned nil")
	}
	
	if km.keys == nil {
		t.Error("KeyManager keys map not initialized")
	}
}

func TestGenerateKey(t *testing.T) {
	km := NewKeyManager()
	
	key, err := km.GenerateKey("test-key")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	
	if len(key) != 32 {
		t.Errorf("Expected key length 32, got %d", len(key))
	}
	
	// Verify key is stored
	storedKey, exists := km.GetKey("test-key")
	if !exists {
		t.Error("Key should be stored in manager")
	}
	
	if !bytes.Equal(key, storedKey) {
		t.Error("Stored key should match generated key")
	}
}

func TestGetKey(t *testing.T) {
	km := NewKeyManager()
	
	// Test getting non-existent key
	_, exists := km.GetKey("nonexistent")
	if exists {
		t.Error("GetKey should return false for non-existent key")
	}
	
	// Generate and get key
	originalKey, err := km.GenerateKey("test-key")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	
	retrievedKey, exists := km.GetKey("test-key")
	if !exists {
		t.Error("GetKey should return true for existing key")
	}
	
	if !bytes.Equal(originalKey, retrievedKey) {
		t.Error("Retrieved key should match original")
	}
}

func TestDeleteKey(t *testing.T) {
	km := NewKeyManager()
	
	// Generate key
	_, err := km.GenerateKey("test-key")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	
	// Verify key exists
	_, exists := km.GetKey("test-key")
	if !exists {
		t.Error("Key should exist before deletion")
	}
	
	// Delete key
	km.DeleteKey("test-key")
	
	// Verify key is deleted
	_, exists = km.GetKey("test-key")
	if exists {
		t.Error("Key should not exist after deletion")
	}
}

func TestRotateKey(t *testing.T) {
	km := NewKeyManager()
	
	// Generate initial key
	originalKey, err := km.GenerateKey("test-key")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	
	// Rotate key
	newKey, err := km.RotateKey("test-key")
	if err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}
	
	if bytes.Equal(originalKey, newKey) {
		t.Error("Rotated key should be different from original")
	}
	
	// Verify new key is stored
	storedKey, exists := km.GetKey("test-key")
	if !exists {
		t.Error("Rotated key should be stored")
	}
	
	if !bytes.Equal(newKey, storedKey) {
		t.Error("Stored key should match rotated key")
	}
}

func TestDeriveKeyFromPassword(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}
	
	key1, err := DeriveKeyFromPassword("password", salt)
	if err != nil {
		t.Fatalf("DeriveKeyFromPassword failed: %v", err)
	}
	
	if len(key1) != 32 {
		t.Errorf("Expected key length 32, got %d", len(key1))
	}
	
	// Same password and salt should produce same key
	key2, err := DeriveKeyFromPassword("password", salt)
	if err != nil {
		t.Fatalf("DeriveKeyFromPassword failed: %v", err)
	}
	
	if !bytes.Equal(key1, key2) {
		t.Error("Same password and salt should produce same key")
	}
	
	// Different salt should produce different key
	salt2, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}
	
	key3, err := DeriveKeyFromPassword("password", salt2)
	if err != nil {
		t.Fatalf("DeriveKeyFromPassword failed: %v", err)
	}
	
	if bytes.Equal(key1, key3) {
		t.Error("Different salt should produce different key")
	}
}

func TestHashPassword(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}
	
	hash1 := HashPassword("password", salt)
	if len(hash1) == 0 {
		t.Error("Password hash should not be empty")
	}
	
	// Same password and salt should produce same hash
	hash2 := HashPassword("password", salt)
	if !bytes.Equal(hash1, hash2) {
		t.Error("Same password and salt should produce same hash")
	}
	
	// Different password should produce different hash
	hash3 := HashPassword("different", salt)
	if bytes.Equal(hash1, hash3) {
		t.Error("Different password should produce different hash")
	}
}

func TestDefaultEncryptionConfig(t *testing.T) {
	config := DefaultEncryptionConfig()
	if config == nil {
		t.Fatal("DefaultEncryptionConfig returned nil")
	}
	
	if !config.Enabled {
		t.Error("Encryption should be enabled by default")
	}
	
	if config.Algorithm != "AES-256-GCM" {
		t.Errorf("Expected algorithm 'AES-256-GCM', got '%s'", config.Algorithm)
	}
	
	if config.KeySize != 256 {
		t.Errorf("Expected key size 256, got %d", config.KeySize)
	}
	
	if !config.UseClientSide {
		t.Error("Client-side encryption should be enabled by default")
	}
}

func TestNewEncryptionManager(t *testing.T) {
	config := DefaultEncryptionConfig()
	
	// Test with password
	em, err := NewEncryptionManager(config, "testpassword")
	if err != nil {
		t.Fatalf("NewEncryptionManager failed: %v", err)
	}
	
	if em == nil {
		t.Fatal("NewEncryptionManager returned nil")
	}
	
	if em.provider == nil {
		t.Error("Encryption provider should be initialized")
	}
	
	// Test with nil config
	em2, err := NewEncryptionManager(nil, "testpassword")
	if err != nil {
		t.Fatalf("NewEncryptionManager with nil config failed: %v", err)
	}
	
	if em2 == nil {
		t.Fatal("NewEncryptionManager with nil config returned nil")
	}
}

func TestEncryptionManagerOperations(t *testing.T) {
	config := DefaultEncryptionConfig()
	em, err := NewEncryptionManager(config, "testpassword")
	if err != nil {
		t.Fatalf("NewEncryptionManager failed: %v", err)
	}
	
	originalData := []byte("Test data for encryption manager")
	
	// Test Encrypt/Decrypt
	encryptedData, err := em.Encrypt(originalData)
	if err != nil {
		t.Fatalf("EncryptionManager Encrypt failed: %v", err)
	}
	
	decryptedData, err := em.Decrypt(encryptedData)
	if err != nil {
		t.Fatalf("EncryptionManager Decrypt failed: %v", err)
	}
	
	if !bytes.Equal(originalData, decryptedData) {
		t.Error("Decrypted data should match original")
	}
	
	// Test EncryptStream/DecryptStream
	var encryptedBuffer bytes.Buffer
	reader := bytes.NewReader(originalData)
	
	err = em.EncryptStream(reader, &encryptedBuffer)
	if err != nil {
		t.Fatalf("EncryptionManager EncryptStream failed: %v", err)
	}
	
	var decryptedBuffer bytes.Buffer
	encryptedReader := bytes.NewReader(encryptedBuffer.Bytes())
	
	err = em.DecryptStream(encryptedReader, &decryptedBuffer)
	if err != nil {
		t.Fatalf("EncryptionManager DecryptStream failed: %v", err)
	}
	
	if !bytes.Equal(originalData, decryptedBuffer.Bytes()) {
		t.Error("Decrypted stream data should match original")
	}
}

func TestEncryptionManagerDisabled(t *testing.T) {
	config := DefaultEncryptionConfig()
	config.Enabled = false
	
	em, err := NewEncryptionManager(config, "")
	if err != nil {
		t.Fatalf("NewEncryptionManager failed: %v", err)
	}
	
	if em.IsEnabled() {
		t.Error("Encryption should be disabled")
	}
	
	originalData := []byte("Test data")
	
	// When disabled, operations should return data unchanged
	encryptedData, err := em.Encrypt(originalData)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	
	if !bytes.Equal(originalData, encryptedData) {
		t.Error("When disabled, Encrypt should return original data")
	}
	
	decryptedData, err := em.Decrypt(originalData)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	
	if !bytes.Equal(originalData, decryptedData) {
		t.Error("When disabled, Decrypt should return original data")
	}
}