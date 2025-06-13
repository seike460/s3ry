package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/seike460/s3ry/internal/config"
)

// AdvancedTelemetryCollector は高度なテレメトリ収集システム
type AdvancedTelemetryCollector struct {
	mu               sync.RWMutex
	config          *config.Config
	enabledFeatures map[string]bool
	metrics         *PerformanceMetrics
	usageStats      *UsageStatistics
	errorTracking   *ErrorTracker
	buffer          []TelemetryEvent
	flushInterval   time.Duration
	maxBufferSize   int
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// TelemetryEvent はテレメトリイベント
type TelemetryEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   string                 `json:"event_type"`
	UserID      string                 `json:"user_id,omitempty"`
	SessionID   string                 `json:"session_id"`
	Operation   string                 `json:"operation"`
	Duration    time.Duration          `json:"duration"`
	Success     bool                   `json:"success"`
	ErrorCode   string                 `json:"error_code,omitempty"`
	FileSize    int64                  `json:"file_size,omitempty"`
	Throughput  float64                `json:"throughput,omitempty"`
	WorkerCount int                    `json:"worker_count,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PerformanceMetrics はパフォーマンス指標
type PerformanceMetrics struct {
	mu                   sync.RWMutex
	OperationsCount      int64   `json:"operations_count"`
	TotalDuration        int64   `json:"total_duration_ms"`
	AverageThroughput    float64 `json:"average_throughput_mbps"`
	PeakThroughput       float64 `json:"peak_throughput_mbps"`
	TotalBytesTransfer   int64   `json:"total_bytes_transferred"`
	SuccessRate          float64 `json:"success_rate"`
	AverageWorkerCount   float64 `json:"average_worker_count"`
	MemoryUsagePeak      int64   `json:"memory_usage_peak_mb"`
	CPUUtilizationPeak   float64 `json:"cpu_utilization_peak"`
	PerformanceImprove   float64 `json:"performance_improvement_factor"`
}

// UsageStatistics は使用統計
type UsageStatistics struct {
	mu                sync.RWMutex
	ActiveUsers       int64            `json:"active_users"`
	TotalSessions     int64            `json:"total_sessions"`
	OperationCounts   map[string]int64 `json:"operation_counts"`
	RegionUsage       map[string]int64 `json:"region_usage"`
	StorageClassUsage map[string]int64 `json:"storage_class_usage"`
	PlatformUsage     map[string]int64 `json:"platform_usage"`
	VersionUsage      map[string]int64 `json:"version_usage"`
	FeatureUsage      map[string]int64 `json:"feature_usage"`
}

// ErrorTracker はエラー追跡
type ErrorTracker struct {
	mu            sync.RWMutex
	ErrorCounts   map[string]int64      `json:"error_counts"`
	ErrorPatterns map[string]*ErrorStat `json:"error_patterns"`
	LastErrors    []ErrorInfo           `json:"last_errors"`
}

// ErrorStat はエラー統計
type ErrorStat struct {
	Count       int64     `json:"count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Frequency   float64   `json:"frequency"`
	Severity    string    `json:"severity"`
	Resolution  string    `json:"resolution,omitempty"`
}

// ErrorInfo はエラー情報
type ErrorInfo struct {
	Timestamp   time.Time `json:"timestamp"`
	Operation   string    `json:"operation"`
	ErrorCode   string    `json:"error_code"`
	ErrorMsg    string    `json:"error_message"`
	StackTrace  string    `json:"stack_trace,omitempty"`
	Context     string    `json:"context"`
	Severity    string    `json:"severity"`
}

// NewAdvancedTelemetryCollector は高度テレメトリコレクターを作成
func NewAdvancedTelemetryCollector(cfg *config.Config) *AdvancedTelemetryCollector {
	ctx, cancel := context.WithCancel(context.Background())
	
	collector := &AdvancedTelemetryCollector{
		config:          cfg,
		enabledFeatures: make(map[string]bool),
		metrics:         &PerformanceMetrics{},
		usageStats: &UsageStatistics{
			OperationCounts:   make(map[string]int64),
			RegionUsage:       make(map[string]int64),
			StorageClassUsage: make(map[string]int64),
			PlatformUsage:     make(map[string]int64),
			VersionUsage:      make(map[string]int64),
			FeatureUsage:      make(map[string]int64),
		},
		errorTracking: &ErrorTracker{
			ErrorCounts:   make(map[string]int64),
			ErrorPatterns: make(map[string]*ErrorStat),
			LastErrors:    make([]ErrorInfo, 0, 100),
		},
		buffer:        make([]TelemetryEvent, 0, 1000),
		flushInterval: 5 * time.Minute,
		maxBufferSize: 1000,
		ctx:           ctx,
		cancel:        cancel,
	}

	// デフォルト機能を有効化
	collector.enabledFeatures["performance_metrics"] = true
	collector.enabledFeatures["error_tracking"] = true
	collector.enabledFeatures["usage_statistics"] = cfg.TelemetryEnabled
	collector.enabledFeatures["crash_reporting"] = false // オプトイン必須

	return collector
}

// Start はテレメトリ収集を開始
func (a *AdvancedTelemetryCollector) Start() error {
	if !a.config.TelemetryEnabled {
		return fmt.Errorf("telemetry is disabled")
	}

	a.wg.Add(1)
	go a.flushWorker()

	a.wg.Add(1)
	go a.metricsCollector()

	return nil
}

// Stop はテレメトリ収集を停止
func (a *AdvancedTelemetryCollector) Stop() error {
	a.cancel()
	a.wg.Wait()
	
	// 最終フラッシュ
	return a.flush()
}

// RecordEvent はイベントを記録
func (a *AdvancedTelemetryCollector) RecordEvent(event TelemetryEvent) {
	if !a.config.TelemetryEnabled {
		return
	}

	event.Timestamp = time.Now()
	event.SessionID = a.getSessionID()

	a.mu.Lock()
	a.buffer = append(a.buffer, event)
	if len(a.buffer) >= a.maxBufferSize {
		go a.flush()
	}
	a.mu.Unlock()

	// リアルタイム統計更新
	a.updateMetrics(event)
	a.updateUsageStats(event)
	if !event.Success && event.ErrorCode != "" {
		a.trackError(event)
	}
}

// RecordOperation は操作を記録
func (a *AdvancedTelemetryCollector) RecordOperation(operation string, duration time.Duration, success bool, metadata map[string]interface{}) {
	event := TelemetryEvent{
		EventType: "operation",
		Operation: operation,
		Duration:  duration,
		Success:   success,
		Metadata:  metadata,
	}

	if fileSize, ok := metadata["file_size"].(int64); ok {
		event.FileSize = fileSize
		if duration > 0 {
			// スループット計算 (MB/s)
			throughputMBps := float64(fileSize) / (1024 * 1024) / duration.Seconds()
			event.Throughput = throughputMBps
		}
	}

	if workers, ok := metadata["worker_count"].(int); ok {
		event.WorkerCount = workers
	}

	a.RecordEvent(event)
}

// RecordError はエラーを記録
func (a *AdvancedTelemetryCollector) RecordError(operation, errorCode, errorMsg, context string) {
	event := TelemetryEvent{
		EventType: "error",
		Operation: operation,
		Success:   false,
		ErrorCode: errorCode,
		Metadata: map[string]interface{}{
			"error_message": errorMsg,
			"context":       context,
		},
	}

	a.RecordEvent(event)
}

// RecordPerformanceImprovement はパフォーマンス改善を記録
func (a *AdvancedTelemetryCollector) RecordPerformanceImprovement(operation string, improvementFactor float64, baselineDuration, optimizedDuration time.Duration) {
	event := TelemetryEvent{
		EventType: "performance_improvement",
		Operation: operation,
		Duration:  optimizedDuration,
		Success:   true,
		Metadata: map[string]interface{}{
			"improvement_factor":  improvementFactor,
			"baseline_duration":   baselineDuration.Milliseconds(),
			"optimized_duration":  optimizedDuration.Milliseconds(),
			"performance_gain":    baselineDuration.Milliseconds() - optimizedDuration.Milliseconds(),
		},
	}

	a.RecordEvent(event)
}

// GetMetrics は現在のメトリクスを取得
func (a *AdvancedTelemetryCollector) GetMetrics() *PerformanceMetrics {
	a.metrics.mu.RLock()
	defer a.metrics.mu.RUnlock()

	// 現在のシステム情報を追加
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	metrics := *a.metrics
	metrics.MemoryUsagePeak = int64(m.HeapInuse / 1024 / 1024) // MB
	
	return &metrics
}

// GetUsageStats は使用統計を取得
func (a *AdvancedTelemetryCollector) GetUsageStats() *UsageStatistics {
	a.usageStats.mu.RLock()
	defer a.usageStats.mu.RUnlock()
	
	stats := *a.usageStats
	return &stats
}

// GetErrorStats はエラー統計を取得
func (a *AdvancedTelemetryCollector) GetErrorStats() *ErrorTracker {
	a.errorTracking.mu.RLock()
	defer a.errorTracking.mu.RUnlock()
	
	tracker := *a.errorTracking
	return &tracker
}

// flushWorker は定期的にバッファをフラッシュ
func (a *AdvancedTelemetryCollector) flushWorker() {
	defer a.wg.Done()
	
	ticker := time.NewTicker(a.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.flush()
		}
	}
}

// metricsCollector はシステムメトリクスを収集
func (a *AdvancedTelemetryCollector) metricsCollector() {
	defer a.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.collectSystemMetrics()
		}
	}
}

// flush はバッファを送信
func (a *AdvancedTelemetryCollector) flush() error {
	a.mu.Lock()
	if len(a.buffer) == 0 {
		a.mu.Unlock()
		return nil
	}
	
	events := make([]TelemetryEvent, len(a.buffer))
	copy(events, a.buffer)
	a.buffer = a.buffer[:0]
	a.mu.Unlock()

	// テレメトリサーバーに送信
	return a.sendTelemetryData(events)
}

// sendTelemetryData はテレメトリデータを送信
func (a *AdvancedTelemetryCollector) sendTelemetryData(events []TelemetryEvent) error {
	if a.config.TelemetryEndpoint == "" {
		return nil // エンドポイント未設定の場合はローカル保存のみ
	}

	payload := map[string]interface{}{
		"version":   a.config.Version,
		"timestamp": time.Now(),
		"events":    events,
		"metrics":   a.GetMetrics(),
		"usage":     a.GetUsageStats(),
		"errors":    a.GetErrorStats(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry data: %w", err)
	}

	resp, err := http.Post(a.config.TelemetryEndpoint, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to send telemetry data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telemetry server returned status: %d", resp.StatusCode)
	}

	return nil
}

// updateMetrics はメトリクスを更新
func (a *AdvancedTelemetryCollector) updateMetrics(event TelemetryEvent) {
	a.metrics.mu.Lock()
	defer a.metrics.mu.Unlock()

	a.metrics.OperationsCount++
	a.metrics.TotalDuration += event.Duration.Milliseconds()

	if event.FileSize > 0 {
		a.metrics.TotalBytesTransfer += event.FileSize
	}

	if event.Throughput > 0 {
		if event.Throughput > a.metrics.PeakThroughput {
			a.metrics.PeakThroughput = event.Throughput
		}
		
		// 移動平均でスループットを更新
		if a.metrics.AverageThroughput == 0 {
			a.metrics.AverageThroughput = event.Throughput
		} else {
			a.metrics.AverageThroughput = (a.metrics.AverageThroughput*0.9) + (event.Throughput*0.1)
		}
	}

	if event.WorkerCount > 0 {
		if a.metrics.AverageWorkerCount == 0 {
			a.metrics.AverageWorkerCount = float64(event.WorkerCount)
		} else {
			a.metrics.AverageWorkerCount = (a.metrics.AverageWorkerCount*0.9) + (float64(event.WorkerCount)*0.1)
		}
	}

	// 成功率を計算
	successCount := a.metrics.OperationsCount
	if !event.Success {
		successCount--
	}
	a.metrics.SuccessRate = float64(successCount) / float64(a.metrics.OperationsCount) * 100

	// パフォーマンス改善率を計算 (基準値との比較)
	if improvementFactor, ok := event.Metadata["improvement_factor"].(float64); ok {
		if a.metrics.PerformanceImprove == 0 {
			a.metrics.PerformanceImprove = improvementFactor
		} else {
			a.metrics.PerformanceImprove = (a.metrics.PerformanceImprove*0.9) + (improvementFactor*0.1)
		}
	}
}

// updateUsageStats は使用統計を更新
func (a *AdvancedTelemetryCollector) updateUsageStats(event TelemetryEvent) {
	a.usageStats.mu.Lock()
	defer a.usageStats.mu.Unlock()

	a.usageStats.OperationCounts[event.Operation]++

	if region, ok := event.Metadata["region"].(string); ok {
		a.usageStats.RegionUsage[region]++
	}

	if storageClass, ok := event.Metadata["storage_class"].(string); ok {
		a.usageStats.StorageClassUsage[storageClass]++
	}

	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	a.usageStats.PlatformUsage[platform]++

	if a.config.Version != "" {
		a.usageStats.VersionUsage[a.config.Version]++
	}

	if features, ok := event.Metadata["features"].([]string); ok {
		for _, feature := range features {
			a.usageStats.FeatureUsage[feature]++
		}
	}
}

// trackError はエラーを追跡
func (a *AdvancedTelemetryCollector) trackError(event TelemetryEvent) {
	a.errorTracking.mu.Lock()
	defer a.errorTracking.mu.Unlock()

	a.errorTracking.ErrorCounts[event.ErrorCode]++

	// エラーパターン分析
	patternKey := fmt.Sprintf("%s:%s", event.Operation, event.ErrorCode)
	if stat, exists := a.errorTracking.ErrorPatterns[patternKey]; exists {
		stat.Count++
		stat.LastSeen = event.Timestamp
		stat.Frequency = float64(stat.Count) / float64(time.Since(stat.FirstSeen).Hours())
	} else {
		a.errorTracking.ErrorPatterns[patternKey] = &ErrorStat{
			Count:     1,
			FirstSeen: event.Timestamp,
			LastSeen:  event.Timestamp,
			Frequency: 1,
			Severity:  a.determineSeverity(event.ErrorCode),
		}
	}

	// 最新エラー履歴
	errorInfo := ErrorInfo{
		Timestamp: event.Timestamp,
		Operation: event.Operation,
		ErrorCode: event.ErrorCode,
		Severity:  a.determineSeverity(event.ErrorCode),
	}

	if errorMsg, ok := event.Metadata["error_message"].(string); ok {
		errorInfo.ErrorMsg = errorMsg
	}

	if context, ok := event.Metadata["context"].(string); ok {
		errorInfo.Context = context
	}

	a.errorTracking.LastErrors = append(a.errorTracking.LastErrors, errorInfo)
	if len(a.errorTracking.LastErrors) > 100 {
		a.errorTracking.LastErrors = a.errorTracking.LastErrors[1:]
	}
}

// collectSystemMetrics はシステムメトリクスを収集
func (a *AdvancedTelemetryCollector) collectSystemMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryUsageMB := int64(m.HeapInuse / 1024 / 1024)

	a.metrics.mu.Lock()
	if memoryUsageMB > a.metrics.MemoryUsagePeak {
		a.metrics.MemoryUsagePeak = memoryUsageMB
	}
	a.metrics.mu.Unlock()

	// システムメトリクスイベントを記録
	event := TelemetryEvent{
		EventType: "system_metrics",
		Operation: "metrics_collection",
		Success:   true,
		Metadata: map[string]interface{}{
			"memory_usage_mb":    memoryUsageMB,
			"goroutines_count":   runtime.NumGoroutine(),
			"gc_cycles":          m.NumGC,
			"heap_objects":       m.HeapObjects,
			"stack_inuse_mb":     m.StackInuse / 1024 / 1024,
		},
	}

	a.RecordEvent(event)
}

// determineSeverity はエラーコードから重要度を判定
func (a *AdvancedTelemetryCollector) determineSeverity(errorCode string) string {
	switch {
	case errorCode == "NETWORK_ERROR" || errorCode == "TIMEOUT":
		return "medium"
	case errorCode == "PERMISSION_DENIED" || errorCode == "INVALID_CREDENTIALS":
		return "high"
	case errorCode == "FILE_NOT_FOUND" || errorCode == "INVALID_PATH":
		return "low"
	default:
		return "medium"
	}
}

// getSessionID はセッションIDを取得（簡易実装）
func (a *AdvancedTelemetryCollector) getSessionID() string {
	// 実際の実装では、より堅牢なセッション管理が必要
	return fmt.Sprintf("session_%d", time.Now().Unix())
}

// EnableFeature は機能を有効化
func (a *AdvancedTelemetryCollector) EnableFeature(feature string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabledFeatures[feature] = true
}

// DisableFeature は機能を無効化
func (a *AdvancedTelemetryCollector) DisableFeature(feature string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabledFeatures[feature] = false
}

// IsFeatureEnabled は機能が有効かチェック
func (a *AdvancedTelemetryCollector) IsFeatureEnabled(feature string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabledFeatures[feature]
}

// ExportMetrics はメトリクスをエクスポート
func (a *AdvancedTelemetryCollector) ExportMetrics(format string) ([]byte, error) {
	metrics := a.GetMetrics()
	usage := a.GetUsageStats()
	errors := a.GetErrorStats()

	data := map[string]interface{}{
		"timestamp":          time.Now(),
		"performance_metrics": metrics,
		"usage_statistics":   usage,
		"error_tracking":     errors,
		"version":            a.config.Version,
		"platform":           fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH),
	}

	switch format {
	case "json":
		return json.MarshalIndent(data, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}