package sla

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSLAMonitor_CreateSLA(t *testing.T) {
	storage := &MockSLAStorage{}
	alertManager, _ := NewSimpleSLAAlertManager(nil)
	monitor, err := NewSLAMonitor(nil, alertManager, storage)
	require.NoError(t, err)

	sla := &SLA{
		Name:        "Test SLA",
		Description: "Test SLA for unit testing",
		Service:     "test-service",
		Category:    SLACategoryAvailability,
		Type:        SLATypeUptime,
		Targets: []SLATarget{
			{
				ID:        "test-target",
				Name:      "Uptime Target",
				Metric:    "uptime_percentage",
				Threshold: 99.9,
				Operator:  OperatorGreaterEqual,
				Unit:      "percent",
				Window:    TimeWindow{Duration: time.Hour, Type: WindowTypeRolling},
				Severity:  SeverityCritical,
				Enabled:   true,
			},
		},
		Enabled: true,
	}

	err = monitor.CreateSLA(sla)
	assert.NoError(t, err)
	assert.NotEmpty(t, sla.ID)
	assert.Equal(t, SLAStatusHealthy, sla.CurrentStatus)
}

func TestSLAMonitor_CheckSLA(t *testing.T) {
	storage := &MockSLAStorage{}
	alertManager, _ := NewSimpleSLAAlertManager(nil)
	monitor, err := NewSLAMonitor(nil, alertManager, storage)
	require.NoError(t, err)

	sla := &SLA{
		ID:       "test-sla",
		Name:     "Test SLA",
		Service:  "test-service",
		Category: SLACategoryAvailability,
		Type:     SLATypeUptime,
		Targets: []SLATarget{
			{
				ID:        "test-target",
				Name:      "Uptime Target",
				Metric:    "uptime_percentage",
				Threshold: 99.9,
				Operator:  OperatorGreaterEqual,
				Unit:      "percent",
				Window:    TimeWindow{Duration: time.Hour, Type: WindowTypeRolling},
				Severity:  SeverityCritical,
				Enabled:   true,
			},
		},
		Alerting: AlertingConfig{Enabled: true},
		Enabled:  true,
	}

	monitor.slas[sla.ID] = sla

	// Test SLA check
	monitor.checkSLA(sla)

	// Verify SLA was updated
	assert.False(t, sla.LastChecked.IsZero())
	assert.Equal(t, SLAStatusHealthy, sla.CurrentStatus)
}

func TestSLAMonitor_ViolationHandling(t *testing.T) {
	storage := &MockSLAStorage{}
	alertManager, _ := NewSimpleSLAAlertManager(nil)
	monitor, err := NewSLAMonitor(nil, alertManager, storage)
	require.NoError(t, err)

	sla := &SLA{
		ID:       "test-sla",
		Name:     "Test SLA",
		Service:  "test-service",
		Category: SLACategoryAvailability,
		Type:     SLATypeUptime,
		Targets: []SLATarget{
			{
				ID:        "test-target",
				Name:      "Uptime Target",
				Metric:    "uptime_percentage",
				Threshold: 99.9,
				Operator:  OperatorGreaterEqual,
				Unit:      "percent",
				Severity:  SeverityCritical,
				Enabled:   true,
			},
		},
		Alerting: AlertingConfig{Enabled: true},
		Enabled:  true,
	}

	target := sla.Targets[0]
	
	// Test violation detection
	assert.True(t, monitor.isTargetViolated(target, 98.0)) // Below threshold
	assert.False(t, monitor.isTargetViolated(target, 99.95)) // Above threshold

	// Test violation handling
	monitor.handleViolation(sla, target, 98.0)
	
	// Check that violation was recorded
	assert.Greater(t, sla.ViolationCount, 0)
	assert.Equal(t, SLAStatusViolation, sla.CurrentStatus)
}

func TestSLAMonitor_OperatorLogic(t *testing.T) {
	storage := &MockSLAStorage{}
	alertManager, _ := NewSimpleSLAAlertManager(nil)
	monitor, err := NewSLAMonitor(nil, alertManager, storage)
	require.NoError(t, err)

	testCases := []struct {
		operator  Operator
		threshold float64
		value     float64
		violated  bool
	}{
		{OperatorGreaterEqual, 99.9, 99.95, false},
		{OperatorGreaterEqual, 99.9, 98.0, true},
		{OperatorLessEqual, 2.0, 1.5, false},
		{OperatorLessEqual, 2.0, 2.5, true},
		{OperatorEqual, 100.0, 100.0, false},
		{OperatorEqual, 100.0, 99.0, true},
		{OperatorNotEqual, 0.0, 1.0, false},
		{OperatorNotEqual, 0.0, 0.0, true},
	}

	for _, tc := range testCases {
		target := SLATarget{
			Operator:  tc.operator,
			Threshold: tc.threshold,
		}
		
		result := monitor.isTargetViolated(target, tc.value)
		assert.Equal(t, tc.violated, result, 
			"Operator %s, threshold %f, value %f should be violated=%t", 
			tc.operator, tc.threshold, tc.value, tc.violated)
	}
}

func TestSLAMonitor_BusinessImpactAssessment(t *testing.T) {
	storage := &MockSLAStorage{}
	alertManager, _ := NewSimpleSLAAlertManager(nil)
	monitor, err := NewSLAMonitor(nil, alertManager, storage)
	require.NoError(t, err)

	testCases := []struct {
		severity       Severity
		expectedImpact BusinessImpact
		expectedRisk   RiskLevel
	}{
		{SeverityCritical, BusinessImpactCritical, RiskLevelCritical},
		{SeverityError, BusinessImpactHigh, RiskLevelHigh},
		{SeverityWarning, BusinessImpactMedium, RiskLevelMedium},
		{SeverityInfo, BusinessImpactLow, RiskLevelLow},
	}

	for _, tc := range testCases {
		impact := monitor.assessBusinessImpact(tc.severity)
		risk := monitor.assessRiskLevel(tc.severity)
		
		assert.Equal(t, tc.expectedImpact, impact)
		assert.Equal(t, tc.expectedRisk, risk)
	}
}

func TestSLAMonitor_StartStop(t *testing.T) {
	storage := &MockSLAStorage{}
	alertManager, _ := NewSimpleSLAAlertManager(nil)
	config := DefaultSLAConfig()
	config.CheckInterval = time.Millisecond * 100 // Fast for testing
	
	monitor, err := NewSLAMonitor(config, alertManager, storage)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test start
	err = monitor.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, monitor.running)

	// Test double start
	err = monitor.Start(ctx)
	assert.Error(t, err)

	// Wait a bit for monitoring loop
	time.Sleep(time.Millisecond * 150)

	// Test stop
	err = monitor.Stop()
	assert.NoError(t, err)
	assert.False(t, monitor.running)
}

func TestSLAMonitor_Metrics(t *testing.T) {
	storage := &MockSLAStorage{}
	alertManager, _ := NewSimpleSLAAlertManager(nil)
	monitor, err := NewSLAMonitor(nil, alertManager, storage)
	require.NoError(t, err)

	// Add some SLAs
	healthySLA := &SLA{
		ID:            "healthy-sla",
		CurrentStatus: SLAStatusHealthy,
		Enabled:       true,
	}
	violatingSLA := &SLA{
		ID:            "violating-sla",
		CurrentStatus: SLAStatusViolation,
		Enabled:       true,
	}
	disabledSLA := &SLA{
		ID:            "disabled-sla",
		CurrentStatus: SLAStatusHealthy,
		Enabled:       false,
	}

	monitor.slas[healthySLA.ID] = healthySLA
	monitor.slas[violatingSLA.ID] = violatingSLA
	monitor.slas[disabledSLA.ID] = disabledSLA

	// Update metrics
	monitor.updateMetrics()

	metrics := monitor.GetMetrics()
	assert.Equal(t, 3, metrics.TotalSLAs)
	assert.Equal(t, 2, metrics.ActiveSLAs) // Only enabled SLAs
	assert.Equal(t, 1, metrics.HealthySLAs)
	assert.Equal(t, 1, metrics.ViolatingSLAs)
}

func TestSLAMonitor_GetViolationsWithFilters(t *testing.T) {
	storage := &MockSLAStorage{}
	alertManager, _ := NewSimpleSLAAlertManager(nil)
	monitor, err := NewSLAMonitor(nil, alertManager, storage)
	require.NoError(t, err)

	// Add test violations
	violation1 := &Violation{
		ID:       "violation-1",
		SLAID:    "sla-1",
		Severity: SeverityCritical,
		Status:   ViolationStatusActive,
	}
	violation2 := &Violation{
		ID:       "violation-2",
		SLAID:    "sla-2",
		Severity: SeverityWarning,
		Status:   ViolationStatusResolved,
	}

	monitor.violations[violation1.ID] = violation1
	monitor.violations[violation2.ID] = violation2

	// Test filtering by SLA ID
	filters := map[string]interface{}{"sla_id": "sla-1"}
	violations := monitor.GetViolations(filters)
	assert.Len(t, violations, 1)
	assert.Equal(t, "violation-1", violations[0].ID)

	// Test filtering by status
	filters = map[string]interface{}{"status": string(ViolationStatusActive)}
	violations = monitor.GetViolations(filters)
	assert.Len(t, violations, 1)
	assert.Equal(t, "violation-1", violations[0].ID)

	// Test filtering by severity
	filters = map[string]interface{}{"severity": string(SeverityWarning)}
	violations = monitor.GetViolations(filters)
	assert.Len(t, violations, 1)
	assert.Equal(t, "violation-2", violations[0].ID)

	// Test no filters
	violations = monitor.GetViolations(nil)
	assert.Len(t, violations, 2)
}

// MockSLAStorage is a mock implementation of SLAStorage for testing
type MockSLAStorage struct {
	slas       map[string]*SLA
	violations map[string]*Violation
}

func (m *MockSLAStorage) SaveSLA(sla *SLA) error {
	if m.slas == nil {
		m.slas = make(map[string]*SLA)
	}
	m.slas[sla.ID] = sla
	return nil
}

func (m *MockSLAStorage) LoadSLA(id string) (*SLA, error) {
	if m.slas == nil {
		m.slas = make(map[string]*SLA)
	}
	sla, exists := m.slas[id]
	if !exists {
		return nil, fmt.Errorf("SLA %s not found", id)
	}
	return sla, nil
}

func (m *MockSLAStorage) ListSLAs() ([]*SLA, error) {
	if m.slas == nil {
		return []*SLA{}, nil
	}
	slas := make([]*SLA, 0, len(m.slas))
	for _, sla := range m.slas {
		slas = append(slas, sla)
	}
	return slas, nil
}

func (m *MockSLAStorage) DeleteSLA(id string) error {
	if m.slas == nil {
		return nil
	}
	delete(m.slas, id)
	return nil
}

func (m *MockSLAStorage) SaveViolation(violation *Violation) error {
	if m.violations == nil {
		m.violations = make(map[string]*Violation)
	}
	m.violations[violation.ID] = violation
	return nil
}

func (m *MockSLAStorage) LoadViolation(id string) (*Violation, error) {
	if m.violations == nil {
		m.violations = make(map[string]*Violation)
	}
	violation, exists := m.violations[id]
	if !exists {
		return nil, fmt.Errorf("violation %s not found", id)
	}
	return violation, nil
}

func (m *MockSLAStorage) ListViolations(filters map[string]interface{}) ([]*Violation, error) {
	if m.violations == nil {
		return []*Violation{}, nil
	}
	violations := make([]*Violation, 0, len(m.violations))
	for _, violation := range m.violations {
		violations = append(violations, violation)
	}
	return violations, nil
}

func (m *MockSLAStorage) GetSLAMetrics(timeRange time.Duration) (*SLAMetrics, error) {
	return NewSLAMetrics(), nil
}

func (m *MockSLAStorage) Cleanup(retentionPeriod time.Duration) error {
	return nil
}