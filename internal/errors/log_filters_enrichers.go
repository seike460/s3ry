package errors

import (
	"runtime"
	"strings"
	"time"
)

// LevelFilter filters log entries based on minimum level
type LevelFilter struct {
	MinLevel LogLevel
}

// ShouldLog returns true if the entry should be logged
func (f *LevelFilter) ShouldLog(entry LogEntry) bool {
	return entry.Level >= f.MinLevel
}

// GetName returns the filter name
func (f *LevelFilter) GetName() string {
	return "level_filter"
}

// ComponentFilter filters log entries based on component
type ComponentFilter struct {
	AllowedComponents []string
	DeniedComponents  []string
}

// ShouldLog returns true if the entry should be logged
func (f *ComponentFilter) ShouldLog(entry LogEntry) bool {
	// Check denied components first
	for _, denied := range f.DeniedComponents {
		if entry.Component == denied {
			return false
		}
	}

	// If no allowed components specified, allow all (except denied)
	if len(f.AllowedComponents) == 0 {
		return true
	}

	// Check if component is in allowed list
	for _, allowed := range f.AllowedComponents {
		if entry.Component == allowed {
			return true
		}
	}

	return false
}

// GetName returns the filter name
func (f *ComponentFilter) GetName() string {
	return "component_filter"
}

// ErrorCodeFilter filters log entries based on error codes
type ErrorCodeFilter struct {
	AllowedCodes []string
	DeniedCodes  []string
}

// ShouldLog returns true if the entry should be logged
func (f *ErrorCodeFilter) ShouldLog(entry LogEntry) bool {
	if entry.ErrorCode == "" {
		return true // Allow non-error entries
	}

	// Check denied codes first
	for _, denied := range f.DeniedCodes {
		if entry.ErrorCode == denied {
			return false
		}
	}

	// If no allowed codes specified, allow all (except denied)
	if len(f.AllowedCodes) == 0 {
		return true
	}

	// Check if error code is in allowed list
	for _, allowed := range f.AllowedCodes {
		if entry.ErrorCode == allowed {
			return true
		}
	}

	return false
}

// GetName returns the filter name
func (f *ErrorCodeFilter) GetName() string {
	return "error_code_filter"
}

// RateLimitFilter filters log entries based on rate limiting
type RateLimitFilter struct {
	MaxEntriesPerSecond int
	lastLogTime         time.Time
	logCount            int
}

// ShouldLog returns true if the entry should be logged
func (f *RateLimitFilter) ShouldLog(entry LogEntry) bool {
	now := time.Now()

	// Reset counter if a second has passed
	if now.Sub(f.lastLogTime) >= time.Second {
		f.logCount = 0
		f.lastLogTime = now
	}

	// Check if we've exceeded the rate limit
	if f.logCount >= f.MaxEntriesPerSecond {
		return false
	}

	f.logCount++
	return true
}

// GetName returns the filter name
func (f *RateLimitFilter) GetName() string {
	return "rate_limit_filter"
}

// TimestampEnricher adds timestamp information
type TimestampEnricher struct{}

// Enrich enriches the log entry with timestamp information
func (e *TimestampEnricher) Enrich(entry *LogEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Add timestamp fields
	if entry.Fields == nil {
		entry.Fields = make(map[string]interface{})
	}
	entry.Fields["timestamp_unix"] = entry.Timestamp.Unix()
	entry.Fields["timestamp_nano"] = entry.Timestamp.UnixNano()

	return nil
}

// GetName returns the enricher name
func (e *TimestampEnricher) GetName() string {
	return "timestamp_enricher"
}

// ComponentEnricher enriches log entries with component information
type ComponentEnricher struct{}

// Enrich enriches the log entry with component information
func (e *ComponentEnricher) Enrich(entry *LogEntry) error {
	if entry.Component == "" {
		// Try to derive component from source
		if entry.Source.File != "" {
			parts := strings.Split(entry.Source.File, "/")
			if len(parts) > 0 {
				// Extract component from file path
				for i, part := range parts {
					if part == "internal" && i+1 < len(parts) {
						entry.Component = parts[i+1]
						break
					}
				}
			}
		}
	}

	// Add component fields
	if entry.Fields == nil {
		entry.Fields = make(map[string]interface{})
	}
	if entry.Component != "" {
		entry.Fields["component"] = entry.Component
	}

	return nil
}

// GetName returns the enricher name
func (e *ComponentEnricher) GetName() string {
	return "component_enricher"
}

// ErrorEnricher enriches log entries with error information
type ErrorEnricher struct{}

// Enrich enriches the log entry with error information
func (e *ErrorEnricher) Enrich(entry *LogEntry) error {
	if entry.Error != nil {
		if entry.Fields == nil {
			entry.Fields = make(map[string]interface{})
		}

		// Add error type
		entry.Fields["error_type"] = getErrorType(entry.Error)

		// Add S3ry-specific error information
		if s3ryErr, ok := entry.Error.(*S3ryError); ok {
			entry.Fields["s3ry_error"] = true
			entry.Fields["error_operation"] = s3ryErr.Operation
			entry.Fields["error_timestamp"] = s3ryErr.Timestamp

			// Add context fields
			for k, v := range s3ryErr.Context {
				entry.Fields["ctx_"+k] = v
			}

			// Add stack trace information
			if len(s3ryErr.Stack) > 0 {
				entry.Fields["stack_depth"] = len(s3ryErr.Stack)
				entry.Fields["stack_top_function"] = s3ryErr.Stack[0].Function
				entry.Fields["stack_top_file"] = s3ryErr.Stack[0].File
				entry.Fields["stack_top_line"] = s3ryErr.Stack[0].Line
			}
		} else {
			entry.Fields["s3ry_error"] = false
		}
	}

	return nil
}

// GetName returns the enricher name
func (e *ErrorEnricher) GetName() string {
	return "error_enricher"
}

// SourceEnricher enriches log entries with source code information
type SourceEnricher struct{}

// Enrich enriches the log entry with source information
func (e *SourceEnricher) Enrich(entry *LogEntry) error {
	if entry.Source.File == "unknown" {
		// Capture source information
		if pc, file, line, ok := runtime.Caller(4); ok { // Skip enricher call stack
			entry.Source.File = file
			entry.Source.Line = line

			if fn := runtime.FuncForPC(pc); fn != nil {
				entry.Source.Function = fn.Name()
			}
		}
	}

	// Add source fields
	if entry.Fields == nil {
		entry.Fields = make(map[string]interface{})
	}
	entry.Fields["source_file"] = entry.Source.File
	entry.Fields["source_line"] = entry.Source.Line
	entry.Fields["source_function"] = entry.Source.Function

	return nil
}

// GetName returns the enricher name
func (e *SourceEnricher) GetName() string {
	return "source_enricher"
}

// PerformanceEnricher enriches log entries with performance information
type PerformanceEnricher struct{}

// Enrich enriches the log entry with performance information
func (e *PerformanceEnricher) Enrich(entry *LogEntry) error {
	if entry.Fields == nil {
		entry.Fields = make(map[string]interface{})
	}

	// Add runtime information
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	entry.Fields["memory_alloc"] = m.Alloc
	entry.Fields["memory_total_alloc"] = m.TotalAlloc
	entry.Fields["memory_sys"] = m.Sys
	entry.Fields["gc_cycles"] = m.NumGC
	entry.Fields["goroutines"] = runtime.NumGoroutine()

	return nil
}

// GetName returns the enricher name
func (e *PerformanceEnricher) GetName() string {
	return "performance_enricher"
}

// ContextEnricher enriches log entries with context information
type ContextEnricher struct {
	DefaultFields map[string]interface{}
}

// Enrich enriches the log entry with context information
func (e *ContextEnricher) Enrich(entry *LogEntry) error {
	if entry.Fields == nil {
		entry.Fields = make(map[string]interface{})
	}

	// Add default fields
	for k, v := range e.DefaultFields {
		if _, exists := entry.Fields[k]; !exists {
			entry.Fields[k] = v
		}
	}

	return nil
}

// GetName returns the enricher name
func (e *ContextEnricher) GetName() string {
	return "context_enricher"
}

// Helper functions

func getErrorType(err error) string {
	if err == nil {
		return ""
	}

	// Get the type name of the error
	switch err.(type) {
	case *S3ryError:
		return "S3ryError"
	default:
		return "Error"
	}
}
