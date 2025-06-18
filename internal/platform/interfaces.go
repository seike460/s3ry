// Package platform provides cross-platform abstractions for S3ry
package platform

import (
	"context"
	"io"
	"time"
)

// PlatformAdapter abstracts platform-specific operations
type PlatformAdapter interface {
	// Platform identification
	GetPlatform() Platform
	GetVersion() string

	// UI and interaction
	CreateUI() (UIInterface, error)
	ShowNotification(title, message string) error

	// File system operations
	GetConfigPath() string
	GetCachePath() string
	GetLogPath() string

	// Performance monitoring
	GetSystemMetrics() *SystemMetrics
	OptimizeForPlatform(config *PlatformConfig) error
}

// UIInterface abstracts different UI implementations
type UIInterface interface {
	// Core UI operations
	Initialize(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// Event handling
	HandleEvent(event *UIEvent) error
	RegisterEventHandler(eventType EventType, handler EventHandler) error

	// Display operations
	Update(state *UIState) error
	Render() error

	// Performance optimization
	SetFrameRate(fps int) error
	GetMetrics() *UIMetrics
}

// Platform represents different execution platforms
type Platform int

const (
	PlatformCLI Platform = iota
	PlatformDesktop
	PlatformWeb
	PlatformTUI
	PlatformVSCode
)

func (p Platform) String() string {
	switch p {
	case PlatformCLI:
		return "cli"
	case PlatformDesktop:
		return "desktop"
	case PlatformWeb:
		return "web"
	case PlatformTUI:
		return "tui"
	case PlatformVSCode:
		return "vscode"
	default:
		return "unknown"
	}
}

// PlatformConfig contains platform-specific configuration
type PlatformConfig struct {
	Platform     Platform            `json:"platform"`
	WindowSize   *WindowSize         `json:"window_size,omitempty"`
	Theme        string              `json:"theme,omitempty"`
	Optimization *OptimizationConfig `json:"optimization,omitempty"`
	Features     []string            `json:"features,omitempty"`
}

// WindowSize represents window dimensions
type WindowSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// OptimizationConfig contains performance optimization settings
type OptimizationConfig struct {
	EnableGPUAcceleration bool `json:"enable_gpu_acceleration"`
	MaxFrameRate          int  `json:"max_frame_rate"`
	EnableCaching         bool `json:"enable_caching"`
	MemoryLimitMB         int  `json:"memory_limit_mb"`
	CPUThrottling         bool `json:"cpu_throttling"`
}

// SystemMetrics contains system performance metrics
type SystemMetrics struct {
	CPUUsage       float64       `json:"cpu_usage"`
	MemoryUsage    uint64        `json:"memory_usage"`
	MemoryTotal    uint64        `json:"memory_total"`
	DiskUsage      uint64        `json:"disk_usage"`
	NetworkLatency time.Duration `json:"network_latency"`
	GPUUsage       float64       `json:"gpu_usage,omitempty"`
	Timestamp      time.Time     `json:"timestamp"`
}

// UIEvent represents a UI event
type UIEvent struct {
	Type      EventType   `json:"type"`
	Source    string      `json:"source"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// EventType represents different event types
type EventType int

const (
	EventKeyPress EventType = iota
	EventMouseClick
	EventWindowResize
	EventFileSelect
	EventMenuAction
	EventCustom
)

// EventHandler handles UI events
type EventHandler func(event *UIEvent) error

// UIState represents the current UI state
type UIState struct {
	CurrentView  string                 `json:"current_view"`
	Data         interface{}            `json:"data,omitempty"`
	LoadingState bool                   `json:"loading_state"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// UIMetrics contains UI performance metrics
type UIMetrics struct {
	FrameRate      float64       `json:"frame_rate"`
	RenderTime     time.Duration `json:"render_time"`
	MemoryUsage    uint64        `json:"memory_usage"`
	EventQueueSize int           `json:"event_queue_size"`
	LastUpdateTime time.Time     `json:"last_update_time"`
}

// PerformanceProfiler provides cross-platform performance profiling
type PerformanceProfiler interface {
	// Profiling operations
	StartProfiling(profileType ProfileType) error
	StopProfiling(profileType ProfileType) (*ProfileResult, error)

	// Metrics collection
	CollectMetrics() (*PerformanceSnapshot, error)
	GetHistoricalData(duration time.Duration) ([]*PerformanceSnapshot, error)

	// Analysis and recommendations
	AnalyzePerformance() (*PerformanceAnalysis, error)
	GetOptimizationSuggestions() ([]OptimizationSuggestion, error)
}

// ProfileType represents different profiling types
type ProfileType int

const (
	ProfileCPU ProfileType = iota
	ProfileMemory
	ProfileNetwork
	ProfileDisk
	ProfileGPU
)

// ProfileResult contains profiling results
type ProfileResult struct {
	Type      ProfileType   `json:"type"`
	Duration  time.Duration `json:"duration"`
	Data      []byte        `json:"data"`
	Summary   string        `json:"summary"`
	Hotspots  []Hotspot     `json:"hotspots,omitempty"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
}

// Hotspot represents a performance hotspot
type Hotspot struct {
	Function    string        `json:"function"`
	File        string        `json:"file"`
	Line        int           `json:"line"`
	CPUTime     time.Duration `json:"cpu_time"`
	MemoryUsage uint64        `json:"memory_usage"`
	CallCount   int64         `json:"call_count"`
}

// PerformanceSnapshot represents a point-in-time performance measurement
type PerformanceSnapshot struct {
	Timestamp          time.Time           `json:"timestamp"`
	SystemMetrics      *SystemMetrics      `json:"system_metrics"`
	ApplicationMetrics *ApplicationMetrics `json:"application_metrics"`
}

// ApplicationMetrics contains application-specific metrics
type ApplicationMetrics struct {
	S3Operations   int64         `json:"s3_operations"`
	AverageLatency time.Duration `json:"average_latency"`
	ThroughputMBps float64       `json:"throughput_mbps"`
	ActiveWorkers  int           `json:"active_workers"`
	QueueLength    int           `json:"queue_length"`
	ErrorRate      float64       `json:"error_rate"`
}

// PerformanceAnalysis contains performance analysis results
type PerformanceAnalysis struct {
	OverallScore        float64                  `json:"overall_score"`
	BottleneckAreas     []BottleneckArea         `json:"bottleneck_areas"`
	PerformanceTrends   []PerformanceTrend       `json:"performance_trends"`
	ResourceUtilization *ResourceUtilization     `json:"resource_utilization"`
	Recommendations     []OptimizationSuggestion `json:"recommendations"`
}

// BottleneckArea represents a performance bottleneck
type BottleneckArea struct {
	Area        string  `json:"area"`
	Severity    float64 `json:"severity"`
	Impact      string  `json:"impact"`
	Description string  `json:"description"`
}

// PerformanceTrend represents a performance trend over time
type PerformanceTrend struct {
	Metric    string    `json:"metric"`
	Direction string    `json:"direction"` // "improving", "degrading", "stable"
	Magnitude float64   `json:"magnitude"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// ResourceUtilization contains resource utilization statistics
type ResourceUtilization struct {
	CPU     *ResourceStats `json:"cpu"`
	Memory  *ResourceStats `json:"memory"`
	Disk    *ResourceStats `json:"disk"`
	Network *ResourceStats `json:"network"`
}

// ResourceStats contains statistics for a specific resource
type ResourceStats struct {
	Average     float64 `json:"average"`
	Peak        float64 `json:"peak"`
	Minimum     float64 `json:"minimum"`
	Utilization float64 `json:"utilization"`
}

// OptimizationSuggestion represents a performance optimization suggestion
type OptimizationSuggestion struct {
	ID          string               `json:"id"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Category    OptimizationCategory `json:"category"`
	Priority    Priority             `json:"priority"`
	Impact      ImpactLevel          `json:"impact"`
	Effort      EffortLevel          `json:"effort"`
	Actions     []OptimizationAction `json:"actions"`
}

// OptimizationCategory represents different optimization categories
type OptimizationCategory int

const (
	CategoryCPU OptimizationCategory = iota
	CategoryMemory
	CategoryNetwork
	CategoryDisk
	CategoryUI
	CategoryAlgorithm
)

// Priority represents optimization priority levels
type Priority int

const (
	PriorityLow Priority = iota
	PriorityMedium
	PriorityHigh
	PriorityCritical
)

// ImpactLevel represents the expected impact of an optimization
type ImpactLevel int

const (
	ImpactLow ImpactLevel = iota
	ImpactMedium
	ImpactHigh
	ImpactTransformative
)

// EffortLevel represents the effort required for an optimization
type EffortLevel int

const (
	EffortLow EffortLevel = iota
	EffortMedium
	EffortHigh
	EffortExtensive
)

// OptimizationAction represents a specific action to take
type OptimizationAction struct {
	Type        ActionType             `json:"type"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	AutoApply   bool                   `json:"auto_apply"`
}

// ActionType represents different action types
type ActionType int

const (
	ActionConfigChange ActionType = iota
	ActionCodeChange
	ActionSystemTuning
	ActionResourceAllocation
	ActionFeatureToggle
)
