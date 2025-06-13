package security

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()
	
	assert.NotNil(t, config)
	assert.NotEmpty(t, config.AllowedFileExtensions)
	assert.NotEmpty(t, config.BlockedFileExtensions)
	assert.Greater(t, config.MaxFileSize, int64(0))
	assert.Greater(t, config.MaxFilenameLength, 0)
	assert.True(t, config.ScanForSecrets)
}

func TestNewValidator(t *testing.T) {
	// Test with custom config
	config := &SecurityConfig{
		MaxFileSize: 1024,
	}
	validator := NewValidator(config)
	assert.Equal(t, config, validator.config)
	
	// Test with nil config (should use default)
	validator = NewValidator(nil)
	assert.NotNil(t, validator.config)
	assert.Equal(t, DefaultSecurityConfig().MaxFileSize, validator.config.MaxFileSize)
}

func TestValidateFilename(t *testing.T) {
	validator := NewValidator(DefaultSecurityConfig())
	
	// Valid filenames
	validNames := []string{
		"test.txt",
		"document.pdf",
		"image.jpg",
		"data.csv",
		"normal-file_name.json",
	}
	
	for _, name := range validNames {
		err := validator.ValidateFilename(name)
		assert.NoError(t, err, "Should be valid: %s", name)
	}
	
	// Invalid filenames
	invalidTests := []struct {
		filename string
		reason   string
	}{
		{strings.Repeat("a", 300), "too long"},
		{"file\x00.txt", "null byte"},
		{"../test.txt", "path traversal"},
		{"file//test.txt", "double slash"},
		{"CON.txt", "reserved name"},
		{"test.exe", "blocked extension"},
		{"test.unknown", "not in allowed list"},
	}
	
	for _, test := range invalidTests {
		err := validator.ValidateFilename(test.filename)
		assert.Error(t, err, "Should be invalid (%s): %s", test.reason, test.filename)
	}
}

func TestValidateFileSize(t *testing.T) {
	config := &SecurityConfig{MaxFileSize: 1024}
	validator := NewValidator(config)
	
	// Valid sizes
	assert.NoError(t, validator.ValidateFileSize(0))
	assert.NoError(t, validator.ValidateFileSize(512))
	assert.NoError(t, validator.ValidateFileSize(1024))
	
	// Invalid sizes
	assert.Error(t, validator.ValidateFileSize(-1))
	assert.Error(t, validator.ValidateFileSize(1025))
}

func TestValidateFilePath(t *testing.T) {
	validator := NewValidator(DefaultSecurityConfig())
	
	// Valid paths
	validPaths := []string{
		"test.txt",
		"folder/test.txt",
		"./test.txt",
		"folder/subfolder/test.txt",
	}
	
	for _, path := range validPaths {
		err := validator.ValidateFilePath(path)
		assert.NoError(t, err, "Should be valid: %s", path)
	}
	
	// Invalid paths
	invalidPaths := []string{
		"/absolute/path",
		"../outside",
		"folder/../outside",
		"folder/../../outside",
	}
	
	for _, path := range invalidPaths {
		err := validator.ValidateFilePath(path)
		assert.Error(t, err, "Should be invalid: %s", path)
	}
}

func TestGetSecretPatterns(t *testing.T) {
	patterns := GetSecretPatterns()
	
	assert.NotEmpty(t, patterns)
	
	// Check that all patterns have required fields
	for _, pattern := range patterns {
		assert.NotEmpty(t, pattern.Name)
		assert.NotNil(t, pattern.Pattern)
		assert.NotEmpty(t, pattern.Description)
	}
}

func TestScanForSecrets(t *testing.T) {
	validator := NewValidator(DefaultSecurityConfig())
	
	// Content with secrets
	secretContent := []byte(`
		AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
		JWT_TOKEN=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
		API_KEY=1234567890abcdef1234567890abcdef12345678
	`)
	
	matches := validator.ScanForSecrets(secretContent)
	assert.NotEmpty(t, matches)
	
	// Clean content should have no matches
	cleanContent := []byte("This is just normal text with no secrets")
	matches = validator.ScanForSecrets(cleanContent)
	assert.Empty(t, matches)
	
	// Test with scanning disabled
	config := DefaultSecurityConfig()
	config.ScanForSecrets = false
	validator = NewValidator(config)
	
	matches = validator.ScanForSecrets(secretContent)
	assert.Empty(t, matches)
}

func TestLogSecurityEvent(t *testing.T) {
	// This function logs to the standard logger
	// We can't easily test the output without capturing it
	// but we can ensure it doesn't panic
	LogSecurityEvent("TEST_EVENT", "test details")
}

func TestGenerateSecureRandomString(t *testing.T) {
	// Test different lengths
	lengths := []int{8, 16, 32, 64}
	
	for _, length := range lengths {
		str, err := GenerateSecureRandomString(length)
		assert.NoError(t, err)
		assert.Len(t, str, length)
		assert.NotEmpty(t, str)
		
		// Ensure we get different strings each time
		str2, err := GenerateSecureRandomString(length)
		assert.NoError(t, err)
		assert.NotEqual(t, str, str2)
	}
}

func TestHashContent(t *testing.T) {
	content1 := []byte("test content")
	content2 := []byte("different content")
	content3 := []byte("test content") // same as content1
	
	hash1 := HashContent(content1)
	hash2 := HashContent(content2)
	hash3 := HashContent(content3)
	
	assert.NotEmpty(t, hash1)
	assert.NotEmpty(t, hash2)
	assert.NotEmpty(t, hash3)
	
	// Same content should produce same hash
	assert.Equal(t, hash1, hash3)
	
	// Different content should produce different hash
	assert.NotEqual(t, hash1, hash2)
	
	// Hash should be consistent length (SHA256 = 64 hex chars)
	assert.Len(t, hash1, 64)
}

func TestSanitizeForLogging(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			"Normal log message",
			"Normal log message",
		},
		{
			"AWS key: AKIAIOSFODNN7EXAMPLE",
			"AWS key: [REDACTED]",
		},
		{
			"JWT: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			"JWT: [REDACTED]",
		},
	}
	
	for _, tc := range testCases {
		result := SanitizeForLogging(tc.input)
		if strings.Contains(tc.input, "AKIA") || strings.Contains(tc.input, "eyJ") {
			assert.Contains(t, result, "[REDACTED]")
		} else {
			assert.Equal(t, tc.expected, result)
		}
	}
}

func TestIsPathTraversal(t *testing.T) {
	traversalPaths := []string{
		"../",
		"../test",
		"folder/../outside",
		"../../../etc/passwd",
		"folder\\..\\outside", // Windows style
	}
	
	for _, path := range traversalPaths {
		assert.True(t, IsPathTraversal(path), "Should detect traversal: %s", path)
	}
	
	safePaths := []string{
		"test.txt",
		"folder/test.txt",
		"./test.txt",
		"folder/subfolder/test.txt",
	}
	
	for _, path := range safePaths {
		assert.False(t, IsPathTraversal(path), "Should be safe: %s", path)
	}
}

func TestValidateS3BucketName(t *testing.T) {
	// Valid bucket names
	validNames := []string{
		"my-bucket",
		"test123",
		"bucket-name-123",
		"a.b.c",
	}
	
	for _, name := range validNames {
		err := ValidateS3BucketName(name)
		assert.NoError(t, err, "Should be valid: %s", name)
	}
	
	// Invalid bucket names
	invalidTests := []struct {
		name   string
		reason string
	}{
		{"ab", "too short"},
		{strings.Repeat("a", 64), "too long"},
		{"Bucket", "uppercase"},
		{"bucket_name", "underscore"},
		{"bucket..name", "consecutive periods"},
		{"192.168.1.1", "IP address format"},
		{"-bucket", "starts with dash"},
		{"bucket-", "ends with dash"},
	}
	
	for _, test := range invalidTests {
		err := ValidateS3BucketName(test.name)
		assert.Error(t, err, "Should be invalid (%s): %s", test.reason, test.name)
	}
}

func TestValidateS3ObjectKey(t *testing.T) {
	// Valid object keys
	validKeys := []string{
		"test.txt",
		"folder/test.txt",
		"path/to/file.jpg",
		"file with spaces.txt",
		"файл.txt", // Unicode
	}
	
	for _, key := range validKeys {
		err := ValidateS3ObjectKey(key)
		assert.NoError(t, err, "Should be valid: %s", key)
	}
	
	// Invalid object keys
	invalidTests := []struct {
		key    string
		reason string
	}{
		{"", "empty"},
		{strings.Repeat("a", 1025), "too long"},
		{"file\x00.txt", "null byte"},
		{"file\x7F.txt", "DEL character"},
	}
	
	for _, test := range invalidTests {
		err := ValidateS3ObjectKey(test.key)
		assert.Error(t, err, "Should be invalid (%s): %s", test.reason, test.key)
	}
}

func BenchmarkValidateFilename(b *testing.B) {
	validator := NewValidator(DefaultSecurityConfig())
	filename := "test-file.txt"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateFilename(filename)
	}
}

func BenchmarkScanForSecrets(b *testing.B) {
	validator := NewValidator(DefaultSecurityConfig())
	content := []byte("This is test content with no secrets in it for benchmarking purposes")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ScanForSecrets(content)
	}
}

func BenchmarkHashContent(b *testing.B) {
	content := []byte("This is test content for hashing benchmark")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashContent(content)
	}
}