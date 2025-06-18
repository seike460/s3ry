package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	// "github.com/seike460/s3ry/internal/security/enterprise"
)

// SecurityAuditor conducts comprehensive security audits
type SecurityAuditor struct {
	// scanner     *enterprise.VulnerabilityScanner
	config   *AuditConfig
	basePath string
	findings []*SecurityFinding
}

// AuditConfig holds security audit configuration
type AuditConfig struct {
	Enabled               bool     `json:"enabled"`
	IncludeCodeScan       bool     `json:"include_code_scan"`
	IncludeConfigScan     bool     `json:"include_config_scan"`
	IncludeCredentialScan bool     `json:"include_credential_scan"`
	IncludeDependencyScan bool     `json:"include_dependency_scan"`
	ExcludePaths          []string `json:"exclude_paths"`
	ScanTimeoutMinutes    int      `json:"scan_timeout_minutes"`
	ReportFormat          string   `json:"report_format"`
	ReportPath            string   `json:"report_path"`
}

// SecurityFinding represents a security audit finding
type SecurityFinding struct {
	ID          string              `json:"id"`
	Type        SecurityFindingType `json:"type"`
	Severity    string              `json:"severity"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	File        string              `json:"file,omitempty"`
	Line        int                 `json:"line,omitempty"`
	Code        string              `json:"code,omitempty"`
	Impact      string              `json:"impact"`
	Remediation string              `json:"remediation"`
	References  []string            `json:"references"`
	DetectedAt  time.Time           `json:"detected_at"`
	Status      FindingStatus       `json:"status"`
}

// SecurityFindingType represents the type of security finding
type SecurityFindingType string

const (
	FindingTypeHardcodedCredentials SecurityFindingType = "HARDCODED_CREDENTIALS"
	FindingTypeWeakCrypto           SecurityFindingType = "WEAK_CRYPTO"
	FindingTypeInsecureConfig       SecurityFindingType = "INSECURE_CONFIG"
	FindingTypeCodeInjection        SecurityFindingType = "CODE_INJECTION"
	FindingTypePathTraversal        SecurityFindingType = "PATH_TRAVERSAL"
	FindingTypeInsecureLogging      SecurityFindingType = "INSECURE_LOGGING"
	FindingTypeMissingValidation    SecurityFindingType = "MISSING_VALIDATION"
	FindingTypeInsecureHTTP         SecurityFindingType = "INSECURE_HTTP"
	FindingTypeWeakAuth             SecurityFindingType = "WEAK_AUTH"
	FindingTypeDependencyVuln       SecurityFindingType = "DEPENDENCY_VULNERABILITY"
)

// FindingStatus represents the status of a security finding
type FindingStatus string

const (
	FindingStatusNew           FindingStatus = "NEW"
	FindingStatusReviewed      FindingStatus = "REVIEWED"
	FindingStatusMitigated     FindingStatus = "MITIGATED"
	FindingStatusAccepted      FindingStatus = "ACCEPTED"
	FindingStatusFalsePositive FindingStatus = "FALSE_POSITIVE"
)

// SecurityAuditReport represents the comprehensive audit report
type SecurityAuditReport struct {
	GeneratedAt        time.Time                   `json:"generated_at"`
	AuditID            string                      `json:"audit_id"`
	ProjectPath        string                      `json:"project_path"`
	TotalFindings      int                         `json:"total_findings"`
	FindingsBySeverity map[string]int              `json:"findings_by_severity"`
	FindingsByType     map[SecurityFindingType]int `json:"findings_by_type"`
	Findings           []*SecurityFinding          `json:"findings"`
	ScanResult         interface{}                 `json:"scan_result"`
	Summary            *AuditSummary               `json:"summary"`
	Recommendations    []string                    `json:"recommendations"`
}

// AuditSummary provides a high-level summary of the audit
type AuditSummary struct {
	RiskLevel       string   `json:"risk_level"`
	CriticalIssues  int      `json:"critical_issues"`
	HighIssues      int      `json:"high_issues"`
	MediumIssues    int      `json:"medium_issues"`
	LowIssues       int      `json:"low_issues"`
	InfoIssues      int      `json:"info_issues"`
	TopConcerns     []string `json:"top_concerns"`
	ComplianceScore int      `json:"compliance_score"`
}

// NewSecurityAuditor creates a new security auditor
func NewSecurityAuditor(basePath string, config *AuditConfig) (*SecurityAuditor, error) {
	if config == nil {
		config = DefaultAuditConfig()
	}

	// scanConfig := enterprise.DefaultVulnerabilityScanConfig()
	// scanner := enterprise.NewVulnerabilityScanner(scanConfig)

	return &SecurityAuditor{
		// scanner:  scanner, // TODO: Enable when vulnerability scanner is implemented
		config:   config,
		basePath: basePath,
		findings: []*SecurityFinding{},
	}, nil
}

// DefaultAuditConfig returns default audit configuration
func DefaultAuditConfig() *AuditConfig {
	return &AuditConfig{
		Enabled:               true,
		IncludeCodeScan:       true,
		IncludeConfigScan:     true,
		IncludeCredentialScan: true,
		IncludeDependencyScan: true,
		ExcludePaths:          []string{".git", "node_modules", ".cache", "vendor"},
		ScanTimeoutMinutes:    30,
		ReportFormat:          "json",
		ReportPath:            "./security_audit_report.json",
	}
}

// PerformComprehensiveAudit conducts a full security audit
func (sa *SecurityAuditor) PerformComprehensiveAudit(ctx context.Context) (*SecurityAuditReport, error) {
	auditID := fmt.Sprintf("audit_%d", time.Now().Unix())

	// Initialize report
	report := &SecurityAuditReport{
		GeneratedAt: time.Now(),
		AuditID:     auditID,
		ProjectPath: sa.basePath,
		// FindingsBySeverity: make(map[enterprise.VulnerabilitySeverity]int),
		FindingsByType:  make(map[SecurityFindingType]int),
		Findings:        []*SecurityFinding{},
		Recommendations: []string{},
	}

	// 1. Code Security Scan
	if sa.config.IncludeCodeScan {
		codeFindings, err := sa.scanCodeSecurity(ctx)
		if err != nil {
			return nil, fmt.Errorf("code security scan failed: %w", err)
		}
		sa.findings = append(sa.findings, codeFindings...)
	}

	// 2. Configuration Security Scan
	if sa.config.IncludeConfigScan {
		configFindings, err := sa.scanConfigurationSecurity(ctx)
		if err != nil {
			return nil, fmt.Errorf("configuration security scan failed: %w", err)
		}
		sa.findings = append(sa.findings, configFindings...)
	}

	// 3. Credential Security Scan
	if sa.config.IncludeCredentialScan {
		credFindings, err := sa.scanCredentialSecurity(ctx)
		if err != nil {
			return nil, fmt.Errorf("credential security scan failed: %w", err)
		}
		sa.findings = append(sa.findings, credFindings...)
	}

	// 4. Dependency Security Scan
	if sa.config.IncludeDependencyScan {
		depFindings, err := sa.scanDependencySecurity(ctx)
		if err != nil {
			return nil, fmt.Errorf("dependency security scan failed: %w", err)
		}
		sa.findings = append(sa.findings, depFindings...)
	}

	// TODO: 5. Run vulnerability scanner when available
	// scanResult, err := sa.scanner.PerformComprehensiveScan()
	// if err != nil {
	//     return nil, fmt.Errorf("vulnerability scan failed: %w", err)
	// }
	// report.ScanResult = scanResult

	// Compile final report
	report.Findings = sa.findings
	report.TotalFindings = len(sa.findings)

	// TODO: Calculate statistics when report fields are implemented
	// for _, finding := range sa.findings {
	//     report.FindingsBySeverity[finding.Severity]++
	//     report.FindingsByType[finding.Type]++
	// }

	// Generate summary and recommendations
	report.Summary = sa.generateSummary(report)
	report.Recommendations = sa.generateRecommendations(report)

	return report, nil
}

// scanCodeSecurity performs code-level security scanning
func (sa *SecurityAuditor) scanCodeSecurity(ctx context.Context) ([]*SecurityFinding, error) {
	var findings []*SecurityFinding

	// Define security patterns to scan for
	patterns := []SecurityPattern{
		{
			Pattern:     regexp.MustCompile(`(?i)(password|pwd|secret|key|token)\s*[:=]\s*["'][^"']{1,}["']`),
			Type:        FindingTypeHardcodedCredentials,
			Severity:    "CRITICAL",
			Title:       "Hardcoded Credentials Detected",
			Description: "Hardcoded credentials found in source code",
			Impact:      "Credentials may be exposed to unauthorized users",
			Remediation: "Move credentials to environment variables or secure credential store",
		},
		{
			Pattern:     regexp.MustCompile(`(?i)md5|sha1`),
			Type:        FindingTypeWeakCrypto,
			Severity:    "MEDIUM",
			Title:       "Weak Cryptographic Algorithm",
			Description: "Use of weak cryptographic algorithms detected",
			Impact:      "Data may be vulnerable to cryptographic attacks",
			Remediation: "Use stronger algorithms like SHA-256 or SHA-3",
		},
		{
			Pattern:     regexp.MustCompile(`exec\.Command\([^)]*\)`),
			Type:        FindingTypeCodeInjection,
			Severity:    "HIGH",
			Title:       "Potential Command Injection",
			Description: "Direct command execution without proper validation",
			Impact:      "System may be vulnerable to command injection attacks",
			Remediation: "Validate and sanitize all inputs before command execution",
		},
		{
			Pattern:     regexp.MustCompile(`\.\.\/|\.\.\\`),
			Type:        FindingTypePathTraversal,
			Severity:    "MEDIUM",
			Title:       "Path Traversal Pattern",
			Description: "Potential path traversal vulnerability",
			Impact:      "Files outside intended directory may be accessible",
			Remediation: "Validate and restrict file paths to safe directories",
		},
		{
			Pattern: regexp.MustCompile(`fmt\.Printf\([^)]*%v[^)]*\)`),
			Type:    FindingTypeInsecureLogging,
			// Severity:    enterprise.SeverityLow,
			Title:       "Potential Information Disclosure",
			Description: "Use of %v in logging may expose sensitive information",
			Impact:      "Sensitive data may be logged unintentionally",
			Remediation: "Use specific format specifiers and avoid logging sensitive data",
		},
	}

	// Scan Go files
	err := filepath.Walk(sa.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded paths
		for _, exclude := range sa.config.ExcludePaths {
			if strings.Contains(path, exclude) {
				return nil
			}
		}

		// Only scan Go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		lines := strings.Split(string(content), "\n")

		for _, pattern := range patterns {
			matches := pattern.Pattern.FindAllStringIndex(string(content), -1)
			for _, match := range matches {
				// Find line number
				lineNum := findLineNumber(string(content), match[0])

				finding := &SecurityFinding{
					ID:          fmt.Sprintf("%s_%d", pattern.Type, time.Now().UnixNano()),
					Type:        pattern.Type,
					Severity:    pattern.Severity,
					Title:       pattern.Title,
					Description: pattern.Description,
					File:        strings.TrimPrefix(path, sa.basePath),
					Line:        lineNum,
					Code:        strings.TrimSpace(lines[lineNum-1]),
					Impact:      pattern.Impact,
					Remediation: pattern.Remediation,
					DetectedAt:  time.Now(),
					Status:      FindingStatusNew,
				}
				findings = append(findings, finding)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return findings, nil
}

// scanConfigurationSecurity scans configuration files for security issues
func (sa *SecurityAuditor) scanConfigurationSecurity(ctx context.Context) ([]*SecurityFinding, error) {
	var findings []*SecurityFinding

	configPatterns := []string{
		"*.yml", "*.yaml", "*.json", "*.toml", "*.env", "Dockerfile", "docker-compose.yml",
	}

	for _, pattern := range configPatterns {
		matches, err := filepath.Glob(filepath.Join(sa.basePath, pattern))
		if err != nil {
			continue
		}

		for _, configFile := range matches {
			content, err := os.ReadFile(configFile)
			if err != nil {
				continue
			}

			// Check for insecure configurations
			if strings.Contains(string(content), "debug: true") ||
				strings.Contains(string(content), "debug=true") {
				finding := &SecurityFinding{
					ID:          fmt.Sprintf("CONFIG_DEBUG_%d", time.Now().UnixNano()),
					Type:        FindingTypeInsecureConfig,
					Severity:    "MEDIUM",
					Title:       "Debug Mode Enabled",
					Description: "Debug mode is enabled in configuration",
					File:        strings.TrimPrefix(configFile, sa.basePath),
					Impact:      "Debug information may be exposed to attackers",
					Remediation: "Disable debug mode in production environments",
					DetectedAt:  time.Now(),
					Status:      FindingStatusNew,
				}
				findings = append(findings, finding)
			}

			// Check for HTTP usage
			if strings.Contains(string(content), "http://") {
				finding := &SecurityFinding{
					ID:          fmt.Sprintf("CONFIG_HTTP_%d", time.Now().UnixNano()),
					Type:        FindingTypeInsecureHTTP,
					Severity:    "MEDIUM",
					Title:       "Insecure HTTP Usage",
					Description: "HTTP protocol used instead of HTTPS",
					File:        strings.TrimPrefix(configFile, sa.basePath),
					Impact:      "Data transmitted over unencrypted connections",
					Remediation: "Use HTTPS for all network communications",
					DetectedAt:  time.Now(),
					Status:      FindingStatusNew,
				}
				findings = append(findings, finding)
			}
		}
	}

	return findings, nil
}

// scanCredentialSecurity scans for credential-related security issues
func (sa *SecurityAuditor) scanCredentialSecurity(ctx context.Context) ([]*SecurityFinding, error) {
	var findings []*SecurityFinding

	// Check for .env files with exposed credentials
	envFiles, _ := filepath.Glob(filepath.Join(sa.basePath, ".env*"))
	for _, envFile := range envFiles {
		info, err := os.Stat(envFile)
		if err != nil {
			continue
		}

		// Check file permissions
		if info.Mode().Perm() > 0600 {
			finding := &SecurityFinding{
				ID:          fmt.Sprintf("CRED_PERM_%d", time.Now().UnixNano()),
				Type:        FindingTypeInsecureConfig,
				Severity:    "HIGH",
				Title:       "Insecure Credential File Permissions",
				Description: "Environment file has overly permissive access rights",
				File:        strings.TrimPrefix(envFile, sa.basePath),
				Impact:      "Credentials may be accessible to unauthorized users",
				Remediation: "Set file permissions to 600 (owner read/write only)",
				DetectedAt:  time.Now(),
				Status:      FindingStatusNew,
			}
			findings = append(findings, finding)
		}
	}

	// Check for AWS credentials in unusual locations
	awsCredsPattern := regexp.MustCompile(`(?i)(AKIA[0-9A-Z]{16}|aws_access_key_id|aws_secret_access_key)`)
	err := filepath.Walk(sa.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		// Skip binary files and large files
		if info.Size() > 1024*1024 || isBinaryFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if awsCredsPattern.Match(content) {
			finding := &SecurityFinding{
				ID:          fmt.Sprintf("CRED_AWS_%d", time.Now().UnixNano()),
				Type:        FindingTypeHardcodedCredentials,
				Severity:    "CRITICAL",
				Title:       "AWS Credentials in Source Code",
				Description: "AWS credentials found in source code or configuration",
				File:        strings.TrimPrefix(path, sa.basePath),
				Impact:      "AWS account may be compromised",
				Remediation: "Remove credentials from code and use IAM roles or environment variables",
				DetectedAt:  time.Now(),
				Status:      FindingStatusNew,
			}
			findings = append(findings, finding)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return findings, nil
}

// scanDependencySecurity scans for known vulnerabilities in dependencies
func (sa *SecurityAuditor) scanDependencySecurity(ctx context.Context) ([]*SecurityFinding, error) {
	var findings []*SecurityFinding

	// Check go.mod for known vulnerable packages
	goModPath := filepath.Join(sa.basePath, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		content, err := os.ReadFile(goModPath)
		if err == nil {
			// Simple check for some known vulnerable patterns
			vulnPatterns := map[string]string{
				"github.com/gin-gonic/gin v1.6.": "Known XSS vulnerability in Gin versions < 1.7.0",
				"gopkg.in/yaml.v2 v2.2.":         "Known vulnerability in yaml.v2 versions < 2.4.0",
			}

			for pattern, description := range vulnPatterns {
				if strings.Contains(string(content), pattern) {
					finding := &SecurityFinding{
						ID:          fmt.Sprintf("DEP_VULN_%d", time.Now().UnixNano()),
						Type:        FindingTypeDependencyVuln,
						Severity:    "HIGH",
						Title:       "Vulnerable Dependency",
						Description: description,
						File:        "go.mod",
						Impact:      "Application may be vulnerable to known exploits",
						Remediation: "Update dependency to latest secure version",
						DetectedAt:  time.Now(),
						Status:      FindingStatusNew,
					}
					findings = append(findings, finding)
				}
			}
		}
	}

	return findings, nil
}

// generateSummary creates audit summary
func (sa *SecurityAuditor) generateSummary(report *SecurityAuditReport) *AuditSummary {
	summary := &AuditSummary{
		// CriticalIssues:  report.FindingsBySeverity[enterprise.SeverityCritical],
		// HighIssues:      report.FindingsBySeverity[enterprise.SeverityHigh],
		// MediumIssues:    report.FindingsBySeverity[enterprise.SeverityMedium],
		// LowIssues:       report.FindingsBySeverity[enterprise.SeverityLow],
		// InfoIssues:      report.FindingsBySeverity[enterprise.SeverityInfo],
		TopConcerns: []string{},
	}

	// Determine risk level
	if summary.CriticalIssues > 0 {
		summary.RiskLevel = "CRITICAL"
	} else if summary.HighIssues > 5 {
		summary.RiskLevel = "HIGH"
	} else if summary.HighIssues > 0 || summary.MediumIssues > 10 {
		summary.RiskLevel = "MEDIUM"
	} else {
		summary.RiskLevel = "LOW"
	}

	// Calculate compliance score (0-100)
	totalIssues := summary.CriticalIssues + summary.HighIssues + summary.MediumIssues
	if totalIssues == 0 {
		summary.ComplianceScore = 100
	} else {
		penalty := summary.CriticalIssues*20 + summary.HighIssues*10 + summary.MediumIssues*5
		summary.ComplianceScore = max(0, 100-penalty)
	}

	// Identify top concerns
	if summary.CriticalIssues > 0 {
		summary.TopConcerns = append(summary.TopConcerns, "Critical security vulnerabilities require immediate attention")
	}
	if report.FindingsByType[FindingTypeHardcodedCredentials] > 0 {
		summary.TopConcerns = append(summary.TopConcerns, "Hardcoded credentials pose significant security risk")
	}
	if report.FindingsByType[FindingTypeDependencyVuln] > 0 {
		summary.TopConcerns = append(summary.TopConcerns, "Vulnerable dependencies need updating")
	}

	return summary
}

// generateRecommendations creates actionable recommendations
func (sa *SecurityAuditor) generateRecommendations(report *SecurityAuditReport) []string {
	var recommendations []string

	// if report.FindingsBySeverity[enterprise.SeverityCritical] > 0 {
	// recommendations = append(recommendations, "Immediately address all critical security findings")
	// }

	if report.FindingsByType[FindingTypeHardcodedCredentials] > 0 {
		recommendations = append(recommendations, "Implement secure credential management system")
		recommendations = append(recommendations, "Use environment variables or AWS IAM roles for credentials")
	}

	if report.FindingsByType[FindingTypeWeakCrypto] > 0 {
		recommendations = append(recommendations, "Update cryptographic implementations to use strong algorithms")
	}

	if report.FindingsByType[FindingTypeDependencyVuln] > 0 {
		recommendations = append(recommendations, "Update all dependencies to latest secure versions")
		recommendations = append(recommendations, "Implement automated dependency vulnerability scanning")
	}

	if report.Summary.ComplianceScore < 80 {
		recommendations = append(recommendations, "Develop security remediation plan to improve compliance score")
	}

	recommendations = append(recommendations, "Implement regular security audits and monitoring")
	recommendations = append(recommendations, "Provide security training for development team")

	return recommendations
}

// SecurityPattern represents a security scanning pattern
type SecurityPattern struct {
	Pattern     *regexp.Regexp
	Type        SecurityFindingType
	Severity    string
	Title       string
	Description string
	Impact      string
	Remediation string
}

// Helper functions
func findLineNumber(content string, pos int) int {
	lines := strings.Split(content[:pos], "\n")
	return len(lines)
}

func isBinaryFile(filename string) bool {
	binaryExts := []string{".exe", ".bin", ".so", ".dll", ".jpg", ".png", ".gif", ".pdf", ".zip", ".tar", ".gz"}
	ext := strings.ToLower(filepath.Ext(filename))
	for _, binaryExt := range binaryExts {
		if ext == binaryExt {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
