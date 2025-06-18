package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Config represents the application configuration
type Config struct {
	AWS struct {
		Region   string `yaml:"region" json:"region"`
		Profile  string `yaml:"profile" json:"profile"`
		Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	} `yaml:"aws" json:"aws"`

	UI struct {
		Mode     string `yaml:"mode" json:"mode"`         // "legacy", "bubbles"
		Language string `yaml:"language" json:"language"` // "en", "ja"
		Theme    string `yaml:"theme" json:"theme"`       // "default", "dark", "light"
	} `yaml:"ui" json:"ui"`

	Performance struct {
		Workers                int `yaml:"workers" json:"workers"`
		ChunkSize              int `yaml:"chunk_size" json:"chunk_size"`
		Timeout                int `yaml:"timeout" json:"timeout"`
		MaxConcurrentDownloads int `yaml:"max_concurrent_downloads" json:"max_concurrent_downloads"`
		MaxConcurrentUploads   int `yaml:"max_concurrent_uploads" json:"max_concurrent_uploads"`
	} `yaml:"performance" json:"performance"`

	Logging struct {
		Level  string `yaml:"level" json:"level"`   // "debug", "info", "warn", "error"
		Format string `yaml:"format" json:"format"` // "text", "json"
		File   string `yaml:"file,omitempty" json:"file,omitempty"`
	} `yaml:"logging" json:"logging"`

	// Enhanced fields for compatibility
	LogLevel    string `yaml:"log_level" json:"log_level"`
	LogFormat   string `yaml:"log_format" json:"log_format"`
	LogFile     string `yaml:"log_file" json:"log_file"`
	DebugLevel  string `yaml:"debug_level" json:"debug_level"`
	DebugFile   string `yaml:"debug_file" json:"debug_file"`
	ProfileDir  string `yaml:"profile_dir" json:"profile_dir"`
	Environment string `yaml:"environment" json:"environment"`
	Version     string `yaml:"version" json:"version"`
}

// Default returns a configuration with sensible defaults
func Default() *Config {
	return &Config{
		AWS: struct {
			Region   string `yaml:"region" json:"region"`
			Profile  string `yaml:"profile" json:"profile"`
			Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
		}{
			Region:  "ap-northeast-1",
			Profile: "",
		},
		UI: struct {
			Mode     string `yaml:"mode" json:"mode"`
			Language string `yaml:"language" json:"language"`
			Theme    string `yaml:"theme" json:"theme"`
		}{
			Mode:     "bubbles", // Default to new UI
			Language: "ja",
			Theme:    "default",
		},
		Performance: struct {
			Workers                int `yaml:"workers" json:"workers"`
			ChunkSize              int `yaml:"chunk_size" json:"chunk_size"`
			Timeout                int `yaml:"timeout" json:"timeout"`
			MaxConcurrentDownloads int `yaml:"max_concurrent_downloads" json:"max_concurrent_downloads"`
			MaxConcurrentUploads   int `yaml:"max_concurrent_uploads" json:"max_concurrent_uploads"`
		}{
			Workers:                4,
			ChunkSize:              1024 * 1024 * 5, // 5MB
			Timeout:                30,              // 30 seconds
			MaxConcurrentDownloads: 10,
			MaxConcurrentUploads:   10,
		},
		Logging: struct {
			Level  string `yaml:"level" json:"level"`
			Format string `yaml:"format" json:"format"`
			File   string `yaml:"file,omitempty" json:"file,omitempty"`
		}{
			Level:  "info",
			Format: "text",
		},

		// Initialize enhanced fields
		LogLevel:    "INFO",
		LogFormat:   "text",
		LogFile:     "",
		DebugLevel:  "OFF",
		DebugFile:   "",
		ProfileDir:  "./profiles",
		Environment: "development",
		Version:     "2.0.0",
	}
}

// Load attempts to load configuration from various sources
func Load() (*Config, error) {
	cfg := Default()

	// Try to load from config file
	if err := cfg.loadFromFile(); err != nil {
		// If config file doesn't exist, that's fine - use defaults
		// Only return error for actual parsing issues
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	// Override with environment variables
	cfg.loadFromEnv()

	return cfg, nil
}

// loadFromFile loads configuration from YAML file
func (c *Config) loadFromFile() error {
	configPaths := []string{
		"s3ry.yml",
		"s3ry.yaml",
		".s3ry.yml",
		".s3ry.yaml",
	}

	// Also check user's home directory
	if home, err := os.UserHomeDir(); err == nil {
		configPaths = append(configPaths,
			filepath.Join(home, ".s3ry.yml"),
			filepath.Join(home, ".s3ry.yaml"),
			filepath.Join(home, ".config", "s3ry", "config.yml"),
			filepath.Join(home, ".config", "s3ry", "config.yaml"),
		)
	}

	for _, path := range configPaths {
		if data, err := os.ReadFile(path); err == nil {
			return yaml.Unmarshal(data, c)
		}
	}

	return os.ErrNotExist
}

// loadFromEnv loads configuration from environment variables
func (c *Config) loadFromEnv() {
	if region := os.Getenv("AWS_REGION"); region != "" {
		c.AWS.Region = region
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" && c.AWS.Region == "ap-northeast-1" {
		c.AWS.Region = region
	}
	if profile := os.Getenv("AWS_PROFILE"); profile != "" {
		c.AWS.Profile = profile
	}
	if endpoint := os.Getenv("AWS_ENDPOINT_URL"); endpoint != "" {
		c.AWS.Endpoint = endpoint
	}
	if mode := os.Getenv("S3RY_UI_MODE"); mode != "" {
		c.UI.Mode = mode
	}
	if lang := os.Getenv("S3RY_LANGUAGE"); lang != "" {
		c.UI.Language = lang
	}
	if level := os.Getenv("S3RY_LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}
}

// Save saves the current configuration to a file
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return os.WriteFile(path, data, 0644)
}

// IsNewUIEnabled returns true if the new Bubble Tea UI should be used
func (c *Config) IsNewUIEnabled() bool {
	return c.UI.Mode == "bubbles" || c.UI.Mode == "new"
}

// GetRegion returns the configured AWS region
func (c *Config) GetRegion() string {
	if c.AWS.Region == "" {
		return "ap-northeast-1" // fallback default
	}
	return c.AWS.Region
}

// GetLanguage returns the configured UI language
func (c *Config) GetLanguage() string {
	if c.UI.Language == "" {
		return "ja" // fallback default
	}
	return c.UI.Language
}

// SetLanguage sets the UI language and can be used to persist changes
func (c *Config) SetLanguage(lang string) {
	c.UI.Language = lang
}

// ValidateLanguage checks if the given language is supported
func (c *Config) ValidateLanguage(lang string) bool {
	supportedLangs := []string{"en", "ja", "english", "japanese"}
	for _, supported := range supportedLangs {
		if lang == supported {
			return true
		}
	}
	return false
}

// NormalizeLanguage converts language names to standard codes
func (c *Config) NormalizeLanguage(lang string) string {
	switch lang {
	case "japanese", "jp":
		return "ja"
	case "english":
		return "en"
	default:
		return lang
	}
}
