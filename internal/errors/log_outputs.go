package errors

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// ConsoleOutput writes log entries to console
type ConsoleOutput struct {
	writer    io.Writer
	formatter LogFormatter
}

// NewConsoleOutput creates a new console output
func NewConsoleOutput(formatter LogFormatter) *ConsoleOutput {
	return &ConsoleOutput{
		writer:    os.Stdout,
		formatter: formatter,
	}
}

// Write writes a log entry to console
func (c *ConsoleOutput) Write(entry LogEntry) error {
	data, err := c.formatter.Format(entry)
	if err != nil {
		return fmt.Errorf("failed to format log entry: %w", err)
	}

	_, err = c.writer.Write(append(data, '\n'))
	return err
}

// Close closes the console output
func (c *ConsoleOutput) Close() error {
	// Console output doesn't need explicit closing
	return nil
}

// GetType returns the output type
func (c *ConsoleOutput) GetType() string {
	return "console"
}

// FileOutput writes log entries to a file
type FileOutput struct {
	file      *os.File
	formatter LogFormatter
	filePath  string
}

// NewFileOutput creates a new file output
func NewFileOutput(filePath string, formatter LogFormatter) (*FileOutput, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", filePath, err)
	}

	return &FileOutput{
		file:      file,
		formatter: formatter,
		filePath:  filePath,
	}, nil
}

// Write writes a log entry to file
func (f *FileOutput) Write(entry LogEntry) error {
	data, err := f.formatter.Format(entry)
	if err != nil {
		return fmt.Errorf("failed to format log entry: %w", err)
	}

	_, err = f.file.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	// Ensure data is written to disk
	return f.file.Sync()
}

// Close closes the file output
func (f *FileOutput) Close() error {
	if f.file != nil {
		return f.file.Close()
	}
	return nil
}

// GetType returns the output type
func (f *FileOutput) GetType() string {
	return "file"
}

// JSONFormatter formats log entries as JSON
type JSONFormatter struct{}

// Format formats a log entry as JSON
func (j *JSONFormatter) Format(entry LogEntry) ([]byte, error) {
	return json.Marshal(entry)
}

// GetFormat returns the formatter type
func (j *JSONFormatter) GetFormat() string {
	return "json"
}

// TextFormatter formats log entries as plain text
type TextFormatter struct{}

// Format formats a log entry as plain text
func (t *TextFormatter) Format(entry LogEntry) ([]byte, error) {
	timestamp := entry.Timestamp.Format(time.RFC3339)
	level := entry.Level.String()

	var text string
	if entry.Error != nil {
		text = fmt.Sprintf("[%s] %s: %s - Error: %v", timestamp, level, entry.Message, entry.Error)
	} else {
		text = fmt.Sprintf("[%s] %s: %s", timestamp, level, entry.Message)
	}

	// Add fields if present
	if len(entry.Fields) > 0 {
		fieldsStr := ""
		for k, v := range entry.Fields {
			fieldsStr += fmt.Sprintf(" %s=%v", k, v)
		}
		text += " -" + fieldsStr
	}

	// Add component and operation if present
	if entry.Component != "" {
		text += fmt.Sprintf(" [%s]", entry.Component)
	}
	if entry.Operation != "" {
		text += fmt.Sprintf(" (%s)", entry.Operation)
	}

	return []byte(text), nil
}

// GetFormat returns the formatter type
func (t *TextFormatter) GetFormat() string {
	return "text"
}

// StructuredFormatter formats log entries with structure for debugging
type StructuredFormatter struct{}

// Format formats a log entry with structure
func (s *StructuredFormatter) Format(entry LogEntry) ([]byte, error) {
	timestamp := entry.Timestamp.Format(time.RFC3339)
	level := entry.Level.String()

	text := fmt.Sprintf("=== %s [%s] ===\n", timestamp, level)
	text += fmt.Sprintf("Message: %s\n", entry.Message)

	if entry.Error != nil {
		text += fmt.Sprintf("Error: %v\n", entry.Error)
	}

	if entry.Component != "" {
		text += fmt.Sprintf("Component: %s\n", entry.Component)
	}

	if entry.Operation != "" {
		text += fmt.Sprintf("Operation: %s\n", entry.Operation)
	}

	if entry.ErrorCode != "" {
		text += fmt.Sprintf("Error Code: %s\n", entry.ErrorCode)
	}

	if entry.TraceID != "" {
		text += fmt.Sprintf("Trace ID: %s\n", entry.TraceID)
	}

	if entry.RequestID != "" {
		text += fmt.Sprintf("Request ID: %s\n", entry.RequestID)
	}

	if entry.Duration > 0 {
		text += fmt.Sprintf("Duration: %v\n", entry.Duration)
	}

	if len(entry.Fields) > 0 {
		text += "Fields:\n"
		for k, v := range entry.Fields {
			text += fmt.Sprintf("  %s: %v\n", k, v)
		}
	}

	if entry.Source.File != "unknown" {
		text += fmt.Sprintf("Source: %s:%d (%s)\n", entry.Source.File, entry.Source.Line, entry.Source.Function)
	}

	if entry.StackTrace != "" {
		text += fmt.Sprintf("Stack Trace:\n%s\n", entry.StackTrace)
	}

	text += "========================\n"

	return []byte(text), nil
}

// GetFormat returns the formatter type
func (s *StructuredFormatter) GetFormat() string {
	return "structured"
}
