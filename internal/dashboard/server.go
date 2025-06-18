package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// DashboardServer provides a centralized management dashboard
type DashboardServer struct {
	config        *DashboardConfig
	server        *http.Server
	metrics       *SystemMetrics
	templates     *template.Template
	websockets    map[string]*WebSocketConnection
	wsLock        sync.RWMutex
	alertManager  AlertManager
	systemMonitor *SystemMonitor
}

// DashboardConfig holds dashboard configuration
type DashboardConfig struct {
	Enabled         bool          `json:"enabled"`
	Port            int           `json:"port"`
	Host            string        `json:"host"`
	TLSEnabled      bool          `json:"tls_enabled"`
	CertFile        string        `json:"cert_file"`
	KeyFile         string        `json:"key_file"`
	RefreshInterval time.Duration `json:"refresh_interval"`
	MaxConnections  int           `json:"max_connections"`
	AuthRequired    bool          `json:"auth_required"`
	AdminUsers      []string      `json:"admin_users"`
}

// DefaultDashboardConfig returns default dashboard configuration
func DefaultDashboardConfig() *DashboardConfig {
	return &DashboardConfig{
		Enabled:         true,
		Port:            8080,
		Host:            "localhost",
		TLSEnabled:      false,
		RefreshInterval: time.Second * 30,
		MaxConnections:  100,
		AuthRequired:    true,
		AdminUsers:      []string{"admin"},
	}
}

// SystemMetrics holds comprehensive system metrics
type SystemMetrics struct {
	// Performance Metrics
	RequestsPerSecond float64       `json:"requests_per_second"`
	AverageLatency    time.Duration `json:"average_latency"`
	ErrorRate         float64       `json:"error_rate"`
	Throughput        float64       `json:"throughput"`

	// Resource Metrics
	CPUUsage    float64      `json:"cpu_usage"`
	MemoryUsage float64      `json:"memory_usage"`
	DiskUsage   float64      `json:"disk_usage"`
	NetworkIO   NetworkStats `json:"network_io"`

	// S3 Metrics
	S3Operations S3OperationStats `json:"s3_operations"`
	S3Errors     map[string]int64 `json:"s3_errors"`

	// Security Metrics
	AuthenticationStats AuthStats       `json:"authentication_stats"`
	SecurityAlerts      []SecurityAlert `json:"security_alerts"`

	// System Health
	ServiceStatus       map[string]string `json:"service_status"`
	DatabaseConnections int               `json:"database_connections"`
	QueueSizes          map[string]int    `json:"queue_sizes"`

	// Business Metrics
	ActiveUsers     int64         `json:"active_users"`
	DataTransferred int64         `json:"data_transferred"`
	CostMetrics     CostBreakdown `json:"cost_metrics"`

	LastUpdated time.Time `json:"last_updated"`
	mutex       sync.RWMutex
}

// NetworkStats holds network I/O statistics
type NetworkStats struct {
	BytesIn    int64 `json:"bytes_in"`
	BytesOut   int64 `json:"bytes_out"`
	PacketsIn  int64 `json:"packets_in"`
	PacketsOut int64 `json:"packets_out"`
}

// S3OperationStats holds S3 operation statistics
type S3OperationStats struct {
	Downloads int64 `json:"downloads"`
	Uploads   int64 `json:"uploads"`
	Deletes   int64 `json:"deletes"`
	Lists     int64 `json:"lists"`
}

// AuthStats holds authentication statistics
type AuthStats struct {
	SuccessfulLogins int64 `json:"successful_logins"`
	FailedLogins     int64 `json:"failed_logins"`
	ActiveSessions   int64 `json:"active_sessions"`
	MFAEnabled       int64 `json:"mfa_enabled"`
}

// SecurityAlert represents a security alert
type SecurityAlert struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	UserID    string    `json:"user_id,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	Resolved  bool      `json:"resolved"`
}

// CostBreakdown holds cost analysis data
type CostBreakdown struct {
	S3Storage    float64 `json:"s3_storage"`
	S3Requests   float64 `json:"s3_requests"`
	DataTransfer float64 `json:"data_transfer"`
	Compute      float64 `json:"compute"`
	TotalCost    float64 `json:"total_cost"`
	Currency     string  `json:"currency"`
}

// WebSocketConnection represents a WebSocket connection
type WebSocketConnection struct {
	ID       string
	UserID   string
	LastPing time.Time
	// In a real implementation, this would include the actual WebSocket connection
}

// AlertManager interface for managing alerts
type AlertManager interface {
	GetActiveAlerts() []Alert
	AcknowledgeAlert(alertID string, userID string) error
	CreateAlert(alert Alert) error
}

// Alert represents a system alert
type Alert struct {
	ID          string            `json:"id"`
	Timestamp   time.Time         `json:"timestamp"`
	Severity    AlertSeverity     `json:"severity"`
	Category    string            `json:"category"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Source      string            `json:"source"`
	Tags        map[string]string `json:"tags"`
	Status      AlertStatus       `json:"status"`
	AckedBy     string            `json:"acked_by,omitempty"`
	AckedAt     time.Time         `json:"acked_at,omitempty"`
}

// AlertSeverity represents the severity of an alert
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "INFO"
	AlertSeverityWarning  AlertSeverity = "WARNING"
	AlertSeverityError    AlertSeverity = "ERROR"
	AlertSeverityCritical AlertSeverity = "CRITICAL"
)

// AlertStatus represents the status of an alert
type AlertStatus string

const (
	AlertStatusActive       AlertStatus = "ACTIVE"
	AlertStatusAcknowledged AlertStatus = "ACKNOWLEDGED"
	AlertStatusResolved     AlertStatus = "RESOLVED"
)

// SystemMonitor monitors system health and performance
type SystemMonitor struct {
	config  *MonitorConfig
	metrics *SystemMetrics
	stopCh  chan struct{}
	running bool
	mutex   sync.RWMutex
}

// MonitorConfig holds monitoring configuration
type MonitorConfig struct {
	CollectionInterval time.Duration      `json:"collection_interval"`
	RetentionPeriod    time.Duration      `json:"retention_period"`
	AlertThresholds    map[string]float64 `json:"alert_thresholds"`
}

// NewDashboardServer creates a new dashboard server
func NewDashboardServer(config *DashboardConfig) (*DashboardServer, error) {
	if config == nil {
		config = DefaultDashboardConfig()
	}

	server := &DashboardServer{
		config:        config,
		metrics:       NewSystemMetrics(),
		websockets:    make(map[string]*WebSocketConnection),
		alertManager:  NewSimpleAlertManager(),
		systemMonitor: NewSystemMonitor(),
	}

	// Load HTML templates
	if err := server.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	server.setupRoutes(mux)

	server.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler: mux,
	}

	return server, nil
}

// NewSystemMetrics creates a new system metrics instance
func NewSystemMetrics() *SystemMetrics {
	return &SystemMetrics{
		S3Errors:       make(map[string]int64),
		ServiceStatus:  make(map[string]string),
		QueueSizes:     make(map[string]int),
		SecurityAlerts: make([]SecurityAlert, 0),
		LastUpdated:    time.Now(),
	}
}

// NewSystemMonitor creates a new system monitor
func NewSystemMonitor() *SystemMonitor {
	return &SystemMonitor{
		config: &MonitorConfig{
			CollectionInterval: time.Second * 30,
			RetentionPeriod:    time.Hour * 24,
			AlertThresholds: map[string]float64{
				"cpu_usage":    80.0,
				"memory_usage": 85.0,
				"disk_usage":   90.0,
				"error_rate":   5.0,
			},
		},
		metrics: NewSystemMetrics(),
		stopCh:  make(chan struct{}),
	}
}

// NewSimpleAlertManager creates a simple alert manager
func NewSimpleAlertManager() AlertManager {
	return &SimpleAlertManager{
		alerts: make(map[string]Alert),
	}
}

// SimpleAlertManager implements AlertManager interface
type SimpleAlertManager struct {
	alerts map[string]Alert
	mutex  sync.RWMutex
}

func (s *SimpleAlertManager) GetActiveAlerts() []Alert {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var activeAlerts []Alert
	for _, alert := range s.alerts {
		if alert.Status == AlertStatusActive {
			activeAlerts = append(activeAlerts, alert)
		}
	}
	return activeAlerts
}

func (s *SimpleAlertManager) AcknowledgeAlert(alertID string, userID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	alert, exists := s.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert %s not found", alertID)
	}

	alert.Status = AlertStatusAcknowledged
	alert.AckedBy = userID
	alert.AckedAt = time.Now()
	s.alerts[alertID] = alert

	return nil
}

func (s *SimpleAlertManager) CreateAlert(alert Alert) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert_%d", time.Now().UnixNano())
	}
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}
	if alert.Status == "" {
		alert.Status = AlertStatusActive
	}

	s.alerts[alert.ID] = alert
	return nil
}

// Start starts the dashboard server
func (d *DashboardServer) Start(ctx context.Context) error {
	if !d.config.Enabled {
		return fmt.Errorf("dashboard is disabled")
	}

	// Start system monitor
	if err := d.systemMonitor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start system monitor: %w", err)
	}

	// Start HTTP server
	go func() {
		var err error
		if d.config.TLSEnabled {
			err = d.server.ListenAndServeTLS(d.config.CertFile, d.config.KeyFile)
		} else {
			err = d.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			fmt.Printf("Dashboard server error: %v\n", err)
		}
	}()

	fmt.Printf("Dashboard server started on %s\n", d.server.Addr)
	return nil
}

// Stop stops the dashboard server
func (d *DashboardServer) Stop(ctx context.Context) error {
	// Stop system monitor
	d.systemMonitor.Stop()

	// Stop HTTP server
	return d.server.Shutdown(ctx)
}

// loadTemplates loads HTML templates
func (d *DashboardServer) loadTemplates() error {
	// In a real implementation, these would be loaded from files
	templateContent := `
<!DOCTYPE html>
<html>
<head>
    <title>S3ry Management Dashboard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background-color: #f5f5f5; }
        .header { background-color: #2c3e50; color: white; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .card { background: white; padding: 20px; border-radius: 5px; box-shadow: 0 2px 5px rgba(0,0,0,0.1); }
        .metric { display: flex; justify-content: space-between; margin: 10px 0; }
        .metric-value { font-weight: bold; color: #2c3e50; }
        .alert { padding: 10px; margin: 5px 0; border-radius: 3px; }
        .alert-critical { background-color: #e74c3c; color: white; }
        .alert-warning { background-color: #f39c12; color: white; }
        .alert-info { background-color: #3498db; color: white; }
        .status-healthy { color: #27ae60; }
        .status-warning { color: #f39c12; }
        .status-error { color: #e74c3c; }
        .refresh-btn { background-color: #3498db; color: white; border: none; padding: 10px 20px; border-radius: 3px; cursor: pointer; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚀 S3ry Enterprise Management Dashboard</h1>
        <p>Real-time system monitoring and management</p>
        <button class="refresh-btn" onclick="location.reload()">🔄 Refresh</button>
    </div>

    <div class="grid">
        <div class="card">
            <h2>📊 Performance Metrics</h2>
            <div class="metric">
                <span>Requests/Second:</span>
                <span class="metric-value">{{.Metrics.RequestsPerSecond}}</span>
            </div>
            <div class="metric">
                <span>Average Latency:</span>
                <span class="metric-value">{{.Metrics.AverageLatency}}</span>
            </div>
            <div class="metric">
                <span>Error Rate:</span>
                <span class="metric-value">{{.Metrics.ErrorRate}}%</span>
            </div>
            <div class="metric">
                <span>Throughput:</span>
                <span class="metric-value">{{.Metrics.Throughput}} MB/s</span>
            </div>
        </div>

        <div class="card">
            <h2>💾 Resource Usage</h2>
            <div class="metric">
                <span>CPU Usage:</span>
                <span class="metric-value">{{.Metrics.CPUUsage}}%</span>
            </div>
            <div class="metric">
                <span>Memory Usage:</span>
                <span class="metric-value">{{.Metrics.MemoryUsage}}%</span>
            </div>
            <div class="metric">
                <span>Disk Usage:</span>
                <span class="metric-value">{{.Metrics.DiskUsage}}%</span>
            </div>
        </div>

        <div class="card">
            <h2>☁️ S3 Operations</h2>
            <div class="metric">
                <span>Downloads:</span>
                <span class="metric-value">{{.Metrics.S3Operations.Downloads}}</span>
            </div>
            <div class="metric">
                <span>Uploads:</span>
                <span class="metric-value">{{.Metrics.S3Operations.Uploads}}</span>
            </div>
            <div class="metric">
                <span>Lists:</span>
                <span class="metric-value">{{.Metrics.S3Operations.Lists}}</span>
            </div>
            <div class="metric">
                <span>Deletes:</span>
                <span class="metric-value">{{.Metrics.S3Operations.Deletes}}</span>
            </div>
        </div>

        <div class="card">
            <h2>🔐 Security Status</h2>
            <div class="metric">
                <span>Active Sessions:</span>
                <span class="metric-value">{{.Metrics.AuthenticationStats.ActiveSessions}}</span>
            </div>
            <div class="metric">
                <span>Failed Logins:</span>
                <span class="metric-value">{{.Metrics.AuthenticationStats.FailedLogins}}</span>
            </div>
            <div class="metric">
                <span>MFA Enabled:</span>
                <span class="metric-value">{{.Metrics.AuthenticationStats.MFAEnabled}}</span>
            </div>
        </div>

        <div class="card">
            <h2>🚨 Active Alerts</h2>
            {{range .Alerts}}
            <div class="alert alert-{{.Severity}}">
                <strong>{{.Title}}</strong><br>
                {{.Description}}<br>
                <small>{{.Timestamp.Format "15:04:05"}}</small>
            </div>
            {{else}}
            <p class="status-healthy">✅ No active alerts</p>
            {{end}}
        </div>

        <div class="card">
            <h2>💰 Cost Analytics</h2>
            <div class="metric">
                <span>S3 Storage:</span>
                <span class="metric-value">${{.Metrics.CostMetrics.S3Storage}}</span>
            </div>
            <div class="metric">
                <span>S3 Requests:</span>
                <span class="metric-value">${{.Metrics.CostMetrics.S3Requests}}</span>
            </div>
            <div class="metric">
                <span>Data Transfer:</span>
                <span class="metric-value">${{.Metrics.CostMetrics.DataTransfer}}</span>
            </div>
            <div class="metric">
                <span><strong>Total Cost:</strong></span>
                <span class="metric-value"><strong>${{.Metrics.CostMetrics.TotalCost}}</strong></span>
            </div>
        </div>
    </div>

    <script>
        // Auto-refresh every 30 seconds
        setTimeout(function() {
            location.reload();
        }, 30000);
    </script>
</body>
</html>
`

	var err error
	d.templates, err = template.New("dashboard").Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	return nil
}

// setupRoutes sets up HTTP routes
func (d *DashboardServer) setupRoutes(mux *http.ServeMux) {
	// Main dashboard
	mux.HandleFunc("/", d.authMiddleware(d.handleDashboard))

	// API endpoints
	mux.HandleFunc("/api/metrics", d.authMiddleware(d.handleAPIMetrics))
	mux.HandleFunc("/api/alerts", d.authMiddleware(d.handleAPIAlerts))
	mux.HandleFunc("/api/health", d.handleAPIHealth)

	// Management endpoints
	mux.HandleFunc("/api/users", d.authMiddleware(d.handleAPIUsers))
	mux.HandleFunc("/api/settings", d.authMiddleware(d.handleAPISettings))
}

// authMiddleware provides authentication middleware
func (d *DashboardServer) authMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.config.AuthRequired {
			handler(w, r)
			return
		}

		// Simple authentication check - in production, use proper auth
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Dashboard"`)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Authentication required"))
			return
		}

		// Validate credentials - simple check for demo
		// In a real implementation, validate against proper authentication system
		if username != "admin" || password != "admin" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Dashboard"`)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Invalid credentials"))
			return
		}

		handler(w, r)
	}
}

// handleDashboard serves the main dashboard
func (d *DashboardServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Metrics *SystemMetrics
		Alerts  []Alert
	}{
		Metrics: d.getMetrics(),
		Alerts:  d.alertManager.GetActiveAlerts(),
	}

	w.Header().Set("Content-Type", "text/html")
	if err := d.templates.Execute(w, data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}

// handleAPIMetrics serves metrics as JSON
func (d *DashboardServer) handleAPIMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d.getMetrics())
}

// handleAPIAlerts serves alerts as JSON
func (d *DashboardServer) handleAPIAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		json.NewEncoder(w).Encode(d.alertManager.GetActiveAlerts())
	case "POST":
		if r.URL.Query().Get("action") == "acknowledge" {
			alertID := r.URL.Query().Get("id")
			userID := r.URL.Query().Get("user")
			if err := d.alertManager.AcknowledgeAlert(alertID, userID); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "acknowledged"})
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAPIHealth serves health check
func (d *DashboardServer) handleAPIHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "2.0.0",
		"services": map[string]string{
			"dashboard":  "healthy",
			"security":   "healthy",
			"monitoring": "healthy",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleAPIUsers serves user management
func (d *DashboardServer) handleAPIUsers(w http.ResponseWriter, r *http.Request) {
	// Placeholder for user management API
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "User management API"})
}

// handleAPISettings serves settings management
func (d *DashboardServer) handleAPISettings(w http.ResponseWriter, r *http.Request) {
	// Placeholder for settings management API
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Settings management API"})
}

// getMetrics returns current system metrics
func (d *DashboardServer) getMetrics() *SystemMetrics {
	return d.systemMonitor.GetCurrentMetrics()
}

// Start starts the system monitor
func (s *SystemMonitor) Start(ctx context.Context) error {
	s.mutex.Lock()
	if s.running {
		s.mutex.Unlock()
		return fmt.Errorf("system monitor already running")
	}
	s.running = true
	s.mutex.Unlock()

	go s.collectMetrics(ctx)
	return nil
}

// Stop stops the system monitor
func (s *SystemMonitor) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

// collectMetrics periodically collects system metrics
func (s *SystemMonitor) collectMetrics(ctx context.Context) {
	ticker := time.NewTicker(s.config.CollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.updateMetrics()
		}
	}
}

// updateMetrics updates system metrics
func (s *SystemMonitor) updateMetrics() {
	s.metrics.mutex.Lock()
	defer s.metrics.mutex.Unlock()

	// Simulate metric collection
	s.metrics.RequestsPerSecond = 150.5 + (rand.Float64()-0.5)*20
	s.metrics.AverageLatency = time.Millisecond * time.Duration(50+(rand.Float64()-0.5)*20)
	s.metrics.ErrorRate = 0.5 + (rand.Float64()-0.5)*1.0
	s.metrics.Throughput = 45.2 + (rand.Float64()-0.5)*10

	s.metrics.CPUUsage = 35.0 + (rand.Float64()-0.5)*20
	s.metrics.MemoryUsage = 60.0 + (rand.Float64()-0.5)*15
	s.metrics.DiskUsage = 45.0 + (rand.Float64()-0.5)*10

	s.metrics.S3Operations.Downloads = s.metrics.S3Operations.Downloads + int64(rand.Intn(10))
	s.metrics.S3Operations.Uploads = s.metrics.S3Operations.Uploads + int64(rand.Intn(5))
	s.metrics.S3Operations.Lists = s.metrics.S3Operations.Lists + int64(rand.Intn(20))

	s.metrics.AuthenticationStats.ActiveSessions = 25 + int64(rand.Intn(10))
	s.metrics.AuthenticationStats.SuccessfulLogins = s.metrics.AuthenticationStats.SuccessfulLogins + int64(rand.Intn(5))

	s.metrics.CostMetrics.S3Storage = 123.45
	s.metrics.CostMetrics.S3Requests = 45.67
	s.metrics.CostMetrics.DataTransfer = 78.90
	s.metrics.CostMetrics.TotalCost = s.metrics.CostMetrics.S3Storage + s.metrics.CostMetrics.S3Requests + s.metrics.CostMetrics.DataTransfer
	s.metrics.CostMetrics.Currency = "USD"

	s.metrics.LastUpdated = time.Now()
}

// GetCurrentMetrics returns current system metrics
func (s *SystemMonitor) GetCurrentMetrics() *SystemMetrics {
	s.metrics.mutex.RLock()
	defer s.metrics.mutex.RUnlock()

	// Return a copy to avoid race conditions
	metrics := *s.metrics
	return &metrics
}
