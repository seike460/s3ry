package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
)

// SecurityConfig holds security configuration
type SecurityConfig struct {
	AllowedFileExtensions []string
	BlockedFileExtensions []string
	MaxFileSize          int64 // in bytes
	MaxFilenameLength    int
	ScanForSecrets       bool
}

// DefaultSecurityConfig returns a default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		AllowedFileExtensions: []string{".txt", ".csv", ".json", ".xml", ".log", ".md", ".pdf", ".jpg", ".png", ".gif"},
		BlockedFileExtensions: []string{".exe", ".bat", ".cmd", ".sh", ".ps1", ".scr", ".com", ".pif"},
		MaxFileSize:          100 * 1024 * 1024, // 100MB
		MaxFilenameLength:    255,
		ScanForSecrets:       true,
	}
}

// Validator handles security validation
type Validator struct {
	config *SecurityConfig
}

// NewValidator creates a new security validator
func NewValidator(config *SecurityConfig) *Validator {
	if config == nil {
		config = DefaultSecurityConfig()
	}
	return &Validator{config: config}
}

// ValidateFilename checks if a filename is safe
func (v *Validator) ValidateFilename(filename string) error {
	// Check filename length
	if len(filename) > v.config.MaxFilenameLength {
		return fmt.Errorf("filename too long: %d characters (max: %d)", len(filename), v.config.MaxFilenameLength)
	}
	
	// Check for null bytes and control characters
	if strings.ContainsAny(filename, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f") {
		return fmt.Errorf("filename contains invalid control characters")
	}
	
	// Check for path traversal attempts
	if strings.Contains(filename, "..") || strings.Contains(filename, "//") {
		return fmt.Errorf("filename contains path traversal patterns")
	}
	
	// Check for reserved Windows filenames
	reservedNames := []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
	baseName := strings.ToUpper(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	for _, reserved := range reservedNames {
		if baseName == reserved {
			return fmt.Errorf("filename uses reserved name: %s", reserved)
		}
	}
	
	// Check file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" {
		// Check blocked extensions
		for _, blocked := range v.config.BlockedFileExtensions {
			if ext == strings.ToLower(blocked) {
				return fmt.Errorf("file extension not allowed: %s", ext)
			}
		}
		
		// Check allowed extensions (if specified)
		if len(v.config.AllowedFileExtensions) > 0 {
			allowed := false
			for _, allowedExt := range v.config.AllowedFileExtensions {
				if ext == strings.ToLower(allowedExt) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("file extension not in allowed list: %s", ext)
			}
		}
	}
	
	return nil
}

// ValidateFileSize checks if a file size is within limits
func (v *Validator) ValidateFileSize(size int64) error {
	if size > v.config.MaxFileSize {
		return fmt.Errorf("file size too large: %d bytes (max: %d)", size, v.config.MaxFileSize)
	}
	if size < 0 {
		return fmt.Errorf("invalid file size: %d", size)
	}
	return nil
}

// ValidateFilePath ensures a file path is safe and within allowed boundaries
func (v *Validator) ValidateFilePath(filePath string) error {
	// Check for path traversal using the original path before cleaning
	if IsPathTraversal(filePath) {
		return fmt.Errorf("path traversal detected: %s", filePath)
	}
	
	// Clean the path
	cleanPath := filepath.Clean(filePath)
	
	// Check for absolute paths (should be relative)
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("absolute paths not allowed: %s", filePath)
	}
	
	// Validate each component of the path
	components := strings.Split(cleanPath, string(filepath.Separator))
	for _, component := range components {
		if component == "" || component == "." {
			continue
		}
		if err := v.ValidateFilename(component); err != nil {
			return fmt.Errorf("invalid path component '%s': %w", component, err)
		}
	}
	
	return nil
}

// SecretPattern represents a pattern to detect secrets
type SecretPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	Description string
}

// GetSecretPatterns returns common secret patterns to scan for
func GetSecretPatterns() []SecretPattern {
	return []SecretPattern{
		{
			Name:        "AWS Access Key",
			Pattern:     regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
			Description: "AWS Access Key ID",
		},
		{
			Name:        "AWS Secret Key",
			Pattern:     regexp.MustCompile(`[0-9a-zA-Z/+]{40}`),
			Description: "AWS Secret Access Key",
		},
		{
			Name:        "Generic API Key",
			Pattern:     regexp.MustCompile(`(?i)api[_-]?key[_-]?[:=]\s*[0-9a-zA-Z]{20,}`),
			Description: "Generic API Key",
		},
		{
			Name:        "JWT Token",
			Pattern:     regexp.MustCompile(`eyJ[0-9a-zA-Z_-]*\.eyJ[0-9a-zA-Z_-]*\.[0-9a-zA-Z_-]*`),
			Description: "JSON Web Token",
		},
		{
			Name:        "Private Key",
			Pattern:     regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`),
			Description: "Private Key",
		},
		{
			Name:        "Database URL",
			Pattern:     regexp.MustCompile(`(?i)(mysql|postgres|mongodb)://[^\\s]+`),
			Description: "Database Connection String",
		},
	}
}

// ScanForSecrets scans content for potential secrets
func (v *Validator) ScanForSecrets(content []byte) []SecretMatch {
	if !v.config.ScanForSecrets {
		return nil
	}
	
	var matches []SecretMatch
	patterns := GetSecretPatterns()
	contentStr := string(content)
	
	for _, pattern := range patterns {
		if pattern.Pattern.MatchString(contentStr) {
			match := SecretMatch{
				Pattern:     pattern.Name,
				Description: pattern.Description,
				Content:     pattern.Pattern.FindString(contentStr),
			}
			matches = append(matches, match)
		}
	}
	
	return matches
}

// SecretMatch represents a detected secret
type SecretMatch struct {
	Pattern     string
	Description string
	Content     string
}

// LogSecurityEvent logs a security-related event
func LogSecurityEvent(event, details string) {
	log.Printf("[SECURITY] %s: %s", event, details)
}

// GenerateSecureRandomString generates a cryptographically secure random string
func GenerateSecureRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes)[:length], nil
}

// HashContent creates a SHA256 hash of content
func HashContent(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// SanitizeForLogging removes potentially sensitive information from strings for logging
func SanitizeForLogging(input string) string {
	// Remove common secret patterns
	patterns := []string{
		`AKIA[0-9A-Z]{16}`,           // AWS Access Keys
		`eyJ[0-9a-zA-Z_-]*\.eyJ[0-9a-zA-Z_-]*\.[0-9a-zA-Z_-]*`, // JWT tokens
	}
	
	sanitized := input
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		sanitized = re.ReplaceAllString(sanitized, "[REDACTED]")
	}
	
	return sanitized
}

// IsPathTraversal checks if a path contains traversal attempts
func IsPathTraversal(path string) bool {
	// Check original path for traversal patterns before cleaning
	if strings.Contains(path, "../") || strings.Contains(path, "..\\") {
		return true
	}
	if strings.Contains(path, "/..") || strings.Contains(path, "\\..") {
		return true
	}
	
	cleanPath := filepath.Clean(path)
	// Check for direct .. at start or .. followed by separator
	if strings.HasPrefix(cleanPath, "..") {
		return true
	}
	// Check for .. in the path (after cleaning)
	if strings.Contains(cleanPath, string(filepath.Separator)+"..") || strings.Contains(cleanPath, ".."+string(filepath.Separator)) {
		return true
	}
	
	return false
}

// ValidateS3BucketName validates S3 bucket name according to AWS rules
func ValidateS3BucketName(bucketName string) error {
	if len(bucketName) < 3 || len(bucketName) > 63 {
		return fmt.Errorf("bucket name must be between 3 and 63 characters")
	}
	
	// Check for valid characters and patterns
	validBucketName := regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*[a-z0-9]$`)
	if !validBucketName.MatchString(bucketName) {
		return fmt.Errorf("bucket name contains invalid characters")
	}
	
	// Check for consecutive periods
	if strings.Contains(bucketName, "..") {
		return fmt.Errorf("bucket name cannot contain consecutive periods")
	}
	
	// Check for IP address format
	ipPattern := regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
	if ipPattern.MatchString(bucketName) {
		return fmt.Errorf("bucket name cannot be formatted as an IP address")
	}
	
	return nil
}

// ValidateS3ObjectKey validates S3 object key
func ValidateS3ObjectKey(objectKey string) error {
	if len(objectKey) == 0 {
		return fmt.Errorf("object key cannot be empty")
	}
	
	if len(objectKey) > 1024 {
		return fmt.Errorf("object key too long: %d characters (max: 1024)", len(objectKey))
	}
	
	// Check for invalid characters
	invalidChars := []string{"\x00", "\x7F"}
	for _, char := range invalidChars {
		if strings.Contains(objectKey, char) {
			return fmt.Errorf("object key contains invalid character")
		}
	}
	
	return nil
}