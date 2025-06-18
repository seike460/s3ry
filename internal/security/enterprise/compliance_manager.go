package enterprise

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ComplianceManager manages enterprise compliance requirements
type ComplianceManager struct {
	config      *ComplianceConfig
	frameworks  map[string]*ComplianceFramework
	assessments []*ComplianceAssessment
	auditLogger *AuditLogger
	mutex       sync.RWMutex
}

// ComplianceFramework represents a compliance framework (SOC2, ISO27001, etc.)
type ComplianceFramework struct {
	Name         string                    `json:"name"`
	Version      string                    `json:"version"`
	Description  string                    `json:"description"`
	Controls     []*ComplianceControl      `json:"controls"`
	Status       ComplianceFrameworkStatus `json:"status"`
	LastAssessed time.Time                 `json:"last_assessed"`
	NextReview   time.Time                 `json:"next_review"`
}

// ComplianceControl represents a specific compliance control
type ComplianceControl struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Requirement    string             `json:"requirement"`
	Status         ComplianceStatus   `json:"status"`
	Evidence       []string           `json:"evidence"`
	Implementation string             `json:"implementation"`
	TestProcedure  string             `json:"test_procedure"`
	LastTested     time.Time          `json:"last_tested"`
	NextTest       time.Time          `json:"next_test"`
	Owner          string             `json:"owner"`
	Priority       CompliancePriority `json:"priority"`
}

// ComplianceAssessment represents a compliance assessment
type ComplianceAssessment struct {
	ID              string                      `json:"id"`
	Framework       string                      `json:"framework"`
	AssessmentDate  time.Time                   `json:"assessment_date"`
	Assessor        string                      `json:"assessor"`
	Scope           string                      `json:"scope"`
	Results         *ComplianceAssessmentResult `json:"results"`
	Findings        []*ComplianceFinding        `json:"findings"`
	Recommendations []string                    `json:"recommendations"`
	Status          AssessmentStatus            `json:"status"`
}

// ComplianceAssessmentResult holds assessment results
type ComplianceAssessmentResult struct {
	OverallStatus              ComplianceStatus `json:"overall_status"`
	TotalControls              int              `json:"total_controls"`
	CompliantControls          int              `json:"compliant_controls"`
	NonCompliantControls       int              `json:"non_compliant_controls"`
	PartiallyCompliantControls int              `json:"partially_compliant_controls"`
	ComplianceScore            float64          `json:"compliance_score"`
}

// ComplianceFinding represents a compliance finding
type ComplianceFinding struct {
	ID          string          `json:"id"`
	ControlID   string          `json:"control_id"`
	Severity    FindingSeverity `json:"severity"`
	Description string          `json:"description"`
	Evidence    string          `json:"evidence"`
	Remediation string          `json:"remediation"`
	DueDate     time.Time       `json:"due_date"`
	Status      FindingStatus   `json:"status"`
	AssignedTo  string          `json:"assigned_to"`
}

// Enum types for compliance management
type ComplianceFrameworkStatus string
type ComplianceStatus string
type CompliancePriority string
type AssessmentStatus string
type FindingSeverity string
type FindingStatus string

const (
	// ComplianceFrameworkStatus values
	FrameworkStatusActive   ComplianceFrameworkStatus = "ACTIVE"
	FrameworkStatusInactive ComplianceFrameworkStatus = "INACTIVE"
	FrameworkStatusPending  ComplianceFrameworkStatus = "PENDING"

	// ComplianceStatus values
	StatusCompliant          ComplianceStatus = "COMPLIANT"
	StatusNonCompliant       ComplianceStatus = "NON_COMPLIANT"
	StatusPartiallyCompliant ComplianceStatus = "PARTIALLY_COMPLIANT"
	StatusNotApplicable      ComplianceStatus = "NOT_APPLICABLE"
	StatusInProgress         ComplianceStatus = "IN_PROGRESS"

	// CompliancePriority values
	PriorityHigh   CompliancePriority = "HIGH"
	PriorityMedium CompliancePriority = "MEDIUM"
	PriorityLow    CompliancePriority = "LOW"

	// AssessmentStatus values
	AssessmentStatusPlanned    AssessmentStatus = "PLANNED"
	AssessmentStatusInProgress AssessmentStatus = "IN_PROGRESS"
	AssessmentStatusCompleted  AssessmentStatus = "COMPLETED"
	AssessmentStatusCancelled  AssessmentStatus = "CANCELLED"

	// FindingSeverity values
	FindingSeverityCritical FindingSeverity = "CRITICAL"
	FindingSeverityHigh     FindingSeverity = "HIGH"
	FindingSeverityMedium   FindingSeverity = "MEDIUM"
	FindingSeverityLow      FindingSeverity = "LOW"

	// FindingStatus values
	FindingStatusOpen       FindingStatus = "OPEN"
	FindingStatusInProgress FindingStatus = "IN_PROGRESS"
	FindingStatusResolved   FindingStatus = "RESOLVED"
	FindingStatusAccepted   FindingStatus = "ACCEPTED"
)

// NewComplianceManager creates a new compliance manager
func NewComplianceManager(config *ComplianceConfig, auditLogger *AuditLogger) *ComplianceManager {
	cm := &ComplianceManager{
		config:      config,
		frameworks:  make(map[string]*ComplianceFramework),
		assessments: []*ComplianceAssessment{},
		auditLogger: auditLogger,
	}

	// Initialize compliance frameworks
	cm.initializeComplianceFrameworks()

	return cm
}

// initializeComplianceFrameworks sets up standard compliance frameworks
func (cm *ComplianceManager) initializeComplianceFrameworks() {
	// SOC 2 Framework
	if cm.config.SOC2Enabled {
		soc2 := cm.createSOC2Framework()
		cm.frameworks["SOC2"] = soc2
	}

	// ISO 27001 Framework
	if cm.config.ISO27001Enabled {
		iso27001 := cm.createISO27001Framework()
		cm.frameworks["ISO27001"] = iso27001
	}

	// GDPR Framework
	if cm.config.GDPREnabled {
		gdpr := cm.createGDPRFramework()
		cm.frameworks["GDPR"] = gdpr
	}

	// CCPA Framework
	if cm.config.CCPAEnabled {
		ccpa := cm.createCCPAFramework()
		cm.frameworks["CCPA"] = ccpa
	}
}

// createSOC2Framework creates SOC 2 compliance framework
func (cm *ComplianceManager) createSOC2Framework() *ComplianceFramework {
	controls := []*ComplianceControl{
		{
			ID:             "CC1.1",
			Name:           "Control Environment - Integrity and Ethical Values",
			Description:    "Organization demonstrates commitment to integrity and ethical values",
			Requirement:    "Policies and procedures for ethical conduct must be established",
			Status:         StatusInProgress,
			Implementation: "Code of conduct and ethics policies implemented",
			TestProcedure:  "Review policies and interview staff",
			Owner:          "Security Team",
			Priority:       PriorityHigh,
		},
		{
			ID:             "CC2.1",
			Name:           "Communication and Information - Internal Communication",
			Description:    "Organization obtains or generates and uses relevant information",
			Requirement:    "Information systems must support internal communication needs",
			Status:         StatusCompliant,
			Implementation: "Internal communication systems and procedures established",
			TestProcedure:  "Review communication logs and procedures",
			Owner:          "IT Team",
			Priority:       PriorityMedium,
		},
		{
			ID:             "CC6.1",
			Name:           "Logical and Physical Access Controls",
			Description:    "Organization implements logical access security measures",
			Requirement:    "Access controls must restrict unauthorized access to data",
			Status:         StatusCompliant,
			Implementation: "RBAC system with MFA implemented",
			TestProcedure:  "Test access controls and review user permissions",
			Owner:          "Security Team",
			Priority:       PriorityHigh,
		},
		{
			ID:             "CC6.7",
			Name:           "Data Transmission",
			Description:    "Organization restricts transmission of data to authorized users",
			Requirement:    "Data transmission must be encrypted and secured",
			Status:         StatusCompliant,
			Implementation: "TLS 1.2+ encryption for all data transmission",
			TestProcedure:  "Verify encryption protocols and test data transmission",
			Owner:          "Security Team",
			Priority:       PriorityHigh,
		},
	}

	return &ComplianceFramework{
		Name:         "SOC 2 Type II",
		Version:      "2017",
		Description:  "Service Organization Control 2 Type II compliance framework",
		Controls:     controls,
		Status:       FrameworkStatusActive,
		LastAssessed: time.Now().AddDate(0, -3, 0), // 3 months ago
		NextReview:   time.Now().AddDate(0, 9, 0),  // 9 months from now
	}
}

// createISO27001Framework creates ISO 27001 compliance framework
func (cm *ComplianceManager) createISO27001Framework() *ComplianceFramework {
	controls := []*ComplianceControl{
		{
			ID:             "A.9.1.1",
			Name:           "Access Control Policy",
			Description:    "Establish and maintain access control policy",
			Requirement:    "Access control policy must be documented and communicated",
			Status:         StatusCompliant,
			Implementation: "Access control policy documented and implemented",
			TestProcedure:  "Review policy documents and implementation",
			Owner:          "Security Team",
			Priority:       PriorityHigh,
		},
		{
			ID:             "A.12.6.1",
			Name:           "Management of Technical Vulnerabilities",
			Description:    "Timely information about technical vulnerabilities",
			Requirement:    "Technical vulnerabilities must be identified and managed",
			Status:         StatusCompliant,
			Implementation: "Vulnerability scanning and management system in place",
			TestProcedure:  "Review vulnerability management procedures",
			Owner:          "Security Team",
			Priority:       PriorityHigh,
		},
		{
			ID:             "A.10.1.1",
			Name:           "Audit Logging",
			Description:    "Event logs recording user activities and exceptions",
			Requirement:    "Audit logs must be generated and protected",
			Status:         StatusCompliant,
			Implementation: "Comprehensive audit logging system implemented",
			TestProcedure:  "Review audit logs and logging procedures",
			Owner:          "Security Team",
			Priority:       PriorityHigh,
		},
	}

	return &ComplianceFramework{
		Name:         "ISO 27001:2013",
		Version:      "2013",
		Description:  "International Standard for Information Security Management",
		Controls:     controls,
		Status:       FrameworkStatusActive,
		LastAssessed: time.Now().AddDate(0, -6, 0), // 6 months ago
		NextReview:   time.Now().AddDate(1, 0, 0),  // 1 year from now
	}
}

// createGDPRFramework creates GDPR compliance framework
func (cm *ComplianceManager) createGDPRFramework() *ComplianceFramework {
	controls := []*ComplianceControl{
		{
			ID:             "Art.25",
			Name:           "Data Protection by Design and by Default",
			Description:    "Implement technical and organizational measures for data protection",
			Requirement:    "Data protection must be integrated into system design",
			Status:         StatusPartiallyCompliant,
			Implementation: "Basic data protection measures implemented",
			TestProcedure:  "Review data protection implementations",
			Owner:          "Data Protection Officer",
			Priority:       PriorityHigh,
		},
		{
			ID:             "Art.32",
			Name:           "Security of Processing",
			Description:    "Implement appropriate technical and organizational measures",
			Requirement:    "Personal data processing must be secured",
			Status:         StatusCompliant,
			Implementation: "Encryption and access controls implemented",
			TestProcedure:  "Review security measures and test implementations",
			Owner:          "Security Team",
			Priority:       PriorityHigh,
		},
		{
			ID:             "Art.33",
			Name:           "Notification of Personal Data Breach",
			Description:    "Report data breaches to supervisory authority",
			Requirement:    "Data breaches must be reported within 72 hours",
			Status:         StatusCompliant,
			Implementation: "Incident response procedures established",
			TestProcedure:  "Review incident response procedures",
			Owner:          "Security Team",
			Priority:       PriorityHigh,
		},
	}

	return &ComplianceFramework{
		Name:         "GDPR",
		Version:      "2018",
		Description:  "General Data Protection Regulation",
		Controls:     controls,
		Status:       FrameworkStatusActive,
		LastAssessed: time.Now().AddDate(0, -4, 0), // 4 months ago
		NextReview:   time.Now().AddDate(0, 8, 0),  // 8 months from now
	}
}

// createCCPAFramework creates CCPA compliance framework
func (cm *ComplianceManager) createCCPAFramework() *ComplianceFramework {
	controls := []*ComplianceControl{
		{
			ID:             "CCPA.1999.1",
			Name:           "Consumer Rights Notice",
			Description:    "Provide notice of consumer rights regarding personal information",
			Requirement:    "Consumers must be informed of their privacy rights",
			Status:         StatusCompliant,
			Implementation: "Privacy notices and consumer rights information provided",
			TestProcedure:  "Review privacy notices and consumer information",
			Owner:          "Legal Team",
			Priority:       PriorityMedium,
		},
		{
			ID:             "CCPA.1999.2",
			Name:           "Data Security",
			Description:    "Implement reasonable security measures for personal information",
			Requirement:    "Personal information must be protected with reasonable security",
			Status:         StatusCompliant,
			Implementation: "Security measures for personal information implemented",
			TestProcedure:  "Review security measures and test effectiveness",
			Owner:          "Security Team",
			Priority:       PriorityHigh,
		},
	}

	return &ComplianceFramework{
		Name:         "CCPA",
		Version:      "2020",
		Description:  "California Consumer Privacy Act",
		Controls:     controls,
		Status:       FrameworkStatusActive,
		LastAssessed: time.Now().AddDate(0, -2, 0), // 2 months ago
		NextReview:   time.Now().AddDate(0, 10, 0), // 10 months from now
	}
}

// PerformComplianceAssessment performs a comprehensive compliance assessment
func (cm *ComplianceManager) PerformComplianceAssessment(framework string, assessor string) (*ComplianceAssessment, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	fw, exists := cm.frameworks[framework]
	if !exists {
		return nil, fmt.Errorf("compliance framework %s not found", framework)
	}

	assessmentID := fmt.Sprintf("ASSESS_%s_%d", framework, time.Now().Unix())

	assessment := &ComplianceAssessment{
		ID:              assessmentID,
		Framework:       framework,
		AssessmentDate:  time.Now(),
		Assessor:        assessor,
		Scope:           "Full compliance assessment",
		Status:          AssessmentStatusInProgress,
		Findings:        []*ComplianceFinding{},
		Recommendations: []string{},
	}

	// Assess each control
	compliantCount := 0
	nonCompliantCount := 0
	partiallyCompliantCount := 0

	for _, control := range fw.Controls {
		switch control.Status {
		case StatusCompliant:
			compliantCount++
		case StatusNonCompliant:
			nonCompliantCount++
			// Create a finding for non-compliant controls
			finding := &ComplianceFinding{
				ID:          fmt.Sprintf("FIND_%s_%d", control.ID, time.Now().Unix()),
				ControlID:   control.ID,
				Severity:    cm.determineFindingSeverity(control.Priority),
				Description: fmt.Sprintf("Control %s is non-compliant", control.ID),
				Evidence:    "Assessment determined control is not properly implemented",
				Remediation: "Implement control according to framework requirements",
				DueDate:     time.Now().AddDate(0, 0, 30), // 30 days
				Status:      FindingStatusOpen,
				AssignedTo:  control.Owner,
			}
			assessment.Findings = append(assessment.Findings, finding)
		case StatusPartiallyCompliant:
			partiallyCompliantCount++
			// Create a finding for partially compliant controls
			finding := &ComplianceFinding{
				ID:          fmt.Sprintf("FIND_%s_%d", control.ID, time.Now().Unix()),
				ControlID:   control.ID,
				Severity:    FindingSeverityMedium,
				Description: fmt.Sprintf("Control %s is partially compliant", control.ID),
				Evidence:    "Assessment found gaps in control implementation",
				Remediation: "Address implementation gaps to achieve full compliance",
				DueDate:     time.Now().AddDate(0, 0, 60), // 60 days
				Status:      FindingStatusOpen,
				AssignedTo:  control.Owner,
			}
			assessment.Findings = append(assessment.Findings, finding)
		}
	}

	totalControls := len(fw.Controls)
	complianceScore := float64(compliantCount) / float64(totalControls) * 100

	assessment.Results = &ComplianceAssessmentResult{
		TotalControls:              totalControls,
		CompliantControls:          compliantCount,
		NonCompliantControls:       nonCompliantCount,
		PartiallyCompliantControls: partiallyCompliantCount,
		ComplianceScore:            complianceScore,
	}

	// Determine overall status
	if nonCompliantCount == 0 && partiallyCompliantCount == 0 {
		assessment.Results.OverallStatus = StatusCompliant
	} else if nonCompliantCount > 0 {
		assessment.Results.OverallStatus = StatusNonCompliant
	} else {
		assessment.Results.OverallStatus = StatusPartiallyCompliant
	}

	// Generate recommendations
	if nonCompliantCount > 0 {
		assessment.Recommendations = append(assessment.Recommendations,
			"Address all non-compliant controls as high priority")
	}
	if partiallyCompliantCount > 0 {
		assessment.Recommendations = append(assessment.Recommendations,
			"Complete implementation of partially compliant controls")
	}
	if complianceScore < 90 {
		assessment.Recommendations = append(assessment.Recommendations,
			"Implement comprehensive compliance monitoring program")
	}

	assessment.Status = AssessmentStatusCompleted
	cm.assessments = append(cm.assessments, assessment)

	// Update framework last assessed date
	fw.LastAssessed = time.Now()

	// Log the assessment
	if cm.auditLogger != nil {
		cm.auditLogger.LogAction("system", "compliance_assessment", framework, "COMPLETED", map[string]interface{}{
			"assessment_id":    assessmentID,
			"compliance_score": complianceScore,
			"findings_count":   len(assessment.Findings),
		})
	}

	return assessment, nil
}

// determineFindingSeverity determines finding severity based on control priority
func (cm *ComplianceManager) determineFindingSeverity(priority CompliancePriority) FindingSeverity {
	switch priority {
	case PriorityHigh:
		return FindingSeverityHigh
	case PriorityMedium:
		return FindingSeverityMedium
	case PriorityLow:
		return FindingSeverityLow
	default:
		return FindingSeverityMedium
	}
}

// GetComplianceReport generates a comprehensive compliance report
func (cm *ComplianceManager) GetComplianceReport() (*ComplianceReport, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	report := &ComplianceReport{
		GeneratedAt:       time.Now(),
		Frameworks:        []*ComplianceFrameworkSummary{},
		OverallCompliance: &OverallComplianceStatus{},
		RecentAssessments: []*ComplianceAssessment{},
		OpenFindings:      []*ComplianceFinding{},
		Recommendations:   []string{},
	}

	totalControls := 0
	compliantControls := 0
	activeFrameworks := 0

	// Analyze each framework
	for name, framework := range cm.frameworks {
		if framework.Status != FrameworkStatusActive {
			continue
		}

		activeFrameworks++
		frameworkCompliant := 0
		frameworkTotal := len(framework.Controls)

		for _, control := range framework.Controls {
			if control.Status == StatusCompliant {
				frameworkCompliant++
			}
		}

		compliancePercentage := float64(frameworkCompliant) / float64(frameworkTotal) * 100

		summary := &ComplianceFrameworkSummary{
			Name:                 name,
			Status:               framework.Status,
			TotalControls:        frameworkTotal,
			CompliantControls:    frameworkCompliant,
			CompliancePercentage: compliancePercentage,
			LastAssessed:         framework.LastAssessed,
			NextReview:           framework.NextReview,
		}

		report.Frameworks = append(report.Frameworks, summary)

		totalControls += frameworkTotal
		compliantControls += frameworkCompliant
	}

	// Calculate overall compliance
	if totalControls > 0 {
		overallPercentage := float64(compliantControls) / float64(totalControls) * 100
		report.OverallCompliance = &OverallComplianceStatus{
			TotalFrameworks:      activeFrameworks,
			TotalControls:        totalControls,
			CompliantControls:    compliantControls,
			CompliancePercentage: overallPercentage,
		}

		// Determine overall status
		if overallPercentage == 100 {
			report.OverallCompliance.Status = StatusCompliant
		} else if overallPercentage >= 90 {
			report.OverallCompliance.Status = StatusPartiallyCompliant
		} else {
			report.OverallCompliance.Status = StatusNonCompliant
		}
	}

	// Add recent assessments (last 5)
	assessmentCount := len(cm.assessments)
	if assessmentCount > 0 {
		start := assessmentCount - 5
		if start < 0 {
			start = 0
		}
		report.RecentAssessments = cm.assessments[start:]
	}

	// Collect open findings
	for _, assessment := range cm.assessments {
		for _, finding := range assessment.Findings {
			if finding.Status == FindingStatusOpen || finding.Status == FindingStatusInProgress {
				report.OpenFindings = append(report.OpenFindings, finding)
			}
		}
	}

	// Generate recommendations
	if report.OverallCompliance.CompliancePercentage < 95 {
		report.Recommendations = append(report.Recommendations,
			"Improve overall compliance score to achieve 95% or higher")
	}
	if len(report.OpenFindings) > 0 {
		report.Recommendations = append(report.Recommendations,
			"Address all open compliance findings in a timely manner")
	}

	return report, nil
}

// ComplianceReport represents a comprehensive compliance report
type ComplianceReport struct {
	GeneratedAt       time.Time                     `json:"generated_at"`
	Frameworks        []*ComplianceFrameworkSummary `json:"frameworks"`
	OverallCompliance *OverallComplianceStatus      `json:"overall_compliance"`
	RecentAssessments []*ComplianceAssessment       `json:"recent_assessments"`
	OpenFindings      []*ComplianceFinding          `json:"open_findings"`
	Recommendations   []string                      `json:"recommendations"`
}

// ComplianceFrameworkSummary provides a summary of a compliance framework
type ComplianceFrameworkSummary struct {
	Name                 string                    `json:"name"`
	Status               ComplianceFrameworkStatus `json:"status"`
	TotalControls        int                       `json:"total_controls"`
	CompliantControls    int                       `json:"compliant_controls"`
	CompliancePercentage float64                   `json:"compliance_percentage"`
	LastAssessed         time.Time                 `json:"last_assessed"`
	NextReview           time.Time                 `json:"next_review"`
}

// OverallComplianceStatus provides overall compliance status
type OverallComplianceStatus struct {
	Status               ComplianceStatus `json:"status"`
	TotalFrameworks      int              `json:"total_frameworks"`
	TotalControls        int              `json:"total_controls"`
	CompliantControls    int              `json:"compliant_controls"`
	CompliancePercentage float64          `json:"compliance_percentage"`
}

// UpdateControlStatus updates the status of a specific control
func (cm *ComplianceManager) UpdateControlStatus(framework, controlID string, status ComplianceStatus) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	fw, exists := cm.frameworks[framework]
	if !exists {
		return fmt.Errorf("framework %s not found", framework)
	}

	for _, control := range fw.Controls {
		if control.ID == controlID {
			oldStatus := control.Status
			control.Status = status
			control.LastTested = time.Now()

			// Log the status change
			if cm.auditLogger != nil {
				cm.auditLogger.LogAction("system", "control_status_update", controlID, "SUCCESS", map[string]interface{}{
					"framework":  framework,
					"old_status": oldStatus,
					"new_status": status,
				})
			}

			return nil
		}
	}

	return fmt.Errorf("control %s not found in framework %s", controlID, framework)
}

// GetFramework returns a specific compliance framework
func (cm *ComplianceManager) GetFramework(name string) (*ComplianceFramework, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	framework, exists := cm.frameworks[name]
	return framework, exists
}

// GetAssessment returns a specific compliance assessment
func (cm *ComplianceManager) GetAssessment(id string) (*ComplianceAssessment, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	for _, assessment := range cm.assessments {
		if assessment.ID == id {
			return assessment, true
		}
	}

	return nil, false
}

// ExportComplianceData exports compliance data in JSON format
func (cm *ComplianceManager) ExportComplianceData() ([]byte, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	data := struct {
		Frameworks  map[string]*ComplianceFramework `json:"frameworks"`
		Assessments []*ComplianceAssessment         `json:"assessments"`
		ExportedAt  time.Time                       `json:"exported_at"`
	}{
		Frameworks:  cm.frameworks,
		Assessments: cm.assessments,
		ExportedAt:  time.Now(),
	}

	return json.MarshalIndent(data, "", "  ")
}
