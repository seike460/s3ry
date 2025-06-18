// Package telemetry provides opt-in usage analytics and error reporting for s3ry
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Event represents a telemetry event
type Event struct {
	EventID     string            `json:"event_id"`
	UserID      string            `json:"user_id"`
	SessionID   string            `json:"session_id"`
	Timestamp   time.Time         `json:"timestamp"`
	EventType   string            `json:"event_type"`
	Command     string            `json:"command,omitempty"`
	Duration    int64             `json:"duration_ms,omitempty"`
	Success     bool              `json:"success"`
	Error       string            `json:"error,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Performance *PerformanceData  `json:"performance,omitempty"`
	System      *SystemInfo       `json:"system"`
	Version     string            `json:"version"`
}

// PerformanceData contains performance metrics
type PerformanceData struct {
	ObjectsProcessed int64   `json:"objects_processed"`
	BytesTransferred int64   `json:"bytes_transferred"`
	Throughput       float64 `json:"throughput_mbps"`
	WorkerPoolSize   int     `json:"worker_pool_size"`
	MemoryUsage      int64   `json:"memory_usage_bytes"`
}

// SystemInfo contains system information
type SystemInfo struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	GoVersion     string `json:"go_version"`
	NumCPU        int    `json:"num_cpu"`
	IsContainer   bool   `json:"is_container"`
	CloudProvider string `json:"cloud_provider,omitempty"`
}

// Client handles telemetry collection and reporting
type Client struct {
	mu         sync.RWMutex
	enabled    bool
	userID     string
	sessionID  string
	endpoint   string
	httpClient *http.Client
	events     chan Event
	version    string
}

// Config contains telemetry configuration
type Config struct {
	Enabled  bool   `json:"enabled"`
	UserID   string `json:"user_id"`
	Endpoint string `json:"endpoint"`
	Debug    bool   `json:"debug"`
}

const (
	defaultEndpoint = "https://telemetry.s3ry.dev/events"
	configFile      = ".s3ry/telemetry.json"
	maxEventQueue   = 1000
	flushInterval   = 30 * time.Second
)

// NewClient creates a new telemetry client
func NewClient(version string) (*Client, error) {
	config, err := loadConfig()
	if err != nil {
		// If config doesn't exist or is invalid, create default disabled config
		config = &Config{
			Enabled:  false,
			UserID:   uuid.New().String(),
			Endpoint: defaultEndpoint,
			Debug:    false,
		}
		saveConfig(config) // Save default config
	}

	client := &Client{
		enabled:   config.Enabled,
		userID:    config.UserID,
		sessionID: uuid.New().String(),
		endpoint:  config.Endpoint,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		events:  make(chan Event, maxEventQueue),
		version: version,
	}

	if client.enabled {
		go client.worker()
	}

	return client, nil
}

// Enable enables telemetry collection with user consent
func (c *Client) Enable() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.enabled {
		return nil
	}

	c.enabled = true
	c.events = make(chan Event, maxEventQueue)
	go c.worker()

	config := &Config{
		Enabled:  true,
		UserID:   c.userID,
		Endpoint: c.endpoint,
		Debug:    false,
	}

	return saveConfig(config)
}

// Disable disables telemetry collection
func (c *Client) Disable() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.enabled = false
	if c.events != nil {
		close(c.events)
		c.events = nil
	}

	config := &Config{
		Enabled:  false,
		UserID:   c.userID,
		Endpoint: c.endpoint,
		Debug:    false,
	}

	return saveConfig(config)
}

// IsEnabled returns whether telemetry is enabled
func (c *Client) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

// Track records a telemetry event
func (c *Client) Track(eventType, command string, duration time.Duration, success bool, err error, metadata map[string]string) {
	if !c.IsEnabled() {
		return
	}

	event := Event{
		EventID:   uuid.New().String(),
		UserID:    c.userID,
		SessionID: c.sessionID,
		Timestamp: time.Now().UTC(),
		EventType: eventType,
		Command:   command,
		Duration:  duration.Milliseconds(),
		Success:   success,
		Metadata:  metadata,
		System:    getSystemInfo(),
		Version:   c.version,
	}

	if err != nil {
		event.Error = err.Error()
	}

	select {
	case c.events <- event:
	default:
		// Event queue is full, drop the event
	}
}

// TrackPerformance records a performance event with metrics
func (c *Client) TrackPerformance(command string, duration time.Duration, perf *PerformanceData, success bool, err error) {
	if !c.IsEnabled() {
		return
	}

	event := Event{
		EventID:     uuid.New().String(),
		UserID:      c.userID,
		SessionID:   c.sessionID,
		Timestamp:   time.Now().UTC(),
		EventType:   "performance",
		Command:     command,
		Duration:    duration.Milliseconds(),
		Success:     success,
		Performance: perf,
		System:      getSystemInfo(),
		Version:     c.version,
	}

	if err != nil {
		event.Error = err.Error()
	}

	select {
	case c.events <- event:
	default:
		// Event queue is full, drop the event
	}
}

// TrackError records an error event
func (c *Client) TrackError(command, errorType string, err error, metadata map[string]string) {
	if !c.IsEnabled() {
		return
	}

	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["error_type"] = errorType

	event := Event{
		EventID:   uuid.New().String(),
		UserID:    c.userID,
		SessionID: c.sessionID,
		Timestamp: time.Now().UTC(),
		EventType: "error",
		Command:   command,
		Success:   false,
		Error:     err.Error(),
		Metadata:  metadata,
		System:    getSystemInfo(),
		Version:   c.version,
	}

	select {
	case c.events <- event:
	default:
		// Event queue is full, drop the event
	}
}

// Flush sends all pending events
func (c *Client) Flush() {
	if !c.IsEnabled() {
		return
	}

	// Send a flush signal (empty event with special type)
	event := Event{
		EventType: "flush",
		Timestamp: time.Now().UTC(),
	}

	select {
	case c.events <- event:
	default:
	}
}

// Close shuts down the telemetry client
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.events != nil {
		close(c.events)
		c.events = nil
	}
}

// worker processes telemetry events
func (c *Client) worker() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	var batch []Event

	for {
		select {
		case event, ok := <-c.events:
			if !ok {
				// Channel closed, send final batch
				if len(batch) > 0 {
					c.sendBatch(batch)
				}
				return
			}

			if event.EventType == "flush" {
				// Flush signal received
				if len(batch) > 0 {
					c.sendBatch(batch)
					batch = nil
				}
				continue
			}

			batch = append(batch, event)

			// Send batch if it reaches a certain size
			if len(batch) >= 50 {
				c.sendBatch(batch)
				batch = nil
			}

		case <-ticker.C:
			// Periodic flush
			if len(batch) > 0 {
				c.sendBatch(batch)
				batch = nil
			}
		}
	}
}

// sendBatch sends a batch of events to the telemetry endpoint
func (c *Client) sendBatch(events []Event) {
	if len(events) == 0 {
		return
	}

	payload := map[string]interface{}{
		"events":      events,
		"client":      "s3ry",
		"sdk_version": "1.0.0",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(data))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("s3ry/%s", c.version))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// getSystemInfo returns system information
func getSystemInfo() *SystemInfo {
	return &SystemInfo{
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		GoVersion:     runtime.Version(),
		NumCPU:        runtime.NumCPU(),
		IsContainer:   isContainer(),
		CloudProvider: detectCloudProvider(),
	}
}

// isContainer detects if running in a container
func isContainer() bool {
	// Check for common container indicators
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check for Kubernetes
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	return false
}

// detectCloudProvider attempts to detect the cloud provider
func detectCloudProvider() string {
	// AWS
	if os.Getenv("AWS_REGION") != "" || os.Getenv("AWS_DEFAULT_REGION") != "" {
		return "aws"
	}

	// GCP
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" || os.Getenv("GCP_PROJECT") != "" {
		return "gcp"
	}

	// Azure
	if os.Getenv("AZURE_SUBSCRIPTION_ID") != "" {
		return "azure"
	}

	return ""
}

// loadConfig loads telemetry configuration
func loadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := fmt.Sprintf("%s/%s", homeDir, configFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// saveConfig saves telemetry configuration
func saveConfig(config *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := fmt.Sprintf("%s/.s3ry", homeDir)
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		return err
	}

	configPath := fmt.Sprintf("%s/%s", homeDir, configFile)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
