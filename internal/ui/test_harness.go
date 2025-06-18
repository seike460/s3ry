package ui

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seike460/s3ry/internal/ui/components"
)

// TestHarness provides performance testing capabilities for UI components
type TestHarness struct {
	startTime      time.Time
	endTime        time.Time
	metrics        []PerformanceMetric
	mu             sync.RWMutex
	maxSamples     int
	sampleInterval time.Duration
	running        bool
}

// PerformanceMetric represents a single performance measurement
type PerformanceMetric struct {
	Timestamp      time.Time
	FrameRate      float64
	MemoryUsage    uint64
	GoroutineCount int
	RenderTime     time.Duration
	UpdateTime     time.Duration
	ComponentType  string
	ItemCount      int
	CPUUsage       float64
}

// TestResult represents the results of a performance test
type TestResult struct {
	TestName          string
	Duration          time.Duration
	SampleCount       int
	AverageFrameRate  float64
	MinFrameRate      float64
	MaxFrameRate      float64
	AverageMemory     uint64
	MaxMemory         uint64
	AverageRenderTime time.Duration
	MaxRenderTime     time.Duration
	TargetsMet        map[string]bool
	Success           bool
	Details           map[string]interface{}
}

// NewTestHarness creates a new performance test harness
func NewTestHarness() *TestHarness {
	return &TestHarness{
		metrics:        make([]PerformanceMetric, 0),
		maxSamples:     1000,                  // Keep last 1000 samples
		sampleInterval: time.Millisecond * 16, // 60fps sampling
	}
}

// StartTest begins performance monitoring
func (th *TestHarness) StartTest(testName string) {
	th.mu.Lock()
	defer th.mu.Unlock()

	th.startTime = time.Now()
	th.running = true
	th.metrics = make([]PerformanceMetric, 0)

	fmt.Printf("🚀 Starting performance test: %s\n", testName)
}

// StopTest ends performance monitoring and returns results
func (th *TestHarness) StopTest(testName string) TestResult {
	th.mu.Lock()
	defer th.mu.Unlock()

	th.endTime = time.Now()
	th.running = false

	result := th.calculateResults(testName)
	th.printResults(result)

	return result
}

// RecordMetric records a performance metric
func (th *TestHarness) RecordMetric(metric PerformanceMetric) {
	th.mu.Lock()
	defer th.mu.Unlock()

	if !th.running {
		return
	}

	metric.Timestamp = time.Now()
	th.metrics = append(th.metrics, metric)

	// Maintain max samples
	if len(th.metrics) > th.maxSamples {
		th.metrics = th.metrics[1:]
	}
}

// TestListPerformance tests list component performance with large datasets
func (th *TestHarness) TestListPerformance(itemCounts []int) map[int]TestResult {
	results := make(map[int]TestResult)

	for _, count := range itemCounts {
		testName := fmt.Sprintf("List Performance - %d items", count)
		th.StartTest(testName)

		// Create test items
		items := make([]components.ListItem, count)
		for i := 0; i < count; i++ {
			items[i] = components.ListItem{
				Title:       fmt.Sprintf("Test Item %d", i),
				Description: fmt.Sprintf("Description for item %d with some longer text to test rendering", i),
				Tag:         fmt.Sprintf("tag%d", i%10),
				Data:        i,
			}
		}

		// Create list component
		list := components.NewList("Performance Test", items)
		list.OptimizeForLargeList()

		// Simulate window size
		list.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

		// Run performance test
		th.runListOperations(list, count)

		results[count] = th.StopTest(testName)
	}

	return results
}

// TestSpinnerPerformance tests spinner component at different frame rates
func (th *TestHarness) TestSpinnerPerformance(targetFPS []int) map[int]TestResult {
	results := make(map[int]TestResult)

	for _, fps := range targetFPS {
		testName := fmt.Sprintf("Spinner Performance - %d FPS", fps)
		th.StartTest(testName)

		spinner := components.NewSpinner("Performance Test")
		spinner.SetFrameRate(fps)

		// Run spinner for 5 seconds
		th.runSpinnerTest(spinner, fps, 5*time.Second)

		results[fps] = th.StopTest(testName)
	}

	return results
}

// TestProgressPerformance tests progress component with real-time updates
func (th *TestHarness) TestProgressPerformance(updateIntervals []time.Duration) map[time.Duration]TestResult {
	results := make(map[time.Duration]TestResult)

	for _, interval := range updateIntervals {
		testName := fmt.Sprintf("Progress Performance - %v updates", interval)
		th.StartTest(testName)

		progress := components.NewProgress("Performance Test", 1000000)

		// Run progress updates for 10 seconds
		th.runProgressTest(progress, interval, 10*time.Second)

		results[interval] = th.StopTest(testName)
	}

	return results
}

// TestErrorDisplayPerformance tests error display with multiple errors
func (th *TestHarness) TestErrorDisplayPerformance(errorCounts []int) map[int]TestResult {
	results := make(map[int]TestResult)

	for _, count := range errorCounts {
		testName := fmt.Sprintf("Error Display Performance - %d errors", count)
		th.StartTest(testName)

		errorDisplay := components.NewErrorDisplay()

		// Add multiple errors
		for i := 0; i < count; i++ {
			level := components.ErrorLevel(i % 4) // Cycle through error levels
			errorDisplay.AddError(
				level,
				fmt.Sprintf("Test Error %d", i),
				fmt.Sprintf("Error message %d", i),
				fmt.Sprintf("Suggestion %d", i),
				fmt.Sprintf("Technical details %d", i),
				true,
			)
		}

		// Test rendering performance
		th.runErrorDisplayTest(errorDisplay, count)

		results[count] = th.StopTest(testName)
	}

	return results
}

// RunComprehensiveTest runs all performance tests
func (th *TestHarness) RunComprehensiveTest() map[string]interface{} {
	fmt.Println("🔍 Running comprehensive UI performance test suite...")

	results := make(map[string]interface{})

	// Test list performance with different item counts
	fmt.Println("\n📋 Testing List Component Performance...")
	listResults := th.TestListPerformance([]int{100, 1000, 5000, 10000, 50000})
	results["list_performance"] = listResults

	// Test spinner performance at different frame rates
	fmt.Println("\n⏳ Testing Spinner Component Performance...")
	spinnerResults := th.TestSpinnerPerformance([]int{30, 60, 120})
	results["spinner_performance"] = spinnerResults

	// Test progress component performance
	fmt.Println("\n📊 Testing Progress Component Performance...")
	progressResults := th.TestProgressPerformance([]time.Duration{
		time.Millisecond * 16,  // 60fps
		time.Millisecond * 33,  // 30fps
		time.Millisecond * 100, // 10fps
	})
	results["progress_performance"] = progressResults

	// Test error display performance
	fmt.Println("\n❌ Testing Error Display Performance...")
	errorResults := th.TestErrorDisplayPerformance([]int{1, 5, 10, 20})
	results["error_display_performance"] = errorResults

	// Generate summary
	summary := th.generateSummary(results)
	results["summary"] = summary

	fmt.Println("\n✅ Comprehensive performance test completed!")
	return results
}

// Helper methods for running specific tests

func (th *TestHarness) runListOperations(list *components.List, itemCount int) {
	start := time.Now()

	// Simulate various list operations
	operations := []string{"down", "up", "page_down", "page_up", "home", "end"}

	for i := 0; i < 100; i++ { // Run 100 operations
		operation := operations[i%len(operations)]

		renderStart := time.Now()
		list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(operation)})
		view := list.View()
		renderTime := time.Since(renderStart)

		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		metric := PerformanceMetric{
			MemoryUsage:    m.Alloc,
			GoroutineCount: runtime.NumGoroutine(),
			RenderTime:     renderTime,
			ComponentType:  "List",
			ItemCount:      itemCount,
		}

		// Calculate frame rate
		if i > 0 {
			metric.FrameRate = 1.0 / time.Since(start).Seconds()
		}

		th.RecordMetric(metric)

		// Prevent from using too much CPU
		if len(view) > 0 {
			time.Sleep(time.Millisecond)
		}
	}
}

func (th *TestHarness) runSpinnerTest(spinner *components.Spinner, targetFPS int, duration time.Duration) {
	start := time.Now()
	frameInterval := time.Duration(1000/targetFPS) * time.Millisecond

	for time.Since(start) < duration {
		renderStart := time.Now()
		spinner.Update(components.SpinnerTickMsg(time.Now()))
		spinner.View()
		renderTime := time.Since(renderStart)

		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		metric := PerformanceMetric{
			FrameRate:      float64(targetFPS),
			MemoryUsage:    m.Alloc,
			GoroutineCount: runtime.NumGoroutine(),
			RenderTime:     renderTime,
			ComponentType:  "Spinner",
		}

		th.RecordMetric(metric)

		time.Sleep(frameInterval)
	}
}

func (th *TestHarness) runProgressTest(progress *components.Progress, updateInterval time.Duration, duration time.Duration) {
	start := time.Now()
	total := int64(1000000)
	current := int64(0)

	for time.Since(start) < duration {
		renderStart := time.Now()

		// Update progress
		current += 10000
		if current > total {
			current = 0
		}

		progress.SetProgress(current, total, fmt.Sprintf("Progress: %d/%d", current, total))
		progress.View()
		renderTime := time.Since(renderStart)

		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		metric := PerformanceMetric{
			MemoryUsage:    m.Alloc,
			GoroutineCount: runtime.NumGoroutine(),
			RenderTime:     renderTime,
			ComponentType:  "Progress",
		}

		th.RecordMetric(metric)

		time.Sleep(updateInterval)
	}
}

func (th *TestHarness) runErrorDisplayTest(errorDisplay *components.ErrorDisplay, errorCount int) {
	for i := 0; i < 50; i++ { // 50 render cycles
		renderStart := time.Now()
		errorDisplay.View()
		renderTime := time.Since(renderStart)

		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		metric := PerformanceMetric{
			MemoryUsage:    m.Alloc,
			GoroutineCount: runtime.NumGoroutine(),
			RenderTime:     renderTime,
			ComponentType:  "ErrorDisplay",
			ItemCount:      errorCount,
		}

		th.RecordMetric(metric)

		time.Sleep(time.Millisecond * 16) // 60fps
	}
}

// Analysis methods

func (th *TestHarness) calculateResults(testName string) TestResult {
	if len(th.metrics) == 0 {
		return TestResult{
			TestName: testName,
			Success:  false,
		}
	}

	duration := th.endTime.Sub(th.startTime)

	// Calculate statistics
	var totalFrameRate, minFrameRate, maxFrameRate float64
	var totalMemory, maxMemory uint64
	var totalRenderTime, maxRenderTime time.Duration

	minFrameRate = 999999

	for _, metric := range th.metrics {
		totalFrameRate += metric.FrameRate
		totalMemory += metric.MemoryUsage
		totalRenderTime += metric.RenderTime

		if metric.FrameRate < minFrameRate {
			minFrameRate = metric.FrameRate
		}
		if metric.FrameRate > maxFrameRate {
			maxFrameRate = metric.FrameRate
		}
		if metric.MemoryUsage > maxMemory {
			maxMemory = metric.MemoryUsage
		}
		if metric.RenderTime > maxRenderTime {
			maxRenderTime = metric.RenderTime
		}
	}

	sampleCount := len(th.metrics)
	avgFrameRate := totalFrameRate / float64(sampleCount)
	avgMemory := totalMemory / uint64(sampleCount)
	avgRenderTime := totalRenderTime / time.Duration(sampleCount)

	// Check if targets are met
	targetsMet := map[string]bool{
		"60fps_avg":    avgFrameRate >= 60.0,
		"60fps_min":    minFrameRate >= 30.0,                // Min should be at least 30fps
		"memory_limit": maxMemory < 100*1024*1024,           // 100MB limit
		"render_speed": avgRenderTime < time.Millisecond*10, // 10ms render limit
	}

	success := true
	for _, met := range targetsMet {
		if !met {
			success = false
			break
		}
	}

	return TestResult{
		TestName:          testName,
		Duration:          duration,
		SampleCount:       sampleCount,
		AverageFrameRate:  avgFrameRate,
		MinFrameRate:      minFrameRate,
		MaxFrameRate:      maxFrameRate,
		AverageMemory:     avgMemory,
		MaxMemory:         maxMemory,
		AverageRenderTime: avgRenderTime,
		MaxRenderTime:     maxRenderTime,
		TargetsMet:        targetsMet,
		Success:           success,
		Details: map[string]interface{}{
			"total_metrics": sampleCount,
			"test_duration": duration.String(),
		},
	}
}

func (th *TestHarness) printResults(result TestResult) {
	fmt.Printf("\n📊 Test Results: %s\n", result.TestName)
	fmt.Printf("   Duration: %v\n", result.Duration)
	fmt.Printf("   Samples: %d\n", result.SampleCount)
	fmt.Printf("   Frame Rate: %.1f avg (%.1f min, %.1f max)\n",
		result.AverageFrameRate, result.MinFrameRate, result.MaxFrameRate)
	fmt.Printf("   Memory: %.1f MB avg (%.1f MB max)\n",
		float64(result.AverageMemory)/(1024*1024), float64(result.MaxMemory)/(1024*1024))
	fmt.Printf("   Render Time: %v avg (%v max)\n",
		result.AverageRenderTime, result.MaxRenderTime)

	fmt.Printf("   Targets Met:\n")
	for target, met := range result.TargetsMet {
		status := "❌"
		if met {
			status = "✅"
		}
		fmt.Printf("     %s %s\n", status, target)
	}

	if result.Success {
		fmt.Printf("   Overall: ✅ PASSED\n")
	} else {
		fmt.Printf("   Overall: ❌ FAILED\n")
	}
}

func (th *TestHarness) generateSummary(results map[string]interface{}) map[string]interface{} {
	summary := map[string]interface{}{
		"total_tests":     0,
		"passed_tests":    0,
		"failed_tests":    0,
		"overall_success": true,
	}

	// Count results from all test categories
	for category, categoryResults := range results {
		if category == "summary" {
			continue
		}

		switch cr := categoryResults.(type) {
		case map[int]TestResult:
			for _, result := range cr {
				summary["total_tests"] = summary["total_tests"].(int) + 1
				if result.Success {
					summary["passed_tests"] = summary["passed_tests"].(int) + 1
				} else {
					summary["failed_tests"] = summary["failed_tests"].(int) + 1
					summary["overall_success"] = false
				}
			}
		case map[time.Duration]TestResult:
			for _, result := range cr {
				summary["total_tests"] = summary["total_tests"].(int) + 1
				if result.Success {
					summary["passed_tests"] = summary["passed_tests"].(int) + 1
				} else {
					summary["failed_tests"] = summary["failed_tests"].(int) + 1
					summary["overall_success"] = false
				}
			}
		}
	}

	return summary
}
