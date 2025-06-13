// Package plugins provides a plugin architecture for extending S3 operations
package plugins

import (
	"context"
	"io"

	"github.com/seike460/s3ry/pkg/types"
)

// S3Operation represents the type of S3 operation
type S3Operation string

const (
	OperationDownload S3Operation = "download"
	OperationUpload   S3Operation = "upload"
	OperationDelete   S3Operation = "delete"
	OperationList     S3Operation = "list"
	OperationCopy     S3Operation = "copy"
	OperationSync     S3Operation = "sync"
	OperationSelect   S3Operation = "select" // S3 Select (SQL on S3)
	OperationBatch    S3Operation = "batch"  // Batch operations
)

// PluginMetadata contains information about a plugin
type PluginMetadata struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	Website     string            `json:"website,omitempty"`
	License     string            `json:"license,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
}

// OperationContext provides context for plugin operations
type OperationContext struct {
	Context   context.Context
	Operation S3Operation
	Bucket    string
	Key       string
	Metadata  map[string]interface{}
	Progress  ProgressCallback
	Logger    Logger
}

// ProgressCallback is called to report operation progress
type ProgressCallback func(current, total int64, message string)

// Logger interface for plugin logging
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// S3Plugin is the main interface that all S3 plugins must implement
type S3Plugin interface {
	// Metadata returns plugin information
	Metadata() PluginMetadata

	// SupportedOperations returns the list of operations this plugin supports
	SupportedOperations() []S3Operation

	// Initialize is called when the plugin is loaded
	Initialize(config map[string]interface{}) error

	// Execute performs the plugin operation
	Execute(ctx OperationContext, args map[string]interface{}) (*OperationResult, error)

	// Cleanup is called when the plugin is unloaded
	Cleanup() error
}

// OperationResult contains the result of a plugin operation
type OperationResult struct {
	Success      bool                   `json:"success"`
	Message      string                 `json:"message,omitempty"`
	Data         interface{}            `json:"data,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	BytesTotal   int64                  `json:"bytes_total,omitempty"`
	BytesProcessed int64                `json:"bytes_processed,omitempty"`
	Duration     int64                  `json:"duration_ms,omitempty"`
	Error        string                 `json:"error,omitempty"`
}

// PreProcessor plugins can modify operations before they execute
type PreProcessor interface {
	S3Plugin
	PreProcess(ctx OperationContext, args map[string]interface{}) (map[string]interface{}, error)
}

// PostProcessor plugins can modify results after operations execute
type PostProcessor interface {
	S3Plugin
	PostProcess(ctx OperationContext, result *OperationResult) (*OperationResult, error)
}

// StreamProcessor plugins can process data streams during operations
type StreamProcessor interface {
	S3Plugin
	ProcessStream(ctx OperationContext, stream io.ReadWriter) (io.ReadWriter, error)
}

// BatchProcessor plugins can handle batch operations efficiently
type BatchProcessor interface {
	S3Plugin
	ProcessBatch(ctx OperationContext, items []types.BatchItem) (*OperationResult, error)
}

// SelectProcessor plugins can handle S3 Select operations (SQL on S3)
type SelectProcessor interface {
	S3Plugin
	ProcessSelect(ctx OperationContext, query string, format string) (*OperationResult, error)
}

// AdvancedS3Plugin provides access to more advanced S3 operations
type AdvancedS3Plugin interface {
	S3Plugin

	// GetCapabilities returns advanced capabilities this plugin provides
	GetCapabilities() []string

	// ExecuteAdvanced handles advanced operations with full control
	ExecuteAdvanced(ctx OperationContext, operation string, args map[string]interface{}) (*OperationResult, error)
}

// PluginPriority defines execution order for plugins
type PluginPriority int

const (
	PriorityLowest  PluginPriority = 0
	PriorityLow     PluginPriority = 25
	PriorityMedium  PluginPriority = 50
	PriorityHigh    PluginPriority = 75
	PriorityHighest PluginPriority = 100
)

// PrioritizedPlugin allows plugins to specify their execution priority
type PrioritizedPlugin interface {
	S3Plugin
	Priority() PluginPriority
}

// ConditionalPlugin allows plugins to determine if they should execute
type ConditionalPlugin interface {
	S3Plugin
	ShouldExecute(ctx OperationContext, args map[string]interface{}) bool
}