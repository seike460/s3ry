package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// EnhancedConfig は拡張設定構造体
type EnhancedConfig struct {
	*Config // 既存の設定を埋め込み

	// ログ設定の拡張
	LogLevel  string `yaml:"log_level" json:"log_level"`   // "TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"
	LogFormat string `yaml:"log_format" json:"log_format"` // "text", "json"
	LogFile   string `yaml:"log_file" json:"log_file"`     // ログファイルパス

	// デバッグ設定
	DebugLevel string `yaml:"debug_level" json:"debug_level"` // "OFF", "BASIC", "VERBOSE", "TRACE"
	DebugFile  string `yaml:"debug_file" json:"debug_file"`   // デバッグファイルパス
	ProfileDir string `yaml:"profile_dir" json:"profile_dir"` // プロファイルディレクトリ

	// エラー追跡設定
	ErrorTrackingEndpoint string `yaml:"error_tracking_endpoint" json:"error_tracking_endpoint"`
	ErrorTrackingEnabled  bool   `yaml:"error_tracking_enabled" json:"error_tracking_enabled"`

	// 環境情報
	Environment string `yaml:"environment" json:"environment"` // "development", "staging", "production"
	Version     string `yaml:"version" json:"version"`         // アプリケーションバージョン

	// パフォーマンス監視
	MetricsEnabled     bool          `yaml:"metrics_enabled" json:"metrics_enabled"`
	MetricsInterval    time.Duration `yaml:"metrics_interval" json:"metrics_interval"`
	HealthCheckEnabled bool          `yaml:"health_check_enabled" json:"health_check_enabled"`

	// セキュリティ設定
	SecurityLevel     string `yaml:"security_level" json:"security_level"` // "low", "medium", "high"
	EncryptionEnabled bool   `yaml:"encryption_enabled" json:"encryption_enabled"`
	AuditLogEnabled   bool   `yaml:"audit_log_enabled" json:"audit_log_enabled"`

	// 通知設定
	NotificationChannels []NotificationChannel `yaml:"notification_channels" json:"notification_channels"`
	AlertThresholds      AlertThresholds       `yaml:"alert_thresholds" json:"alert_thresholds"`
}

// NotificationChannel は通知チャネル設定
type NotificationChannel struct {
	Type    string                 `yaml:"type" json:"type"` // "email", "slack", "webhook"
	Enabled bool                   `yaml:"enabled" json:"enabled"`
	Config  map[string]interface{} `yaml:"config" json:"config"`
}

// AlertThresholds はアラート閾値設定
type AlertThresholds struct {
	ErrorRate      float64       `yaml:"error_rate" json:"error_rate"`           // エラー率閾値 (%)
	ResponseTime   time.Duration `yaml:"response_time" json:"response_time"`     // レスポンス時間閾値
	MemoryUsage    uint64        `yaml:"memory_usage" json:"memory_usage"`       // メモリ使用量閾値 (bytes)
	DiskUsage      float64       `yaml:"disk_usage" json:"disk_usage"`           // ディスク使用率閾値 (%)
	NetworkLatency time.Duration `yaml:"network_latency" json:"network_latency"` // ネットワーク遅延閾値
}

// LoadEnhancedConfig は拡張設定を読み込み
func LoadEnhancedConfig(configPath string) (*EnhancedConfig, error) {
	// デフォルト設定を作成
	config := DefaultEnhancedConfig()

	// 設定ファイルが存在する場合は読み込み
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, err
			}

			if err := yaml.Unmarshal(data, config); err != nil {
				return nil, err
			}
		}
	}

	// 環境変数から設定を上書き
	config.loadFromEnvironment()

	return config, nil
}

// DefaultEnhancedConfig はデフォルト拡張設定を返す
func DefaultEnhancedConfig() *EnhancedConfig {
	baseConfig := Default()

	return &EnhancedConfig{
		Config:                baseConfig,
		LogLevel:              "INFO",
		LogFormat:             "text",
		LogFile:               "",
		DebugLevel:            "OFF",
		DebugFile:             "",
		ProfileDir:            "./profiles",
		ErrorTrackingEndpoint: "",
		ErrorTrackingEnabled:  false,
		Environment:           "development",
		Version:               "2.0.0",
		MetricsEnabled:        true,
		MetricsInterval:       30 * time.Second,
		HealthCheckEnabled:    true,
		SecurityLevel:         "medium",
		EncryptionEnabled:     false,
		AuditLogEnabled:       false,
		NotificationChannels:  []NotificationChannel{},
		AlertThresholds: AlertThresholds{
			ErrorRate:      5.0, // 5%
			ResponseTime:   5 * time.Second,
			MemoryUsage:    1024 * 1024 * 1024, // 1GB
			DiskUsage:      80.0,               // 80%
			NetworkLatency: 1 * time.Second,
		},
	}
}

// loadFromEnvironment は環境変数から設定を読み込み
func (c *EnhancedConfig) loadFromEnvironment() {
	// ログ設定
	if level := os.Getenv("S3RY_LOG_LEVEL"); level != "" {
		c.LogLevel = level
	}
	if format := os.Getenv("S3RY_LOG_FORMAT"); format != "" {
		c.LogFormat = format
	}
	if file := os.Getenv("S3RY_LOG_FILE"); file != "" {
		c.LogFile = file
	}

	// デバッグ設定
	if level := os.Getenv("S3RY_DEBUG_LEVEL"); level != "" {
		c.DebugLevel = level
	}
	if file := os.Getenv("S3RY_DEBUG_FILE"); file != "" {
		c.DebugFile = file
	}
	if dir := os.Getenv("S3RY_PROFILE_DIR"); dir != "" {
		c.ProfileDir = dir
	}

	// エラー追跡設定
	if endpoint := os.Getenv("S3RY_ERROR_TRACKING_ENDPOINT"); endpoint != "" {
		c.ErrorTrackingEndpoint = endpoint
	}
	if enabled := os.Getenv("S3RY_ERROR_TRACKING_ENABLED"); enabled != "" {
		c.ErrorTrackingEnabled = strings.ToLower(enabled) == "true"
	}

	// 環境情報
	if env := os.Getenv("S3RY_ENVIRONMENT"); env != "" {
		c.Environment = env
	}
	if version := os.Getenv("S3RY_VERSION"); version != "" {
		c.Version = version
	}

	// パフォーマンス監視
	if enabled := os.Getenv("S3RY_METRICS_ENABLED"); enabled != "" {
		c.MetricsEnabled = strings.ToLower(enabled) == "true"
	}
	if interval := os.Getenv("S3RY_METRICS_INTERVAL"); interval != "" {
		if duration, err := time.ParseDuration(interval); err == nil {
			c.MetricsInterval = duration
		}
	}
	if enabled := os.Getenv("S3RY_HEALTH_CHECK_ENABLED"); enabled != "" {
		c.HealthCheckEnabled = strings.ToLower(enabled) == "true"
	}

	// セキュリティ設定
	if level := os.Getenv("S3RY_SECURITY_LEVEL"); level != "" {
		c.SecurityLevel = level
	}
	if enabled := os.Getenv("S3RY_ENCRYPTION_ENABLED"); enabled != "" {
		c.EncryptionEnabled = strings.ToLower(enabled) == "true"
	}
	if enabled := os.Getenv("S3RY_AUDIT_LOG_ENABLED"); enabled != "" {
		c.AuditLogEnabled = strings.ToLower(enabled) == "true"
	}

	// アラート閾値
	if rate := os.Getenv("S3RY_ALERT_ERROR_RATE"); rate != "" {
		if value, err := strconv.ParseFloat(rate, 64); err == nil {
			c.AlertThresholds.ErrorRate = value
		}
	}
	if responseTime := os.Getenv("S3RY_ALERT_RESPONSE_TIME"); responseTime != "" {
		if duration, err := time.ParseDuration(responseTime); err == nil {
			c.AlertThresholds.ResponseTime = duration
		}
	}
	if memUsage := os.Getenv("S3RY_ALERT_MEMORY_USAGE"); memUsage != "" {
		if value, err := strconv.ParseUint(memUsage, 10, 64); err == nil {
			c.AlertThresholds.MemoryUsage = value
		}
	}
	if diskUsage := os.Getenv("S3RY_ALERT_DISK_USAGE"); diskUsage != "" {
		if value, err := strconv.ParseFloat(diskUsage, 64); err == nil {
			c.AlertThresholds.DiskUsage = value
		}
	}
	if netLatency := os.Getenv("S3RY_ALERT_NETWORK_LATENCY"); netLatency != "" {
		if duration, err := time.ParseDuration(netLatency); err == nil {
			c.AlertThresholds.NetworkLatency = duration
		}
	}
}

// Save は設定をファイルに保存
func (c *EnhancedConfig) Save(configPath string) error {
	// ディレクトリを作成
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// YAML形式で保存
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// Validate は設定の妥当性をチェック
func (c *EnhancedConfig) Validate() error {
	// ログレベルの検証
	validLogLevels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}
	if !contains(validLogLevels, c.LogLevel) {
		return fmt.Errorf("invalid log level: %s", c.LogLevel)
	}

	// ログフォーマットの検証
	validLogFormats := []string{"text", "json"}
	if !contains(validLogFormats, c.LogFormat) {
		return fmt.Errorf("invalid log format: %s", c.LogFormat)
	}

	// デバッグレベルの検証
	validDebugLevels := []string{"OFF", "BASIC", "VERBOSE", "TRACE"}
	if !contains(validDebugLevels, c.DebugLevel) {
		return fmt.Errorf("invalid debug level: %s", c.DebugLevel)
	}

	// 環境の検証
	validEnvironments := []string{"development", "staging", "production"}
	if !contains(validEnvironments, c.Environment) {
		return fmt.Errorf("invalid environment: %s", c.Environment)
	}

	// セキュリティレベルの検証
	validSecurityLevels := []string{"low", "medium", "high"}
	if !contains(validSecurityLevels, c.SecurityLevel) {
		return fmt.Errorf("invalid security level: %s", c.SecurityLevel)
	}

	// 閾値の検証
	if c.AlertThresholds.ErrorRate < 0 || c.AlertThresholds.ErrorRate > 100 {
		return fmt.Errorf("invalid error rate threshold: %f", c.AlertThresholds.ErrorRate)
	}

	if c.AlertThresholds.DiskUsage < 0 || c.AlertThresholds.DiskUsage > 100 {
		return fmt.Errorf("invalid disk usage threshold: %f", c.AlertThresholds.DiskUsage)
	}

	return nil
}

// IsProduction は本番環境かどうかを返す
func (c *EnhancedConfig) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment は開発環境かどうかを返す
func (c *EnhancedConfig) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsDebugEnabled はデバッグが有効かどうかを返す
func (c *EnhancedConfig) IsDebugEnabled() bool {
	return c.DebugLevel != "OFF"
}

// IsVerboseDebugEnabled は詳細デバッグが有効かどうかを返す
func (c *EnhancedConfig) IsVerboseDebugEnabled() bool {
	return c.DebugLevel == "VERBOSE" || c.DebugLevel == "TRACE"
}

// IsTraceEnabled はトレースが有効かどうかを返す
func (c *EnhancedConfig) IsTraceEnabled() bool {
	return c.DebugLevel == "TRACE"
}

// GetLogFilePath はログファイルパスを取得
func (c *EnhancedConfig) GetLogFilePath() string {
	if c.LogFile != "" {
		return c.LogFile
	}

	// デフォルトのログファイルパス
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".s3ry", "logs", "s3ry.log")
}

// GetDebugFilePath はデバッグファイルパスを取得
func (c *EnhancedConfig) GetDebugFilePath() string {
	if c.DebugFile != "" {
		return c.DebugFile
	}

	// デフォルトのデバッグファイルパス
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".s3ry", "debug", "debug.log")
}

// GetProfileDir はプロファイルディレクトリを取得
func (c *EnhancedConfig) GetProfileDir() string {
	if c.ProfileDir != "" {
		return c.ProfileDir
	}

	// デフォルトのプロファイルディレクトリ
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".s3ry", "profiles")
}

// contains はスライスに要素が含まれているかチェック
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetNotificationChannel は指定タイプの通知チャネルを取得
func (c *EnhancedConfig) GetNotificationChannel(channelType string) *NotificationChannel {
	for _, channel := range c.NotificationChannels {
		if channel.Type == channelType && channel.Enabled {
			return &channel
		}
	}
	return nil
}

// AddNotificationChannel は通知チャネルを追加
func (c *EnhancedConfig) AddNotificationChannel(channel NotificationChannel) {
	c.NotificationChannels = append(c.NotificationChannels, channel)
}

// EnableNotificationChannel は通知チャネルを有効化
func (c *EnhancedConfig) EnableNotificationChannel(channelType string) {
	for i := range c.NotificationChannels {
		if c.NotificationChannels[i].Type == channelType {
			c.NotificationChannels[i].Enabled = true
			break
		}
	}
}

// DisableNotificationChannel は通知チャネルを無効化
func (c *EnhancedConfig) DisableNotificationChannel(channelType string) {
	for i := range c.NotificationChannels {
		if c.NotificationChannels[i].Type == channelType {
			c.NotificationChannels[i].Enabled = false
			break
		}
	}
}
