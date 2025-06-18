package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/seike460/s3ry/internal/config"
)

// LogLevel represents the severity level of a log entry
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger represents an enhanced logger with structured logging capabilities
type Logger struct {
	mu      sync.RWMutex
	slogger *slog.Logger
	level   LogLevel
	format  string
	outputs []io.Writer
	fields  map[string]interface{}
	hooks   []Hook
	config  *config.Config
	metrics *LogMetrics
	buffer  *LogBuffer
	ctx     context.Context
	cancel  context.CancelFunc
}

// Hook represents a function that can be called on log events
type Hook func(entry *LogEntry)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Caller    *CallerInfo            `json:"caller,omitempty"`
	Error     error                  `json:"error,omitempty"`
	TraceID   string                 `json:"trace_id,omitempty"`
	SpanID    string                 `json:"span_id,omitempty"`
}

// CallerInfo represents information about the caller
type CallerInfo struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

// LogMetrics tracks logging statistics
type LogMetrics struct {
	mu          sync.RWMutex
	TotalLogs   int64            `json:"total_logs"`
	LevelCounts map[string]int64 `json:"level_counts"`
	ErrorRate   float64          `json:"error_rate"`
	LastError   time.Time        `json:"last_error"`
	StartTime   time.Time        `json:"start_time"`
}

// LogBuffer provides buffered logging for performance
type LogBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	size    int
	maxSize int
	ticker  *time.Ticker
	done    chan struct{}
}

// NewLogger creates a new enhanced logger instance
func NewLogger(cfg *config.Config) *Logger {
	ctx, cancel := context.WithCancel(context.Background())

	logger := &Logger{
		level:   parseLogLevel(cfg.Logging.Level),
		format:  cfg.Logging.Format,
		outputs: []io.Writer{os.Stdout},
		fields:  make(map[string]interface{}),
		hooks:   make([]Hook, 0),
		config:  cfg,
		metrics: &LogMetrics{
			LevelCounts: make(map[string]int64),
			StartTime:   time.Now(),
		},
		buffer: &LogBuffer{
			entries: make([]LogEntry, 0),
			maxSize: 1000,
			done:    make(chan struct{}),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Setup structured logger
	logger.setupSlogger()

	// Setup log file if specified
	if cfg.Logging.File != "" {
		if err := logger.addFileOutput(cfg.Logging.File); err != nil {
			fmt.Printf("Warning: Failed to setup log file %s: %v\n", cfg.Logging.File, err)
		}
	}

	// Start buffer flush routine
	logger.startBufferFlush()

	// Add default fields
	logger.fields["version"] = cfg.Version
	logger.fields["environment"] = cfg.Environment
	logger.fields["pid"] = os.Getpid()

	return logger
}

// setupSlogger configures the structured logger
func (l *Logger) setupSlogger() {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level:     slog.Level(l.level),
		AddSource: l.level == LevelDebug,
	}

	switch l.format {
	case "json":
		handler = slog.NewJSONHandler(l.outputs[0], opts)
	default:
		handler = slog.NewTextHandler(l.outputs[0], opts)
	}

	l.slogger = slog.New(handler)
}

// addFileOutput adds a file output to the logger
func (l *Logger) addFileOutput(filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.mu.Lock()
	l.outputs = append(l.outputs, file)
	l.mu.Unlock()

	return nil
}

// startBufferFlush starts the buffer flush routine
func (l *Logger) startBufferFlush() {
	l.buffer.ticker = time.NewTicker(5 * time.Second)

	go func() {
		for {
			select {
			case <-l.buffer.ticker.C:
				l.flushBuffer()
			case <-l.buffer.done:
				l.flushBuffer()
				return
			case <-l.ctx.Done():
				l.flushBuffer()
				return
			}
		}
	}()
}

// flushBuffer flushes the log buffer
func (l *Logger) flushBuffer() {
	l.buffer.mu.Lock()
	defer l.buffer.mu.Unlock()

	if len(l.buffer.entries) == 0 {
		return
	}

	// Process buffered entries
	for _, entry := range l.buffer.entries {
		l.writeEntry(&entry)
	}

	// Clear buffer
	l.buffer.entries = l.buffer.entries[:0]
}

// writeEntry writes a log entry to all outputs
func (l *Logger) writeEntry(entry *LogEntry) {
	// Update metrics
	l.updateMetrics(entry)

	// Execute hooks
	for _, hook := range l.hooks {
		hook(entry)
	}

	// Format and write to outputs
	var output string
	switch l.format {
	case "json":
		if data, err := json.Marshal(entry); err == nil {
			output = string(data) + "\n"
		}
	default:
		output = l.formatTextEntry(entry)
	}

	l.mu.RLock()
	for _, writer := range l.outputs {
		writer.Write([]byte(output))
	}
	l.mu.RUnlock()
}

// formatTextEntry formats a log entry as text
func (l *Logger) formatTextEntry(entry *LogEntry) string {
	var builder strings.Builder

	// Timestamp
	builder.WriteString(entry.Timestamp.Format("2006-01-02 15:04:05.000"))
	builder.WriteString(" ")

	// Level
	builder.WriteString(fmt.Sprintf("[%s]", entry.Level.String()))
	builder.WriteString(" ")

	// Caller info (if debug level)
	if entry.Caller != nil && l.level == LevelDebug {
		builder.WriteString(fmt.Sprintf("%s:%d ", filepath.Base(entry.Caller.File), entry.Caller.Line))
	}

	// Message
	builder.WriteString(entry.Message)

	// Fields
	if len(entry.Fields) > 0 {
		builder.WriteString(" ")
		for k, v := range entry.Fields {
			builder.WriteString(fmt.Sprintf("%s=%v ", k, v))
		}
	}

	// Error
	if entry.Error != nil {
		builder.WriteString(fmt.Sprintf(" error=%v", entry.Error))
	}

	builder.WriteString("\n")
	return builder.String()
}

// updateMetrics updates logging metrics
func (l *Logger) updateMetrics(entry *LogEntry) {
	l.metrics.mu.Lock()
	defer l.metrics.mu.Unlock()

	l.metrics.TotalLogs++
	l.metrics.LevelCounts[entry.Level.String()]++

	if entry.Level >= LevelError {
		l.metrics.LastError = entry.Timestamp
		errorCount := l.metrics.LevelCounts["ERROR"] + l.metrics.LevelCounts["FATAL"]
		l.metrics.ErrorRate = float64(errorCount) / float64(l.metrics.TotalLogs) * 100
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...interface{}) {
	if l.level > LevelDebug {
		return
	}
	l.log(LevelDebug, msg, nil, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...interface{}) {
	if l.level > LevelInfo {
		return
	}
	l.log(LevelInfo, msg, nil, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...interface{}) {
	if l.level > LevelWarn {
		return
	}
	l.log(LevelWarn, msg, nil, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, err error, fields ...interface{}) {
	l.log(LevelError, msg, err, fields...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, err error, fields ...interface{}) {
	l.log(LevelFatal, msg, err, fields...)
	l.Close()
	os.Exit(1)
}

// log is the core logging method
func (l *Logger) log(level LogLevel, msg string, err error, fields ...interface{}) {
	entry := &LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   msg,
		Fields:    l.parseFields(fields...),
		Error:     err,
	}

	// Add caller info for debug level
	if level == LevelDebug || level >= LevelError {
		entry.Caller = l.getCaller(3)
	}

	// Add trace information if available
	if traceID := l.getTraceID(); traceID != "" {
		entry.TraceID = traceID
	}

	// Buffer or write immediately based on level
	if level >= LevelError {
		l.writeEntry(entry)
	} else {
		l.bufferEntry(entry)
	}
}

// bufferEntry adds an entry to the buffer
func (l *Logger) bufferEntry(entry *LogEntry) {
	l.buffer.mu.Lock()
	defer l.buffer.mu.Unlock()

	l.buffer.entries = append(l.buffer.entries, *entry)

	// Flush if buffer is full
	if len(l.buffer.entries) >= l.buffer.maxSize {
		go l.flushBuffer()
	}
}

// parseFields parses variadic fields into a map
func (l *Logger) parseFields(fields ...interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy existing fields
	for k, v := range l.fields {
		result[k] = v
	}

	// Parse new fields
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			if key, ok := fields[i].(string); ok {
				result[key] = fields[i+1]
			}
		}
	}

	return result
}

// getCaller gets caller information
func (l *Logger) getCaller(skip int) *CallerInfo {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return nil
	}

	fn := runtime.FuncForPC(pc)
	var funcName string
	if fn != nil {
		funcName = fn.Name()
	}

	return &CallerInfo{
		File:     file,
		Line:     line,
		Function: funcName,
	}
}

// getTraceID gets trace ID from context (placeholder for tracing integration)
func (l *Logger) getTraceID() string {
	// This would integrate with tracing systems like OpenTelemetry
	return ""
}

// WithField adds a field to the logger context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := *l
	newLogger.fields = make(map[string]interface{})
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return &newLogger
}

// WithFields adds multiple fields to the logger context
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := *l
	newLogger.fields = make(map[string]interface{})
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return &newLogger
}

// WithError adds an error field to the logger context
func (l *Logger) WithError(err error) *Logger {
	return l.WithField("error", err.Error())
}

// AddHook adds a hook function
func (l *Logger) AddHook(hook Hook) {
	l.mu.Lock()
	l.hooks = append(l.hooks, hook)
	l.mu.Unlock()
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	l.level = level
	l.mu.Unlock()
	l.setupSlogger()
}

// GetMetrics returns current logging metrics
func (l *Logger) GetMetrics() *LogMetrics {
	l.metrics.mu.RLock()
	defer l.metrics.mu.RUnlock()

	// Create a copy to avoid race conditions
	metrics := &LogMetrics{
		TotalLogs:   l.metrics.TotalLogs,
		LevelCounts: make(map[string]int64),
		ErrorRate:   l.metrics.ErrorRate,
		LastError:   l.metrics.LastError,
		StartTime:   l.metrics.StartTime,
	}

	for k, v := range l.metrics.LevelCounts {
		metrics.LevelCounts[k] = v
	}

	return metrics
}

// Close closes the logger and flushes any remaining logs
func (l *Logger) Close() error {
	l.cancel()

	if l.buffer.ticker != nil {
		l.buffer.ticker.Stop()
	}

	close(l.buffer.done)
	l.flushBuffer()

	// Close file outputs
	l.mu.Lock()
	for _, output := range l.outputs[1:] { // Skip stdout
		if closer, ok := output.(io.Closer); ok {
			closer.Close()
		}
	}
	l.mu.Unlock()

	return nil
}

// parseLogLevel parses a string log level
func parseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// Global logger instance
var globalLogger *Logger
var globalLoggerOnce sync.Once

// InitGlobalLogger initializes the global logger
func InitGlobalLogger(cfg *config.Config) {
	globalLoggerOnce.Do(func() {
		globalLogger = NewLogger(cfg)
	})
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *Logger {
	if globalLogger == nil {
		// Fallback to default config if not initialized
		cfg := config.Default()
		InitGlobalLogger(cfg)
	}
	return globalLogger
}

// Convenience functions using global logger
func Debug(msg string, fields ...interface{}) {
	GetGlobalLogger().Debug(msg, fields...)
}

func Info(msg string, fields ...interface{}) {
	GetGlobalLogger().Info(msg, fields...)
}

func Warn(msg string, fields ...interface{}) {
	GetGlobalLogger().Warn(msg, fields...)
}

func Error(msg string, err error, fields ...interface{}) {
	GetGlobalLogger().Error(msg, err, fields...)
}

func Fatal(msg string, err error, fields ...interface{}) {
	GetGlobalLogger().Fatal(msg, err, fields...)
}
