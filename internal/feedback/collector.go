package feedback

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/seike460/s3ry/internal/config"
)

// FeedbackType represents different types of user feedback
type FeedbackType string

const (
	FeedbackTypeUsability   FeedbackType = "usability"
	FeedbackTypePerformance FeedbackType = "performance"
	FeedbackTypeError       FeedbackType = "error"
	FeedbackTypeFeature     FeedbackType = "feature"
	FeedbackTypeBug         FeedbackType = "bug"
	FeedbackTypeGeneral     FeedbackType = "general"
)

// FeedbackSeverity represents the severity level of feedback
type FeedbackSeverity string

const (
	SeverityLow      FeedbackSeverity = "low"
	SeverityMedium   FeedbackSeverity = "medium"
	SeverityHigh     FeedbackSeverity = "high"
	SeverityCritical FeedbackSeverity = "critical"
)

// UserFeedback represents a piece of user feedback
type UserFeedback struct {
	ID          string           `json:"id"`
	Type        FeedbackType     `json:"type"`
	Severity    FeedbackSeverity `json:"severity"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Context     FeedbackContext  `json:"context"`
	Timestamp   time.Time        `json:"timestamp"`
	UserID      string           `json:"user_id,omitempty"`
	Version     string           `json:"version"`
	Platform    string           `json:"platform"`
	Resolved    bool             `json:"resolved"`
	Resolution  string           `json:"resolution,omitempty"`
}

// FeedbackContext provides context about when the feedback was given
type FeedbackContext struct {
	CurrentView     string                 `json:"current_view"`
	LastOperation   string                 `json:"last_operation"`
	ErrorMessages   []string               `json:"error_messages,omitempty"`
	PerformanceData map[string]interface{} `json:"performance_data,omitempty"`
	UserActions     []UserAction           `json:"user_actions,omitempty"`
	SystemInfo      SystemInfo             `json:"system_info"`
}

// UserAction represents a user action for context
type UserAction struct {
	Action    string    `json:"action"`
	Timestamp time.Time `json:"timestamp"`
	Duration  int64     `json:"duration_ms"`
	Success   bool      `json:"success"`
}

// SystemInfo provides system context
type SystemInfo struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	TerminalSize string `json:"terminal_size"`
	AWSRegion    string `json:"aws_region,omitempty"`
	ConfigPath   string `json:"config_path,omitempty"`
}

// FeedbackCollector manages user feedback collection and analysis
type FeedbackCollector struct {
	config      *config.Config
	feedbackDir string
	userActions []UserAction
	maxActions  int
	enabled     bool
	anonymous   bool
}

// NewFeedbackCollector creates a new feedback collector
func NewFeedbackCollector(cfg *config.Config) *FeedbackCollector {
	homeDir, _ := os.UserHomeDir()
	feedbackDir := filepath.Join(homeDir, ".s3ry", "feedback")

	// Create feedback directory if it doesn't exist
	os.MkdirAll(feedbackDir, 0755)

	return &FeedbackCollector{
		config:      cfg,
		feedbackDir: feedbackDir,
		userActions: make([]UserAction, 0),
		maxActions:  100, // Keep last 100 actions for context
		enabled:     cfg.Telemetry.Enabled,
		anonymous:   cfg.Telemetry.Anonymous,
	}
}

// RecordUserAction records a user action for context
func (fc *FeedbackCollector) RecordUserAction(action string, duration time.Duration, success bool) {
	if !fc.enabled {
		return
	}

	userAction := UserAction{
		Action:    action,
		Timestamp: time.Now(),
		Duration:  duration.Milliseconds(),
		Success:   success,
	}

	fc.userActions = append(fc.userActions, userAction)

	// Keep only the last maxActions
	if len(fc.userActions) > fc.maxActions {
		fc.userActions = fc.userActions[1:]
	}
}

// CollectFeedback collects user feedback with context
func (fc *FeedbackCollector) CollectFeedback(feedbackType FeedbackType, severity FeedbackSeverity, title, description string, context FeedbackContext) error {
	if !fc.enabled {
		return nil
	}

	feedback := UserFeedback{
		ID:          generateFeedbackID(),
		Type:        feedbackType,
		Severity:    severity,
		Title:       title,
		Description: description,
		Context:     context,
		Timestamp:   time.Now(),
		Version:     getVersion(),
		Platform:    getPlatform(),
		Resolved:    false,
	}

	// Add user ID if not anonymous
	if !fc.anonymous {
		feedback.UserID = getUserID()
	}

	// Add recent user actions to context
	feedback.Context.UserActions = fc.getRecentActions(10)

	// Save feedback to local file
	return fc.saveFeedback(feedback)
}

// CollectErrorFeedback specifically collects error-related feedback
func (fc *FeedbackCollector) CollectErrorFeedback(errorMsg string, context FeedbackContext) error {
	return fc.CollectFeedback(
		FeedbackTypeError,
		SeverityHigh,
		"Error Encountered",
		fmt.Sprintf("User encountered error: %s", errorMsg),
		context,
	)
}

// CollectPerformanceFeedback collects performance-related feedback
func (fc *FeedbackCollector) CollectPerformanceFeedback(operation string, duration time.Duration, context FeedbackContext) error {
	severity := SeverityLow
	if duration > time.Second*10 {
		severity = SeverityMedium
	}
	if duration > time.Second*30 {
		severity = SeverityHigh
	}

	return fc.CollectFeedback(
		FeedbackTypePerformance,
		severity,
		"Performance Issue",
		fmt.Sprintf("Operation '%s' took %v", operation, duration),
		context,
	)
}

// CollectUsabilityFeedback collects usability-related feedback
func (fc *FeedbackCollector) CollectUsabilityFeedback(title, description string, context FeedbackContext) error {
	return fc.CollectFeedback(
		FeedbackTypeUsability,
		SeverityMedium,
		title,
		description,
		context,
	)
}

// AnalyzeFeedback analyzes collected feedback and generates insights
func (fc *FeedbackCollector) AnalyzeFeedback() (*FeedbackAnalysis, error) {
	feedbacks, err := fc.loadAllFeedback()
	if err != nil {
		return nil, err
	}

	analysis := &FeedbackAnalysis{
		TotalFeedback:   len(feedbacks),
		ByType:          make(map[FeedbackType]int),
		BySeverity:      make(map[FeedbackSeverity]int),
		CommonIssues:    make([]CommonIssue, 0),
		Trends:          make([]FeedbackTrend, 0),
		Recommendations: make([]string, 0),
	}

	// Analyze feedback by type and severity
	for _, feedback := range feedbacks {
		analysis.ByType[feedback.Type]++
		analysis.BySeverity[feedback.Severity]++
	}

	// Identify common issues
	analysis.CommonIssues = fc.identifyCommonIssues(feedbacks)

	// Analyze trends
	analysis.Trends = fc.analyzeTrends(feedbacks)

	// Generate recommendations
	analysis.Recommendations = fc.generateRecommendations(analysis)

	return analysis, nil
}

// FeedbackAnalysis represents the analysis of collected feedback
type FeedbackAnalysis struct {
	TotalFeedback   int                      `json:"total_feedback"`
	ByType          map[FeedbackType]int     `json:"by_type"`
	BySeverity      map[FeedbackSeverity]int `json:"by_severity"`
	CommonIssues    []CommonIssue            `json:"common_issues"`
	Trends          []FeedbackTrend          `json:"trends"`
	Recommendations []string                 `json:"recommendations"`
	GeneratedAt     time.Time                `json:"generated_at"`
}

// CommonIssue represents a commonly reported issue
type CommonIssue struct {
	Title       string           `json:"title"`
	Count       int              `json:"count"`
	Severity    FeedbackSeverity `json:"severity"`
	FirstSeen   time.Time        `json:"first_seen"`
	LastSeen    time.Time        `json:"last_seen"`
	Description string           `json:"description"`
}

// FeedbackTrend represents a trend in feedback
type FeedbackTrend struct {
	Type      FeedbackType `json:"type"`
	Direction string       `json:"direction"` // "increasing", "decreasing", "stable"
	Change    float64      `json:"change"`    // percentage change
	Period    string       `json:"period"`    // time period
}

// Helper methods

func (fc *FeedbackCollector) saveFeedback(feedback UserFeedback) error {
	filename := fmt.Sprintf("feedback_%s_%s.json",
		feedback.Timestamp.Format("2006-01-02"),
		feedback.ID)
	filepath := filepath.Join(fc.feedbackDir, filename)

	data, err := json.MarshalIndent(feedback, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}

func (fc *FeedbackCollector) loadAllFeedback() ([]UserFeedback, error) {
	files, err := os.ReadDir(fc.feedbackDir)
	if err != nil {
		return nil, err
	}

	var feedbacks []UserFeedback

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(fc.feedbackDir, file.Name()))
		if err != nil {
			continue
		}

		var feedback UserFeedback
		if err := json.Unmarshal(data, &feedback); err != nil {
			continue
		}

		feedbacks = append(feedbacks, feedback)
	}

	return feedbacks, nil
}

func (fc *FeedbackCollector) getRecentActions(count int) []UserAction {
	if len(fc.userActions) <= count {
		return fc.userActions
	}
	return fc.userActions[len(fc.userActions)-count:]
}

func (fc *FeedbackCollector) identifyCommonIssues(feedbacks []UserFeedback) []CommonIssue {
	issueMap := make(map[string]*CommonIssue)

	for _, feedback := range feedbacks {
		key := fmt.Sprintf("%s:%s", feedback.Type, feedback.Title)

		if issue, exists := issueMap[key]; exists {
			issue.Count++
			if feedback.Timestamp.After(issue.LastSeen) {
				issue.LastSeen = feedback.Timestamp
			}
			if feedback.Timestamp.Before(issue.FirstSeen) {
				issue.FirstSeen = feedback.Timestamp
			}
		} else {
			issueMap[key] = &CommonIssue{
				Title:       feedback.Title,
				Count:       1,
				Severity:    feedback.Severity,
				FirstSeen:   feedback.Timestamp,
				LastSeen:    feedback.Timestamp,
				Description: feedback.Description,
			}
		}
	}

	var issues []CommonIssue
	for _, issue := range issueMap {
		if issue.Count >= 2 { // Only include issues reported multiple times
			issues = append(issues, *issue)
		}
	}

	return issues
}

func (fc *FeedbackCollector) analyzeTrends(feedbacks []UserFeedback) []FeedbackTrend {
	// Simplified trend analysis - could be enhanced with more sophisticated algorithms
	trends := make([]FeedbackTrend, 0)

	// Analyze trends by type over the last 30 days
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	recentFeedback := make(map[FeedbackType]int)
	for _, feedback := range feedbacks {
		if feedback.Timestamp.After(thirtyDaysAgo) {
			recentFeedback[feedback.Type]++
		}
	}

	for feedbackType, count := range recentFeedback {
		direction := "stable"
		if count > 5 {
			direction = "increasing"
		} else if count < 2 {
			direction = "decreasing"
		}

		trends = append(trends, FeedbackTrend{
			Type:      feedbackType,
			Direction: direction,
			Change:    0, // Would need historical data for accurate calculation
			Period:    "30 days",
		})
	}

	return trends
}

func (fc *FeedbackCollector) generateRecommendations(analysis *FeedbackAnalysis) []string {
	recommendations := make([]string, 0)

	// Generate recommendations based on analysis
	if analysis.BySeverity[SeverityCritical] > 0 {
		recommendations = append(recommendations, "Address critical issues immediately - they significantly impact user experience")
	}

	if analysis.ByType[FeedbackTypeUsability] > analysis.TotalFeedback/4 {
		recommendations = append(recommendations, "Focus on usability improvements - users are struggling with the interface")
	}

	if analysis.ByType[FeedbackTypePerformance] > analysis.TotalFeedback/3 {
		recommendations = append(recommendations, "Optimize performance - users are experiencing slow operations")
	}

	if analysis.ByType[FeedbackTypeError] > analysis.TotalFeedback/2 {
		recommendations = append(recommendations, "Improve error handling and prevention - users encounter too many errors")
	}

	if len(analysis.CommonIssues) > 0 {
		recommendations = append(recommendations, "Prioritize fixing common issues that affect multiple users")
	}

	return recommendations
}

// Utility functions
func generateFeedbackID() string {
	return fmt.Sprintf("fb_%d", time.Now().UnixNano())
}

func getVersion() string {
	// Would be set during build
	return "2.0.0"
}

func getPlatform() string {
	return fmt.Sprintf("%s/%s", os.Getenv("GOOS"), os.Getenv("GOARCH"))
}

func getUserID() string {
	// Generate anonymous but consistent user ID
	hostname, _ := os.Hostname()
	return fmt.Sprintf("user_%x", []byte(hostname))
}
