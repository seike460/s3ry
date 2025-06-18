package security

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	// "github.com/seike460/s3ry/internal/security/enterprise"
)

// S3SecurityWrapper provides security enhancements for S3 operations
type S3SecurityWrapper struct {
	client          *s3.Client
	securityManager *SecurityManager
	config          *S3SecurityConfig
}

// S3SecurityConfig holds S3-specific security configuration
type S3SecurityConfig struct {
	EnableClientSideEncryption   bool     `json:"enable_client_side_encryption" yaml:"enable_client_side_encryption"`
	EnableServerSideEncryption   bool     `json:"enable_server_side_encryption" yaml:"enable_server_side_encryption"`
	SSEAlgorithm                 string   `json:"sse_algorithm" yaml:"sse_algorithm"`
	KMSKeyID                     string   `json:"kms_key_id,omitempty" yaml:"kms_key_id,omitempty"`
	EncryptFileExtensions        []string `json:"encrypt_file_extensions" yaml:"encrypt_file_extensions"`
	RequireEncryptionForPatterns []string `json:"require_encryption_for_patterns" yaml:"require_encryption_for_patterns"`
	AuditAllOperations           bool     `json:"audit_all_operations" yaml:"audit_all_operations"`
	ValidateCertificates         bool     `json:"validate_certificates" yaml:"validate_certificates"`
	EnableIntegrityChecking      bool     `json:"enable_integrity_checking" yaml:"enable_integrity_checking"`
}

// DefaultS3SecurityConfig returns default S3 security configuration
func DefaultS3SecurityConfig() *S3SecurityConfig {
	return &S3SecurityConfig{
		EnableClientSideEncryption:   true,
		EnableServerSideEncryption:   true,
		SSEAlgorithm:                 "AES256",
		KMSKeyID:                     "",
		EncryptFileExtensions:        []string{".txt", ".csv", ".json", ".xml", ".log"},
		RequireEncryptionForPatterns: []string{"*sensitive*", "*private*", "*confidential*"},
		AuditAllOperations:           true,
		ValidateCertificates:         true,
		EnableIntegrityChecking:      true,
	}
}

// NewS3SecurityWrapper creates a new S3 security wrapper
func NewS3SecurityWrapper(client *s3.Client, securityManager *SecurityManager) *S3SecurityWrapper {
	return &S3SecurityWrapper{
		client:          client,
		securityManager: securityManager,
		config:          DefaultS3SecurityConfig(),
	}
}

// SecureGetObject performs a secure S3 GetObject operation with decryption
func (s3w *S3SecurityWrapper) SecureGetObject(ctx context.Context, input *s3.GetObjectInput, userID string) (*s3.GetObjectOutput, error) {
	// TODO: Audit the operation when audit logger interface is implemented
	// if s3w.config.AuditAllOperations && s3w.securityManager.IsSecurityEnabled() {
	//     auditLogger := s3w.securityManager.GetAuditLogger()
	//     if auditLogger != nil {
	//         auditLogger.LogAction(userID, "s3_get_object", aws.ToString(input.Key), "STARTED", map[string]interface{}{
	//             "bucket": aws.ToString(input.Bucket),
	//             "key":    aws.ToString(input.Key),
	//         })
	//     }
	// }

	// Authorize the operation
	err := s3w.securityManager.AuthorizeAction(ctx, userID, "s3:read", fmt.Sprintf("bucket:%s", aws.ToString(input.Bucket)), "", "")
	if err != nil {
		return nil, fmt.Errorf("authorization failed: %w", err)
	}

	// Execute with concurrent guarding
	var result *s3.GetObjectOutput
	err = s3w.securityManager.GuardWorkerExecution(ctx, 0, func() error {
		var execErr error
		result, execErr = s3w.client.GetObject(ctx, input)
		return execErr
	})

	if err != nil {
		// Handle error securely
		secureErr := s3w.securityManager.HandleSecureError(ctx, err, "s3_get_object")
		return nil, fmt.Errorf("S3 operation failed: %s", secureErr.SafeMessage)
	}

	// Apply client-side decryption if enabled
	if s3w.config.EnableClientSideEncryption && s3w.shouldDecryptObject(aws.ToString(input.Key)) {
		result, err = s3w.decryptObjectContent(ctx, result)
		if err != nil {
			return nil, fmt.Errorf("decryption failed: %w", err)
		}
	}

	// Verify integrity if enabled
	if s3w.config.EnableIntegrityChecking {
		if err := s3w.verifyObjectIntegrity(result); err != nil {
			return nil, fmt.Errorf("integrity check failed: %w", err)
		}
	}

	// Log successful operation
	if s3w.config.AuditAllOperations && s3w.securityManager.IsSecurityEnabled() {
		auditLogger := s3w.securityManager.GetAuditLogger()
		if auditLogger != nil {
			// TODO: auditLogger.LogAction(userID, "s3_get_object", aws.ToString(input.Key), "SUCCESS", map[string]interface{}{
			//     "bucket":       aws.ToString(input.Bucket),
			//     "key":          aws.ToString(input.Key),
			//     "content_type": aws.ToString(result.ContentType),
			// })
		}
	}

	return result, nil
}

// SecurePutObject performs a secure S3 PutObject operation with encryption
func (s3w *S3SecurityWrapper) SecurePutObject(ctx context.Context, input *s3.PutObjectInput, userID string) (*s3.PutObjectOutput, error) {
	// Audit the operation
	if s3w.config.AuditAllOperations && s3w.securityManager.IsSecurityEnabled() {
		auditLogger := s3w.securityManager.GetAuditLogger()
		if auditLogger != nil {
			// TODO: auditLogger.LogAction(userID, "s3_put_object", aws.ToString(input.Key), "STARTED", map[string]interface{}{
			//     "bucket": aws.ToString(input.Bucket),
			//     "key":    aws.ToString(input.Key),
			// })
		}
	}

	// Authorize the operation
	err := s3w.securityManager.AuthorizeAction(ctx, userID, "s3:write", fmt.Sprintf("bucket:%s", aws.ToString(input.Bucket)), "", "")
	if err != nil {
		return nil, fmt.Errorf("authorization failed: %w", err)
	}

	// Apply server-side encryption if enabled
	if s3w.config.EnableServerSideEncryption {
		s3w.addServerSideEncryption(input)
	}

	// Apply client-side encryption if enabled and required
	if s3w.config.EnableClientSideEncryption && s3w.shouldEncryptObject(aws.ToString(input.Key)) {
		input, err = s3w.encryptObjectContent(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("encryption failed: %w", err)
		}
	}

	// Add integrity metadata
	if s3w.config.EnableIntegrityChecking {
		s3w.addIntegrityMetadata(input)
	}

	// Execute with concurrent guarding
	var result *s3.PutObjectOutput
	err = s3w.securityManager.GuardWorkerExecution(ctx, 0, func() error {
		var execErr error
		result, execErr = s3w.client.PutObject(ctx, input)
		return execErr
	})

	if err != nil {
		// Handle error securely
		secureErr := s3w.securityManager.HandleSecureError(ctx, err, "s3_put_object")
		return nil, fmt.Errorf("S3 operation failed: %s", secureErr.SafeMessage)
	}

	// Log successful operation
	if s3w.config.AuditAllOperations && s3w.securityManager.IsSecurityEnabled() {
		auditLogger := s3w.securityManager.GetAuditLogger()
		if auditLogger != nil {
			// TODO: auditLogger.LogAction(userID, "s3_put_object", aws.ToString(input.Key), "SUCCESS", map[string]interface{}{
			//	"bucket":       aws.ToString(input.Bucket),
			//	"key":          aws.ToString(input.Key),
			//	"content_type": aws.ToString(input.ContentType),
			//	"etag":         aws.ToString(result.ETag),
			// })
		}
	}

	return result, nil
}

// SecureListObjects performs a secure S3 ListObjects operation
func (s3w *S3SecurityWrapper) SecureListObjects(ctx context.Context, input *s3.ListObjectsV2Input, userID string) (*s3.ListObjectsV2Output, error) {
	// Audit the operation
	if s3w.config.AuditAllOperations && s3w.securityManager.IsSecurityEnabled() {
		auditLogger := s3w.securityManager.GetAuditLogger()
		if auditLogger != nil {
			// TODO: auditLogger.LogAction(userID, "s3_list_objects", aws.ToString(input.Prefix), "STARTED", map[string]interface{}{
			//	"bucket": aws.ToString(input.Bucket),
			//	"prefix": aws.ToString(input.Prefix),
			// })
		}
	}

	// Authorize the operation
	err := s3w.securityManager.AuthorizeAction(ctx, userID, "s3:list", fmt.Sprintf("bucket:%s", aws.ToString(input.Bucket)), "", "")
	if err != nil {
		return nil, fmt.Errorf("authorization failed: %w", err)
	}

	// Execute with concurrent guarding
	var result *s3.ListObjectsV2Output
	err = s3w.securityManager.GuardWorkerExecution(ctx, 0, func() error {
		var execErr error
		result, execErr = s3w.client.ListObjectsV2(ctx, input)
		return execErr
	})

	if err != nil {
		// Handle error securely
		secureErr := s3w.securityManager.HandleSecureError(ctx, err, "s3_list_objects")
		return nil, fmt.Errorf("S3 operation failed: %s", secureErr.SafeMessage)
	}

	// Filter sensitive objects based on user permissions
	result = s3w.filterSensitiveObjects(result, userID)

	// Log successful operation
	if s3w.config.AuditAllOperations && s3w.securityManager.IsSecurityEnabled() {
		auditLogger := s3w.securityManager.GetAuditLogger()
		if auditLogger != nil {
			// TODO: auditLogger.LogAction(userID, "s3_list_objects", aws.ToString(input.Prefix), "SUCCESS", map[string]interface{}{
			//	"bucket":       aws.ToString(input.Bucket),
			//	"prefix":       aws.ToString(input.Prefix),
			//	"object_count": len(result.Contents),
			// })
		}
	}

	return result, nil
}

// SecureDeleteObject performs a secure S3 DeleteObject operation
func (s3w *S3SecurityWrapper) SecureDeleteObject(ctx context.Context, input *s3.DeleteObjectInput, userID string) (*s3.DeleteObjectOutput, error) {
	// Audit the operation
	if s3w.config.AuditAllOperations && s3w.securityManager.IsSecurityEnabled() {
		auditLogger := s3w.securityManager.GetAuditLogger()
		if auditLogger != nil {
			// TODO: auditLogger.LogAction(userID, "s3_delete_object", aws.ToString(input.Key), "STARTED", map[string]interface{}{
			//	"bucket": aws.ToString(input.Bucket),
			//	"key":    aws.ToString(input.Key),
			// })
		}
	}

	// Authorize the operation (delete requires higher permissions)
	err := s3w.securityManager.AuthorizeAction(ctx, userID, "s3:delete", fmt.Sprintf("bucket:%s", aws.ToString(input.Bucket)), "", "")
	if err != nil {
		return nil, fmt.Errorf("authorization failed: %w", err)
	}

	// Additional security check for delete operations
	if s3w.isSensitiveObject(aws.ToString(input.Key)) {
		// Require additional verification for sensitive objects
		err = s3w.securityManager.AuthorizeAction(ctx, userID, "s3:admin", fmt.Sprintf("bucket:%s", aws.ToString(input.Bucket)), "", "")
		if err != nil {
			return nil, fmt.Errorf("admin authorization required for sensitive object deletion: %w", err)
		}
	}

	// Execute with concurrent guarding
	var result *s3.DeleteObjectOutput
	err = s3w.securityManager.GuardWorkerExecution(ctx, 0, func() error {
		var execErr error
		result, execErr = s3w.client.DeleteObject(ctx, input)
		return execErr
	})

	if err != nil {
		// Handle error securely
		secureErr := s3w.securityManager.HandleSecureError(ctx, err, "s3_delete_object")
		return nil, fmt.Errorf("S3 operation failed: %s", secureErr.SafeMessage)
	}

	// Log successful operation
	if s3w.config.AuditAllOperations && s3w.securityManager.IsSecurityEnabled() {
		auditLogger := s3w.securityManager.GetAuditLogger()
		if auditLogger != nil {
			// TODO: auditLogger.LogAction(userID, "s3_delete_object", aws.ToString(input.Key), "SUCCESS", map[string]interface{}{
			//	"bucket": aws.ToString(input.Bucket),
			//	"key":    aws.ToString(input.Key),
			// })
		}
	}

	return result, nil
}

// Helper methods for encryption and security

// shouldEncryptObject determines if an object should be encrypted
func (s3w *S3SecurityWrapper) shouldEncryptObject(key string) bool {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(key))
	for _, encryptExt := range s3w.config.EncryptFileExtensions {
		if ext == encryptExt {
			return true
		}
	}

	// Check pattern requirements
	keyLower := strings.ToLower(key)
	for _, pattern := range s3w.config.RequireEncryptionForPatterns {
		if strings.Contains(keyLower, strings.Trim(pattern, "*")) {
			return true
		}
	}

	return false
}

// shouldDecryptObject determines if an object should be decrypted
func (s3w *S3SecurityWrapper) shouldDecryptObject(key string) bool {
	return s3w.shouldEncryptObject(key)
}

// isSensitiveObject determines if an object is considered sensitive
func (s3w *S3SecurityWrapper) isSensitiveObject(key string) bool {
	keyLower := strings.ToLower(key)
	sensitivePatterns := []string{"sensitive", "private", "confidential", "secret", "credential"}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(keyLower, pattern) {
			return true
		}
	}

	return false
}

// encryptObjectContent encrypts the object content using client-side encryption
func (s3w *S3SecurityWrapper) encryptObjectContent(ctx context.Context, input *s3.PutObjectInput) (*s3.PutObjectInput, error) {
	if input.Body == nil {
		return input, nil
	}

	// Read the content
	content, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object content: %w", err)
	}

	// Encrypt the content
	encryptedContent, err := s3w.securityManager.EncryptData(content)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt content: %w", err)
	}

	// Create new input with encrypted content
	encryptedInput := *input
	encryptedInput.Body = strings.NewReader(string(encryptedContent))
	encryptedInput.ContentLength = aws.Int64(int64(len(encryptedContent)))

	// Add encryption metadata
	if encryptedInput.Metadata == nil {
		encryptedInput.Metadata = make(map[string]string)
	}
	encryptedInput.Metadata["s3ry-encrypted"] = "true"
	encryptedInput.Metadata["s3ry-encryption-version"] = "1"

	return &encryptedInput, nil
}

// decryptObjectContent decrypts the object content using client-side decryption
func (s3w *S3SecurityWrapper) decryptObjectContent(ctx context.Context, output *s3.GetObjectOutput) (*s3.GetObjectOutput, error) {
	// Check if object is encrypted
	if output.Metadata != nil {
		if encrypted, exists := output.Metadata["s3ry-encrypted"]; !exists || encrypted != "true" {
			return output, nil // Not encrypted by us
		}
	} else {
		return output, nil // No metadata, assume not encrypted
	}

	if output.Body == nil {
		return output, nil
	}

	// Read the encrypted content
	encryptedContent, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read encrypted content: %w", err)
	}

	// Decrypt the content
	decryptedContent, err := s3w.securityManager.DecryptData(encryptedContent)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt content: %w", err)
	}

	// Create new output with decrypted content
	decryptedOutput := *output
	decryptedOutput.Body = io.NopCloser(strings.NewReader(string(decryptedContent)))
	decryptedOutput.ContentLength = aws.Int64(int64(len(decryptedContent)))

	return &decryptedOutput, nil
}

// addServerSideEncryption adds server-side encryption parameters
func (s3w *S3SecurityWrapper) addServerSideEncryption(input *s3.PutObjectInput) {
	switch s3w.config.SSEAlgorithm {
	case "AES256":
		input.ServerSideEncryption = types.ServerSideEncryptionAes256
	case "aws:kms":
		input.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		if s3w.config.KMSKeyID != "" {
			input.SSEKMSKeyId = aws.String(s3w.config.KMSKeyID)
		}
	}
}

// addIntegrityMetadata adds integrity checking metadata
func (s3w *S3SecurityWrapper) addIntegrityMetadata(input *s3.PutObjectInput) {
	if input.Metadata == nil {
		input.Metadata = make(map[string]string)
	}
	input.Metadata["s3ry-integrity-enabled"] = "true"
	input.Metadata["s3ry-created-at"] = fmt.Sprintf("%d", time.Now().Unix())
}

// verifyObjectIntegrity verifies object integrity
func (s3w *S3SecurityWrapper) verifyObjectIntegrity(output *s3.GetObjectOutput) error {
	if output.Metadata == nil {
		return nil // No integrity metadata
	}

	if enabled, exists := output.Metadata["s3ry-integrity-enabled"]; !exists || enabled != "true" {
		return nil // Integrity checking not enabled for this object
	}

	// Additional integrity checks would be implemented here
	// For example, checksum verification, timestamp validation, etc.

	return nil
}

// filterSensitiveObjects filters out objects the user shouldn't see
func (s3w *S3SecurityWrapper) filterSensitiveObjects(output *s3.ListObjectsV2Output, userID string) *s3.ListObjectsV2Output {
	if output.Contents == nil {
		return output
	}

	var filteredContents []types.Object
	for _, obj := range output.Contents {
		key := aws.ToString(obj.Key)

		// Check if user has permission to see this object
		if s3w.isSensitiveObject(key) {
			// Verify user has admin permissions for sensitive objects
			if err := s3w.securityManager.AuthorizeAction(context.Background(), userID, "s3:admin", "", "", ""); err != nil {
				continue // Skip this object
			}
		}

		filteredContents = append(filteredContents, obj)
	}

	filteredOutput := *output
	filteredOutput.Contents = filteredContents
	filteredOutput.KeyCount = aws.Int32(int32(len(filteredContents)))

	return &filteredOutput
}

// SetConfig updates the S3 security configuration
func (s3w *S3SecurityWrapper) SetConfig(config *S3SecurityConfig) {
	s3w.config = config
}

// GetClient returns the underlying S3 client
func (s3w *S3SecurityWrapper) GetClient() *s3.Client {
	return s3w.client
}
