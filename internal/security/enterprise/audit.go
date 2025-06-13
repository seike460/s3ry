package enterprise

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditLevel represents the severity level of an audit event
type AuditLevel string

const (
	AuditLevelInfo     AuditLevel = "INFO"
	AuditLevelWarning  AuditLevel = "WARNING"
	AuditLevelError    AuditLevel = "ERROR"
	AuditLevelCritical AuditLevel = "CRITICAL"
)

// AuditEvent represents an audit log entry
type AuditEvent struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	Level      AuditLevel             `json:"level"`
	UserID     string                 `json:"user_id,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	Action     string                 `json:"action"`
	Resource   string                 `json:"resource,omitempty"`
	Result     string                 `json:"result"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Duration   time.Duration          `json:"duration,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// AuditLogger handles audit logging
type AuditLogger struct {
	config   *AuditConfig
	logFile  *os.File
	mutex    sync.Mutex
	buffer   []*AuditEvent
	stopCh   chan struct{}
	flushCh  chan struct{}
}

// AuditConfig holds audit logging configuration
type AuditConfig struct {
	Enabled         bool          `json:"enabled"`
	LogLevel        AuditLevel    `json:"log_level"`
	LogFile         string        `json:"log_file"`
	MaxFileSize     int64         `json:"max_file_size"`     // in bytes
	MaxFiles        int           `json:"max_files"`         // number of rotated files to keep
	BufferSize      int           `json:"buffer_size"`       // number of events to buffer
	FlushInterval   time.Duration `json:"flush_interval"`    // how often to flush buffer
	IncludeRequest  bool          `json:"include_request"`   // include request details
	IncludeResponse bool          `json:"include_response"`  // include response details
	RetentionDays   int           `json:"retention_days"`    // how long to keep logs
	Compression     bool          `json:"compression"`       // compress rotated logs
}

// DefaultAuditConfig returns default audit configuration
func DefaultAuditConfig() *AuditConfig {
	return &AuditConfig{
		Enabled:         true,
		LogLevel:        AuditLevelInfo,
		LogFile:         "audit.log",
		MaxFileSize:     100 * 1024 * 1024, // 100MB
		MaxFiles:        10,
		BufferSize:      1000,
		FlushInterval:   time.Second * 30,
		IncludeRequest:  true,
		IncludeResponse: false,
		RetentionDays:   90,
		Compression:     true,
	}
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(config *AuditConfig) (*AuditLogger, error) {
	if config == nil {
		config = DefaultAuditConfig()
	}

	if !config.Enabled {
		return &AuditLogger{config: config}, nil
	}

	// Create log directory if it doesn't exist
	logDir := filepath.Dir(config.LogFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	logFile, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	logger := &AuditLogger{
		config:  config,
		logFile: logFile,
		buffer:  make([]*AuditEvent, 0, config.BufferSize),
		stopCh:  make(chan struct{}),
		flushCh: make(chan struct{}, 1),
	}

	// Start background flusher
	go logger.flusher()

	return logger, nil
}

// Log records an audit event
func (a *AuditLogger) Log(event *AuditEvent) {
	if !a.config.Enabled {
		return
	}

	// Check log level
	if !a.shouldLog(event.Level) {
		return
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Generate ID if not set
	if event.ID == "" {
		event.ID = generateEventID()
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Add to buffer
	a.buffer = append(a.buffer, event)

	// Flush if buffer is full
	if len(a.buffer) >= a.config.BufferSize {
		select {
		case a.flushCh <- struct{}{}:
		default:
		}
	}
}

// LogAction is a convenience method for logging actions
func (a *AuditLogger) LogAction(userID, action, resource, result string, details map[string]interface{}) {
	event := &AuditEvent{
		Level:    AuditLevelInfo,
		UserID:   userID,
		Action:   action,
		Resource: resource,
		Result:   result,
		Details:  details,
	}
	a.Log(event)
}

// LogError logs an error event
func (a *AuditLogger) LogError(userID, action, resource, errorMsg string, details map[string]interface{}) {
	event := &AuditEvent{
		Level:    AuditLevelError,
		UserID:   userID,
		Action:   action,
		Resource: resource,
		Result:   "FAILED",
		Error:    errorMsg,
		Details:  details,
	}
	a.Log(event)
}

// LogSecurityEvent logs a security-related event
func (a *AuditLogger) LogSecurityEvent(level AuditLevel, userID, action, details string) {
	event := &AuditEvent{
		Level:  level,
		UserID: userID,
		Action: action,
		Result: "SECURITY_EVENT",
		Details: map[string]interface{}{
			"security_event": details,
		},
	}
	a.Log(event)
}

// LogAccess logs access attempts
func (a *AuditLogger) LogAccess(userID, sessionID, action, resource, result, ipAddress, userAgent string) {
	event := &AuditEvent{
		Level:     AuditLevelInfo,
		UserID:    userID,
		SessionID: sessionID,
		Action:    action,
		Resource:  resource,
		Result:    result,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}
	a.Log(event)
}

// LogWithDuration logs an event with execution duration
func (a *AuditLogger) LogWithDuration(userID, action, resource, result string, duration time.Duration, details map[string]interface{}) {
	event := &AuditEvent{
		Level:    AuditLevelInfo,
		UserID:   userID,
		Action:   action,
		Resource: resource,
		Result:   result,
		Duration: duration,
		Details:  details,
	}
	a.Log(event)
}

// Flush flushes the buffer to disk
func (a *AuditLogger) Flush() error {
	if !a.config.Enabled || a.logFile == nil {
		return nil
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.flushBuffer()
}

// Close closes the audit logger
func (a *AuditLogger) Close() error {
	if !a.config.Enabled {
		return nil
	}

	// Stop the flusher
	close(a.stopCh)

	// Flush remaining events
	if err := a.Flush(); err != nil {
		return err
	}

	// Close log file
	if a.logFile != nil {
		return a.logFile.Close()
	}

	return nil
}

// shouldLog checks if an event should be logged based on level
func (a *AuditLogger) shouldLog(level AuditLevel) bool {
	levelOrder := map[AuditLevel]int{
		AuditLevelInfo:     0,
		AuditLevelWarning:  1,
		AuditLevelError:    2,
		AuditLevelCritical: 3,
	}

	eventLevel, exists := levelOrder[level]
	if !exists {
		return true
	}

	configLevel, exists := levelOrder[a.config.LogLevel]
	if !exists {
		return true
	}

	return eventLevel >= configLevel
}

// flusher runs in background to periodically flush the buffer
func (a *AuditLogger) flusher() {
	ticker := time.NewTicker(a.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.Flush()
		case <-a.flushCh:
			a.Flush()
		case <-a.stopCh:
			return
		}
	}
}

// flushBuffer writes buffered events to disk
func (a *AuditLogger) flushBuffer() error {
	if len(a.buffer) == 0 {
		return nil
	}

	// Check if file rotation is needed
	if err := a.rotateIfNeeded(); err != nil {
		log.Printf("Failed to rotate audit log: %v", err)
	}

	// Write events
	for _, event := range a.buffer {
		jsonData, err := json.Marshal(event)
		if err != nil {
			log.Printf("Failed to marshal audit event: %v", err)
			continue
		}

		if _, err := a.logFile.Write(append(jsonData, '\n')); err != nil {
			log.Printf("Failed to write audit event: %v", err)
			continue
		}
	}

	// Clear buffer
	a.buffer = a.buffer[:0]

	// Sync to disk
	return a.logFile.Sync()
}

// rotateIfNeeded rotates the log file if it exceeds the maximum size
func (a *AuditLogger) rotateIfNeeded() error {
	if a.logFile == nil {
		return nil
	}

	// Check file size
	stat, err := a.logFile.Stat()
	if err != nil {
		return err
	}

	if stat.Size() < a.config.MaxFileSize {
		return nil
	}

	// Close current file
	if err := a.logFile.Close(); err != nil {
		return err
	}

	// Rotate files
	for i := a.config.MaxFiles - 1; i > 0; i-- {
		oldName := fmt.Sprintf("%s.%d", a.config.LogFile, i-1)
		newName := fmt.Sprintf("%s.%d", a.config.LogFile, i)

		if i == 1 {
			oldName = a.config.LogFile
		}

		if _, err := os.Stat(oldName); err == nil {
			if err := os.Rename(oldName, newName); err != nil {
				return err
			}
		}
	}

	// Open new file
	logFile, err := os.OpenFile(a.config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	a.logFile = logFile
	return nil
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Query represents an audit log query
type Query struct {
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	UserID     string            `json:"user_id,omitempty"`
	Action     string            `json:"action,omitempty"`
	Resource   string            `json:"resource,omitempty"`
	Level      AuditLevel        `json:"level,omitempty"`
	Result     string            `json:"result,omitempty"`
	IPAddress  string            `json:"ip_address,omitempty"`
	Limit      int               `json:"limit,omitempty"`
	Offset     int               `json:"offset,omitempty"`
	SortBy     string            `json:"sort_by,omitempty"`
	SortOrder  string            `json:"sort_order,omitempty"`
}

// AuditReader provides methods to read and query audit logs
type AuditReader struct {
	config *AuditConfig
}

// NewAuditReader creates a new audit reader
func NewAuditReader(config *AuditConfig) *AuditReader {
	if config == nil {
		config = DefaultAuditConfig()
	}
	return &AuditReader{config: config}
}

// QueryEvents queries audit events (basic file-based implementation)
func (r *AuditReader) QueryEvents(query *Query) ([]*AuditEvent, error) {
	// This is a basic implementation that reads from the current log file
	// A production implementation would use a proper database or search engine
	
	file, err := os.Open(r.config.LogFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}
	defer file.Close()

	var events []*AuditEvent
	decoder := json.NewDecoder(file)

	for decoder.More() {
		var event AuditEvent
		if err := decoder.Decode(&event); err != nil {
			continue // Skip malformed entries
		}

		// Apply filters
		if r.matchesQuery(&event, query) {
			events = append(events, &event)
		}

		// Apply limit
		if query.Limit > 0 && len(events) >= query.Limit {
			break
		}
	}

	return events, nil
}

// matchesQuery checks if an event matches the query criteria
func (r *AuditReader) matchesQuery(event *AuditEvent, query *Query) bool {
	if !query.StartTime.IsZero() && event.Timestamp.Before(query.StartTime) {
		return false
	}
	if !query.EndTime.IsZero() && event.Timestamp.After(query.EndTime) {
		return false
	}
	if query.UserID != "" && event.UserID != query.UserID {
		return false
	}
	if query.Action != "" && event.Action != query.Action {
		return false
	}
	if query.Resource != "" && event.Resource != query.Resource {
		return false
	}
	if query.Level != "" && event.Level != query.Level {
		return false
	}
	if query.Result != "" && event.Result != query.Result {
		return false
	}
	if query.IPAddress != "" && event.IPAddress != query.IPAddress {
		return false
	}

	return true
}

// GetEventsByUser returns events for a specific user
func (r *AuditReader) GetEventsByUser(userID string, limit int) ([]*AuditEvent, error) {
	query := &Query{
		UserID: userID,
		Limit:  limit,
	}
	return r.QueryEvents(query)
}

// GetSecurityEvents returns security-related events
func (r *AuditReader) GetSecurityEvents(startTime, endTime time.Time) ([]*AuditEvent, error) {
	query := &Query{
		StartTime: startTime,
		EndTime:   endTime,
		Action:    "SECURITY_EVENT",
	}
	return r.QueryEvents(query)
}

// GetFailedEvents returns failed events within a time range
func (r *AuditReader) GetFailedEvents(startTime, endTime time.Time) ([]*AuditEvent, error) {
	query := &Query{
		StartTime: startTime,
		EndTime:   endTime,
		Result:    "FAILED",
	}
	return r.QueryEvents(query)
}