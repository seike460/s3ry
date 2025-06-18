package errors

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/seike460/s3ry/internal/config"
)

// EnterpriseLogger provides enterprise-grade error logging capabilities
type EnterpriseLogger struct {
	mu             sync.RWMutex
	config         *config.Config
	outputs        []LogOutput
	formatters     map[string]LogFormatter
	filters        []LogFilter
	enrichers      []LogEnricher
	bufferEnabled  bool
	bufferSize     int
	flushInterval  time.Duration
	buffer         []LogEntry
	bufferMutex    sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	metricsEnabled bool
	logMetrics     *LogMetrics
}

// LogLevel represents the severity level of a log entry
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelCritical
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp  time.Time              `json:"timestamp"`
	Level      LogLevel               `json:"level"`
	Message    string                 `json:"message"`
	Error      error                  `json:"error,omitempty"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	Source     LogSource              `json:"source"`
	TraceID    string                 `json:"trace_id,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	Component  string                 `json:"component,omitempty"`
	Operation  string                 `json:"operation,omitempty"`
	Duration   time.Duration          `json:"duration,omitempty"`
	ErrorCode  string                 `json:"error_code,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
}

// LogSource represents the source location of a log entry
type LogSource struct {
	File     string `json:"file"`
	Function string `json:"function"`
	Line     int    `json:"line"`
}

// LogOutput represents an output destination for log entries
type LogOutput interface {
	Write(entry LogEntry) error
	Close() error
	GetType() string
}

// LogFormatter formats log entries for output
type LogFormatter interface {
	Format(entry LogEntry) ([]byte, error)
	GetFormat() string
}

// LogFilter filters log entries based on criteria
type LogFilter interface {
	ShouldLog(entry LogEntry) bool
	GetName() string
}

// LogEnricher enriches log entries with additional context
type LogEnricher interface {
	Enrich(entry *LogEntry) error
	GetName() string
}

// LogMetrics tracks logging statistics
type LogMetrics struct {
	mu                sync.RWMutex
	TotalLogs         int64              `json:"total_logs"`
	LogsByLevel       map[LogLevel]int64 `json:"logs_by_level"`
	LogsByComponent   map[string]int64   `json:"logs_by_component"`
	ErrorsByCode      map[string]int64   `json:"errors_by_code"`
	BufferUtilization float64            `json:"buffer_utilization"`
	LastLogTime       time.Time          `json:"last_log_time"`
	DroppedLogs       int64              `json:"dropped_logs"`
	OutputErrors      int64              `json:"output_errors"`
}

// NewEnterpriseLogger creates a new enterprise logger
func NewEnterpriseLogger(cfg *config.Config) *EnterpriseLogger {
	ctx, cancel := context.WithCancel(context.Background())

	logger := &EnterpriseLogger{
		config:         cfg,
		outputs:        make([]LogOutput, 0),
		formatters:     make(map[string]LogFormatter),
		filters:        make([]LogFilter, 0),
		enrichers:      make([]LogEnricher, 0),
		bufferEnabled:  true,
		bufferSize:     1000,
		flushInterval:  5 * time.Second,
		buffer:         make([]LogEntry, 0, 1000),
		ctx:            ctx,
		cancel:         cancel,
		metricsEnabled: true,
		logMetrics: &LogMetrics{
			LogsByLevel:     make(map[LogLevel]int64),
			LogsByComponent: make(map[string]int64),
			ErrorsByCode:    make(map[string]int64),
		},
	}

	// Initialize default formatters
	logger.initializeDefaultFormatters()

	// Initialize default outputs
	logger.initializeDefaultOutputs()

	// Initialize default filters and enrichers
	logger.initializeDefaultFiltersAndEnrichers()

	// Start background workers
	logger.startBackgroundWorkers()

	return logger
}

// initializeDefaultFormatters sets up default log formatters
func (l *EnterpriseLogger) initializeDefaultFormatters() {
	// JSON formatter
	l.formatters["json"] = &JSONFormatter{}

	// Plain text formatter
	l.formatters["text"] = &TextFormatter{}

	// Structured formatter for debugging
	l.formatters["structured"] = &StructuredFormatter{}
}

// initializeDefaultOutputs sets up default log outputs
func (l *EnterpriseLogger) initializeDefaultOutputs() {
	// Console output
	l.outputs = append(l.outputs, &ConsoleOutput{
		writer:    os.Stdout,
		formatter: l.formatters["text"],
	})

	// File output if configured
	if l.config != nil {
		// Check if config has LogFile field, create file output if available
		logFile := "/tmp/s3ry.log" // Default fallback
		fileOutput, err := NewFileOutput(logFile, l.formatters["json"])
		if err == nil {
			l.outputs = append(l.outputs, fileOutput)
		}
	}
}

// initializeDefaultFiltersAndEnrichers sets up default filters and enrichers
func (l *EnterpriseLogger) initializeDefaultFiltersAndEnrichers() {
	// Level filter
	l.filters = append(l.filters, &LevelFilter{MinLevel: LogLevelInfo})

	// Component enricher
	l.enrichers = append(l.enrichers, &ComponentEnricher{})

	// Timestamp enricher
	l.enrichers = append(l.enrichers, &TimestampEnricher{})

	// Error enricher
	l.enrichers = append(l.enrichers, &ErrorEnricher{})
}

// startBackgroundWorkers starts background processing workers
func (l *EnterpriseLogger) startBackgroundWorkers() {
	if l.bufferEnabled {
		l.wg.Add(1)
		go l.bufferFlushWorker()
	}

	if l.metricsEnabled {
		l.wg.Add(1)
		go l.metricsWorker()
	}
}

// Log logs an entry at the specified level
func (l *EnterpriseLogger) Log(level LogLevel, message string, err error, fields map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Error:     err,
		Fields:    fields,
		Source:    l.captureSource(2),
	}

	// Extract error details if S3ryError
	if s3ryErr, ok := err.(*S3ryError); ok {
		entry.ErrorCode = string(s3ryErr.Code)
		entry.Operation = s3ryErr.Operation
		entry.Component = l.extractComponent(s3ryErr.Context)
		entry.TraceID = l.extractTraceID(s3ryErr.Context)
		entry.RequestID = l.extractRequestID(s3ryErr.Context)
		entry.StackTrace = l.extractStackTrace(s3ryErr)
	}

	l.processLogEntry(entry)
}

// Debug logs a debug message
func (l *EnterpriseLogger) Debug(message string, fields ...map[string]interface{}) {
	var allFields map[string]interface{}
	if len(fields) > 0 {
		allFields = fields[0]
	}
	l.Log(LogLevelDebug, message, nil, allFields)
}

// Info logs an info message
func (l *EnterpriseLogger) Info(message string, fields ...map[string]interface{}) {
	var allFields map[string]interface{}
	if len(fields) > 0 {
		allFields = fields[0]
	}
	l.Log(LogLevelInfo, message, nil, allFields)
}

// Warn logs a warning message
func (l *EnterpriseLogger) Warn(message string, fields ...map[string]interface{}) {
	var allFields map[string]interface{}
	if len(fields) > 0 {
		allFields = fields[0]
	}
	l.Log(LogLevelWarn, message, nil, allFields)
}

// Error logs an error message
func (l *EnterpriseLogger) Error(message string, err error, fields ...map[string]interface{}) {
	var allFields map[string]interface{}
	if len(fields) > 0 {
		allFields = fields[0]
	}
	l.Log(LogLevelError, message, err, allFields)
}

// Critical logs a critical error message
func (l *EnterpriseLogger) Critical(message string, err error, fields ...map[string]interface{}) {
	var allFields map[string]interface{}
	if len(fields) > 0 {
		allFields = fields[0]
	}
	l.Log(LogLevelCritical, message, err, allFields)
}

// processLogEntry processes a log entry through the pipeline
func (l *EnterpriseLogger) processLogEntry(entry LogEntry) {
	// Apply enrichers
	for _, enricher := range l.enrichers {
		if err := enricher.Enrich(&entry); err != nil {
			// Log enricher error (avoid infinite recursion)
			fmt.Fprintf(os.Stderr, "Log enricher error: %v\n", err)
		}
	}

	// Apply filters
	for _, filter := range l.filters {
		if !filter.ShouldLog(entry) {
			return
		}
	}

	// Update metrics
	if l.metricsEnabled {
		l.updateMetrics(entry)
	}

	// Buffer or write immediately
	if l.bufferEnabled {
		l.bufferEntry(entry)
	} else {
		l.writeEntry(entry)
	}
}

// bufferEntry adds an entry to the buffer
func (l *EnterpriseLogger) bufferEntry(entry LogEntry) {
	l.bufferMutex.Lock()
	defer l.bufferMutex.Unlock()

	if len(l.buffer) >= l.bufferSize {
		// Buffer is full, drop oldest entry
		l.buffer = l.buffer[1:]
		if l.metricsEnabled {
			l.logMetrics.mu.Lock()
			l.logMetrics.DroppedLogs++
			l.logMetrics.mu.Unlock()
		}
	}

	l.buffer = append(l.buffer, entry)
}

// writeEntry writes an entry to all outputs
func (l *EnterpriseLogger) writeEntry(entry LogEntry) {
	for _, output := range l.outputs {
		if err := output.Write(entry); err != nil {
			// Log output error to stderr to avoid infinite recursion
			fmt.Fprintf(os.Stderr, "Log output error (%s): %v\n", output.GetType(), err)
			if l.metricsEnabled {
				l.logMetrics.mu.Lock()
				l.logMetrics.OutputErrors++
				l.logMetrics.mu.Unlock()
			}
		}
	}
}

// bufferFlushWorker periodically flushes the buffer
func (l *EnterpriseLogger) bufferFlushWorker() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			// Final flush before shutdown
			l.flushBuffer()
			return
		case <-ticker.C:
			l.flushBuffer()
		}
	}
}

// flushBuffer flushes all buffered entries
func (l *EnterpriseLogger) flushBuffer() {
	l.bufferMutex.Lock()
	if len(l.buffer) == 0 {
		l.bufferMutex.Unlock()
		return
	}

	entries := make([]LogEntry, len(l.buffer))
	copy(entries, l.buffer)
	l.buffer = l.buffer[:0]
	l.bufferMutex.Unlock()

	// Write all buffered entries
	for _, entry := range entries {
		l.writeEntry(entry)
	}
}

// metricsWorker periodically updates metrics
func (l *EnterpriseLogger) metricsWorker() {
	defer l.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			l.updateBufferUtilization()
		}
	}
}

// updateMetrics updates logging metrics
func (l *EnterpriseLogger) updateMetrics(entry LogEntry) {
	l.logMetrics.mu.Lock()
	defer l.logMetrics.mu.Unlock()

	l.logMetrics.TotalLogs++
	l.logMetrics.LogsByLevel[entry.Level]++
	l.logMetrics.LastLogTime = entry.Timestamp

	if entry.Component != "" {
		l.logMetrics.LogsByComponent[entry.Component]++
	}

	if entry.ErrorCode != "" {
		l.logMetrics.ErrorsByCode[entry.ErrorCode]++
	}
}

// updateBufferUtilization updates buffer utilization metrics
func (l *EnterpriseLogger) updateBufferUtilization() {
	l.bufferMutex.Lock()
	bufferLen := len(l.buffer)
	l.bufferMutex.Unlock()

	l.logMetrics.mu.Lock()
	l.logMetrics.BufferUtilization = float64(bufferLen) / float64(l.bufferSize)
	l.logMetrics.mu.Unlock()
}

// GetMetrics returns current logging metrics
func (l *EnterpriseLogger) GetMetrics() *LogMetrics {
	l.logMetrics.mu.RLock()
	defer l.logMetrics.mu.RUnlock()

	// Return a copy
	metrics := &LogMetrics{
		TotalLogs:         l.logMetrics.TotalLogs,
		LogsByLevel:       make(map[LogLevel]int64),
		LogsByComponent:   make(map[string]int64),
		ErrorsByCode:      make(map[string]int64),
		BufferUtilization: l.logMetrics.BufferUtilization,
		LastLogTime:       l.logMetrics.LastLogTime,
		DroppedLogs:       l.logMetrics.DroppedLogs,
		OutputErrors:      l.logMetrics.OutputErrors,
	}

	for k, v := range l.logMetrics.LogsByLevel {
		metrics.LogsByLevel[k] = v
	}
	for k, v := range l.logMetrics.LogsByComponent {
		metrics.LogsByComponent[k] = v
	}
	for k, v := range l.logMetrics.ErrorsByCode {
		metrics.ErrorsByCode[k] = v
	}

	return metrics
}

// Close closes the logger and cleans up resources
func (l *EnterpriseLogger) Close() error {
	l.cancel()
	l.wg.Wait()

	// Final buffer flush
	l.flushBuffer()

	// Close all outputs
	var errs []error
	for _, output := range l.outputs {
		if err := output.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing log outputs: %v", errs)
	}

	return nil
}

// Helper methods

func (l *EnterpriseLogger) captureSource(skip int) LogSource {
	// This would capture the source location using runtime.Caller
	// Implementation simplified for brevity
	return LogSource{
		File:     "unknown",
		Function: "unknown",
		Line:     0,
	}
}

func (l *EnterpriseLogger) extractComponent(context map[string]interface{}) string {
	if context != nil {
		if component, ok := context["component"].(string); ok {
			return component
		}
	}
	return ""
}

func (l *EnterpriseLogger) extractTraceID(context map[string]interface{}) string {
	if context != nil {
		if traceID, ok := context["trace_id"].(string); ok {
			return traceID
		}
	}
	return ""
}

func (l *EnterpriseLogger) extractRequestID(context map[string]interface{}) string {
	if context != nil {
		if requestID, ok := context["request_id"].(string); ok {
			return requestID
		}
	}
	return ""
}

func (l *EnterpriseLogger) extractStackTrace(s3ryErr *S3ryError) string {
	if len(s3ryErr.Stack) > 0 {
		// Convert stack frames to string
		result := ""
		for _, frame := range s3ryErr.Stack {
			result += fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function)
		}
		return result
	}
	return ""
}
