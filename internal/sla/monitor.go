package sla

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// SLAMonitor provides comprehensive SLA monitoring and alerting
type SLAMonitor struct {
	config        *SLAConfig
	slas          map[string]*SLA
	violations    map[string]*Violation
	metrics       *SLAMetrics
	alertManager  AlertManager
	storage       SLAStorage
	stopCh        chan struct{}
	running       bool
	mutex         sync.RWMutex
}

// SLAConfig holds SLA monitoring configuration
type SLAConfig struct {
	Enabled            bool          `json:"enabled"`
	CheckInterval      time.Duration `json:"check_interval"`
	ViolationThreshold int           `json:"violation_threshold"`
	AlertCooldown      time.Duration `json:"alert_cooldown"`
	RetentionPeriod    time.Duration `json:"retention_period"`
	AutoRemediation    bool          `json:"auto_remediation"`
	ConfigDir          string        `json:"config_dir"`
}

// DefaultSLAConfig returns default SLA configuration
func DefaultSLAConfig() *SLAConfig {
	return &SLAConfig{
		Enabled:            true,
		CheckInterval:      time.Minute * 5,
		ViolationThreshold: 3,
		AlertCooldown:      time.Minute * 15,
		RetentionPeriod:    time.Hour * 24 * 30, // 30 days
		AutoRemediation:    false,
		ConfigDir:          "config/sla",
	}
}

// SLA represents a Service Level Agreement
type SLA struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	Service          string              `json:"service"`
	Category         SLACategory         `json:"category"`
	Type             SLAType             `json:"type"`
	Targets          []SLATarget         `json:"targets"`
	Measurement      MeasurementConfig   `json:"measurement"`
	Alerting         AlertingConfig      `json:"alerting"`
	Remediation      RemediationConfig   `json:"remediation"`
	Schedule         ScheduleConfig      `json:"schedule"`
	Tags             map[string]string   `json:"tags"`
	Enabled          bool                `json:"enabled"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	LastChecked      time.Time           `json:"last_checked"`
	CurrentStatus    SLAStatus           `json:"current_status"`
	ViolationCount   int                 `json:"violation_count"`
	UptimePercent    float64             `json:"uptime_percent"`
	ResponseTime     ResponseTimeMetrics `json:"response_time"`
}

// SLACategory categorizes SLAs
type SLACategory string

const (
	SLACategoryAvailability  SLACategory = "AVAILABILITY"
	SLACategoryPerformance   SLACategory = "PERFORMANCE"
	SLACategoryReliability   SLACategory = "RELIABILITY"
	SLACategoryCapacity      SLACategory = "CAPACITY"
	SLACategorySecurity      SLACategory = "SECURITY"
	SLACategoryCompliance    SLACategory = "COMPLIANCE"
)

// SLAType defines the type of SLA
type SLAType string

const (
	SLATypeUptime        SLAType = "UPTIME"
	SLATypeResponseTime  SLAType = "RESPONSE_TIME"
	SLATypeThroughput    SLAType = "THROUGHPUT"
	SLATypeErrorRate     SLAType = "ERROR_RATE"
	SLATypeRecoveryTime  SLAType = "RECOVERY_TIME"
	SLATypeDataIntegrity SLAType = "DATA_INTEGRITY"
)

// SLAStatus represents the current status of an SLA
type SLAStatus string

const (
	SLAStatusHealthy   SLAStatus = "HEALTHY"
	SLAStatusWarning   SLAStatus = "WARNING"
	SLAStatusViolation SLAStatus = "VIOLATION"
	SLAStatusCritical  SLAStatus = "CRITICAL"
	SLAStatusUnknown   SLAStatus = "UNKNOWN"
)

// SLATarget defines an SLA target/threshold
type SLATarget struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Metric      string      `json:"metric"`
	Threshold   float64     `json:"threshold"`
	Operator    Operator    `json:"operator"` // >=, <=, ==, !=
	Unit        string      `json:"unit"`
	Window      TimeWindow  `json:"window"`
	Severity    Severity    `json:"severity"`
	Enabled     bool        `json:"enabled"`
}

// Operator defines comparison operators
type Operator string

const (
	OperatorGreaterEqual Operator = ">="
	OperatorLessEqual    Operator = "<="
	OperatorEqual        Operator = "=="
	OperatorNotEqual     Operator = "!="
	OperatorGreater      Operator = ">"
	OperatorLess         Operator = "<"
)

// TimeWindow defines a time window for SLA measurement
type TimeWindow struct {
	Duration time.Duration `json:"duration"`
	Type     WindowType    `json:"type"`
}

// WindowType defines the type of time window
type WindowType string

const (
	WindowTypeRolling WindowType = "ROLLING"
	WindowTypeFixed   WindowType = "FIXED"
	WindowTypeDaily   WindowType = "DAILY"
	WindowTypeWeekly  WindowType = "WEEKLY"
	WindowTypeMonthly WindowType = "MONTHLY"
)

// Severity defines alert severity levels
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityError    Severity = "ERROR"
	SeverityCritical Severity = "CRITICAL"
)

// MeasurementConfig defines how to measure SLA compliance
type MeasurementConfig struct {
	DataSource    string            `json:"data_source"`
	Query         string            `json:"query"`
	Aggregation   AggregationType   `json:"aggregation"`
	SampleRate    float64           `json:"sample_rate"`
	Filters       map[string]string `json:"filters"`
	CustomMetrics []CustomMetric    `json:"custom_metrics"`
}

// AggregationType defines how to aggregate measurements
type AggregationType string

const (
	AggregationAverage AggregationType = "AVERAGE"
	AggregationSum     AggregationType = "SUM"
	AggregationMin     AggregationType = "MIN"
	AggregationMax     AggregationType = "MAX"
	AggregationCount   AggregationType = "COUNT"
	AggregationP95     AggregationType = "P95"
	AggregationP99     AggregationType = "P99"
)

// CustomMetric defines custom SLA metrics
type CustomMetric struct {
	Name        string `json:"name"`
	Expression  string `json:"expression"`
	Description string `json:"description"`
}

// AlertingConfig defines how to alert on SLA violations
type AlertingConfig struct {
	Enabled         bool                   `json:"enabled"`
	Channels        []AlertChannel         `json:"channels"`
	Escalation      EscalationPolicy       `json:"escalation"`
	Suppression     SuppressionConfig      `json:"suppression"`
	Templates       map[string]string      `json:"templates"`
	Recipients      []Recipient            `json:"recipients"`
}

// AlertChannel defines alert delivery channels
type AlertChannel struct {
	Type     ChannelType       `json:"type"`
	Config   map[string]string `json:"config"`
	Enabled  bool              `json:"enabled"`
	Severity []Severity        `json:"severity"`
}

// ChannelType defines types of alert channels
type ChannelType string

const (
	ChannelTypeEmail     ChannelType = "EMAIL"
	ChannelTypeSlack     ChannelType = "SLACK"
	ChannelTypeSMS       ChannelType = "SMS"
	ChannelTypeWebhook   ChannelType = "WEBHOOK"
	ChannelTypePagerDuty ChannelType = "PAGERDUTY"
	ChannelTypeOpsGenie  ChannelType = "OPSGENIE"
)

// EscalationPolicy defines how to escalate alerts
type EscalationPolicy struct {
	Enabled  bool               `json:"enabled"`
	Levels   []EscalationLevel  `json:"levels"`
	Timeout  time.Duration      `json:"timeout"`
}

// EscalationLevel defines an escalation level
type EscalationLevel struct {
	Level      int           `json:"level"`
	Delay      time.Duration `json:"delay"`
	Recipients []Recipient   `json:"recipients"`
	Actions    []string      `json:"actions"`
}

// SuppressionConfig defines alert suppression rules
type SuppressionConfig struct {
	Enabled    bool              `json:"enabled"`
	Rules      []SuppressionRule `json:"rules"`
	Maintenance []MaintenanceWindow `json:"maintenance"`
}

// SuppressionRule defines when to suppress alerts
type SuppressionRule struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Condition string            `json:"condition"`
	Duration  time.Duration     `json:"duration"`
	Tags      map[string]string `json:"tags"`
}

// MaintenanceWindow defines maintenance periods
type MaintenanceWindow struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Recurring   bool      `json:"recurring"`
	Pattern     string    `json:"pattern"` // cron-like pattern
	Description string    `json:"description"`
}

// Recipient defines alert recipients
type Recipient struct {
	Type    RecipientType     `json:"type"`
	Address string            `json:"address"`
	Name    string            `json:"name"`
	Tags    map[string]string `json:"tags"`
}

// RecipientType defines types of alert recipients
type RecipientType string

const (
	RecipientTypeEmail RecipientType = "EMAIL"
	RecipientTypeUser  RecipientType = "USER"
	RecipientTypeTeam  RecipientType = "TEAM"
	RecipientTypeRole  RecipientType = "ROLE"
)

// RemediationConfig defines automatic remediation actions
type RemediationConfig struct {
	Enabled    bool                `json:"enabled"`
	Actions    []RemediationAction `json:"actions"`
	Timeout    time.Duration       `json:"timeout"`
	MaxRetries int                 `json:"max_retries"`
}

// RemediationAction defines an automatic remediation action
type RemediationAction struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        ActionType        `json:"type"`
	Command     string            `json:"command"`
	Parameters  map[string]string `json:"parameters"`
	Conditions  []string          `json:"conditions"`
	Timeout     time.Duration     `json:"timeout"`
	RunOrder    int               `json:"run_order"`
}

// ActionType defines types of remediation actions
type ActionType string

const (
	ActionTypeScript    ActionType = "SCRIPT"
	ActionTypeAPI       ActionType = "API"
	ActionTypeRestart   ActionType = "RESTART"
	ActionTypeScale     ActionType = "SCALE"
	ActionTypeFailover  ActionType = "FAILOVER"
	ActionTypeRollback  ActionType = "ROLLBACK"
)

// ScheduleConfig defines when SLA monitoring is active
type ScheduleConfig struct {
	Enabled       bool             `json:"enabled"`
	TimeZone      string           `json:"timezone"`
	BusinessHours BusinessHours    `json:"business_hours"`
	Exclusions    []TimeExclusion  `json:"exclusions"`
}

// BusinessHours defines business hours for SLA monitoring
type BusinessHours struct {
	Monday    DaySchedule `json:"monday"`
	Tuesday   DaySchedule `json:"tuesday"`
	Wednesday DaySchedule `json:"wednesday"`
	Thursday  DaySchedule `json:"thursday"`
	Friday    DaySchedule `json:"friday"`
	Saturday  DaySchedule `json:"saturday"`
	Sunday    DaySchedule `json:"sunday"`
}

// DaySchedule defines hours for a specific day
type DaySchedule struct {
	Enabled bool   `json:"enabled"`
	Start   string `json:"start"` // HH:MM format
	End     string `json:"end"`   // HH:MM format
}

// TimeExclusion defines time periods to exclude from SLA monitoring
type TimeExclusion struct {
	Name        string    `json:"name"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Recurring   bool      `json:"recurring"`
	Pattern     string    `json:"pattern"`
	Description string    `json:"description"`
}

// ResponseTimeMetrics holds response time statistics
type ResponseTimeMetrics struct {
	Average    time.Duration `json:"average"`
	P50        time.Duration `json:"p50"`
	P95        time.Duration `json:"p95"`
	P99        time.Duration `json:"p99"`
	Min        time.Duration `json:"min"`
	Max        time.Duration `json:"max"`
	LastUpdate time.Time     `json:"last_update"`
}

// Violation represents an SLA violation
type Violation struct {
	ID           string            `json:"id"`
	SLAID        string            `json:"sla_id"`
	TargetID     string            `json:"target_id"`
	Timestamp    time.Time         `json:"timestamp"`
	Duration     time.Duration     `json:"duration"`
	Severity     Severity          `json:"severity"`
	Status       ViolationStatus   `json:"status"`
	ActualValue  float64           `json:"actual_value"`
	ExpectedValue float64          `json:"expected_value"`
	Impact       ImpactAssessment  `json:"impact"`
	Remediation  RemediationResult `json:"remediation"`
	Alerts       []AlertRecord     `json:"alerts"`
	Resolution   Resolution        `json:"resolution"`
	Tags         map[string]string `json:"tags"`
}

// ViolationStatus represents the status of a violation
type ViolationStatus string

const (
	ViolationStatusActive    ViolationStatus = "ACTIVE"
	ViolationStatusResolved  ViolationStatus = "RESOLVED"
	ViolationStatusSuppressed ViolationStatus = "SUPPRESSED"
)

// ImpactAssessment assesses the impact of a violation
type ImpactAssessment struct {
	BusinessImpact    BusinessImpact `json:"business_impact"`
	AffectedUsers     int64          `json:"affected_users"`
	AffectedServices  []string       `json:"affected_services"`
	EstimatedCost     float64        `json:"estimated_cost"`
	RiskLevel         RiskLevel      `json:"risk_level"`
}

// BusinessImpact defines business impact levels
type BusinessImpact string

const (
	BusinessImpactLow      BusinessImpact = "LOW"
	BusinessImpactMedium   BusinessImpact = "MEDIUM"
	BusinessImpactHigh     BusinessImpact = "HIGH"
	BusinessImpactCritical BusinessImpact = "CRITICAL"
)

// RiskLevel defines risk levels
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "LOW"
	RiskLevelMedium   RiskLevel = "MEDIUM"
	RiskLevelHigh     RiskLevel = "HIGH"
	RiskLevelCritical RiskLevel = "CRITICAL"
)

// RemediationResult holds the result of remediation actions
type RemediationResult struct {
	Attempted    bool               `json:"attempted"`
	Successful   bool               `json:"successful"`
	Actions      []ActionResult     `json:"actions"`
	StartTime    time.Time          `json:"start_time"`
	EndTime      time.Time          `json:"end_time"`
	ErrorMessage string             `json:"error_message,omitempty"`
}

// ActionResult holds the result of a remediation action
type ActionResult struct {
	ActionID     string        `json:"action_id"`
	ActionName   string        `json:"action_name"`
	Successful   bool          `json:"successful"`
	StartTime    time.Time     `json:"start_time"`
	Duration     time.Duration `json:"duration"`
	Output       string        `json:"output"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// AlertRecord records when alerts were sent
type AlertRecord struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Channel   ChannelType `json:"channel"`
	Recipient string      `json:"recipient"`
	Successful bool       `json:"successful"`
	Message   string      `json:"message"`
}

// Resolution records how a violation was resolved
type Resolution struct {
	ResolvedAt   time.Time `json:"resolved_at"`
	ResolvedBy   string    `json:"resolved_by"`
	Method       string    `json:"method"` // automatic, manual, timeout
	Description  string    `json:"description"`
	RootCause    string    `json:"root_cause"`
	PreventionSteps []string `json:"prevention_steps"`
}

// SLAMetrics holds comprehensive SLA monitoring metrics
type SLAMetrics struct {
	TotalSLAs          int                    `json:"total_slas"`
	ActiveSLAs         int                    `json:"active_slas"`
	HealthySLAs        int                    `json:"healthy_slas"`
	ViolatingSLAs      int                    `json:"violating_slas"`
	TotalViolations    int64                  `json:"total_violations"`
	ActiveViolations   int                    `json:"active_violations"`
	MTTR               time.Duration          `json:"mttr"` // Mean Time To Recovery
	MTBF               time.Duration          `json:"mtbf"` // Mean Time Between Failures
	OverallUptime      float64                `json:"overall_uptime"`
	ServiceMetrics     map[string]ServiceSLA  `json:"service_metrics"`
	ViolationsByType   map[SLAType]int        `json:"violations_by_type"`
	ViolationTrends    []ViolationTrend       `json:"violation_trends"`
	LastUpdated        time.Time              `json:"last_updated"`
	mutex              sync.RWMutex
}

// ServiceSLA holds SLA metrics for a specific service
type ServiceSLA struct {
	ServiceName     string        `json:"service_name"`
	UptimePercent   float64       `json:"uptime_percent"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	ErrorRate       float64       `json:"error_rate"`
	SLACount        int           `json:"sla_count"`
	ViolationCount  int           `json:"violation_count"`
	LastViolation   time.Time     `json:"last_violation"`
	Status          SLAStatus     `json:"status"`
}

// ViolationTrend tracks violation trends over time
type ViolationTrend struct {
	Timestamp   time.Time `json:"timestamp"`
	Violations  int       `json:"violations"`
	Period      string    `json:"period"` // hourly, daily, weekly
}

// AlertManager interface for sending alerts
type AlertManager interface {
	SendAlert(alert SLAAlert) error
	GetActiveAlerts() ([]SLAAlert, error)
	AcknowledgeAlert(alertID, userID string) error
}

// SLAAlert represents an SLA-related alert
type SLAAlert struct {
	ID          string            `json:"id"`
	Type        AlertType         `json:"type"`
	Severity    Severity          `json:"severity"`
	SLAID       string            `json:"sla_id"`
	ViolationID string            `json:"violation_id"`
	Title       string            `json:"title"`
	Message     string            `json:"message"`
	Timestamp   time.Time         `json:"timestamp"`
	Recipients  []Recipient       `json:"recipients"`
	Channels    []ChannelType     `json:"channels"`
	Tags        map[string]string `json:"tags"`
	Status      AlertStatus       `json:"status"`
}

// AlertType defines types of SLA alerts
type AlertType string

const (
	AlertTypeViolation AlertType = "VIOLATION"
	AlertTypeWarning   AlertType = "WARNING"
	AlertTypeRecovery  AlertType = "RECOVERY"
	AlertTypeFlapping  AlertType = "FLAPPING"
)

// AlertStatus defines alert status
type AlertStatus string

const (
	AlertStatusActive      AlertStatus = "ACTIVE"
	AlertStatusAcknowledged AlertStatus = "ACKNOWLEDGED"
	AlertStatusResolved    AlertStatus = "RESOLVED"
)

// SLAStorage interface for storing SLA data
type SLAStorage interface {
	SaveSLA(sla *SLA) error
	LoadSLA(id string) (*SLA, error)
	ListSLAs() ([]*SLA, error)
	DeleteSLA(id string) error
	SaveViolation(violation *Violation) error
	LoadViolation(id string) (*Violation, error)
	ListViolations(filters map[string]interface{}) ([]*Violation, error)
	GetSLAMetrics(timeRange time.Duration) (*SLAMetrics, error)
	Cleanup(retentionPeriod time.Duration) error
}

// NewSLAMonitor creates a new SLA monitor
func NewSLAMonitor(config *SLAConfig, alertManager AlertManager, storage SLAStorage) (*SLAMonitor, error) {
	if config == nil {
		config = DefaultSLAConfig()
	}

	return &SLAMonitor{
		config:       config,
		slas:         make(map[string]*SLA),
		violations:   make(map[string]*Violation),
		metrics:      NewSLAMetrics(),
		alertManager: alertManager,
		storage:      storage,
		stopCh:       make(chan struct{}),
	}, nil
}

// NewSLAMetrics creates new SLA metrics instance
func NewSLAMetrics() *SLAMetrics {
	return &SLAMetrics{
		ServiceMetrics:     make(map[string]ServiceSLA),
		ViolationsByType:   make(map[SLAType]int),
		ViolationTrends:    make([]ViolationTrend, 0),
		LastUpdated:        time.Now(),
	}
}

// Start starts the SLA monitor
func (s *SLAMonitor) Start(ctx context.Context) error {
	s.mutex.Lock()
	if s.running {
		s.mutex.Unlock()
		return fmt.Errorf("SLA monitor already running")
	}
	s.running = true
	s.mutex.Unlock()

	// Load existing SLAs
	if err := s.loadSLAs(); err != nil {
		return fmt.Errorf("failed to load SLAs: %w", err)
	}

	// Start monitoring goroutine
	go s.monitorLoop(ctx)

	fmt.Println("SLA Monitor started successfully")
	return nil
}

// Stop stops the SLA monitor
func (s *SLAMonitor) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		close(s.stopCh)
		s.running = false
	}

	fmt.Println("SLA Monitor stopped")
	return nil
}

// loadSLAs loads SLAs from storage
func (s *SLAMonitor) loadSLAs() error {
	slas, err := s.storage.ListSLAs()
	if err != nil {
		// If no SLAs exist, create default ones
		s.createDefaultSLAs()
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, sla := range slas {
		s.slas[sla.ID] = sla
	}

	fmt.Printf("Loaded %d SLAs\n", len(slas))
	return nil
}

// createDefaultSLAs creates default SLA configurations
func (s *SLAMonitor) createDefaultSLAs() {
	defaultSLAs := []*SLA{
		{
			ID:          "s3-availability",
			Name:        "S3 Service Availability",
			Description: "S3 service must be available 99.9% of the time",
			Service:     "s3",
			Category:    SLACategoryAvailability,
			Type:        SLATypeUptime,
			Targets: []SLATarget{
				{
					ID:        "uptime-target",
					Name:      "Uptime 99.9%",
					Metric:    "uptime_percentage",
					Threshold: 99.9,
					Operator:  OperatorGreaterEqual,
					Unit:      "percent",
					Window:    TimeWindow{Duration: time.Hour * 24, Type: WindowTypeRolling},
					Severity:  SeverityCritical,
					Enabled:   true,
				},
			},
			Measurement: MeasurementConfig{
				DataSource:  "system_metrics",
				Aggregation: AggregationAverage,
				SampleRate:  1.0,
			},
			Alerting: AlertingConfig{
				Enabled: true,
				Channels: []AlertChannel{
					{
						Type:     ChannelTypeEmail,
						Enabled:  true,
						Severity: []Severity{SeverityCritical, SeverityError},
					},
				},
			},
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "s3-response-time",
			Name:        "S3 Response Time",
			Description: "S3 operations must respond within 2 seconds on average",
			Service:     "s3",
			Category:    SLACategoryPerformance,
			Type:        SLATypeResponseTime,
			Targets: []SLATarget{
				{
					ID:        "response-time-target",
					Name:      "Response Time < 2s",
					Metric:    "avg_response_time",
					Threshold: 2000, // milliseconds
					Operator:  OperatorLessEqual,
					Unit:      "ms",
					Window:    TimeWindow{Duration: time.Hour, Type: WindowTypeRolling},
					Severity:  SeverityWarning,
					Enabled:   true,
				},
			},
			Measurement: MeasurementConfig{
				DataSource:  "performance_metrics",
				Aggregation: AggregationAverage,
				SampleRate:  1.0,
			},
			Alerting: AlertingConfig{
				Enabled: true,
				Channels: []AlertChannel{
					{
						Type:     ChannelTypeEmail,
						Enabled:  true,
						Severity: []Severity{SeverityWarning, SeverityError},
					},
				},
			},
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "system-error-rate",
			Name:        "System Error Rate",
			Description: "System error rate must be below 1%",
			Service:     "system",
			Category:    SLACategoryReliability,
			Type:        SLATypeErrorRate,
			Targets: []SLATarget{
				{
					ID:        "error-rate-target",
					Name:      "Error Rate < 1%",
					Metric:    "error_rate",
					Threshold: 1.0,
					Operator:  OperatorLessEqual,
					Unit:      "percent",
					Window:    TimeWindow{Duration: time.Hour, Type: WindowTypeRolling},
					Severity:  SeverityError,
					Enabled:   true,
				},
			},
			Measurement: MeasurementConfig{
				DataSource:  "error_metrics",
				Aggregation: AggregationAverage,
				SampleRate:  1.0,
			},
			Alerting: AlertingConfig{
				Enabled: true,
				Channels: []AlertChannel{
					{
						Type:     ChannelTypeEmail,
						Enabled:  true,
						Severity: []Severity{SeverityError, SeverityCritical},
					},
				},
			},
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, sla := range defaultSLAs {
		s.CreateSLA(sla)
	}

	fmt.Printf("Created %d default SLAs\n", len(defaultSLAs))
}

// monitorLoop runs the main monitoring loop
func (s *SLAMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.performSLAChecks()
		}
	}
}

// performSLAChecks performs SLA compliance checks
func (s *SLAMonitor) performSLAChecks() {
	s.mutex.RLock()
	slas := make([]*SLA, 0, len(s.slas))
	for _, sla := range s.slas {
		if sla.Enabled {
			slas = append(slas, sla)
		}
	}
	s.mutex.RUnlock()

	for _, sla := range slas {
		s.checkSLA(sla)
	}

	s.updateMetrics()
}

// checkSLA checks a single SLA for compliance
func (s *SLAMonitor) checkSLA(sla *SLA) {
	// Update last checked time
	sla.LastChecked = time.Now()

	for _, target := range sla.Targets {
		if !target.Enabled {
			continue
		}

		// Get current metric value
		value, err := s.getCurrentMetricValue(sla, target)
		if err != nil {
			fmt.Printf("Failed to get metric value for SLA %s: %v\n", sla.ID, err)
			continue
		}

		// Check if target is violated
		violated := s.isTargetViolated(target, value)

		if violated {
			s.handleViolation(sla, target, value)
		} else {
			s.handleCompliance(sla, target, value)
		}
	}

	// Update SLA status
	s.updateSLAStatus(sla)

	// Save updated SLA
	if err := s.storage.SaveSLA(sla); err != nil {
		fmt.Printf("Failed to save SLA %s: %v\n", sla.ID, err)
	}
}

// getCurrentMetricValue gets the current value for a metric
func (s *SLAMonitor) getCurrentMetricValue(sla *SLA, target SLATarget) (float64, error) {
	// Simulate metric collection based on metric type
	switch target.Metric {
	case "uptime_percentage":
		// Simulate uptime calculation (normally would query actual metrics)
		return 99.95, nil
	case "avg_response_time":
		// Simulate response time (in milliseconds)
		return 1500.0, nil
	case "error_rate":
		// Simulate error rate percentage
		return 0.5, nil
	default:
		return 0, fmt.Errorf("unknown metric: %s", target.Metric)
	}
}

// isTargetViolated checks if a target is violated
func (s *SLAMonitor) isTargetViolated(target SLATarget, value float64) bool {
	switch target.Operator {
	case OperatorGreaterEqual:
		return value < target.Threshold
	case OperatorLessEqual:
		return value > target.Threshold
	case OperatorEqual:
		return value != target.Threshold
	case OperatorNotEqual:
		return value == target.Threshold
	case OperatorGreater:
		return value <= target.Threshold
	case OperatorLess:
		return value >= target.Threshold
	default:
		return false
	}
}

// handleViolation handles an SLA violation
func (s *SLAMonitor) handleViolation(sla *SLA, target SLATarget, value float64) {
	// Check if this is a new violation or continuation
	violationID := fmt.Sprintf("%s_%s_%d", sla.ID, target.ID, time.Now().Unix())

	violation := &Violation{
		ID:            violationID,
		SLAID:         sla.ID,
		TargetID:      target.ID,
		Timestamp:     time.Now(),
		Severity:      target.Severity,
		Status:        ViolationStatusActive,
		ActualValue:   value,
		ExpectedValue: target.Threshold,
		Impact: ImpactAssessment{
			BusinessImpact: s.assessBusinessImpact(target.Severity),
			RiskLevel:      s.assessRiskLevel(target.Severity),
		},
		Tags: sla.Tags,
	}

	// Store violation
	s.mutex.Lock()
	s.violations[violationID] = violation
	sla.ViolationCount++
	sla.CurrentStatus = SLAStatusViolation
	s.mutex.Unlock()

	// Save violation to storage
	if err := s.storage.SaveViolation(violation); err != nil {
		fmt.Printf("Failed to save violation: %v\n", err)
	}

	// Send alerts
	if sla.Alerting.Enabled {
		s.sendViolationAlert(sla, violation)
	}

	// Attempt remediation if enabled
	if sla.Remediation.Enabled && s.config.AutoRemediation {
		s.attemptRemediation(sla, violation)
	}

	fmt.Printf("SLA Violation: %s - %s (Expected: %f, Actual: %f)\n",
		sla.Name, target.Name, target.Threshold, value)
}

// handleCompliance handles when an SLA is compliant
func (s *SLAMonitor) handleCompliance(sla *SLA, target SLATarget, value float64) {
	// Check if there was a previous violation that's now resolved
	for _, violation := range s.violations {
		if violation.SLAID == sla.ID && violation.TargetID == target.ID && violation.Status == ViolationStatusActive {
			// Mark violation as resolved
			violation.Status = ViolationStatusResolved
			violation.Resolution = Resolution{
				ResolvedAt:  time.Now(),
				Method:      "automatic",
				Description: "SLA target returned to compliance",
			}

			// Send recovery alert
			if sla.Alerting.Enabled {
				s.sendRecoveryAlert(sla, violation)
			}

			break
		}
	}
}

// updateSLAStatus updates the overall status of an SLA
func (s *SLAMonitor) updateSLAStatus(sla *SLA) {
	hasViolations := false
	hasWarnings := false

	for _, violation := range s.violations {
		if violation.SLAID == sla.ID && violation.Status == ViolationStatusActive {
			if violation.Severity == SeverityCritical || violation.Severity == SeverityError {
				hasViolations = true
			} else if violation.Severity == SeverityWarning {
				hasWarnings = true
			}
		}
	}

	if hasViolations {
		sla.CurrentStatus = SLAStatusViolation
	} else if hasWarnings {
		sla.CurrentStatus = SLAStatusWarning
	} else {
		sla.CurrentStatus = SLAStatusHealthy
	}
}

// assessBusinessImpact assesses the business impact of a violation
func (s *SLAMonitor) assessBusinessImpact(severity Severity) BusinessImpact {
	switch severity {
	case SeverityCritical:
		return BusinessImpactCritical
	case SeverityError:
		return BusinessImpactHigh
	case SeverityWarning:
		return BusinessImpactMedium
	default:
		return BusinessImpactLow
	}
}

// assessRiskLevel assesses the risk level of a violation
func (s *SLAMonitor) assessRiskLevel(severity Severity) RiskLevel {
	switch severity {
	case SeverityCritical:
		return RiskLevelCritical
	case SeverityError:
		return RiskLevelHigh
	case SeverityWarning:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

// sendViolationAlert sends an alert for an SLA violation
func (s *SLAMonitor) sendViolationAlert(sla *SLA, violation *Violation) {
	alert := SLAAlert{
		ID:          fmt.Sprintf("alert_%s_%d", violation.ID, time.Now().Unix()),
		Type:        AlertTypeViolation,
		Severity:    violation.Severity,
		SLAID:       sla.ID,
		ViolationID: violation.ID,
		Title:       fmt.Sprintf("SLA Violation: %s", sla.Name),
		Message:     fmt.Sprintf("SLA %s is violating target. Expected: %f, Actual: %f", sla.Name, violation.ExpectedValue, violation.ActualValue),
		Timestamp:   time.Now(),
		Status:      AlertStatusActive,
		Tags:        sla.Tags,
	}

	if err := s.alertManager.SendAlert(alert); err != nil {
		fmt.Printf("Failed to send violation alert: %v\n", err)
	}
}

// sendRecoveryAlert sends an alert when an SLA recovers
func (s *SLAMonitor) sendRecoveryAlert(sla *SLA, violation *Violation) {
	alert := SLAAlert{
		ID:          fmt.Sprintf("recovery_%s_%d", violation.ID, time.Now().Unix()),
		Type:        AlertTypeRecovery,
		Severity:    SeverityInfo,
		SLAID:       sla.ID,
		ViolationID: violation.ID,
		Title:       fmt.Sprintf("SLA Recovery: %s", sla.Name),
		Message:     fmt.Sprintf("SLA %s has recovered and is now compliant", sla.Name),
		Timestamp:   time.Now(),
		Status:      AlertStatusActive,
		Tags:        sla.Tags,
	}

	if err := s.alertManager.SendAlert(alert); err != nil {
		fmt.Printf("Failed to send recovery alert: %v\n", err)
	}
}

// attemptRemediation attempts automatic remediation for a violation
func (s *SLAMonitor) attemptRemediation(sla *SLA, violation *Violation) {
	if !sla.Remediation.Enabled {
		return
	}

	result := RemediationResult{
		Attempted: true,
		StartTime: time.Now(),
		Actions:   make([]ActionResult, 0),
	}

	// Sort actions by run order
	actions := sla.Remediation.Actions
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].RunOrder < actions[j].RunOrder
	})

	// Execute remediation actions
	for _, action := range actions {
		actionResult := s.executeRemediationAction(action)
		result.Actions = append(result.Actions, actionResult)

		if !actionResult.Successful {
			result.Successful = false
			result.ErrorMessage = actionResult.ErrorMessage
			break
		}
	}

	result.EndTime = time.Now()
	if result.ErrorMessage == "" {
		result.Successful = true
	}

	violation.Remediation = result

	fmt.Printf("Remediation attempted for violation %s: Success=%t\n", violation.ID, result.Successful)
}

// executeRemediationAction executes a single remediation action
func (s *SLAMonitor) executeRemediationAction(action RemediationAction) ActionResult {
	result := ActionResult{
		ActionID:   action.ID,
		ActionName: action.Name,
		StartTime:  time.Now(),
	}

	// Simulate action execution
	switch action.Type {
	case ActionTypeRestart:
		result.Output = "Service restart initiated"
		result.Successful = true
	case ActionTypeScale:
		result.Output = "Scaling operation completed"
		result.Successful = true
	case ActionTypeScript:
		result.Output = "Custom script executed"
		result.Successful = true
	default:
		result.ErrorMessage = fmt.Sprintf("Unknown action type: %s", action.Type)
		result.Successful = false
	}

	result.Duration = time.Since(result.StartTime)
	return result
}

// updateMetrics updates SLA monitoring metrics
func (s *SLAMonitor) updateMetrics() {
	s.metrics.mutex.Lock()
	defer s.metrics.mutex.Unlock()

	s.metrics.TotalSLAs = len(s.slas)
	s.metrics.ActiveSLAs = 0
	s.metrics.HealthySLAs = 0
	s.metrics.ViolatingSLAs = 0

	serviceMetrics := make(map[string]ServiceSLA)

	for _, sla := range s.slas {
		if sla.Enabled {
			s.metrics.ActiveSLAs++

			switch sla.CurrentStatus {
			case SLAStatusHealthy:
				s.metrics.HealthySLAs++
			case SLAStatusViolation:
				s.metrics.ViolatingSLAs++
			}

			// Update service metrics
			serviceName := sla.Service
			if _, exists := serviceMetrics[serviceName]; !exists {
				serviceMetrics[serviceName] = ServiceSLA{
					ServiceName: serviceName,
				}
			}

			service := serviceMetrics[serviceName]
			service.SLACount++
			if sla.CurrentStatus == SLAStatusViolation {
				service.ViolationCount++
			}
			serviceMetrics[serviceName] = service
		}
	}

	// Count active violations
	activeViolations := 0
	for _, violation := range s.violations {
		if violation.Status == ViolationStatusActive {
			activeViolations++
		}
	}
	s.metrics.ActiveViolations = activeViolations

	s.metrics.ServiceMetrics = serviceMetrics
	s.metrics.LastUpdated = time.Now()
}

// CreateSLA creates a new SLA
func (s *SLAMonitor) CreateSLA(sla *SLA) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if sla.ID == "" {
		sla.ID = fmt.Sprintf("sla_%d", time.Now().UnixNano())
	}

	sla.CreatedAt = time.Now()
	sla.UpdatedAt = time.Now()
	sla.CurrentStatus = SLAStatusHealthy

	s.slas[sla.ID] = sla

	// Save to storage
	if err := s.storage.SaveSLA(sla); err != nil {
		return fmt.Errorf("failed to save SLA: %w", err)
	}

	fmt.Printf("Created SLA: %s\n", sla.Name)
	return nil
}

// GetSLA retrieves an SLA by ID
func (s *SLAMonitor) GetSLA(id string) (*SLA, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	sla, exists := s.slas[id]
	if !exists {
		return nil, fmt.Errorf("SLA %s not found", id)
	}

	return sla, nil
}

// ListSLAs returns all SLAs
func (s *SLAMonitor) ListSLAs() []*SLA {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	slas := make([]*SLA, 0, len(s.slas))
	for _, sla := range s.slas {
		slas = append(slas, sla)
	}

	return slas
}

// GetMetrics returns current SLA metrics
func (s *SLAMonitor) GetMetrics() *SLAMetrics {
	s.metrics.mutex.RLock()
	defer s.metrics.mutex.RUnlock()

	// Return a copy to prevent external modification
	metrics := *s.metrics
	return &metrics
}

// GetViolations returns violations with optional filters
func (s *SLAMonitor) GetViolations(filters map[string]interface{}) []*Violation {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	violations := make([]*Violation, 0)
	for _, violation := range s.violations {
		if s.matchesFilters(violation, filters) {
			violations = append(violations, violation)
		}
	}

	return violations
}

// matchesFilters checks if a violation matches the given filters
func (s *SLAMonitor) matchesFilters(violation *Violation, filters map[string]interface{}) bool {
	if filters == nil {
		return true
	}

	if slaID, exists := filters["sla_id"]; exists {
		if violation.SLAID != slaID.(string) {
			return false
		}
	}

	if status, exists := filters["status"]; exists {
		if violation.Status != ViolationStatus(status.(string)) {
			return false
		}
	}

	if severity, exists := filters["severity"]; exists {
		if violation.Severity != Severity(severity.(string)) {
			return false
		}
	}

	return true
}

// FileSLAStorage implements SLAStorage using files
type FileSLAStorage struct {
	configDir string
}

// NewFileSLAStorage creates a file-based SLA storage
func NewFileSLAStorage(configDir string) *FileSLAStorage {
	os.MkdirAll(configDir, 0755)
	os.MkdirAll(filepath.Join(configDir, "slas"), 0755)
	os.MkdirAll(filepath.Join(configDir, "violations"), 0755)
	return &FileSLAStorage{configDir: configDir}
}

// SaveSLA saves an SLA to file
func (f *FileSLAStorage) SaveSLA(sla *SLA) error {
	data, err := json.MarshalIndent(sla, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal SLA: %w", err)
	}

	filename := filepath.Join(f.configDir, "slas", fmt.Sprintf("%s.json", sla.ID))
	return os.WriteFile(filename, data, 0644)
}

// LoadSLA loads an SLA from file
func (f *FileSLAStorage) LoadSLA(id string) (*SLA, error) {
	filename := filepath.Join(f.configDir, "slas", fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read SLA file: %w", err)
	}

	var sla SLA
	if err := json.Unmarshal(data, &sla); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SLA: %w", err)
	}

	return &sla, nil
}

// ListSLAs lists all SLAs
func (f *FileSLAStorage) ListSLAs() ([]*SLA, error) {
	slaDir := filepath.Join(f.configDir, "slas")
	entries, err := os.ReadDir(slaDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*SLA{}, nil
		}
		return nil, fmt.Errorf("failed to read SLA directory: %w", err)
	}

	var slas []*SLA
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		sla, err := f.LoadSLA(id)
		if err != nil {
			fmt.Printf("Failed to load SLA %s: %v\n", id, err)
			continue
		}

		slas = append(slas, sla)
	}

	return slas, nil
}

// DeleteSLA deletes an SLA
func (f *FileSLAStorage) DeleteSLA(id string) error {
	filename := filepath.Join(f.configDir, "slas", fmt.Sprintf("%s.json", id))
	return os.Remove(filename)
}

// SaveViolation saves a violation to file
func (f *FileSLAStorage) SaveViolation(violation *Violation) error {
	data, err := json.MarshalIndent(violation, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal violation: %w", err)
	}

	filename := filepath.Join(f.configDir, "violations", fmt.Sprintf("%s.json", violation.ID))
	return os.WriteFile(filename, data, 0644)
}

// LoadViolation loads a violation from file
func (f *FileSLAStorage) LoadViolation(id string) (*Violation, error) {
	filename := filepath.Join(f.configDir, "violations", fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read violation file: %w", err)
	}

	var violation Violation
	if err := json.Unmarshal(data, &violation); err != nil {
		return nil, fmt.Errorf("failed to unmarshal violation: %w", err)
	}

	return &violation, nil
}

// ListViolations lists violations with filters
func (f *FileSLAStorage) ListViolations(filters map[string]interface{}) ([]*Violation, error) {
	violationDir := filepath.Join(f.configDir, "violations")
	entries, err := os.ReadDir(violationDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Violation{}, nil
		}
		return nil, fmt.Errorf("failed to read violation directory: %w", err)
	}

	var violations []*Violation
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		violation, err := f.LoadViolation(id)
		if err != nil {
			fmt.Printf("Failed to load violation %s: %v\n", id, err)
			continue
		}

		violations = append(violations, violation)
	}

	return violations, nil
}

// GetSLAMetrics returns SLA metrics for a time range
func (f *FileSLAStorage) GetSLAMetrics(timeRange time.Duration) (*SLAMetrics, error) {
	// In a real implementation, this would aggregate metrics from stored data
	return NewSLAMetrics(), nil
}

// Cleanup removes old data based on retention period
func (f *FileSLAStorage) Cleanup(retentionPeriod time.Duration) error {
	// Clean up old violations
	violationDir := filepath.Join(f.configDir, "violations")
	entries, err := os.ReadDir(violationDir)
	if err != nil {
		return nil // Directory might not exist
	}

	cutoff := time.Now().Add(-retentionPeriod)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(violationDir, entry.Name()))
		}
	}

	return nil
}