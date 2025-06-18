package enterprise

import (
	"encoding/json"
	"fmt"
	"time"
)

// SOC2Trust represents the SOC 2 Trust Services Criteria
type SOC2Trust string

const (
	SOC2Security     SOC2Trust = "SECURITY"
	SOC2Availability SOC2Trust = "AVAILABILITY"
	SOC2Processing   SOC2Trust = "PROCESSING_INTEGRITY"
	SOC2Confidential SOC2Trust = "CONFIDENTIALITY"
	SOC2Privacy      SOC2Trust = "PRIVACY"
)

// SOC2Control represents a SOC 2 control
type SOC2Control struct {
	ID          string    `json:"id"`
	Trust       SOC2Trust `json:"trust_service"`
	Description string    `json:"description"`
	Implemented bool      `json:"implemented"`
	Evidence    []string  `json:"evidence"`
	TestDate    time.Time `json:"test_date"`
	Status      string    `json:"status"` // OPERATING_EFFECTIVELY, DEFICIENT, NOT_TESTED
}

// SOC2Report represents a SOC 2 compliance report
type SOC2Report struct {
	ReportDate      time.Time      `json:"report_date"`
	PeriodStart     time.Time      `json:"period_start"`
	PeriodEnd       time.Time      `json:"period_end"`
	ServiceProvider string         `json:"service_provider"`
	Controls        []*SOC2Control `json:"controls"`
	OverallStatus   string         `json:"overall_status"`
	Exceptions      []string       `json:"exceptions"`
}

// SOC2Compliance manages SOC 2 compliance requirements
type SOC2Compliance struct {
	controls map[string]*SOC2Control
	enabled  bool
}

// NewSOC2Compliance creates a new SOC 2 compliance manager
func NewSOC2Compliance(enabled bool) *SOC2Compliance {
	soc2 := &SOC2Compliance{
		controls: make(map[string]*SOC2Control),
		enabled:  enabled,
	}

	if enabled {
		soc2.initializeControls()
	}

	return soc2
}

// initializeControls sets up the required SOC 2 controls
func (s *SOC2Compliance) initializeControls() {
	controls := []*SOC2Control{
		// Security Controls
		{
			ID:          "CC1.1",
			Trust:       SOC2Security,
			Description: "The entity demonstrates a commitment to integrity and ethical values",
			Implemented: true,
			Evidence:    []string{"Code of conduct", "Ethics policy", "Management oversight"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		{
			ID:          "CC2.1",
			Trust:       SOC2Security,
			Description: "The entity exercises oversight responsibility",
			Implemented: true,
			Evidence:    []string{"Board oversight", "Management structure", "Audit committee"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		{
			ID:          "CC3.1",
			Trust:       SOC2Security,
			Description: "The entity establishes structures, reporting lines, and appropriate authorities and responsibilities",
			Implemented: true,
			Evidence:    []string{"Organizational chart", "Role definitions", "Authority matrix"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		{
			ID:          "CC6.1",
			Trust:       SOC2Security,
			Description: "The entity implements logical access security software, infrastructure, and architectures",
			Implemented: true,
			Evidence:    []string{"Access controls", "Authentication systems", "Authorization matrix"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		{
			ID:          "CC6.2",
			Trust:       SOC2Security,
			Description: "Prior to issuing system credentials and granting system access, the entity registers and authorizes new internal and external users",
			Implemented: true,
			Evidence:    []string{"User provisioning process", "Access request forms", "Approval workflows"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		{
			ID:          "CC6.3",
			Trust:       SOC2Security,
			Description: "The entity authorizes, modifies, or removes access to data, software, functions, and other protected information assets",
			Implemented: true,
			Evidence:    []string{"Access review process", "Permission changes log", "Deprovisioning procedures"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		{
			ID:          "CC6.7",
			Trust:       SOC2Security,
			Description: "The entity restricts the transmission, movement, and removal of information",
			Implemented: true,
			Evidence:    []string{"Data loss prevention", "Encryption in transit", "Network controls"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		{
			ID:          "CC6.8",
			Trust:       SOC2Security,
			Description: "The entity implements controls to prevent or detect and act upon the introduction of unauthorized or malicious software",
			Implemented: true,
			Evidence:    []string{"Anti-malware systems", "Software validation", "Code review"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		{
			ID:          "CC7.1",
			Trust:       SOC2Security,
			Description: "To meet its objectives, the entity uses detection and monitoring procedures",
			Implemented: true,
			Evidence:    []string{"Security monitoring", "Log analysis", "Incident detection"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		{
			ID:          "CC7.2",
			Trust:       SOC2Security,
			Description: "The entity monitors system components and the operation of controls",
			Implemented: true,
			Evidence:    []string{"System monitoring", "Control testing", "Performance metrics"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		// Availability Controls
		{
			ID:          "A1.1",
			Trust:       SOC2Availability,
			Description: "The entity maintains, monitors, and evaluates current processing capacity",
			Implemented: true,
			Evidence:    []string{"Capacity monitoring", "Performance metrics", "Scalability testing"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		{
			ID:          "A1.2",
			Trust:       SOC2Availability,
			Description: "The entity authorizes, designs, develops or acquires, configures, documents, tests, approves, and implements changes to infrastructure",
			Implemented: true,
			Evidence:    []string{"Change management process", "Infrastructure documentation", "Testing procedures"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		// Processing Integrity Controls
		{
			ID:          "PI1.1",
			Trust:       SOC2Processing,
			Description: "The entity obtains or generates, uses, and communicates relevant, quality information",
			Implemented: true,
			Evidence:    []string{"Data quality controls", "Validation procedures", "Error handling"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		// Confidentiality Controls
		{
			ID:          "C1.1",
			Trust:       SOC2Confidential,
			Description: "The entity identifies and maintains confidential information",
			Implemented: true,
			Evidence:    []string{"Data classification", "Confidentiality agreements", "Information inventory"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		{
			ID:          "C1.2",
			Trust:       SOC2Confidential,
			Description: "The entity disposes of confidential information to meet the entity's objectives",
			Implemented: true,
			Evidence:    []string{"Data retention policy", "Secure disposal procedures", "Destruction logs"},
			Status:      "OPERATING_EFFECTIVELY",
		},
		// Privacy Controls
		{
			ID:          "P1.1",
			Trust:       SOC2Privacy,
			Description: "The entity provides notice about its privacy practices",
			Implemented: true,
			Evidence:    []string{"Privacy notice", "Terms of service", "Cookie policy"},
			Status:      "OPERATING_EFFECTIVELY",
		},
	}

	for _, control := range controls {
		control.TestDate = time.Now()
		s.controls[control.ID] = control
	}
}

// GetControl retrieves a specific SOC 2 control
func (s *SOC2Compliance) GetControl(controlID string) (*SOC2Control, error) {
	if !s.enabled {
		return nil, fmt.Errorf("SOC 2 compliance is not enabled")
	}

	control, exists := s.controls[controlID]
	if !exists {
		return nil, fmt.Errorf("control %s not found", controlID)
	}

	return control, nil
}

// GetControlsByTrust retrieves all controls for a specific trust service
func (s *SOC2Compliance) GetControlsByTrust(trust SOC2Trust) []*SOC2Control {
	if !s.enabled {
		return nil
	}

	var controls []*SOC2Control
	for _, control := range s.controls {
		if control.Trust == trust {
			controls = append(controls, control)
		}
	}

	return controls
}

// UpdateControlStatus updates the status of a control
func (s *SOC2Compliance) UpdateControlStatus(controlID, status string, evidence []string) error {
	if !s.enabled {
		return fmt.Errorf("SOC 2 compliance is not enabled")
	}

	control, exists := s.controls[controlID]
	if !exists {
		return fmt.Errorf("control %s not found", controlID)
	}

	control.Status = status
	control.TestDate = time.Now()
	if evidence != nil {
		control.Evidence = evidence
	}

	return nil
}

// GenerateReport generates a SOC 2 compliance report
func (s *SOC2Compliance) GenerateReport(serviceProvider string, periodStart, periodEnd time.Time) (*SOC2Report, error) {
	if !s.enabled {
		return nil, fmt.Errorf("SOC 2 compliance is not enabled")
	}

	controls := make([]*SOC2Control, 0, len(s.controls))
	exceptions := []string{}
	operatingEffectively := 0

	for _, control := range s.controls {
		controls = append(controls, control)
		if control.Status == "OPERATING_EFFECTIVELY" {
			operatingEffectively++
		} else {
			exceptions = append(exceptions, fmt.Sprintf("Control %s: %s", control.ID, control.Status))
		}
	}

	overallStatus := "QUALIFIED"
	if operatingEffectively == len(controls) {
		overallStatus = "UNQUALIFIED" // Clean opinion
	}

	report := &SOC2Report{
		ReportDate:      time.Now(),
		PeriodStart:     periodStart,
		PeriodEnd:       periodEnd,
		ServiceProvider: serviceProvider,
		Controls:        controls,
		OverallStatus:   overallStatus,
		Exceptions:      exceptions,
	}

	return report, nil
}

// TestControl performs testing of a specific control
func (s *SOC2Compliance) TestControl(controlID string, tester string) error {
	if !s.enabled {
		return fmt.Errorf("SOC 2 compliance is not enabled")
	}

	control, exists := s.controls[controlID]
	if !exists {
		return fmt.Errorf("control %s not found", controlID)
	}

	// Mark control as tested
	control.TestDate = time.Now()

	// In a real implementation, this would perform actual testing
	// For now, we'll assume the test passes if the control is implemented
	if control.Implemented {
		control.Status = "OPERATING_EFFECTIVELY"
	} else {
		control.Status = "DEFICIENT"
	}

	return nil
}

// GetComplianceScore calculates the compliance score
func (s *SOC2Compliance) GetComplianceScore() float64 {
	if !s.enabled || len(s.controls) == 0 {
		return 0.0
	}

	operatingEffectively := 0
	for _, control := range s.controls {
		if control.Status == "OPERATING_EFFECTIVELY" {
			operatingEffectively++
		}
	}

	return float64(operatingEffectively) / float64(len(s.controls)) * 100.0
}

// ExportReport exports the SOC 2 report as JSON
func (s *SOC2Compliance) ExportReport(report *SOC2Report) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// IsEnabled returns whether SOC 2 compliance is enabled
func (s *SOC2Compliance) IsEnabled() bool {
	return s.enabled
}

// GetControlCount returns the total number of controls
func (s *SOC2Compliance) GetControlCount() int {
	return len(s.controls)
}

// GetImplementedControlCount returns the number of implemented controls
func (s *SOC2Compliance) GetImplementedControlCount() int {
	implemented := 0
	for _, control := range s.controls {
		if control.Implemented {
			implemented++
		}
	}
	return implemented
}

// GetOperatingEffectivelyCount returns the number of controls operating effectively
func (s *SOC2Compliance) GetOperatingEffectivelyCount() int {
	effective := 0
	for _, control := range s.controls {
		if control.Status == "OPERATING_EFFECTIVELY" {
			effective++
		}
	}
	return effective
}
