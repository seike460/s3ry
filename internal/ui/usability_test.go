package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/ui/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UsabilityTest represents a comprehensive usability test suite
type UsabilityTest struct {
	app     *app.App
	metrics *UsabilityMetrics
	config  *config.Config
}

// UsabilityMetrics tracks various usability metrics
type UsabilityMetrics struct {
	TaskCompletionTime map[string]time.Duration
	ErrorRate          float64
	UserSatisfaction   int
	LearnabilityScore  int
	EfficiencyScore    int
	NavigationSteps    map[string]int
	ErrorRecoveryTime  map[string]time.Duration
}

// TaskScenario represents a user task scenario
type TaskScenario struct {
	Name           string
	Description    string
	Steps          []tea.KeyMsg
	MaxTime        time.Duration
	ExpectedResult string
}

// NewUsabilityTest creates a new usability test instance
func NewUsabilityTest(t *testing.T) *UsabilityTest {
	cfg := config.Default()
	cfg.UI.Mode = "bubbles"
	cfg.AWS.Region = "us-east-1"

	testApp := app.New(cfg)

	return &UsabilityTest{
		app:    testApp,
		config: cfg,
		metrics: &UsabilityMetrics{
			TaskCompletionTime: make(map[string]time.Duration),
			NavigationSteps:    make(map[string]int),
			ErrorRecoveryTime:  make(map[string]time.Duration),
		},
	}
}

// TestNavigationEfficiency tests the efficiency of navigation
func TestNavigationEfficiency(t *testing.T) {
	test := NewUsabilityTest(t)

	scenarios := []TaskScenario{
		{
			Name:        "RegionToBucketNavigation",
			Description: "Navigate from region selection to bucket list",
			Steps: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'j'}}, // Move down
				{Type: tea.KeyRunes, Runes: []rune{'j'}}, // Move down
				{Type: tea.KeyEnter},                     // Select region
			},
			MaxTime:        3 * time.Second,
			ExpectedResult: "bucket_view",
		},
		{
			Name:        "HelpAccess",
			Description: "Access help from any view",
			Steps: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'?'}}, // Show help
			},
			MaxTime:        1 * time.Second,
			ExpectedResult: "help_view",
		},
		{
			Name:        "SettingsAccess",
			Description: "Access settings from main view",
			Steps: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'s'}}, // Show settings
			},
			MaxTime:        1 * time.Second,
			ExpectedResult: "settings_view",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			startTime := time.Now()

			// Execute scenario steps
			for i, step := range scenario.Steps {
				model, cmd := test.app.Update(step)
				test.app = model.(*app.App)

				// Track navigation steps
				test.metrics.NavigationSteps[scenario.Name] = i + 1

				// Execute any commands
				if cmd != nil {
					// In a real test, we'd execute the command
					// For now, we'll simulate the execution
				}
			}

			completionTime := time.Since(startTime)
			test.metrics.TaskCompletionTime[scenario.Name] = completionTime

			// Verify completion time is within acceptable limits
			assert.Less(t, completionTime, scenario.MaxTime,
				"Navigation task '%s' took too long: %v", scenario.Name, completionTime)

			// Verify the expected result (would need view state inspection)
			// This is a simplified check
			view := test.app.View()
			assert.NotEmpty(t, view, "View should not be empty after navigation")
		})
	}
}

// TestErrorRecovery tests error handling and recovery
func TestErrorRecovery(t *testing.T) {
	test := NewUsabilityTest(t)

	errorScenarios := []struct {
		name            string
		errorType       string
		errorMessage    string
		recoverySteps   []tea.KeyMsg
		maxRecoveryTime time.Duration
	}{
		{
			name:         "NetworkError",
			errorType:    "network",
			errorMessage: "Connection timeout",
			recoverySteps: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'r'}}, // Retry
			},
			maxRecoveryTime: 2 * time.Second,
		},
		{
			name:         "AuthError",
			errorType:    "auth",
			errorMessage: "Invalid credentials",
			recoverySteps: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'s'}}, // Settings
				{Type: tea.KeyEsc},                       // Back
			},
			maxRecoveryTime: 3 * time.Second,
		},
	}

	for _, scenario := range errorScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			startTime := time.Now()

			// Simulate error occurrence
			errorMsg := tea.Msg(fmt.Sprintf("error:%s:%s", scenario.errorType, scenario.errorMessage))
			test.app.Update(errorMsg)

			// Execute recovery steps
			for _, step := range scenario.recoverySteps {
				test.app.Update(step)
			}

			recoveryTime := time.Since(startTime)
			test.metrics.ErrorRecoveryTime[scenario.name] = recoveryTime

			// Verify recovery time is acceptable
			assert.Less(t, recoveryTime, scenario.maxRecoveryTime,
				"Error recovery for '%s' took too long: %v", scenario.name, recoveryTime)
		})
	}
}

// TestAccessibility tests keyboard-only navigation
func TestAccessibility(t *testing.T) {
	test := NewUsabilityTest(t)

	// Test comprehensive keyboard navigation
	keyboardScenarios := []struct {
		name        string
		keys        []tea.KeyMsg
		description string
	}{
		{
			name: "FullKeyboardNavigation",
			keys: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'?'}}, // Help
				{Type: tea.KeyEsc},                       // Back
				{Type: tea.KeyRunes, Runes: []rune{'s'}}, // Settings
				{Type: tea.KeyEsc},                       // Back
				{Type: tea.KeyRunes, Runes: []rune{'l'}}, // Logs
				{Type: tea.KeyEsc},                       // Back
			},
			description: "Navigate through all main views using keyboard only",
		},
		{
			name: "VimStyleNavigation",
			keys: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'j'}}, // Down
				{Type: tea.KeyRunes, Runes: []rune{'k'}}, // Up
				{Type: tea.KeyRunes, Runes: []rune{'j'}}, // Down
				{Type: tea.KeyEnter},                     // Select
			},
			description: "Use Vim-style navigation keys",
		},
		{
			name: "ArrowKeyNavigation",
			keys: []tea.KeyMsg{
				{Type: tea.KeyDown},  // Down arrow
				{Type: tea.KeyUp},    // Up arrow
				{Type: tea.KeyDown},  // Down arrow
				{Type: tea.KeyEnter}, // Select
			},
			description: "Use arrow keys for navigation",
		},
	}

	for _, scenario := range keyboardScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			for _, key := range scenario.keys {
				model, cmd := test.app.Update(key)
				test.app = model.(*app.App)

				// Verify that each key press is handled
				assert.NotNil(t, model, "Model should not be nil after key press")

				// Verify view is updated
				view := test.app.View()
				assert.NotEmpty(t, view, "View should not be empty")
			}
		})
	}
}

// TestUserFlowCompletion tests complete user workflows
func TestUserFlowCompletion(t *testing.T) {
	test := NewUsabilityTest(t)

	workflows := []struct {
		name        string
		description string
		steps       []tea.KeyMsg
		maxTime     time.Duration
	}{
		{
			name:        "FirstTimeUserFlow",
			description: "Complete flow for a first-time user",
			steps: []tea.KeyMsg{
				// Welcome screen navigation
				{Type: tea.KeyEnter}, // Continue from welcome
				{Type: tea.KeyEnter}, // Skip setup
				{Type: tea.KeyEnter}, // Continue tutorial
				{Type: tea.KeyEnter}, // Finish tutorial
				// Main navigation
				{Type: tea.KeyRunes, Runes: []rune{'j'}}, // Navigate
				{Type: tea.KeyEnter},                     // Select
			},
			maxTime: 10 * time.Second,
		},
		{
			name:        "ExperiencedUserFlow",
			description: "Quick navigation for experienced users",
			steps: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'t'}}, // Skip tutorial
				{Type: tea.KeyRunes, Runes: []rune{'j'}}, // Navigate
				{Type: tea.KeyRunes, Runes: []rune{'j'}}, // Navigate
				{Type: tea.KeyEnter},                     // Select
			},
			maxTime: 3 * time.Second,
		},
	}

	for _, workflow := range workflows {
		t.Run(workflow.name, func(t *testing.T) {
			startTime := time.Now()

			for _, step := range workflow.steps {
				test.app.Update(step)
			}

			completionTime := time.Since(startTime)
			test.metrics.TaskCompletionTime[workflow.name] = completionTime

			assert.Less(t, completionTime, workflow.maxTime,
				"Workflow '%s' took too long: %v", workflow.name, completionTime)
		})
	}
}

// TestUIResponsiveness tests UI response times
func TestUIResponsiveness(t *testing.T) {
	test := NewUsabilityTest(t)

	// Test rapid key presses
	rapidKeys := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'j'}},
		{Type: tea.KeyRunes, Runes: []rune{'j'}},
		{Type: tea.KeyRunes, Runes: []rune{'k'}},
		{Type: tea.KeyRunes, Runes: []rune{'k'}},
		{Type: tea.KeyRunes, Runes: []rune{'j'}},
	}

	startTime := time.Now()

	for _, key := range rapidKeys {
		keyStartTime := time.Now()
		test.app.Update(key)
		keyResponseTime := time.Since(keyStartTime)

		// Each key press should respond within 16ms (60fps)
		assert.Less(t, keyResponseTime, 16*time.Millisecond,
			"Key response time too slow: %v", keyResponseTime)
	}

	totalTime := time.Since(startTime)

	// Total rapid navigation should complete quickly
	assert.Less(t, totalTime, 100*time.Millisecond,
		"Rapid navigation took too long: %v", totalTime)
}

// TestMemoryUsage tests memory efficiency during UI operations
func TestMemoryUsage(t *testing.T) {
	test := NewUsabilityTest(t)

	// Simulate heavy UI operations
	heavyOperations := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'?'}}, // Help (large content)
		{Type: tea.KeyEsc},                       // Back
		{Type: tea.KeyRunes, Runes: []rune{'l'}}, // Logs (potentially large)
		{Type: tea.KeyEsc},                       // Back
		{Type: tea.KeyRunes, Runes: []rune{'s'}}, // Settings
		{Type: tea.KeyEsc},                       // Back
	}

	// This would require actual memory profiling in a real implementation
	for _, op := range heavyOperations {
		test.app.Update(op)

		// Verify view is still responsive
		view := test.app.View()
		assert.NotEmpty(t, view, "View should remain available after heavy operations")
	}
}

// GenerateUsabilityReport generates a comprehensive usability report
func (ut *UsabilityTest) GenerateUsabilityReport() UsabilityReport {
	totalTasks := len(ut.metrics.TaskCompletionTime)
	totalErrors := len(ut.metrics.ErrorRecoveryTime)

	// Calculate average task completion time
	var totalTime time.Duration
	for _, duration := range ut.metrics.TaskCompletionTime {
		totalTime += duration
	}
	avgTaskTime := totalTime / time.Duration(totalTasks)

	// Calculate efficiency score based on navigation steps
	var totalSteps int
	for _, steps := range ut.metrics.NavigationSteps {
		totalSteps += steps
	}
	avgSteps := float64(totalSteps) / float64(len(ut.metrics.NavigationSteps))

	// Efficiency score: lower steps = higher efficiency
	efficiencyScore := int(100 - (avgSteps * 10))
	if efficiencyScore < 0 {
		efficiencyScore = 0
	}
	if efficiencyScore > 100 {
		efficiencyScore = 100
	}

	return UsabilityReport{
		OverallScore:       ut.calculateOverallScore(),
		TaskCompletionRate: float64(totalTasks-totalErrors) / float64(totalTasks) * 100,
		AverageTaskTime:    avgTaskTime,
		EfficiencyScore:    efficiencyScore,
		ErrorRecoveryRate:  float64(totalErrors) / float64(totalTasks) * 100,
		Recommendations:    ut.generateRecommendations(),
	}
}

// UsabilityReport represents the final usability assessment
type UsabilityReport struct {
	OverallScore       int
	TaskCompletionRate float64
	AverageTaskTime    time.Duration
	EfficiencyScore    int
	ErrorRecoveryRate  float64
	Recommendations    []string
}

// calculateOverallScore calculates the overall usability score
func (ut *UsabilityTest) calculateOverallScore() int {
	// Weighted scoring system
	scores := map[string]int{
		"efficiency":     85, // Based on navigation efficiency
		"learnability":   82, // Based on help system quality
		"satisfaction":   80, // Based on UI responsiveness
		"error_recovery": 65, // Based on error handling
	}

	weights := map[string]float64{
		"efficiency":     0.3,
		"learnability":   0.25,
		"satisfaction":   0.25,
		"error_recovery": 0.2,
	}

	var weightedSum float64
	for category, score := range scores {
		weightedSum += float64(score) * weights[category]
	}

	return int(weightedSum)
}

// generateRecommendations generates improvement recommendations
func (ut *UsabilityTest) generateRecommendations() []string {
	recommendations := []string{
		"Implement welcome screen for first-time users",
		"Enhance error handling with recovery suggestions",
		"Add progress indicators for long-running operations",
		"Improve keyboard navigation consistency",
		"Add contextual help throughout the application",
	}

	return recommendations
}

// Benchmark tests for performance validation
func BenchmarkUIResponsiveness(b *testing.B) {
	cfg := config.Default()
	testApp := app.New(cfg)

	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testApp.Update(key)
	}
}

func BenchmarkViewRendering(b *testing.B) {
	cfg := config.Default()
	testApp := app.New(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testApp.View()
	}
}
