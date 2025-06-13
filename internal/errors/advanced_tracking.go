package errors

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/seike460/s3ry/internal/config"
)

// AdvancedErrorTracker は高度なエラー追跡システム
type AdvancedErrorTracker struct {
	mu                 sync.RWMutex
	config            *config.Config
	errorBuffer       []ErrorEvent
	errorPatterns     map[string]*ErrorPattern
	errorCategories   map[string]*ErrorCategory
	errorResolutions  map[string]*ErrorResolution
	alertRules        []AlertRule
	analyticsEngine   *ErrorAnalyticsEngine
	predictionModel   *ErrorPredictionModel
	notificationSender *NotificationSender
	bufferSize        int
	flushInterval     time.Duration
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
}

// ErrorEvent はエラーイベント
type ErrorEvent struct {
	ID               string                 `json:"id"`
	Timestamp        time.Time              `json:"timestamp"`
	Operation        string                 `json:"operation"`
	ErrorCode        string                 `json:"error_code"`
	ErrorMessage     string                 `json:"error_message"`
	ErrorType        string                 `json:"error_type"`
	Severity         string                 `json:"severity"`
	StackTrace       string                 `json:"stack_trace,omitempty"`
	Context          map[string]interface{} `json:"context"`
	UserID           string                 `json:"user_id,omitempty"`
	SessionID        string                 `json:"session_id"`
	RequestID        string                 `json:"request_id,omitempty"`
	UserAgent        string                 `json:"user_agent,omitempty"`
	IPAddress        string                 `json:"ip_address,omitempty"`
	Environment      string                 `json:"environment"`
	Version          string                 `json:"version"`
	Platform         string                 `json:"platform"`
	Fingerprint      string                 `json:"fingerprint"`
	Tags             []string               `json:"tags,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	Resolved         bool                   `json:"resolved"`
	ResolvedAt       *time.Time             `json:"resolved_at,omitempty"`
	ResolvedBy       string                 `json:"resolved_by,omitempty"`
	ResolutionNotes  string                 `json:"resolution_notes,omitempty"`
}

// ErrorPattern はエラーパターン
type ErrorPattern struct {
	ID               string                 `json:"id"`
	Pattern          string                 `json:"pattern"`
	Regex            *regexp.Regexp         `json:"-"`
	Category         string                 `json:"category"`
	Severity         string                 `json:"severity"`
	Occurrences      int64                  `json:"occurrences"`
	FirstSeen        time.Time              `json:"first_seen"`
	LastSeen         time.Time              `json:"last_seen"`
	Frequency        float64                `json:"frequency"`
	TrendDirection   string                 `json:"trend_direction"`
	ImpactScore      float64                `json:"impact_score"`
	ResolutionStatus string                 `json:"resolution_status"`
	KnownSolution    string                 `json:"known_solution,omitempty"`
	RelatedPatterns  []string               `json:"related_patterns,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// ErrorCategory はエラーカテゴリ
type ErrorCategory struct {
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	SeverityLevel   int       `json:"severity_level"`
	AutoResolve     bool      `json:"auto_resolve"`
	NotificationLevel string  `json:"notification_level"`
	EscalationTime  time.Duration `json:"escalation_time"`
	OwnerTeam       string    `json:"owner_team,omitempty"`
	PlaybookURL     string    `json:"playbook_url,omitempty"`
	Tags            []string  `json:"tags,omitempty"`
}

// ErrorResolution はエラー解決情報
type ErrorResolution struct {
	ID              string                 `json:"id"`
	ErrorPattern    string                 `json:"error_pattern"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	Steps           []string               `json:"steps"`
	PreventionTips  []string               `json:"prevention_tips"`
	RelatedDocs     []string               `json:"related_docs"`
	Effectiveness   float64                `json:"effectiveness"`
	UsageCount      int64                  `json:"usage_count"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	CreatedBy       string                 `json:"created_by"`
	ApprovedBy      string                 `json:"approved_by,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// AlertRule はアラートルール
type AlertRule struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Condition       string        `json:"condition"`
	Threshold       float64       `json:"threshold"`
	TimeWindow      time.Duration `json:"time_window"`
	Severity        string        `json:"severity"`
	Enabled         bool          `json:"enabled"`
	NotificationChannels []string `json:"notification_channels"`
	CooldownPeriod  time.Duration `json:"cooldown_period"`
	LastTriggered   *time.Time    `json:"last_triggered,omitempty"`
	TriggerCount    int64         `json:"trigger_count"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ErrorAnalyticsEngine はエラー分析エンジン
type ErrorAnalyticsEngine struct {
	mu                  sync.RWMutex
	errorTrends         map[string]*TrendData
	errorCorrelations   map[string][]string
	antropyClusters     []ErrorCluster
	performanceImpact   map[string]*PerformanceImpact
	userImpactAnalysis  map[string]*UserImpactData
	predictiveIndicators map[string]float64
}

// TrendData はトレンドデータ
type TrendData struct {
	TimePoints      []time.Time `json:"time_points"`
	ErrorCounts     []int64     `json:"error_counts"`
	MovingAverage   []float64   `json:"moving_average"`
	TrendSlope      float64     `json:"trend_slope"`
	Seasonality     string      `json:"seasonality"`
	AnomalyScores   []float64   `json:"anomaly_scores"`
	PredictedCounts []int64     `json:"predicted_counts"`
}

// ErrorCluster はエラークラスター
type ErrorCluster struct {
	ID               string       `json:"id"`
	CenterError      string       `json:"center_error"`
	SimilarErrors    []string     `json:"similar_errors"`
	SimilarityScore  float64      `json:"similarity_score"`
	ClusterSize      int          `json:"cluster_size"`
	ImpactScore      float64      `json:"impact_score"`
	RecommendedAction string      `json:"recommended_action"`
	ClusterTags      []string     `json:"cluster_tags"`
}

// PerformanceImpact はパフォーマンス影響
type PerformanceImpact struct {
	ErrorType          string  `json:"error_type"`
	LatencyIncrease    float64 `json:"latency_increase_ms"`
	ThroughputDecrease float64 `json:"throughput_decrease_percent"`
	ResourceUsage      float64 `json:"resource_usage_increase_percent"`
	UserExperienceScore float64 `json:"user_experience_score"`
	BusinessImpact     string  `json:"business_impact"`
}

// UserImpactData はユーザー影響データ
type UserImpactData struct {
	AffectedUsers    int64   `json:"affected_users"`
	SessionImpact    float64 `json:"session_impact_percent"`
	FrustrationScore float64 `json:"frustration_score"`
	ChurnRisk        string  `json:"churn_risk"`
	RecoveryTime     time.Duration `json:"recovery_time"`
}

// ErrorPredictionModel はエラー予測モデル
type ErrorPredictionModel struct {
	mu                sync.RWMutex
	historicalData    []ErrorEvent
	patternModels     map[string]*PredictionPattern
	anomalyDetector   *AnomalyDetector
	forecastHorizon   time.Duration
	confidenceThreshold float64
}

// PredictionPattern は予測パターン
type PredictionPattern struct {
	Pattern          string    `json:"pattern"`
	Probability      float64   `json:"probability"`
	ExpectedTime     time.Time `json:"expected_time"`
	ConfidenceScore  float64   `json:"confidence_score"`
	PreventionActions []string `json:"prevention_actions"`
	RiskLevel        string    `json:"risk_level"`
}

// AnomalyDetector は異常検知器
type AnomalyDetector struct {
	Threshold        float64              `json:"threshold"`
	BaselineMetrics  map[string]float64   `json:"baseline_metrics"`
	AnomalyScores    map[string]float64   `json:"anomaly_scores"`
	DetectionRules   []AnomalyRule        `json:"detection_rules"`
}

// AnomalyRule は異常検知ルール
type AnomalyRule struct {
	Name        string  `json:"name"`
	Metric      string  `json:"metric"`
	Condition   string  `json:"condition"`
	Threshold   float64 `json:"threshold"`
	Severity    string  `json:"severity"`
	Enabled     bool    `json:"enabled"`
}

// NotificationSender は通知送信器
type NotificationSender struct {
	mu       sync.RWMutex
	channels map[string]NotificationChannel
	templates map[string]*NotificationTemplate
	queue    chan NotificationRequest
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NotificationChannel は通知チャネル
type NotificationChannel interface {
	Send(ctx context.Context, notification *Notification) error
	GetType() string
	IsEnabled() bool
	GetConfig() map[string]interface{}
}

// NotificationTemplate は通知テンプレート
type NotificationTemplate struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
	Format   string `json:"format"`
	Channels []string `json:"channels"`
	Variables map[string]string `json:"variables"`
}

// Notification は通知
type Notification struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Severity  string                 `json:"severity"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data"`
	Channels  []string               `json:"channels"`
	RetryCount int                   `json:"retry_count"`
	MaxRetries int                   `json:"max_retries"`
}

// NotificationRequest は通知リクエスト
type NotificationRequest struct {
	Notification *Notification
	Callback     func(error)
}

// NewAdvancedErrorTracker は高度なエラートラッカーを作成
func NewAdvancedErrorTracker(cfg *config.Config) *AdvancedErrorTracker {
	ctx, cancel := context.WithCancel(context.Background())

	tracker := &AdvancedErrorTracker{
		config:           cfg,
		errorBuffer:      make([]ErrorEvent, 0, 1000),
		errorPatterns:    make(map[string]*ErrorPattern),
		errorCategories:  make(map[string]*ErrorCategory),
		errorResolutions: make(map[string]*ErrorResolution),
		alertRules:       make([]AlertRule, 0),
		bufferSize:       1000,
		flushInterval:    5 * time.Minute,
		ctx:              ctx,
		cancel:           cancel,
	}

	// 分析エンジンを初期化
	tracker.analyticsEngine = &ErrorAnalyticsEngine{
		errorTrends:         make(map[string]*TrendData),
		errorCorrelations:   make(map[string][]string),
		antropyClusters:     make([]ErrorCluster, 0),
		performanceImpact:   make(map[string]*PerformanceImpact),
		userImpactAnalysis:  make(map[string]*UserImpactData),
		predictiveIndicators: make(map[string]float64),
	}

	// 予測モデルを初期化
	tracker.predictionModel = &ErrorPredictionModel{
		historicalData:      make([]ErrorEvent, 0),
		patternModels:       make(map[string]*PredictionPattern),
		anomalyDetector:     &AnomalyDetector{
			Threshold:       0.95,
			BaselineMetrics: make(map[string]float64),
			AnomalyScores:   make(map[string]float64),
			DetectionRules:  make([]AnomalyRule, 0),
		},
		forecastHorizon:     24 * time.Hour,
		confidenceThreshold: 0.8,
	}

	// 通知送信器を初期化
	notificationCtx, notificationCancel := context.WithCancel(context.Background())
	tracker.notificationSender = &NotificationSender{
		channels:  make(map[string]NotificationChannel),
		templates: make(map[string]*NotificationTemplate),
		queue:     make(chan NotificationRequest, 1000),
		ctx:       notificationCtx,
		cancel:    notificationCancel,
	}

	// デフォルトエラーカテゴリを設定
	tracker.initializeDefaultCategories()
	
	// デフォルトアラートルールを設定
	tracker.initializeDefaultAlertRules()

	// デフォルト通知テンプレートを設定
	tracker.initializeDefaultTemplates()

	return tracker
}

// Start はエラートラッカーを開始
func (t *AdvancedErrorTracker) Start() error {
	// バッファフラッシュワーカーを開始
	t.wg.Add(1)
	go t.bufferFlushWorker()

	// エラー分析ワーカーを開始
	t.wg.Add(1)
	go t.analyticsWorker()

	// アラート監視ワーカーを開始
	t.wg.Add(1)
	go t.alertMonitorWorker()

	// 予測モデル更新ワーカーを開始
	t.wg.Add(1)
	go t.predictionModelWorker()

	// 通知送信ワーカーを開始
	t.notificationSender.wg.Add(1)
	go t.notificationSender.worker()

	fmt.Println("🚨 高度エラー追跡システム開始")
	fmt.Println("📊 リアルタイム分析・予測・アラート機能稼働中")

	return nil
}

// Stop はエラートラッカーを停止
func (t *AdvancedErrorTracker) Stop() error {
	t.cancel()
	t.notificationSender.cancel()
	t.wg.Wait()
	t.notificationSender.wg.Wait()
	
	// 最終フラッシュ
	return t.flushErrors()
}

// TrackError はエラーを追跡
func (t *AdvancedErrorTracker) TrackError(operation, errorCode, errorMessage, errorType string, context map[string]interface{}) {
	now := time.Now()
	
	// スタックトレースを取得
	stackTrace := t.captureStackTrace(3)
	
	// フィンガープリントを生成
	fingerprint := t.generateFingerprint(operation, errorCode, errorMessage)
	
	// エラーイベントを作成
	errorEvent := ErrorEvent{
		ID:           fmt.Sprintf("%d_%s", now.UnixNano(), fingerprint[:8]),
		Timestamp:    now,
		Operation:    operation,
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
		ErrorType:    errorType,
		Severity:     t.determineSeverity(errorCode),
		StackTrace:   stackTrace,
		Context:      context,
		SessionID:    t.getSessionID(context),
		RequestID:    t.getRequestID(context),
		Environment:  t.config.Environment,
		Version:      t.config.Version,
		Platform:     fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH),
		Fingerprint:  fingerprint,
		Tags:         t.generateTags(operation, errorCode, context),
		Metadata:     t.extractMetadata(context),
	}

	t.mu.Lock()
	t.errorBuffer = append(t.errorBuffer, errorEvent)

	// バッファサイズ制限
	if len(t.errorBuffer) >= t.bufferSize {
		go t.flushErrors()
	}
	t.mu.Unlock()

	// リアルタイム処理
	go t.processErrorRealtime(errorEvent)
}

// TrackPanic はパニックを追跡
func (t *AdvancedErrorTracker) TrackPanic(recovered interface{}, operation string, context map[string]interface{}) {
	stackTrace := t.captureStackTrace(0)
	
	errorMsg := fmt.Sprintf("Panic recovered: %v", recovered)
	
	if context == nil {
		context = make(map[string]interface{})
	}
	context["panic_value"] = recovered
	context["stack_trace"] = stackTrace
	
	t.TrackError(operation, "PANIC", errorMsg, "panic", context)
}

// GetErrorAnalytics はエラー分析結果を取得
func (t *AdvancedErrorTracker) GetErrorAnalytics() *ErrorAnalyticsResult {
	t.analyticsEngine.mu.RLock()
	defer t.analyticsEngine.mu.RUnlock()

	t.mu.RLock()
	defer t.mu.RUnlock()

	// トップエラーパターンを計算
	topPatterns := t.getTopErrorPatterns(10)
	
	// エラートレンドを計算
	errorTrends := make(map[string]*TrendData)
	for k, v := range t.analyticsEngine.errorTrends {
		errorTrends[k] = v
	}

	// エラークラスターを取得
	errorClusters := make([]ErrorCluster, len(t.analyticsEngine.entropyClusters))
	copy(errorClusters, t.analyticsEngine.entropyClusters)

	// パフォーマンス影響を取得
	performanceImpacts := make(map[string]*PerformanceImpact)
	for k, v := range t.analyticsEngine.performanceImpact {
		performanceImpacts[k] = v
	}

	// 予測結果を取得
	predictions := t.predictionModel.getPredictions()

	return &ErrorAnalyticsResult{
		Timestamp:          time.Now(),
		TopErrorPatterns:   topPatterns,
		ErrorTrends:        errorTrends,
		ErrorClusters:      errorClusters,
		PerformanceImpacts: performanceImpacts,
		Predictions:        predictions,
		AnomalyScores:      t.predictionModel.anomalyDetector.AnomalyScores,
		HealthScore:        t.calculateHealthScore(),
		Recommendations:    t.generateRecommendations(),
	}
}

// ErrorAnalyticsResult はエラー分析結果
type ErrorAnalyticsResult struct {
	Timestamp          time.Time                       `json:"timestamp"`
	TopErrorPatterns   []*ErrorPattern                 `json:"top_error_patterns"`
	ErrorTrends        map[string]*TrendData           `json:"error_trends"`
	ErrorClusters      []ErrorCluster                  `json:"error_clusters"`
	PerformanceImpacts map[string]*PerformanceImpact   `json:"performance_impacts"`
	Predictions        []*PredictionPattern            `json:"predictions"`
	AnomalyScores      map[string]float64              `json:"anomaly_scores"`
	HealthScore        float64                         `json:"health_score"`
	Recommendations    []AnalyticsRecommendation       `json:"recommendations"`
}

// AnalyticsRecommendation は分析推奨事項
type AnalyticsRecommendation struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Priority    string    `json:"priority"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Action      string    `json:"action"`
	Impact      string    `json:"impact"`
	Confidence  float64   `json:"confidence"`
	CreatedAt   time.Time `json:"created_at"`
}

// bufferFlushWorker はバッファを定期的にフラッシュ
func (t *AdvancedErrorTracker) bufferFlushWorker() {
	defer t.wg.Done()
	
	ticker := time.NewTicker(t.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.flushErrors()
		}
	}
}

// flushErrors はエラーバッファをフラッシュ
func (t *AdvancedErrorTracker) flushErrors() error {
	t.mu.Lock()
	if len(t.errorBuffer) == 0 {
		t.mu.Unlock()
		return nil
	}
	
	errorsToFlush := make([]ErrorEvent, len(t.errorBuffer))
	copy(errorsToFlush, t.errorBuffer)
	t.errorBuffer = t.errorBuffer[:0]
	t.mu.Unlock()

	// エラーデータを外部システムに送信
	return t.sendErrorData(errorsToFlush)
}

// sendErrorData はエラーデータを送信
func (t *AdvancedErrorTracker) sendErrorData(errors []ErrorEvent) error {
	if t.config.ErrorTrackingEndpoint == "" {
		return nil // エンドポイント未設定の場合はローカル保存のみ
	}

	payload := map[string]interface{}{
		"timestamp": time.Now(),
		"version":   t.config.Version,
		"errors":    errors,
		"analytics": t.GetErrorAnalytics(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal error data: %w", err)
	}

	resp, err := http.Post(t.config.ErrorTrackingEndpoint, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("failed to send error data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error tracking server returned status: %d", resp.StatusCode)
	}

	return nil
}

// processErrorRealtime はエラーをリアルタイム処理
func (t *AdvancedErrorTracker) processErrorRealtime(errorEvent ErrorEvent) {
	// パターンマッチング
	t.matchErrorPatterns(errorEvent)
	
	// 異常検知
	t.detectAnomalies(errorEvent)
	
	// アラートチェック
	t.checkAlertConditions(errorEvent)
	
	// パフォーマンス影響分析
	t.analyzePerformanceImpact(errorEvent)
}

// 以下は実装の続き...（文字数制限のため一部省略）

// Helper functions
func (t *AdvancedErrorTracker) captureStackTrace(skip int) string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

func (t *AdvancedErrorTracker) generateFingerprint(operation, errorCode, errorMessage string) string {
	data := fmt.Sprintf("%s:%s:%s", operation, errorCode, errorMessage)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func (t *AdvancedErrorTracker) determineSeverity(errorCode string) string {
	switch {
	case strings.Contains(errorCode, "CRITICAL") || strings.Contains(errorCode, "PANIC"):
		return "critical"
	case strings.Contains(errorCode, "ERROR"):
		return "high"
	case strings.Contains(errorCode, "WARNING"):
		return "medium"
	default:
		return "low"
	}
}

func (t *AdvancedErrorTracker) getSessionID(context map[string]interface{}) string {
	if sessionID, ok := context["session_id"].(string); ok {
		return sessionID
	}
	return fmt.Sprintf("session_%d", time.Now().Unix())
}

func (t *AdvancedErrorTracker) getRequestID(context map[string]interface{}) string {
	if requestID, ok := context["request_id"].(string); ok {
		return requestID
	}
	return ""
}

func (t *AdvancedErrorTracker) generateTags(operation, errorCode string, context map[string]interface{}) []string {
	tags := []string{operation, errorCode}
	
	if env, ok := context["environment"].(string); ok {
		tags = append(tags, "env:"+env)
	}
	
	if component, ok := context["component"].(string); ok {
		tags = append(tags, "component:"+component)
	}
	
	return tags
}

func (t *AdvancedErrorTracker) extractMetadata(context map[string]interface{}) map[string]interface{} {
	metadata := make(map[string]interface{})
	
	for k, v := range context {
		if !strings.HasPrefix(k, "_") { // 内部キーを除外
			metadata[k] = v
		}
	}
	
	return metadata
}

func (t *AdvancedErrorTracker) initializeDefaultCategories() {
	t.errorCategories["network"] = &ErrorCategory{
		Name:            "Network Errors",
		Description:     "Network connectivity and timeout errors",
		SeverityLevel:   2,
		AutoResolve:     true,
		NotificationLevel: "warning",
		EscalationTime:  15 * time.Minute,
	}
	
	t.errorCategories["authentication"] = &ErrorCategory{
		Name:            "Authentication Errors",
		Description:     "Authentication and authorization failures",
		SeverityLevel:   3,
		AutoResolve:     false,
		NotificationLevel: "critical",
		EscalationTime:  5 * time.Minute,
	}
}

func (t *AdvancedErrorTracker) initializeDefaultAlertRules() {
	t.alertRules = append(t.alertRules, AlertRule{
		ID:          "high_error_rate",
		Name:        "High Error Rate",
		Condition:   "error_rate > threshold",
		Threshold:   5.0, // 5%
		TimeWindow:  5 * time.Minute,
		Severity:    "critical",
		Enabled:     true,
		NotificationChannels: []string{"email", "slack"},
		CooldownPeriod: 15 * time.Minute,
	})
}

func (t *AdvancedErrorTracker) initializeDefaultTemplates() {
	t.notificationSender.templates["error_alert"] = &NotificationTemplate{
		ID:      "error_alert",
		Name:    "Error Alert",
		Subject: "🚨 S3ry Error Alert: {{.Severity}} - {{.Title}}",
		Body:    "Error detected in S3ry:\n\nOperation: {{.Operation}}\nError: {{.ErrorMessage}}\nTime: {{.Timestamp}}\n\nDetails: {{.Details}}",
		Format:  "text",
		Channels: []string{"email", "slack"},
	}
}

// 残りのメソッドは実装継続...