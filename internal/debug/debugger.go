package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/seike460/s3ry/internal/config"
)

// DebugLevel はデバッグレベルを定義
type DebugLevel int

const (
	DebugOff DebugLevel = iota
	DebugBasic
	DebugVerbose
	DebugTrace
)

// String はDebugLevelの文字列表現を返す
func (d DebugLevel) String() string {
	switch d {
	case DebugOff:
		return "OFF"
	case DebugBasic:
		return "BASIC"
	case DebugVerbose:
		return "VERBOSE"
	case DebugTrace:
		return "TRACE"
	default:
		return "UNKNOWN"
	}
}

// DebugInfo はデバッグ情報を表す
type DebugInfo struct {
	Timestamp  time.Time              `json:"timestamp"`
	Level      string                 `json:"level"`
	Component  string                 `json:"component"`
	Operation  string                 `json:"operation"`
	Message    string                 `json:"message"`
	Data       map[string]interface{} `json:"data,omitempty"`
	File       string                 `json:"file,omitempty"`
	Line       int                    `json:"line,omitempty"`
	Function   string                 `json:"function,omitempty"`
	Goroutine  int                    `json:"goroutine"`
	MemStats   *runtime.MemStats      `json:"mem_stats,omitempty"`
	Duration   *time.Duration         `json:"duration,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
}

// Debugger は高度なデバッグシステム
type Debugger struct {
	mu            sync.RWMutex
	level         DebugLevel
	config        *config.Config
	outputs       []io.Writer
	buffer        []DebugInfo
	bufferSize    int
	flushInterval time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup

	// プロファイリング
	cpuProfile *os.File
	memProfile *os.File
	profileDir string

	// メトリクス
	metrics *DebugMetrics

	// トレーシング
	traces   map[string]*TraceInfo
	tracesMu sync.RWMutex

	// パフォーマンス監視
	perfMonitor *PerformanceMonitor
}

// DebugMetrics はデバッグメトリクス
type DebugMetrics struct {
	mu               sync.RWMutex
	totalDebugCalls  int64
	debugByLevel     map[DebugLevel]int64
	debugByComponent map[string]int64
	debugByOperation map[string]int64
	averageLatency   time.Duration
	peakMemoryUsage  uint64
	goroutineCount   int
}

// TraceInfo はトレース情報
type TraceInfo struct {
	ID        string                 `json:"id"`
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
	Duration  *time.Duration         `json:"duration,omitempty"`
	Operation string                 `json:"operation"`
	Component string                 `json:"component"`
	Data      map[string]interface{} `json:"data"`
	Events    []TraceEvent           `json:"events"`
	Status    string                 `json:"status"`
	Error     string                 `json:"error,omitempty"`
}

// TraceEvent はトレースイベント
type TraceEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Event     string                 `json:"event"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Duration  time.Duration          `json:"duration"`
}

// PerformanceMonitor はパフォーマンス監視
type PerformanceMonitor struct {
	mu              sync.RWMutex
	operationTimes  map[string][]time.Duration
	memorySnapshots []MemorySnapshot
	cpuUsage        []CPUSnapshot
	networkStats    []NetworkSnapshot
	diskStats       []DiskSnapshot
}

// MemorySnapshot はメモリスナップショット
type MemorySnapshot struct {
	Timestamp     time.Time `json:"timestamp"`
	Alloc         uint64    `json:"alloc"`
	TotalAlloc    uint64    `json:"total_alloc"`
	Sys           uint64    `json:"sys"`
	NumGC         uint32    `json:"num_gc"`
	GCCPUFraction float64   `json:"gc_cpu_fraction"`
}

// CPUSnapshot はCPUスナップショット
type CPUSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	Usage     float64   `json:"usage"`
	NumCPU    int       `json:"num_cpu"`
}

// NetworkSnapshot はネットワークスナップショット
type NetworkSnapshot struct {
	Timestamp   time.Time `json:"timestamp"`
	BytesSent   uint64    `json:"bytes_sent"`
	BytesRecv   uint64    `json:"bytes_recv"`
	PacketsSent uint64    `json:"packets_sent"`
	PacketsRecv uint64    `json:"packets_recv"`
}

// DiskSnapshot はディスクスナップショット
type DiskSnapshot struct {
	Timestamp  time.Time `json:"timestamp"`
	BytesRead  uint64    `json:"bytes_read"`
	BytesWrite uint64    `json:"bytes_write"`
	ReadsCount uint64    `json:"reads_count"`
	WriteCount uint64    `json:"write_count"`
}

// NewDebugger は新しいデバッガーを作成
func NewDebugger(cfg *config.Config) *Debugger {
	ctx, cancel := context.WithCancel(context.Background())

	debugger := &Debugger{
		level:         parseDebugLevel(cfg.DebugLevel),
		config:        cfg,
		outputs:       []io.Writer{os.Stdout},
		buffer:        make([]DebugInfo, 0, 1000),
		bufferSize:    1000,
		flushInterval: 5 * time.Second,
		ctx:           ctx,
		cancel:        cancel,
		profileDir:    cfg.ProfileDir,
		traces:        make(map[string]*TraceInfo),
		metrics: &DebugMetrics{
			debugByLevel:     make(map[DebugLevel]int64),
			debugByComponent: make(map[string]int64),
			debugByOperation: make(map[string]int64),
		},
		perfMonitor: &PerformanceMonitor{
			operationTimes:  make(map[string][]time.Duration),
			memorySnapshots: make([]MemorySnapshot, 0),
			cpuUsage:        make([]CPUSnapshot, 0),
			networkStats:    make([]NetworkSnapshot, 0),
			diskStats:       make([]DiskSnapshot, 0),
		},
	}

	// デバッグファイル出力を設定
	if cfg.DebugFile != "" {
		if err := debugger.addFileOutput(cfg.DebugFile); err != nil {
			fmt.Printf("Failed to add debug file output: %v\n", err)
		}
	}

	// バッファフラッシュワーカーを開始
	debugger.wg.Add(1)
	go debugger.flushWorker()

	// パフォーマンス監視ワーカーを開始
	if debugger.level >= DebugVerbose {
		debugger.wg.Add(1)
		go debugger.performanceMonitorWorker()
	}

	return debugger
}

// SetLevel はデバッグレベルを設定
func (d *Debugger) SetLevel(level DebugLevel) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.level = level
}

// Debug はデバッグ情報を出力
func (d *Debugger) Debug(level DebugLevel, component, operation, message string, data map[string]interface{}) {
	d.mu.RLock()
	if level < d.level {
		d.mu.RUnlock()
		return
	}
	d.mu.RUnlock()

	// 呼び出し元の情報を取得
	pc, file, line, ok := runtime.Caller(1)
	var function string
	if ok {
		function = runtime.FuncForPC(pc).Name()
		file = filepath.Base(file)
	}

	// メモリ統計を取得（VERBOSEレベル以上）
	var memStats *runtime.MemStats
	if level >= DebugVerbose {
		memStats = &runtime.MemStats{}
		runtime.ReadMemStats(memStats)
	}

	debugInfo := DebugInfo{
		Timestamp: time.Now(),
		Level:     level.String(),
		Component: component,
		Operation: operation,
		Message:   message,
		Data:      data,
		File:      file,
		Line:      line,
		Function:  function,
		Goroutine: runtime.NumGoroutine(),
		MemStats:  memStats,
	}

	// TRACEレベルの場合はスタックトレースを追加
	if level >= DebugTrace {
		debugInfo.StackTrace = d.captureStackTrace()
	}

	// メトリクス更新
	d.updateMetrics(level, component, operation)

	// バッファに追加
	d.mu.Lock()
	d.buffer = append(d.buffer, debugInfo)
	if len(d.buffer) >= d.bufferSize {
		go d.flushBuffer()
	}
	d.mu.Unlock()

	// 即座に出力（TRACEレベル）
	if level >= DebugTrace {
		d.writeDebugInfo(&debugInfo)
	}
}

// StartTrace はトレースを開始
func (d *Debugger) StartTrace(id, operation, component string, data map[string]interface{}) {
	if d.level < DebugVerbose {
		return
	}

	trace := &TraceInfo{
		ID:        id,
		StartTime: time.Now(),
		Operation: operation,
		Component: component,
		Data:      data,
		Events:    make([]TraceEvent, 0),
		Status:    "running",
	}

	d.tracesMu.Lock()
	d.traces[id] = trace
	d.tracesMu.Unlock()

	d.Debug(DebugVerbose, component, operation, fmt.Sprintf("Trace started: %s", id), data)
}

// AddTraceEvent はトレースイベントを追加
func (d *Debugger) AddTraceEvent(id, event string, data map[string]interface{}) {
	if d.level < DebugVerbose {
		return
	}

	d.tracesMu.Lock()
	trace, exists := d.traces[id]
	if exists {
		traceEvent := TraceEvent{
			Timestamp: time.Now(),
			Event:     event,
			Data:      data,
			Duration:  time.Since(trace.StartTime),
		}
		trace.Events = append(trace.Events, traceEvent)
	}
	d.tracesMu.Unlock()

	if exists {
		d.Debug(DebugVerbose, trace.Component, trace.Operation,
			fmt.Sprintf("Trace event [%s]: %s", id, event), data)
	}
}

// EndTrace はトレースを終了
func (d *Debugger) EndTrace(id string, err error) {
	if d.level < DebugVerbose {
		return
	}

	d.tracesMu.Lock()
	trace, exists := d.traces[id]
	if exists {
		endTime := time.Now()
		duration := endTime.Sub(trace.StartTime)

		trace.EndTime = &endTime
		trace.Duration = &duration

		if err != nil {
			trace.Status = "error"
			trace.Error = err.Error()
		} else {
			trace.Status = "completed"
		}

		// パフォーマンス統計を更新
		d.perfMonitor.mu.Lock()
		if _, exists := d.perfMonitor.operationTimes[trace.Operation]; !exists {
			d.perfMonitor.operationTimes[trace.Operation] = make([]time.Duration, 0)
		}
		d.perfMonitor.operationTimes[trace.Operation] = append(
			d.perfMonitor.operationTimes[trace.Operation], duration)
		d.perfMonitor.mu.Unlock()
	}
	d.tracesMu.Unlock()

	if exists {
		status := "completed"
		if err != nil {
			status = "failed"
		}

		d.Debug(DebugVerbose, trace.Component, trace.Operation,
			fmt.Sprintf("Trace %s: %s (duration: %v)", status, id, *trace.Duration),
			map[string]interface{}{
				"duration": trace.Duration.String(),
				"status":   status,
				"error":    err,
			})
	}
}

// StartCPUProfile はCPUプロファイリングを開始
func (d *Debugger) StartCPUProfile() error {
	if d.profileDir == "" {
		return fmt.Errorf("profile directory not configured")
	}

	if err := os.MkdirAll(d.profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create profile directory: %w", err)
	}

	filename := filepath.Join(d.profileDir, fmt.Sprintf("cpu_%d.prof", time.Now().Unix()))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CPU profile file: %w", err)
	}

	if err := pprof.StartCPUProfile(file); err != nil {
		file.Close()
		return fmt.Errorf("failed to start CPU profile: %w", err)
	}

	d.cpuProfile = file
	d.Debug(DebugBasic, "debugger", "profiling", "CPU profiling started",
		map[string]interface{}{"file": filename})

	return nil
}

// StopCPUProfile はCPUプロファイリングを停止
func (d *Debugger) StopCPUProfile() error {
	if d.cpuProfile == nil {
		return fmt.Errorf("CPU profiling not started")
	}

	pprof.StopCPUProfile()
	err := d.cpuProfile.Close()
	d.cpuProfile = nil

	d.Debug(DebugBasic, "debugger", "profiling", "CPU profiling stopped", nil)
	return err
}

// WriteMemProfile はメモリプロファイルを書き出し
func (d *Debugger) WriteMemProfile() error {
	if d.profileDir == "" {
		return fmt.Errorf("profile directory not configured")
	}

	if err := os.MkdirAll(d.profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create profile directory: %w", err)
	}

	filename := filepath.Join(d.profileDir, fmt.Sprintf("mem_%d.prof", time.Now().Unix()))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create memory profile file: %w", err)
	}
	defer file.Close()

	runtime.GC() // メモリプロファイル前にGCを実行
	if err := pprof.WriteHeapProfile(file); err != nil {
		return fmt.Errorf("failed to write memory profile: %w", err)
	}

	d.Debug(DebugBasic, "debugger", "profiling", "Memory profile written",
		map[string]interface{}{"file": filename})

	return nil
}

// GetTrace はトレース情報を取得
func (d *Debugger) GetTrace(id string) *TraceInfo {
	d.tracesMu.RLock()
	defer d.tracesMu.RUnlock()

	if trace, exists := d.traces[id]; exists {
		// コピーを返す
		traceCopy := *trace
		traceCopy.Events = make([]TraceEvent, len(trace.Events))
		copy(traceCopy.Events, trace.Events)
		return &traceCopy
	}

	return nil
}

// GetAllTraces は全てのトレース情報を取得
func (d *Debugger) GetAllTraces() map[string]*TraceInfo {
	d.tracesMu.RLock()
	defer d.tracesMu.RUnlock()

	traces := make(map[string]*TraceInfo)
	for id, trace := range d.traces {
		traceCopy := *trace
		traceCopy.Events = make([]TraceEvent, len(trace.Events))
		copy(traceCopy.Events, trace.Events)
		traces[id] = &traceCopy
	}

	return traces
}

// GetMetrics はデバッグメトリクスを取得
func (d *Debugger) GetMetrics() *DebugMetrics {
	d.metrics.mu.RLock()
	defer d.metrics.mu.RUnlock()

	metrics := &DebugMetrics{
		totalDebugCalls:  d.metrics.totalDebugCalls,
		debugByLevel:     make(map[DebugLevel]int64),
		debugByComponent: make(map[string]int64),
		debugByOperation: make(map[string]int64),
		averageLatency:   d.metrics.averageLatency,
		peakMemoryUsage:  d.metrics.peakMemoryUsage,
		goroutineCount:   d.metrics.goroutineCount,
	}

	for k, v := range d.metrics.debugByLevel {
		metrics.debugByLevel[k] = v
	}
	for k, v := range d.metrics.debugByComponent {
		metrics.debugByComponent[k] = v
	}
	for k, v := range d.metrics.debugByOperation {
		metrics.debugByOperation[k] = v
	}

	return metrics
}

// GetPerformanceReport はパフォーマンスレポートを取得
func (d *Debugger) GetPerformanceReport() *PerformanceReport {
	d.perfMonitor.mu.RLock()
	defer d.perfMonitor.mu.RUnlock()

	report := &PerformanceReport{
		Timestamp:       time.Now(),
		OperationStats:  make(map[string]*OperationStats),
		MemorySnapshots: make([]MemorySnapshot, len(d.perfMonitor.memorySnapshots)),
		CPUUsage:        make([]CPUSnapshot, len(d.perfMonitor.cpuUsage)),
		NetworkStats:    make([]NetworkSnapshot, len(d.perfMonitor.networkStats)),
		DiskStats:       make([]DiskSnapshot, len(d.perfMonitor.diskStats)),
	}

	// 操作統計を計算
	for operation, times := range d.perfMonitor.operationTimes {
		if len(times) == 0 {
			continue
		}

		var total time.Duration
		min := times[0]
		max := times[0]

		for _, t := range times {
			total += t
			if t < min {
				min = t
			}
			if t > max {
				max = t
			}
		}

		report.OperationStats[operation] = &OperationStats{
			Count:   int64(len(times)),
			Total:   total,
			Average: total / time.Duration(len(times)),
			Min:     min,
			Max:     max,
		}
	}

	copy(report.MemorySnapshots, d.perfMonitor.memorySnapshots)
	copy(report.CPUUsage, d.perfMonitor.cpuUsage)
	copy(report.NetworkStats, d.perfMonitor.networkStats)
	copy(report.DiskStats, d.perfMonitor.diskStats)

	return report
}

// PerformanceReport はパフォーマンスレポート
type PerformanceReport struct {
	Timestamp       time.Time                  `json:"timestamp"`
	OperationStats  map[string]*OperationStats `json:"operation_stats"`
	MemorySnapshots []MemorySnapshot           `json:"memory_snapshots"`
	CPUUsage        []CPUSnapshot              `json:"cpu_usage"`
	NetworkStats    []NetworkSnapshot          `json:"network_stats"`
	DiskStats       []DiskSnapshot             `json:"disk_stats"`
}

// OperationStats は操作統計
type OperationStats struct {
	Count   int64         `json:"count"`
	Total   time.Duration `json:"total"`
	Average time.Duration `json:"average"`
	Min     time.Duration `json:"min"`
	Max     time.Duration `json:"max"`
}

// Close はデバッガーを閉じる
func (d *Debugger) Close() error {
	// CPUプロファイリングを停止
	if d.cpuProfile != nil {
		d.StopCPUProfile()
	}

	d.cancel()
	d.wg.Wait()
	return d.flushBuffer()
}

// 内部メソッド

func (d *Debugger) updateMetrics(level DebugLevel, component, operation string) {
	d.metrics.mu.Lock()
	defer d.metrics.mu.Unlock()

	d.metrics.totalDebugCalls++
	d.metrics.debugByLevel[level]++
	d.metrics.debugByComponent[component]++
	d.metrics.debugByOperation[operation]++
	d.metrics.goroutineCount = runtime.NumGoroutine()
}

func (d *Debugger) writeDebugInfo(info *DebugInfo) {
	data, _ := json.Marshal(info)
	output := string(data) + "\n"

	d.mu.RLock()
	for _, writer := range d.outputs {
		writer.Write([]byte(output))
	}
	d.mu.RUnlock()
}

func (d *Debugger) flushWorker() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.flushBuffer()
		}
	}
}

func (d *Debugger) flushBuffer() error {
	d.mu.Lock()
	if len(d.buffer) == 0 {
		d.mu.Unlock()
		return nil
	}

	infos := make([]DebugInfo, len(d.buffer))
	copy(infos, d.buffer)
	d.buffer = d.buffer[:0]
	d.mu.Unlock()

	for _, info := range infos {
		d.writeDebugInfo(&info)
	}

	return nil
}

func (d *Debugger) performanceMonitorWorker() {
	defer d.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.capturePerformanceSnapshot()
		}
	}
}

func (d *Debugger) capturePerformanceSnapshot() {
	now := time.Now()

	// メモリスナップショット
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	memSnapshot := MemorySnapshot{
		Timestamp:     now,
		Alloc:         memStats.Alloc,
		TotalAlloc:    memStats.TotalAlloc,
		Sys:           memStats.Sys,
		NumGC:         memStats.NumGC,
		GCCPUFraction: memStats.GCCPUFraction,
	}

	d.perfMonitor.mu.Lock()
	d.perfMonitor.memorySnapshots = append(d.perfMonitor.memorySnapshots, memSnapshot)

	// 古いスナップショットを削除（最新100個を保持）
	if len(d.perfMonitor.memorySnapshots) > 100 {
		d.perfMonitor.memorySnapshots = d.perfMonitor.memorySnapshots[1:]
	}

	// ピークメモリ使用量を更新
	if memStats.Alloc > d.metrics.peakMemoryUsage {
		d.metrics.peakMemoryUsage = memStats.Alloc
	}
	d.perfMonitor.mu.Unlock()
}

func (d *Debugger) addFileOutput(filename string) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create debug directory: %w", err)
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open debug file: %w", err)
	}

	d.mu.Lock()
	d.outputs = append(d.outputs, file)
	d.mu.Unlock()

	return nil
}

func (d *Debugger) captureStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

func parseDebugLevel(level string) DebugLevel {
	switch level {
	case "OFF", "off":
		return DebugOff
	case "BASIC", "basic":
		return DebugBasic
	case "VERBOSE", "verbose":
		return DebugVerbose
	case "TRACE", "trace":
		return DebugTrace
	default:
		return DebugOff
	}
}

// グローバルデバッガー
var defaultDebugger *Debugger

// InitializeDebugger はグローバルデバッガーを初期化
func InitializeDebugger(cfg *config.Config) {
	defaultDebugger = NewDebugger(cfg)
}

// GetDebugger はグローバルデバッガーを取得
func GetDebugger() *Debugger {
	if defaultDebugger == nil {
		cfg := &config.Config{
			DebugLevel: "OFF",
		}
		defaultDebugger = NewDebugger(cfg)
	}
	return defaultDebugger
}

// パッケージレベルの便利関数
func Debug(level DebugLevel, component, operation, message string, data map[string]interface{}) {
	GetDebugger().Debug(level, component, operation, message, data)
}

func StartTrace(id, operation, component string, data map[string]interface{}) {
	GetDebugger().StartTrace(id, operation, component, data)
}

func AddTraceEvent(id, event string, data map[string]interface{}) {
	GetDebugger().AddTraceEvent(id, event, data)
}

func EndTrace(id string, err error) {
	GetDebugger().EndTrace(id, err)
}
