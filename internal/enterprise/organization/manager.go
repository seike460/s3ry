package organization

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// OrganizationManager manages organizational-level settings and policies
type OrganizationManager struct {
	config       *OrgConfig
	settings     *OrganizationSettings
	policies     map[string]*Policy
	departments  map[string]*Department
	teams        map[string]*Team
	storage      SettingsStorage
	mutex        sync.RWMutex
}

// OrgConfig holds organization manager configuration
type OrgConfig struct {
	ConfigDir         string        `json:"config_dir"`
	AutoSync          bool          `json:"auto_sync"`
	SyncInterval      time.Duration `json:"sync_interval"`
	BackupEnabled     bool          `json:"backup_enabled"`
	BackupInterval    time.Duration `json:"backup_interval"`
	AuditEnabled      bool          `json:"audit_enabled"`
	EnforceCompliance bool          `json:"enforce_compliance"`
}

// OrganizationSettings holds all organizational settings
type OrganizationSettings struct {
	Organization    OrganizationInfo          `json:"organization"`
	GlobalPolicies  map[string]PolicyValue    `json:"global_policies"`
	SecurityConfig  SecuritySettings          `json:"security_config"`
	AccessControl   AccessControlSettings     `json:"access_control"`
	ComplianceReqs  ComplianceRequirements    `json:"compliance_requirements"`
	CostLimits      CostLimitSettings         `json:"cost_limits"`
	UserQuotas      map[string]UserQuota      `json:"user_quotas"`
	TeamQuotas      map[string]TeamQuota      `json:"team_quotas"`
	IntegrationCfg  IntegrationSettings       `json:"integration_settings"`
	NotificationCfg NotificationSettings      `json:"notification_settings"`
	LastUpdated     time.Time                 `json:"last_updated"`
	UpdatedBy       string                    `json:"updated_by"`
	Version         string                    `json:"version"`
}

// OrganizationInfo holds basic organization information
type OrganizationInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Domain      string            `json:"domain"`
	Industry    string            `json:"industry"`
	Size        string            `json:"size"`        // small, medium, large, enterprise
	Region      string            `json:"region"`
	TimeZone    string            `json:"timezone"`
	Currency    string            `json:"currency"`
	Language    string            `json:"language"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Policy represents an organizational policy
type Policy struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Category     PolicyCategory         `json:"category"`
	Type         PolicyType             `json:"type"`
	Scope        PolicyScope            `json:"scope"`
	Rules        []PolicyRule           `json:"rules"`
	Enforcement  EnforcementLevel       `json:"enforcement"`
	Exceptions   []PolicyException      `json:"exceptions"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	CreatedBy    string                 `json:"created_by"`
	Enabled      bool                   `json:"enabled"`
}

// PolicyCategory categorizes policies
type PolicyCategory string

const (
	PolicyCategorySecurity    PolicyCategory = "SECURITY"
	PolicyCategoryCompliance  PolicyCategory = "COMPLIANCE"
	PolicyCategoryAccess      PolicyCategory = "ACCESS"
	PolicyCategoryCost        PolicyCategory = "COST"
	PolicyCategoryUsage       PolicyCategory = "USAGE"
	PolicyCategoryData        PolicyCategory = "DATA"
)

// PolicyType defines the type of policy
type PolicyType string

const (
	PolicyTypeAllow     PolicyType = "ALLOW"
	PolicyTypeDeny      PolicyType = "DENY"
	PolicyTypeRequire   PolicyType = "REQUIRE"
	PolicyTypeLimit     PolicyType = "LIMIT"
	PolicyTypeAudit     PolicyType = "AUDIT"
)

// PolicyScope defines the scope of policy application
type PolicyScope string

const (
	PolicyScopeGlobal     PolicyScope = "GLOBAL"
	PolicyScopeDepartment PolicyScope = "DEPARTMENT"
	PolicyScopeTeam       PolicyScope = "TEAM"
	PolicyScopeUser       PolicyScope = "USER"
	PolicyScopeResource   PolicyScope = "RESOURCE"
)

// EnforcementLevel defines how strictly a policy is enforced
type EnforcementLevel string

const (
	EnforcementAdvisory  EnforcementLevel = "ADVISORY"  // Warning only
	EnforcementSoft      EnforcementLevel = "SOFT"      // Allow with logging
	EnforcementHard      EnforcementLevel = "HARD"      // Block action
	EnforcementCritical  EnforcementLevel = "CRITICAL"  // Block + alert
)

// PolicyRule defines a specific rule within a policy
type PolicyRule struct {
	ID          string                 `json:"id"`
	Condition   string                 `json:"condition"`   // Expression to evaluate
	Action      string                 `json:"action"`      // Action to take
	Parameters  map[string]interface{} `json:"parameters"`
	Priority    int                    `json:"priority"`
	Enabled     bool                   `json:"enabled"`
}

// PolicyException defines an exception to a policy
type PolicyException struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id,omitempty"`
	TeamID      string    `json:"team_id,omitempty"`
	ResourceID  string    `json:"resource_id,omitempty"`
	Reason      string    `json:"reason"`
	ExpiresAt   time.Time `json:"expires_at"`
	ApprovedBy  string    `json:"approved_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// PolicyValue represents a policy configuration value
type PolicyValue struct {
	Value       interface{} `json:"value"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Editable    bool        `json:"editable"`
	Scope       string      `json:"scope"`
}

// SecuritySettings holds security-related settings
type SecuritySettings struct {
	PasswordPolicy      PasswordPolicy      `json:"password_policy"`
	SessionSettings     SessionSettings     `json:"session_settings"`
	EncryptionSettings  EncryptionSettings  `json:"encryption_settings"`
	AuditSettings       AuditSettings       `json:"audit_settings"`
	ThreatDetection     ThreatDetection     `json:"threat_detection"`
}

// PasswordPolicy defines password requirements
type PasswordPolicy struct {
	MinLength         int           `json:"min_length"`
	RequireUppercase  bool          `json:"require_uppercase"`
	RequireLowercase  bool          `json:"require_lowercase"`
	RequireNumbers    bool          `json:"require_numbers"`
	RequireSymbols    bool          `json:"require_symbols"`
	MaxAge            time.Duration `json:"max_age"`
	HistorySize       int           `json:"history_size"`
	LockoutThreshold  int           `json:"lockout_threshold"`
	LockoutDuration   time.Duration `json:"lockout_duration"`
}

// SessionSettings defines session management settings
type SessionSettings struct {
	MaxDuration       time.Duration `json:"max_duration"`
	IdleTimeout       time.Duration `json:"idle_timeout"`
	RequireReauth     bool          `json:"require_reauth"`
	ConcurrentLimit   int           `json:"concurrent_limit"`
	IPValidation      bool          `json:"ip_validation"`
}

// EncryptionSettings defines encryption requirements
type EncryptionSettings struct {
	RequireInTransit  bool   `json:"require_in_transit"`
	RequireAtRest     bool   `json:"require_at_rest"`
	MinKeyLength      int    `json:"min_key_length"`
	AllowedAlgorithms []string `json:"allowed_algorithms"`
	KeyRotationDays   int    `json:"key_rotation_days"`
}

// AuditSettings defines audit logging settings
type AuditSettings struct {
	EnableAuditLog    bool          `json:"enable_audit_log"`
	RetentionDays     int           `json:"retention_days"`
	LogLevel          string        `json:"log_level"`
	IncludePayloads   bool          `json:"include_payloads"`
	RealTimeAlerts    bool          `json:"real_time_alerts"`
}

// ThreatDetection defines threat detection settings
type ThreatDetection struct {
	EnableBehaviorAnalysis bool     `json:"enable_behavior_analysis"`
	AnomalyThreshold      float64  `json:"anomaly_threshold"`
	BlockedCountries      []string `json:"blocked_countries"`
	AllowedIPRanges       []string `json:"allowed_ip_ranges"`
	EnableGeoBlocking     bool     `json:"enable_geo_blocking"`
}

// AccessControlSettings defines access control configuration
type AccessControlSettings struct {
	DefaultUserRole       string              `json:"default_user_role"`
	RequireApproval       map[string]bool     `json:"require_approval"`
	AccessReviewCycle     time.Duration       `json:"access_review_cycle"`
	PrivilegedOperations  []string            `json:"privileged_operations"`
	ResourceRestrictions  map[string][]string `json:"resource_restrictions"`
}

// ComplianceRequirements defines compliance settings
type ComplianceRequirements struct {
	Standards         []string          `json:"standards"`        // SOC2, ISO27001, etc.
	DataResidency     []string          `json:"data_residency"`   // Allowed regions
	RetentionPolicies map[string]int    `json:"retention_policies"` // Data type -> days
	PrivacyControls   PrivacyControls   `json:"privacy_controls"`
	ReportingSchedule ReportingSchedule `json:"reporting_schedule"`
}

// PrivacyControls defines privacy-related controls
type PrivacyControls struct {
	EnableDataMinimization bool     `json:"enable_data_minimization"`
	RequireConsent         bool     `json:"require_consent"`
	AllowDataExport        bool     `json:"allow_data_export"`
	AllowDataDeletion      bool     `json:"allow_data_deletion"`
	PIIClassification      []string `json:"pii_classification"`
}

// ReportingSchedule defines compliance reporting schedule
type ReportingSchedule struct {
	Frequency     string   `json:"frequency"`     // daily, weekly, monthly, quarterly
	Recipients    []string `json:"recipients"`
	AutoGenerate  bool     `json:"auto_generate"`
	IncludeMetrics bool    `json:"include_metrics"`
}

// CostLimitSettings defines cost control settings
type CostLimitSettings struct {
	GlobalLimit     CostLimit            `json:"global_limit"`
	DepartmentLimits map[string]CostLimit `json:"department_limits"`
	ServiceLimits   map[string]CostLimit `json:"service_limits"`
	AlertThresholds []float64            `json:"alert_thresholds"` // Percentages
	Currency        string               `json:"currency"`
	BillingPeriod   string               `json:"billing_period"`
}

// CostLimit defines a cost limit
type CostLimit struct {
	Amount     float64 `json:"amount"`
	Period     string  `json:"period"`     // monthly, quarterly, yearly
	Enabled    bool    `json:"enabled"`
	HardLimit  bool    `json:"hard_limit"` // Block when exceeded
	AutoReset  bool    `json:"auto_reset"`
}

// UserQuota defines quotas for individual users
type UserQuota struct {
	Storage        int64         `json:"storage"`         // Bytes
	Requests       int64         `json:"requests"`        // Per month
	DataTransfer   int64         `json:"data_transfer"`   // Bytes per month
	APICallsPerDay int           `json:"api_calls_per_day"`
	ValidUntil     time.Time     `json:"valid_until"`
	Overrides      map[string]interface{} `json:"overrides"`
}

// TeamQuota defines quotas for teams
type TeamQuota struct {
	Storage        int64         `json:"storage"`
	Requests       int64         `json:"requests"`
	DataTransfer   int64         `json:"data_transfer"`
	Members        int           `json:"members"`
	ValidUntil     time.Time     `json:"valid_until"`
	Overrides      map[string]interface{} `json:"overrides"`
}

// IntegrationSettings defines third-party integration settings
type IntegrationSettings struct {
	SSO            SSOSettings           `json:"sso"`
	LDAP           LDAPSettings          `json:"ldap"`
	SAML           SAMLSettings          `json:"saml"`
	Webhooks       WebhookSettings       `json:"webhooks"`
	APIGateways    []APIGatewaySettings  `json:"api_gateways"`
}

// SSOSettings defines Single Sign-On settings
type SSOSettings struct {
	Enabled      bool              `json:"enabled"`
	Provider     string            `json:"provider"`
	ClientID     string            `json:"client_id"`
	Scopes       []string          `json:"scopes"`
	RedirectURI  string            `json:"redirect_uri"`
	Metadata     map[string]string `json:"metadata"`
}

// LDAPSettings defines LDAP integration settings
type LDAPSettings struct {
	Enabled     bool   `json:"enabled"`
	ServerURL   string `json:"server_url"`
	BaseDN      string `json:"base_dn"`
	UserFilter  string `json:"user_filter"`
	GroupFilter string `json:"group_filter"`
	BindDN      string `json:"bind_dn"`
}

// SAMLSettings defines SAML integration settings
type SAMLSettings struct {
	Enabled         bool   `json:"enabled"`
	EntityID        string `json:"entity_id"`
	SSOURL          string `json:"sso_url"`
	SLOurl          string `json:"slo_url"`
	CertificatePath string `json:"certificate_path"`
}

// WebhookSettings defines webhook configuration
type WebhookSettings struct {
	Enabled     bool              `json:"enabled"`
	Endpoints   []WebhookEndpoint `json:"endpoints"`
	RetryPolicy RetryPolicy       `json:"retry_policy"`
	Security    WebhookSecurity   `json:"security"`
}

// WebhookEndpoint defines a webhook endpoint
type WebhookEndpoint struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	Enabled     bool     `json:"enabled"`
	SecretToken string   `json:"secret_token,omitempty"`
}

// RetryPolicy defines webhook retry settings
type RetryPolicy struct {
	MaxRetries int           `json:"max_retries"`
	Backoff    time.Duration `json:"backoff"`
	Timeout    time.Duration `json:"timeout"`
}

// WebhookSecurity defines webhook security settings
type WebhookSecurity struct {
	RequireSignature bool     `json:"require_signature"`
	AllowedIPs       []string `json:"allowed_ips"`
	RequireHTTPS     bool     `json:"require_https"`
}

// APIGatewaySettings defines API gateway integration
type APIGatewaySettings struct {
	Name        string            `json:"name"`
	Endpoint    string            `json:"endpoint"`
	APIKey      string            `json:"api_key,omitempty"`
	RateLimit   int               `json:"rate_limit"`
	Timeout     time.Duration     `json:"timeout"`
	Headers     map[string]string `json:"headers"`
}

// NotificationSettings defines notification configuration
type NotificationSettings struct {
	Email    EmailSettings    `json:"email"`
	Slack    SlackSettings    `json:"slack"`
	SMS      SMSSettings      `json:"sms"`
	Webhook  WebhookSettings  `json:"webhook"`
	InApp    InAppSettings    `json:"in_app"`
	Channels []string         `json:"channels"` // Enabled channels
}

// EmailSettings defines email notification settings
type EmailSettings struct {
	Enabled    bool     `json:"enabled"`
	SMTPServer string   `json:"smtp_server"`
	Port       int      `json:"port"`
	Username   string   `json:"username"`
	FromEmail  string   `json:"from_email"`
	Templates  []string `json:"templates"`
}

// SlackSettings defines Slack notification settings
type SlackSettings struct {
	Enabled    bool              `json:"enabled"`
	WebhookURL string            `json:"webhook_url"`
	Channel    string            `json:"channel"`
	Username   string            `json:"username"`
	IconEmoji  string            `json:"icon_emoji"`
	Templates  map[string]string `json:"templates"`
}

// SMSSettings defines SMS notification settings
type SMSSettings struct {
	Enabled   bool   `json:"enabled"`
	Provider  string `json:"provider"`
	APIKey    string `json:"api_key,omitempty"`
	FromPhone string `json:"from_phone"`
}

// InAppSettings defines in-app notification settings
type InAppSettings struct {
	Enabled       bool          `json:"enabled"`
	RetentionDays int           `json:"retention_days"`
	MaxPerUser    int           `json:"max_per_user"`
	Categories    []string      `json:"categories"`
}

// Department represents an organizational department
type Department struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	ManagerID   string            `json:"manager_id"`
	ParentID    string            `json:"parent_id,omitempty"`
	Teams       []string          `json:"teams"`
	Budget      DepartmentBudget  `json:"budget"`
	Policies    []string          `json:"policies"`
	Settings    map[string]interface{} `json:"settings"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// DepartmentBudget defines department budget allocation
type DepartmentBudget struct {
	Annual       float64   `json:"annual"`
	Quarterly    float64   `json:"quarterly"`
	Monthly      float64   `json:"monthly"`
	Spent        float64   `json:"spent"`
	Currency     string    `json:"currency"`
	LastUpdated  time.Time `json:"last_updated"`
}

// Team represents a team within a department
type Team struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	DepartmentID string            `json:"department_id"`
	LeaderID     string            `json:"leader_id"`
	Members      []string          `json:"members"`
	Policies     []string          `json:"policies"`
	Quotas       TeamQuota         `json:"quotas"`
	Settings     map[string]interface{} `json:"settings"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// SettingsStorage interface for storing organizational settings
type SettingsStorage interface {
	Save(settings *OrganizationSettings) error
	Load() (*OrganizationSettings, error)
	Backup(version string) error
	ListBackups() ([]string, error)
	RestoreBackup(version string) (*OrganizationSettings, error)
}

// FileSettingsStorage implements SettingsStorage using files
type FileSettingsStorage struct {
	configDir string
}

// NewFileSettingsStorage creates a file-based settings storage
func NewFileSettingsStorage(configDir string) *FileSettingsStorage {
	os.MkdirAll(configDir, 0755)
	os.MkdirAll(filepath.Join(configDir, "backups"), 0755)
	return &FileSettingsStorage{configDir: configDir}
}

// Save saves organization settings to file
func (f *FileSettingsStorage) Save(settings *OrganizationSettings) error {
	settings.LastUpdated = time.Now()
	
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	filename := filepath.Join(f.configDir, "organization.json")
	return os.WriteFile(filename, data, 0644)
}

// Load loads organization settings from file
func (f *FileSettingsStorage) Load() (*OrganizationSettings, error) {
	filename := filepath.Join(f.configDir, "organization.json")
	
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return f.createDefaultSettings(), nil
		}
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	var settings OrganizationSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return &settings, nil
}

// Backup creates a backup of current settings
func (f *FileSettingsStorage) Backup(version string) error {
	current := filepath.Join(f.configDir, "organization.json")
	backup := filepath.Join(f.configDir, "backups", fmt.Sprintf("organization_%s.json", version))
	
	data, err := os.ReadFile(current)
	if err != nil {
		return fmt.Errorf("failed to read current settings: %w", err)
	}

	return os.WriteFile(backup, data, 0644)
}

// ListBackups lists available backups
func (f *FileSettingsStorage) ListBackups() ([]string, error) {
	backupDir := filepath.Join(f.configDir, "backups")
	
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".json" {
			backups = append(backups, entry.Name())
		}
	}

	return backups, nil
}

// RestoreBackup restores settings from a backup
func (f *FileSettingsStorage) RestoreBackup(version string) (*OrganizationSettings, error) {
	backup := filepath.Join(f.configDir, "backups", fmt.Sprintf("organization_%s.json", version))
	
	data, err := os.ReadFile(backup)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file: %w", err)
	}

	var settings OrganizationSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup: %w", err)
	}

	// Save as current
	current := filepath.Join(f.configDir, "organization.json")
	if err := os.WriteFile(current, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to restore backup: %w", err)
	}

	return &settings, nil
}

// createDefaultSettings creates default organization settings
func (f *FileSettingsStorage) createDefaultSettings() *OrganizationSettings {
	return &OrganizationSettings{
		Organization: OrganizationInfo{
			ID:        "default-org",
			Name:      "Default Organization",
			Domain:    "example.com",
			Industry:  "Technology",
			Size:      "medium",
			Region:    "US",
			TimeZone:  "UTC",
			Currency:  "USD",
			Language:  "en",
			Metadata:  make(map[string]string),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		GlobalPolicies: make(map[string]PolicyValue),
		SecurityConfig: SecuritySettings{
			PasswordPolicy: PasswordPolicy{
				MinLength:        8,
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumbers:   true,
				RequireSymbols:   false,
				MaxAge:           time.Hour * 24 * 90, // 90 days
				HistorySize:      5,
				LockoutThreshold: 5,
				LockoutDuration:  time.Minute * 30,
			},
			SessionSettings: SessionSettings{
				MaxDuration:     time.Hour * 8,
				IdleTimeout:     time.Minute * 30,
				RequireReauth:   false,
				ConcurrentLimit: 3,
				IPValidation:    false,
			},
			EncryptionSettings: EncryptionSettings{
				RequireInTransit:  true,
				RequireAtRest:     true,
				MinKeyLength:      256,
				AllowedAlgorithms: []string{"AES-256-GCM", "RSA-2048"},
				KeyRotationDays:   90,
			},
			AuditSettings: AuditSettings{
				EnableAuditLog:  true,
				RetentionDays:   365,
				LogLevel:        "INFO",
				IncludePayloads: false,
				RealTimeAlerts:  true,
			},
		},
		UserQuotas:     make(map[string]UserQuota),
		TeamQuotas:     make(map[string]TeamQuota),
		LastUpdated:    time.Now(),
		UpdatedBy:      "system",
		Version:        "1.0.0",
	}
}

// NewOrganizationManager creates a new organization manager
func NewOrganizationManager(config *OrgConfig) (*OrganizationManager, error) {
	if config == nil {
		config = &OrgConfig{
			ConfigDir:         "config/organization",
			AutoSync:          true,
			SyncInterval:      time.Minute * 5,
			BackupEnabled:     true,
			BackupInterval:    time.Hour * 24,
			AuditEnabled:      true,
			EnforceCompliance: true,
		}
	}

	storage := NewFileSettingsStorage(config.ConfigDir)
	settings, err := storage.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	return &OrganizationManager{
		config:      config,
		settings:    settings,
		policies:    make(map[string]*Policy),
		departments: make(map[string]*Department),
		teams:       make(map[string]*Team),
		storage:     storage,
	}, nil
}

// GetSettings returns current organization settings
func (o *OrganizationManager) GetSettings() *OrganizationSettings {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	
	// Return a copy to prevent external modification
	settingsCopy := *o.settings
	return &settingsCopy
}

// UpdateSettings updates organization settings
func (o *OrganizationManager) UpdateSettings(settings *OrganizationSettings, updatedBy string) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	// Validate settings
	if err := o.validateSettings(settings); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	// Create backup before updating
	if o.config.BackupEnabled {
		version := fmt.Sprintf("%d", time.Now().Unix())
		if err := o.storage.Backup(version); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	settings.UpdatedBy = updatedBy
	settings.LastUpdated = time.Now()

	// Save to storage
	if err := o.storage.Save(settings); err != nil {
		return fmt.Errorf("failed to save settings: %w", err)
	}

	o.settings = settings
	return nil
}

// validateSettings validates organization settings
func (o *OrganizationManager) validateSettings(settings *OrganizationSettings) error {
	if settings.Organization.Name == "" {
		return fmt.Errorf("organization name is required")
	}
	
	if settings.Organization.Domain == "" {
		return fmt.Errorf("organization domain is required")
	}

	// Validate password policy
	if settings.SecurityConfig.PasswordPolicy.MinLength < 8 {
		return fmt.Errorf("minimum password length must be at least 8")
	}

	// Validate session settings
	if settings.SecurityConfig.SessionSettings.MaxDuration < time.Minute*5 {
		return fmt.Errorf("maximum session duration must be at least 5 minutes")
	}

	return nil
}

// CreatePolicy creates a new organizational policy
func (o *OrganizationManager) CreatePolicy(policy *Policy) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("policy_%d", time.Now().UnixNano())
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	o.policies[policy.ID] = policy
	return nil
}

// GetPolicy retrieves a policy by ID
func (o *OrganizationManager) GetPolicy(policyID string) (*Policy, error) {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	policy, exists := o.policies[policyID]
	if !exists {
		return nil, fmt.Errorf("policy %s not found", policyID)
	}

	return policy, nil
}

// ListPolicies returns all policies
func (o *OrganizationManager) ListPolicies() []*Policy {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	policies := make([]*Policy, 0, len(o.policies))
	for _, policy := range o.policies {
		policies = append(policies, policy)
	}

	return policies
}

// CreateDepartment creates a new department
func (o *OrganizationManager) CreateDepartment(dept *Department) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if dept.ID == "" {
		dept.ID = fmt.Sprintf("dept_%d", time.Now().UnixNano())
	}

	dept.CreatedAt = time.Now()
	dept.UpdatedAt = time.Now()

	o.departments[dept.ID] = dept
	return nil
}

// CreateTeam creates a new team
func (o *OrganizationManager) CreateTeam(team *Team) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if team.ID == "" {
		team.ID = fmt.Sprintf("team_%d", time.Now().UnixNano())
	}

	team.CreatedAt = time.Now()
	team.UpdatedAt = time.Now()

	o.teams[team.ID] = team
	return nil
}

// SetUserQuota sets quota for a specific user
func (o *OrganizationManager) SetUserQuota(userID string, quota UserQuota) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.settings.UserQuotas[userID] = quota
	return o.storage.Save(o.settings)
}

// SetTeamQuota sets quota for a specific team
func (o *OrganizationManager) SetTeamQuota(teamID string, quota TeamQuota) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.settings.TeamQuotas[teamID] = quota
	return o.storage.Save(o.settings)
}

// GetUserQuota gets quota for a specific user
func (o *OrganizationManager) GetUserQuota(userID string) (UserQuota, bool) {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	quota, exists := o.settings.UserQuotas[userID]
	return quota, exists
}

// GetTeamQuota gets quota for a specific team
func (o *OrganizationManager) GetTeamQuota(teamID string) (TeamQuota, bool) {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	quota, exists := o.settings.TeamQuotas[teamID]
	return quota, exists
}

// ExportSettings exports settings to JSON
func (o *OrganizationManager) ExportSettings() ([]byte, error) {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	return json.MarshalIndent(o.settings, "", "  ")
}

// ImportSettings imports settings from JSON
func (o *OrganizationManager) ImportSettings(data []byte, updatedBy string) error {
	var settings OrganizationSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return o.UpdateSettings(&settings, updatedBy)
}