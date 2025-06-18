package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/seike460/s3ry/internal/config"
)

func TestLogger_BasicLogging(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "DEBUG",
		LogFormat:   "text",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	logger.Info("Test message")
	logger.Debug("Debug message")
	logger.Error("Error message")

	output := buf.String()

	if !strings.Contains(output, "Test message") {
		t.Error("Info message not found in output")
	}
	if !strings.Contains(output, "Debug message") {
		t.Error("Debug message not found in output")
	}
	if !strings.Contains(output, "Error message") {
		t.Error("Error message not found in output")
	}
}

func TestLogger_JSONFormat(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "INFO",
		LogFormat:   "json",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	logger.Info("JSON test message")

	output := buf.String()

	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if logEntry.Message != "JSON test message" {
		t.Errorf("Expected message 'JSON test message', got '%s'", logEntry.Message)
	}
	if logEntry.Level != "INFO" {
		t.Errorf("Expected level 'INFO', got '%s'", logEntry.Level)
	}
}

func TestLogger_WithFields(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "INFO",
		LogFormat:   "json",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	logger.WithFields(map[string]interface{}{
		"user_id": "12345",
		"action":  "test",
	}).Info("Test with fields")

	output := buf.String()

	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if logEntry.Fields["user_id"] != "12345" {
		t.Error("user_id field not found or incorrect")
	}
	if logEntry.Fields["action"] != "test" {
		t.Error("action field not found or incorrect")
	}
}

func TestLogger_WithContext(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "INFO",
		LogFormat:   "json",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	ctx := context.WithValue(context.Background(), "request_id", "req-123")
	ctx = context.WithValue(ctx, "session_id", "sess-456")

	logger.WithContext(ctx).Info("Test with context")

	output := buf.String()

	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if logEntry.Fields["request_id"] != "req-123" {
		t.Error("request_id field not found or incorrect")
	}
	if logEntry.Fields["session_id"] != "sess-456" {
		t.Error("session_id field not found or incorrect")
	}
}

func TestLogger_LogOperation(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "INFO",
		LogFormat:   "text",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	err := logger.LogOperation("test_operation", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	if err != nil {
		t.Errorf("LogOperation returned error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Operation started") {
		t.Error("Operation started message not found")
	}
	if !strings.Contains(output, "Operation completed successfully") {
		t.Error("Operation completed message not found")
	}
}

func TestLogger_LogOperationWithError(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "INFO",
		LogFormat:   "text",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	testError := fmt.Errorf("test error")
	err := logger.LogOperation("test_operation", func() error {
		return testError
	})

	if err != testError {
		t.Errorf("Expected error %v, got %v", testError, err)
	}

	output := buf.String()

	if !strings.Contains(output, "Operation started") {
		t.Error("Operation started message not found")
	}
	if !strings.Contains(output, "Operation failed") {
		t.Error("Operation failed message not found")
	}
}

func TestLogger_LogLevels(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "WARN",
		LogFormat:   "text",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	logger.Debug("Debug message") // Should not appear
	logger.Info("Info message")   // Should not appear
	logger.Warn("Warn message")   // Should appear
	logger.Error("Error message") // Should appear

	output := buf.String()

	if strings.Contains(output, "Debug message") {
		t.Error("Debug message should not appear with WARN level")
	}
	if strings.Contains(output, "Info message") {
		t.Error("Info message should not appear with WARN level")
	}
	if !strings.Contains(output, "Warn message") {
		t.Error("Warn message should appear with WARN level")
	}
	if !strings.Contains(output, "Error message") {
		t.Error("Error message should appear with WARN level")
	}
}

func TestMetricsHook(t *testing.T) {
	hook := &MetricsHook{
		metrics: make(map[string]int64),
	}

	entry := &LogEntry{
		Level:     "INFO",
		Component: "test",
	}

	err := hook.Fire(entry)
	if err != nil {
		t.Errorf("MetricsHook.Fire returned error: %v", err)
	}

	metrics := hook.GetMetrics()
	if metrics["INFO"] != 1 {
		t.Errorf("Expected INFO count 1, got %d", metrics["INFO"])
	}
	if metrics["test_INFO"] != 1 {
		t.Errorf("Expected test_INFO count 1, got %d", metrics["test_INFO"])
	}
}

func TestLogger_SetComponent(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "INFO",
		LogFormat:   "json",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	componentLogger := logger.SetComponent("test_component")
	componentLogger.Info("Component test")

	output := buf.String()

	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if logEntry.Component != "test_component" {
		t.Errorf("Expected component 'test_component', got '%s'", logEntry.Component)
	}
}

func TestLogger_WithError(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "INFO",
		LogFormat:   "json",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	testError := fmt.Errorf("test error")
	logger.WithError(testError).Error("Error with context")

	output := buf.String()

	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if logEntry.Fields["error"] != "test error" {
		t.Error("Error field not found or incorrect")
	}
}

func TestLogger_WithDuration(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "INFO",
		LogFormat:   "json",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	duration := 100 * time.Millisecond
	logger.WithDuration(duration).Info("Operation completed")

	output := buf.String()

	var logEntry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if logEntry.Fields["duration"] != duration.String() {
		t.Error("Duration field not found or incorrect")
	}
}

func BenchmarkLogger_Info(b *testing.B) {
	cfg := &config.Config{
		LogLevel:    "INFO",
		LogFormat:   "text",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark test message %d", i)
	}
}

func BenchmarkLogger_InfoWithFields(b *testing.B) {
	cfg := &config.Config{
		LogLevel:    "INFO",
		LogFormat:   "json",
		Environment: "test",
		Version:     "1.0.0",
	}

	var buf bytes.Buffer
	logger := NewLogger(cfg)
	logger.outputs = []io.Writer{&buf}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.WithFields(map[string]interface{}{
			"iteration": i,
			"benchmark": true,
		}).Info("Benchmark test message")
	}
}
