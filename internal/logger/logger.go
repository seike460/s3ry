package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/seike460/s3ry/internal/config"
)

// LogLevel はログレベルを定義
type LogLevel int

const (
	TRACE LogLevel = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
)

// String はLogLevelの文字列表現を返す
func (l LogLevel) String() string {
	switch l {
	case TRACE:
		return "TRACE"
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogEntry はログエントリを表す
type LogEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       string                 `json:"level"`
	Message     string                 `json:"message"`
	Component   string                 `json:"component,omitempty"`
	Operation   string                 `json:"operation,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	File        string                 `json:"file,omitempty"`
	Line        int                    `json:"line,omitempty"`
	Function    string                 `json:"function,omitempty"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StackTrace  string                 `json:"stack_trace,omitempty"`
	Duration    *time.Duration         `json:"duration,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Environment string                 `json:"environment"`
	Version     string                 `json:"version"`
	PID         int                    `json:"pid"`
	Hostname    string                 `json:"hostname"`
}

// Logger は統合ログシステム
type Logger struct {
	mu            sync.RWMutex
	level         LogLevel
	format        string // "text" or "json"
	outputs       []io.Writer
	config        *config.Config
	component     string
	fields        map[string]interface{}
	hooks         []Hook
	buffer        []LogEntry
	bufferSize    int
	flushInterval time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	hostname      string
	pid           int
}

// Hook はログフック
type Hook interface {
	Fire(entry *LogEntry) error
	Levels() []LogLevel
}

// MetricsHook はメトリクス収集フック
type MetricsHook struct {
	mu      sync.RWMutex
	metrics map[string]int64
}

// ErrorTrackingHook はエラー追跡フック
type ErrorTrackingHook struct {
	tracker ErrorTracker
}

// ErrorTracker はエラー追跡インターフェース
type ErrorTracker interface {
	TrackError(operation, errorCode, errorMessage, errorType string, context map[string]interface{})
}

// NewLogger は新しいロガーを作成
func NewLogger(cfg *config.Config) *Logger {
	ctx, cancel := context.WithCancel(context.Background())

	hostname, _ := os.Hostname()

	logger := &Logger{
		level:         parseLogLevel(cfg.LogLevel),
		format:        cfg.LogFormat,
		outputs:       []io.Writer{os.Stdout},
		config:        cfg,
		fields:        make(map[string]interface{}),
		hooks:         make([]Hook, 0),
		buffer:        make([]LogEntry, 0, 1000),
		bufferSize:    1000,
		flushInterval: 5 * time.Second,
		ctx:           ctx,
		cancel:        cancel,
		hostname:      hostname,
		pid:           os.Getpid(),
	}

	// ログファイル出力を設定
	if cfg.LogFile != "" {
		if err := logger.addFileOutput(cfg.LogFile); err != nil {
			fmt.Printf("Failed to add file output: %v\n", err)
		}
	}

	// デフォルトフックを追加
	logger.AddHook(&MetricsHook{
		metrics: make(map[string]int64),
	})

	// バッファフラッシュワーカーを開始
	logger.wg.Add(1)
	go logger.flushWorker()

	return logger
}

// SetLevel はログレベルを設定
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetComponent はコンポーネント名を設定
func (l *Logger) SetComponent(component string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	newLogger := *l
	newLogger.component = component
	return &newLogger
}

// WithField はフィールドを追加
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newLogger := *l
	newLogger.fields = make(map[string]interface{})
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return &newLogger
}

// WithFields は複数のフィールドを追加
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

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

// WithError はエラーを追加
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

// WithDuration は実行時間を追加
func (l *Logger) WithDuration(duration time.Duration) *Logger {
	return l.WithField("duration", duration.String())
}

// WithContext はコンテキストから情報を抽出
func (l *Logger) WithContext(ctx context.Context) *Logger {
	logger := l

	if requestID := ctx.Value("request_id"); requestID != nil {
		logger = logger.WithField("request_id", requestID)
	}

	if sessionID := ctx.Value("session_id"); sessionID != nil {
		logger = logger.WithField("session_id", sessionID)
	}

	if userID := ctx.Value("user_id"); userID != nil {
		logger = logger.WithField("user_id", userID)
	}

	return logger
}

// Trace はTRACEレベルのログを出力
func (l *Logger) Trace(msg string, args ...interface{}) {
	l.log(TRACE, fmt.Sprintf(msg, args...))
}

// Debug はDEBUGレベルのログを出力
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(msg, args...))
}

// Info はINFOレベルのログを出力
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(INFO, fmt.Sprintf(msg, args...))
}

// Warn はWARNレベルのログを出力
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(WARN, fmt.Sprintf(msg, args...))
}

// Error はERRORレベルのログを出力
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(msg, args...))
}

// Fatal はFATALレベルのログを出力して終了
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(FATAL, fmt.Sprintf(msg, args...))
	l.Close()
	os.Exit(1)
}

// LogOperation は操作のログを記録
func (l *Logger) LogOperation(operation string, fn func() error) error {
	start := time.Now()
	logger := l.WithField("operation", operation)

	logger.Info("Operation started")

	err := fn()
	duration := time.Since(start)

	if err != nil {
		logger.WithError(err).WithDuration(duration).Error("Operation failed")
		return err
	}

	logger.WithDuration(duration).Info("Operation completed successfully")
	return nil
}

// LogS3Operation はS3操作のログを記録
func (l *Logger) LogS3Operation(operation, bucket, key string, fn func() error) error {
	return l.WithFields(map[string]interface{}{
		"s3_operation": operation,
		"bucket":       bucket,
		"key":          key,
	}).LogOperation(fmt.Sprintf("S3_%s", operation), fn)
}

// AddHook はフックを追加
func (l *Logger) AddHook(hook Hook) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = append(l.hooks, hook)
}

// AddOutput は出力先を追加
func (l *Logger) AddOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.outputs = append(l.outputs, output)
}

// Close はロガーを閉じる
func (l *Logger) Close() error {
	l.cancel()
	l.wg.Wait()
	return l.flushBuffer()
}

// log は実際のログ出力を行う
func (l *Logger) log(level LogLevel, message string) {
	l.mu.RLock()
	if level < l.level {
		l.mu.RUnlock()
		return
	}
	l.mu.RUnlock()

	// 呼び出し元の情報を取得
	pc, file, line, ok := runtime.Caller(2)
	var function string
	if ok {
		function = runtime.FuncForPC(pc).Name()
		file = filepath.Base(file)
	}

	entry := LogEntry{
		Timestamp:   time.Now(),
		Level:       level.String(),
		Message:     message,
		Component:   l.component,
		File:        file,
		Line:        line,
		Function:    function,
		Fields:      l.copyFields(),
		Environment: l.config.Environment,
		Version:     l.config.Version,
		PID:         l.pid,
		Hostname:    l.hostname,
	}

	// エラーレベルの場合はスタックトレースを追加
	if level >= ERROR {
		entry.StackTrace = l.captureStackTrace()
	}

	// フックを実行
	l.executeHooks(&entry, level)

	// バッファに追加
	l.mu.Lock()
	l.buffer = append(l.buffer, entry)
	if len(l.buffer) >= l.bufferSize {
		go l.flushBuffer()
	}
	l.mu.Unlock()

	// 即座に出力（FATAL、ERRORレベル）
	if level >= ERROR {
		l.writeEntry(&entry)
	}
}

// writeEntry はログエントリを出力
func (l *Logger) writeEntry(entry *LogEntry) {
	var output string

	if l.format == "json" {
		data, _ := json.Marshal(entry)
		output = string(data) + "\n"
	} else {
		output = l.formatTextEntry(entry)
	}

	l.mu.RLock()
	for _, writer := range l.outputs {
		writer.Write([]byte(output))
	}
	l.mu.RUnlock()
}

// formatTextEntry はテキスト形式でエントリをフォーマット
func (l *Logger) formatTextEntry(entry *LogEntry) string {
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05.000")

	var parts []string
	parts = append(parts, fmt.Sprintf("[%s]", timestamp))
	parts = append(parts, fmt.Sprintf("[%s]", entry.Level))

	if entry.Component != "" {
		parts = append(parts, fmt.Sprintf("[%s]", entry.Component))
	}

	if entry.File != "" && entry.Line > 0 {
		parts = append(parts, fmt.Sprintf("[%s:%d]", entry.File, entry.Line))
	}

	parts = append(parts, entry.Message)

	// フィールドを追加
	if len(entry.Fields) > 0 {
		var fields []string
		for k, v := range entry.Fields {
			fields = append(fields, fmt.Sprintf("%s=%v", k, v))
		}
		parts = append(parts, fmt.Sprintf("fields={%s}", strings.Join(fields, ", ")))
	}

	result := strings.Join(parts, " ")

	// エラーの場合はスタックトレースを追加
	if entry.StackTrace != "" {
		result += "\n" + entry.StackTrace
	}

	return result + "\n"
}

// executeHooks はフックを実行
func (l *Logger) executeHooks(entry *LogEntry, level LogLevel) {
	l.mu.RLock()
	hooks := make([]Hook, len(l.hooks))
	copy(hooks, l.hooks)
	l.mu.RUnlock()

	for _, hook := range hooks {
		for _, hookLevel := range hook.Levels() {
			if hookLevel == level {
				if err := hook.Fire(entry); err != nil {
					fmt.Printf("Hook execution failed: %v\n", err)
				}
				break
			}
		}
	}
}

// flushWorker はバッファを定期的にフラッシュ
func (l *Logger) flushWorker() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			l.flushBuffer()
		}
	}
}

// flushBuffer はバッファをフラッシュ
func (l *Logger) flushBuffer() error {
	l.mu.Lock()
	if len(l.buffer) == 0 {
		l.mu.Unlock()
		return nil
	}

	entries := make([]LogEntry, len(l.buffer))
	copy(entries, l.buffer)
	l.buffer = l.buffer[:0]
	l.mu.Unlock()

	for _, entry := range entries {
		l.writeEntry(&entry)
	}

	return nil
}

// addFileOutput はファイル出力を追加
func (l *Logger) addFileOutput(filename string) error {
	// ディレクトリを作成
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.AddOutput(file)
	return nil
}

// copyFields はフィールドをコピー
func (l *Logger) copyFields() map[string]interface{} {
	if len(l.fields) == 0 {
		return nil
	}

	fields := make(map[string]interface{})
	for k, v := range l.fields {
		fields[k] = v
	}
	return fields
}

// captureStackTrace はスタックトレースを取得
func (l *Logger) captureStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// parseLogLevel は文字列からLogLevelを解析
func parseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "TRACE":
		return TRACE
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}

// MetricsHook の実装
func (h *MetricsHook) Fire(entry *LogEntry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.metrics[entry.Level]++
	if entry.Component != "" {
		h.metrics[fmt.Sprintf("%s_%s", entry.Component, entry.Level)]++
	}

	return nil
}

func (h *MetricsHook) Levels() []LogLevel {
	return []LogLevel{TRACE, DEBUG, INFO, WARN, ERROR, FATAL}
}

func (h *MetricsHook) GetMetrics() map[string]int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	metrics := make(map[string]int64)
	for k, v := range h.metrics {
		metrics[k] = v
	}
	return metrics
}

// ErrorTrackingHook の実装
func (h *ErrorTrackingHook) Fire(entry *LogEntry) error {
	if entry.Level == "ERROR" || entry.Level == "FATAL" {
		context := make(map[string]interface{})
		if entry.Fields != nil {
			for k, v := range entry.Fields {
				context[k] = v
			}
		}

		context["file"] = entry.File
		context["line"] = entry.Line
		context["function"] = entry.Function
		context["hostname"] = entry.Hostname
		context["pid"] = entry.PID

		h.tracker.TrackError(
			entry.Operation,
			entry.Level,
			entry.Message,
			"application_error",
			context,
		)
	}

	return nil
}

func (h *ErrorTrackingHook) Levels() []LogLevel {
	return []LogLevel{ERROR, FATAL}
}

// グローバルロガー
var defaultLogger *Logger

// InitializeLogger はグローバルロガーを初期化
func InitializeLogger(cfg *config.Config) {
	defaultLogger = NewLogger(cfg)
}

// GetLogger はグローバルロガーを取得
func GetLogger() *Logger {
	if defaultLogger == nil {
		// デフォルト設定でロガーを作成
		cfg := &config.Config{
			LogLevel:    "INFO",
			LogFormat:   "text",
			Environment: "development",
			Version:     "unknown",
		}
		defaultLogger = NewLogger(cfg)
	}
	return defaultLogger
}

// パッケージレベルの便利関数
func Trace(msg string, args ...interface{}) {
	GetLogger().Trace(msg, args...)
}

func Debug(msg string, args ...interface{}) {
	GetLogger().Debug(msg, args...)
}

func Info(msg string, args ...interface{}) {
	GetLogger().Info(msg, args...)
}

func Warn(msg string, args ...interface{}) {
	GetLogger().Warn(msg, args...)
}

func Error(msg string, args ...interface{}) {
	GetLogger().Error(msg, args...)
}

func Fatal(msg string, args ...interface{}) {
	GetLogger().Fatal(msg, args...)
}

func WithField(key string, value interface{}) *Logger {
	return GetLogger().WithField(key, value)
}

func WithFields(fields map[string]interface{}) *Logger {
	return GetLogger().WithFields(fields)
}

func WithError(err error) *Logger {
	return GetLogger().WithError(err)
}

func WithContext(ctx context.Context) *Logger {
	return GetLogger().WithContext(ctx)
}
