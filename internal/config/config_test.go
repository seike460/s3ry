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
	// Region stays empty so the AWS SDK resolution chain applies.
	assert.Equal(t, "", cfg.AWS.Region)
	assert.Equal(t, "", cfg.AWS.Profile)
	assert.Equal(t, "", cfg.AWS.Endpoint)

	assert.Equal(t, "", cfg.UI.Language)
	assert.Equal(t, "default", cfg.UI.Theme)

	assert.Equal(t, 0, cfg.Performance.Concurrency)
	assert.Equal(t, int64(0), cfg.Performance.PartSize)
	assert.Equal(t, 30, cfg.Performance.Timeout)

	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
	assert.Equal(t, "", cfg.Logging.File)
}

// clearEnv isolates a test from ambient AWS and s3ry environment variables
// so Load results only reflect the fixture files.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AWS_REGION", "AWS_DEFAULT_REGION", "AWS_PROFILE",
		"AWS_ENDPOINT_URL", "S3RY_LANGUAGE", "S3RY_LOG_LEVEL",
	} {
		t.Setenv(key, "")
	}
}

func TestLoad_NoConfigFile(t *testing.T) {
	clearEnv(t)
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Chdir(tempDir)

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "default", cfg.UI.Theme)
}

func TestLoad_WithConfigFile(t *testing.T) {
	clearEnv(t)
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "s3ry.yml")

	configContent := `
aws:
  region: us-west-2
  profile: test-profile
  endpoint: http://localhost:4566
ui:
  language: en
  theme: dark
performance:
  concurrency: 8
  part_size: 10485760
  timeout: 60
logging:
  level: debug
  format: json
  file: /var/log/s3ry.log
`

	err := os.WriteFile(configPath, []byte(configContent), 0o600)
	assert.NoError(t, err)

	t.Setenv("HOME", tempDir)
	t.Chdir(tempDir)

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "us-west-2", cfg.AWS.Region)
	assert.Equal(t, "test-profile", cfg.AWS.Profile)
	assert.Equal(t, "http://localhost:4566", cfg.AWS.Endpoint)
	assert.Equal(t, "en", cfg.UI.Language)
	assert.Equal(t, "dark", cfg.UI.Theme)
	assert.Equal(t, 8, cfg.Performance.Concurrency)
	assert.Equal(t, int64(10485760), cfg.Performance.PartSize)
	assert.Equal(t, 60, cfg.Performance.Timeout)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "/var/log/s3ry.log", cfg.Logging.File)
}

func TestLoadFromFile_ReadsAWSRegion(t *testing.T) {
	clearEnv(t)

	configPath := filepath.Join(t.TempDir(), "config.yml")
	configContent := []byte("aws:\n  region: us-west-2\n")
	assert.NoError(t, os.WriteFile(configPath, configContent, 0o600))

	cfg, err := LoadFromFile(configPath)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "us-west-2", cfg.AWS.Region)
}

func TestLoadFromFile_MissingFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yml")

	_, err := LoadFromFile(configPath)

	assert.Error(t, err)
}

func TestLoadFromEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("AWS_REGION", "eu-west-1")
	t.Setenv("AWS_PROFILE", "env-profile")
	t.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	t.Setenv("S3RY_LANGUAGE", "en")
	t.Setenv("S3RY_LOG_LEVEL", "debug")

	cfg := Default()
	cfg.loadFromEnv()

	assert.Equal(t, "eu-west-1", cfg.AWS.Region)
	assert.Equal(t, "env-profile", cfg.AWS.Profile)
	assert.Equal(t, "http://localhost:4566", cfg.AWS.Endpoint)
	assert.Equal(t, "en", cfg.UI.Language)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestLoadFromEnv_AWSDefaultRegion(t *testing.T) {
	clearEnv(t)
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")

	cfg := Default()
	cfg.loadFromEnv()

	assert.Equal(t, "us-east-1", cfg.AWS.Region)
}

func TestLoadFromEnv_AWSRegionWinsOverDefault(t *testing.T) {
	clearEnv(t)
	t.Setenv("AWS_REGION", "eu-west-1")
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")

	cfg := Default()
	cfg.loadFromEnv()

	assert.Equal(t, "eu-west-1", cfg.AWS.Region)
}

func TestSave(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yml")

	cfg := Default()
	cfg.AWS.Region = "us-west-2"
	cfg.UI.Language = "en"

	err := cfg.Save(configPath)
	assert.NoError(t, err)

	assert.FileExists(t, configPath)

	data, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "region: us-west-2")
	assert.Contains(t, string(data), "language: en")
}

func TestSave_CreateDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "test-config-dir")
	configPath := filepath.Join(configDir, "config.yml")

	cfg := Default()
	err := cfg.Save(configPath)

	assert.NoError(t, err)
	assert.FileExists(t, configPath)
	assert.DirExists(t, configDir)
}

func TestSave_RoundTrip(t *testing.T) {
	clearEnv(t)
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "roundtrip.yml")

	cfg := Default()
	cfg.AWS.Region = "ap-southeast-2"
	cfg.Performance.Concurrency = 12
	cfg.Performance.PartSize = 16 * 1024 * 1024

	assert.NoError(t, cfg.Save(configPath))

	loaded, err := LoadFromFile(configPath)
	assert.NoError(t, err)
	assert.Equal(t, "ap-southeast-2", loaded.AWS.Region)
	assert.Equal(t, 12, loaded.Performance.Concurrency)
	assert.Equal(t, int64(16*1024*1024), loaded.Performance.PartSize)
}

func TestConfig_FieldAccess(t *testing.T) {
	cfg := Default()

	assert.NotNil(t, cfg.AWS)
	assert.NotNil(t, cfg.UI)
	assert.NotNil(t, cfg.Performance)
	assert.NotNil(t, cfg.Logging)

	cfg.AWS.Region = "test-region"
	cfg.AWS.Profile = "test-profile"
	cfg.AWS.Endpoint = "test-endpoint"

	assert.Equal(t, "test-region", cfg.AWS.Region)
	assert.Equal(t, "test-profile", cfg.AWS.Profile)
	assert.Equal(t, "test-endpoint", cfg.AWS.Endpoint)

	cfg.UI.Language = "test-lang"
	cfg.UI.Theme = "test-theme"

	assert.Equal(t, "test-lang", cfg.UI.Language)
	assert.Equal(t, "test-theme", cfg.UI.Theme)

	cfg.Performance.Concurrency = 10
	cfg.Performance.PartSize = 1000000
	cfg.Performance.Timeout = 60

	assert.Equal(t, 10, cfg.Performance.Concurrency)
	assert.Equal(t, int64(1000000), cfg.Performance.PartSize)
	assert.Equal(t, 60, cfg.Performance.Timeout)

	cfg.Logging.Level = "test-level"
	cfg.Logging.Format = "test-format"
	cfg.Logging.File = "test-file"

	assert.Equal(t, "test-level", cfg.Logging.Level)
	assert.Equal(t, "test-format", cfg.Logging.Format)
	assert.Equal(t, "test-file", cfg.Logging.File)
}

func TestLoad_InvalidYAML(t *testing.T) {
	clearEnv(t)
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "s3ry.yml")

	invalidYAML := `
aws:
  region: us-west-2
  profile: test-profile
invalid yaml content [
`

	err := os.WriteFile(configPath, []byte(invalidYAML), 0o600)
	assert.NoError(t, err)

	t.Setenv("HOME", tempDir)
	t.Chdir(tempDir)

	_, err = Load()
	assert.Error(t, err)
}

func TestConfig_EdgeCases(t *testing.T) {
	cfg := Default()

	cfg.Performance.Concurrency = 0
	cfg.Performance.PartSize = 0
	cfg.Performance.Timeout = 0

	assert.Equal(t, 0, cfg.Performance.Concurrency)
	assert.Equal(t, int64(0), cfg.Performance.PartSize)
	assert.Equal(t, 0, cfg.Performance.Timeout)
}

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

func BenchmarkNormalizeLanguage(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NormalizeLanguage("japanese")
		NormalizeLanguage("english")
		NormalizeLanguage("en")
	}
}
