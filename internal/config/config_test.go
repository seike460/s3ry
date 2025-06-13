package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	
	assert.NotNil(t, cfg)
	assert.Equal(t, "ap-northeast-1", cfg.AWS.Region)
	assert.Equal(t, "", cfg.AWS.Profile)
	assert.Equal(t, "", cfg.AWS.Endpoint)
	
	assert.Equal(t, "bubbles", cfg.UI.Mode)
	assert.Equal(t, "ja", cfg.UI.Language)
	assert.Equal(t, "default", cfg.UI.Theme)
	
	assert.Equal(t, 4, cfg.Performance.Workers)
	assert.Equal(t, 1024*1024*5, cfg.Performance.ChunkSize)
	assert.Equal(t, 30, cfg.Performance.Timeout)
	
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
	assert.Equal(t, "", cfg.Logging.File)
}

func TestLoad_NoConfigFile(t *testing.T) {
	// Save current working directory
	originalWD, _ := os.Getwd()
	defer os.Chdir(originalWD)
	
	// Create a temporary directory without config files
	tempDir := os.TempDir()
	os.Chdir(tempDir)
	
	cfg, err := Load()
	
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	// Should return default configuration
	assert.Equal(t, "ap-northeast-1", cfg.AWS.Region)
	assert.Equal(t, "bubbles", cfg.UI.Mode)
}

func TestLoad_WithConfigFile(t *testing.T) {
	// Create temporary directory
	tempDir := os.TempDir()
	configPath := filepath.Join(tempDir, "s3ry.yml")
	
	// Create test config file
	configContent := `
aws:
  region: us-west-2
  profile: test-profile
ui:
  mode: bubbles
  language: en
  theme: dark
performance:
  workers: 8
  chunk_size: 10485760
  timeout: 60
logging:
  level: debug
  format: json
  file: /var/log/s3ry.log
`
	
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err)
	defer os.Remove(configPath)
	
	// Save current working directory and change to temp dir
	originalWD, _ := os.Getwd()
	defer os.Chdir(originalWD)
	os.Chdir(tempDir)
	
	cfg, err := Load()
	
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "us-west-2", cfg.AWS.Region)
	assert.Equal(t, "test-profile", cfg.AWS.Profile)
	assert.Equal(t, "bubbles", cfg.UI.Mode)
	assert.Equal(t, "en", cfg.UI.Language)
	assert.Equal(t, "dark", cfg.UI.Theme)
	assert.Equal(t, 8, cfg.Performance.Workers)
	assert.Equal(t, 10485760, cfg.Performance.ChunkSize)
	assert.Equal(t, 60, cfg.Performance.Timeout)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "/var/log/s3ry.log", cfg.Logging.File)
}

func TestLoadFromEnv(t *testing.T) {
	// Save original environment variables
	originalVars := map[string]string{
		"AWS_REGION":         os.Getenv("AWS_REGION"),
		"AWS_DEFAULT_REGION": os.Getenv("AWS_DEFAULT_REGION"),
		"AWS_PROFILE":        os.Getenv("AWS_PROFILE"),
		"AWS_ENDPOINT_URL":   os.Getenv("AWS_ENDPOINT_URL"),
		"S3RY_UI_MODE":       os.Getenv("S3RY_UI_MODE"),
		"S3RY_LANGUAGE":      os.Getenv("S3RY_LANGUAGE"),
		"S3RY_LOG_LEVEL":     os.Getenv("S3RY_LOG_LEVEL"),
	}
	
	// Restore environment variables after test
	defer func() {
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()
	
	// Set test environment variables
	os.Setenv("AWS_REGION", "eu-west-1")
	os.Setenv("AWS_PROFILE", "env-profile")
	os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	os.Setenv("S3RY_UI_MODE", "bubbles")
	os.Setenv("S3RY_LANGUAGE", "en")
	os.Setenv("S3RY_LOG_LEVEL", "debug")
	
	cfg := Default()
	cfg.loadFromEnv()
	
	assert.Equal(t, "eu-west-1", cfg.AWS.Region)
	assert.Equal(t, "env-profile", cfg.AWS.Profile)
	assert.Equal(t, "http://localhost:4566", cfg.AWS.Endpoint)
	assert.Equal(t, "bubbles", cfg.UI.Mode)
	assert.Equal(t, "en", cfg.UI.Language)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestLoadFromEnv_AWSDefaultRegion(t *testing.T) {
	// Save original environment variables
	originalRegion := os.Getenv("AWS_REGION")
	originalDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
	
	defer func() {
		if originalRegion == "" {
			os.Unsetenv("AWS_REGION")
		} else {
			os.Setenv("AWS_REGION", originalRegion)
		}
		if originalDefaultRegion == "" {
			os.Unsetenv("AWS_DEFAULT_REGION")
		} else {
			os.Setenv("AWS_DEFAULT_REGION", originalDefaultRegion)
		}
	}()
	
	// Test AWS_DEFAULT_REGION only takes effect when region is default
	os.Unsetenv("AWS_REGION")
	os.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	
	cfg := Default()
	cfg.loadFromEnv()
	
	assert.Equal(t, "us-east-1", cfg.AWS.Region)
}

func TestSave(t *testing.T) {
	tempDir := os.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yml")
	defer os.Remove(configPath)
	
	cfg := Default()
	cfg.AWS.Region = "us-west-2"
	cfg.UI.Mode = "bubbles"
	cfg.UI.Language = "en"
	
	err := cfg.Save(configPath)
	assert.NoError(t, err)
	
	// Verify file was created
	assert.FileExists(t, configPath)
	
	// Load and verify content
	data, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "region: us-west-2")
	assert.Contains(t, string(data), "mode: bubbles")
	assert.Contains(t, string(data), "language: en")
}

func TestSave_CreateDirectory(t *testing.T) {
	tempDir := os.TempDir()
	configDir := filepath.Join(tempDir, "test-config-dir")
	configPath := filepath.Join(configDir, "config.yml")
	defer os.RemoveAll(configDir)
	
	cfg := Default()
	err := cfg.Save(configPath)
	
	assert.NoError(t, err)
	assert.FileExists(t, configPath)
	assert.DirExists(t, configDir)
}

func TestIsNewUIEnabled(t *testing.T) {
	cfg := Default()
	
	// Test default (bubbles)
	assert.True(t, cfg.IsNewUIEnabled())
	
	// Test bubbles mode
	cfg.UI.Mode = "bubbles"
	assert.True(t, cfg.IsNewUIEnabled())
	
	// Test new mode
	cfg.UI.Mode = "new"
	assert.True(t, cfg.IsNewUIEnabled())
	
	// Test legacy mode
	cfg.UI.Mode = "legacy"
	assert.False(t, cfg.IsNewUIEnabled())
}

func TestGetRegion(t *testing.T) {
	cfg := Default()
	
	// Test with configured region
	cfg.AWS.Region = "us-west-2"
	assert.Equal(t, "us-west-2", cfg.GetRegion())
	
	// Test with empty region (fallback)
	cfg.AWS.Region = ""
	assert.Equal(t, "ap-northeast-1", cfg.GetRegion())
}

func TestGetLanguage(t *testing.T) {
	cfg := Default()
	
	// Test with configured language
	cfg.UI.Language = "en"
	assert.Equal(t, "en", cfg.GetLanguage())
	
	// Test with empty language (fallback)
	cfg.UI.Language = ""
	assert.Equal(t, "ja", cfg.GetLanguage())
}

func TestSetLanguage(t *testing.T) {
	cfg := Default()
	
	cfg.SetLanguage("en")
	assert.Equal(t, "en", cfg.UI.Language)
	
	cfg.SetLanguage("ja")
	assert.Equal(t, "ja", cfg.UI.Language)
}

func TestValidateLanguage(t *testing.T) {
	cfg := Default()
	
	// Test valid languages
	assert.True(t, cfg.ValidateLanguage("en"))
	assert.True(t, cfg.ValidateLanguage("ja"))
	assert.True(t, cfg.ValidateLanguage("english"))
	assert.True(t, cfg.ValidateLanguage("japanese"))
	
	// Test invalid languages
	assert.False(t, cfg.ValidateLanguage("fr"))
	assert.False(t, cfg.ValidateLanguage(""))
	assert.False(t, cfg.ValidateLanguage("invalid"))
}

func TestNormalizeLanguage(t *testing.T) {
	cfg := Default()
	
	// Test normalization
	assert.Equal(t, "ja", cfg.NormalizeLanguage("japanese"))
	assert.Equal(t, "ja", cfg.NormalizeLanguage("jp"))
	assert.Equal(t, "en", cfg.NormalizeLanguage("english"))
	
	// Test pass-through
	assert.Equal(t, "en", cfg.NormalizeLanguage("en"))
	assert.Equal(t, "ja", cfg.NormalizeLanguage("ja"))
	assert.Equal(t, "fr", cfg.NormalizeLanguage("fr"))
}

func TestConfig_FieldAccess(t *testing.T) {
	cfg := Default()
	
	// Test all major struct fields are accessible
	assert.NotNil(t, cfg.AWS)
	assert.NotNil(t, cfg.UI)
	assert.NotNil(t, cfg.Performance)
	assert.NotNil(t, cfg.Logging)
	
	// Test field modifications
	cfg.AWS.Region = "test-region"
	cfg.AWS.Profile = "test-profile"
	cfg.AWS.Endpoint = "test-endpoint"
	
	assert.Equal(t, "test-region", cfg.AWS.Region)
	assert.Equal(t, "test-profile", cfg.AWS.Profile)
	assert.Equal(t, "test-endpoint", cfg.AWS.Endpoint)
	
	cfg.UI.Mode = "test-mode"
	cfg.UI.Language = "test-lang"
	cfg.UI.Theme = "test-theme"
	
	assert.Equal(t, "test-mode", cfg.UI.Mode)
	assert.Equal(t, "test-lang", cfg.UI.Language)
	assert.Equal(t, "test-theme", cfg.UI.Theme)
	
	cfg.Performance.Workers = 10
	cfg.Performance.ChunkSize = 1000000
	cfg.Performance.Timeout = 60
	
	assert.Equal(t, 10, cfg.Performance.Workers)
	assert.Equal(t, 1000000, cfg.Performance.ChunkSize)
	assert.Equal(t, 60, cfg.Performance.Timeout)
	
	cfg.Logging.Level = "test-level"
	cfg.Logging.Format = "test-format"
	cfg.Logging.File = "test-file"
	
	assert.Equal(t, "test-level", cfg.Logging.Level)
	assert.Equal(t, "test-format", cfg.Logging.Format)
	assert.Equal(t, "test-file", cfg.Logging.File)
}

func TestLoad_InvalidYAML(t *testing.T) {
	// Create temporary directory
	tempDir := os.TempDir()
	configPath := filepath.Join(tempDir, "s3ry.yml") // Use the filename that Load() looks for
	
	// Create invalid YAML
	invalidYAML := `
aws:
  region: us-west-2
  profile: test-profile
invalid yaml content [
`
	
	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	assert.NoError(t, err)
	defer os.Remove(configPath)
	
	// Save current working directory and change to temp dir
	originalWD, _ := os.Getwd()
	defer os.Chdir(originalWD)
	os.Chdir(tempDir)
	
	// Should fail to load due to invalid YAML
	_, err = Load()
	assert.Error(t, err)
}

func TestConfig_EdgeCases(t *testing.T) {
	cfg := Default()
	
	// Test zero values
	cfg.Performance.Workers = 0
	cfg.Performance.ChunkSize = 0
	cfg.Performance.Timeout = 0
	
	assert.Equal(t, 0, cfg.Performance.Workers)
	assert.Equal(t, 0, cfg.Performance.ChunkSize)
	assert.Equal(t, 0, cfg.Performance.Timeout)
	
	// Test empty strings
	cfg.AWS.Region = ""
	cfg.UI.Language = ""
	
	assert.Equal(t, "ap-northeast-1", cfg.GetRegion())
	assert.Equal(t, "ja", cfg.GetLanguage())
}

// Benchmark tests
func BenchmarkDefault(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := Default()
		_ = cfg
	}
}

func BenchmarkLoadFromEnv(b *testing.B) {
	cfg := Default()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.loadFromEnv()
	}
}

func BenchmarkValidateLanguage(b *testing.B) {
	cfg := Default()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.ValidateLanguage("en")
		cfg.ValidateLanguage("ja")
		cfg.ValidateLanguage("invalid")
	}
}

func BenchmarkNormalizeLanguage(b *testing.B) {
	cfg := Default()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.NormalizeLanguage("japanese")
		cfg.NormalizeLanguage("english")
		cfg.NormalizeLanguage("en")
	}
}