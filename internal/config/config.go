// Package config loads s3ry settings from YAML files, environment
// variables, and defaults, in that order of increasing precedence.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Config is the root of the s3ry configuration file.
type Config struct {
	AWS struct {
		// Region is passed to the AWS SDK; when empty the SDK's own
		// resolution chain (environment, shared config, defaults) applies.
		Region   string `yaml:"region" json:"region"`
		Profile  string `yaml:"profile" json:"profile"`
		Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	} `yaml:"aws" json:"aws"`

	UI struct {
		// Language is "en" or "ja"; empty follows the system locale.
		Language string `yaml:"language" json:"language"`
		Theme    string `yaml:"theme" json:"theme"`
	} `yaml:"ui" json:"ui"`

	Performance struct {
		// Concurrency is the number of parallel S3 workers for transfers,
		// prefix walks, and single-object deletes. Zero uses the default.
		Concurrency int `yaml:"concurrency" json:"concurrency"`
		// PartSize is the multipart chunk size in bytes. Zero uses the
		// default; values below 5 MiB are rejected.
		PartSize int64 `yaml:"part_size" json:"part_size"`
		// Timeout bounds each blocking S3 call from the TUI, in seconds.
		Timeout int `yaml:"timeout" json:"timeout"`
	} `yaml:"performance" json:"performance"`

	Logging struct {
		Level  string `yaml:"level" json:"level"`   // "debug", "info", "warn", "error"
		Format string `yaml:"format" json:"format"` // "text", "json"
		File   string `yaml:"file,omitempty" json:"file,omitempty"`
	} `yaml:"logging" json:"logging"`
}

// Default returns a configuration with sensible defaults.
func Default() *Config {
	cfg := &Config{}
	cfg.UI.Theme = "default"
	cfg.Performance.Timeout = 30
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "text"
	return cfg
}

// configPaths lists the files Load checks, relative paths first and then the
// home-directory locations.
func configPaths() []string {
	paths := []string{
		"s3ry.yml",
		"s3ry.yaml",
		".s3ry.yml",
		".s3ry.yaml",
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".s3ry.yml"),
			filepath.Join(home, ".s3ry.yaml"),
			filepath.Join(home, ".config", "s3ry", "config.yml"),
			filepath.Join(home, ".config", "s3ry", "config.yaml"),
		)
	}
	return paths
}

// Load reads the first existing file from configPaths and then applies
// environment variable overrides.
func Load() (*Config, error) {
	cfg := Default()
	for _, path := range configPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
		break
	}
	cfg.loadFromEnv()
	return cfg, nil
}

// LoadFromFile loads configuration from the specified YAML file and applies
// environment variable overrides in the same order as Load.
func LoadFromFile(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.loadFromEnv()
	return cfg, nil
}

// loadFromEnv applies the environment variables s3ry understands. The AWS
// SDK additionally honors its own variables (AWS_REGION, AWS_PROFILE,
// AWS_ENDPOINT_URL, ...) when the corresponding field is empty.
func (c *Config) loadFromEnv() {
	if region := os.Getenv("AWS_REGION"); region != "" {
		c.AWS.Region = region
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" && c.AWS.Region == "" {
		c.AWS.Region = region
	}
	if profile := os.Getenv("AWS_PROFILE"); profile != "" {
		c.AWS.Profile = profile
	}
	if endpoint := os.Getenv("AWS_ENDPOINT_URL"); endpoint != "" {
		c.AWS.Endpoint = endpoint
	}
	if lang := os.Getenv("S3RY_LANGUAGE"); lang != "" {
		c.UI.Language = lang
	}
	if level := os.Getenv("S3RY_LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}
}

// Save writes the current configuration to a YAML file.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}

	return os.WriteFile(path, data, 0o600)
}

// NormalizeLanguage converts language names to their standard code.
// Unknown values pass through unchanged so the i18n layer can decide.
func NormalizeLanguage(lang string) string {
	switch lang {
	case "japanese", "jp":
		return "ja"
	case "english":
		return "en"
	default:
		return lang
	}
}
