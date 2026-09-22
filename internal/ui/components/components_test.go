package components

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestErrorDisplayAddAndClear(t *testing.T) {
	e := NewErrorDisplay()
	if e.GetErrorCount() != 0 {
		t.Fatal("new display should be empty")
	}

	e.AddError(ErrorLevelWarning, "t", "m", "s", "tech", true)
	e.AddError(ErrorLevelError, "t2", "m2", "s2", "", false)

	if e.GetErrorCount() != 2 {
		t.Fatalf("count = %d, want 2", e.GetErrorCount())
	}
	if latest := e.GetLatestError(); latest == nil || latest.Title != "t2" {
		t.Fatalf("latest = %+v, want newest first", latest)
	}
	if e.HasCriticalErrors() {
		t.Fatal("no critical errors registered")
	}
	if !strings.Contains(e.View(), "t2") {
		t.Fatal("View did not render the latest error")
	}

	e.ClearErrors()
	if e.GetErrorCount() != 0 || e.View() != "" {
		t.Fatal("ClearErrors did not empty the display")
	}
}

func TestErrorDisplayMaxErrors(t *testing.T) {
	e := NewErrorDisplay()
	e.SetMaxErrors(2)
	for i := 0; i < 5; i++ {
		e.AddError(ErrorLevelInfo, "t", "m", "", "", true)
	}
	if e.GetErrorCount() != 2 {
		t.Fatalf("count = %d, want capped at 2", e.GetErrorCount())
	}
}

func TestErrorDisplayCategorizesAWSErrors(t *testing.T) {
	e := NewErrorDisplay()
	e.AddAWSError(errors.New("NoSuchBucket: nope"))
	if got := e.GetLatestError().Title; got != "S3 Bucket Not Found" {
		t.Fatalf("title = %q", got)
	}
	e.AddAWSError(errors.New("RequestLimitExceeded"))
	if got := e.GetLatestError().Level; got != ErrorLevelWarning {
		t.Fatalf("rate limit level = %v, want warning", got)
	}
	e.AddAWSError(nil)
	if e.GetErrorCount() != 2 {
		t.Fatal("nil error was recorded")
	}
}

func TestErrorDisplayTechnicalAndAutoHide(t *testing.T) {
	e := NewErrorDisplay()
	e.SetShowTechnical(true)
	e.AddError(ErrorLevelError, "t", "m", "s", "stack", true)
	if !strings.Contains(e.View(), "stack") {
		t.Fatal("technical details were not rendered")
	}

	fresh := NewErrorDisplay()
	fresh.SetAutoHide(true, time.Nanosecond)
	fresh.AddError(ErrorLevelInfo, "stale", "m", "", "", true)
	fresh.errors[0].Timestamp = time.Now().Add(-time.Hour)
	fresh.Update(nil)
	if fresh.GetErrorCount() != 0 {
		t.Fatal("auto-hide did not drop the old info error")
	}
}

func TestErrorDisplayRecoveryActions(t *testing.T) {
	e := NewErrorDisplay()
	e.AddErrorWithActions(ErrorLevelError, "t", "m", "s", "", true, []RecoveryAction{
		{Label: "Retry", Description: "try again", Shortcut: "r"},
	})
	view := e.View()
	if !strings.Contains(view, "Available Actions") || !strings.Contains(view, "Retry") {
		t.Fatal("recovery actions were not rendered")
	}
}

func TestErrorDisplayNetworkAndValidation(t *testing.T) {
	e := NewErrorDisplay()
	e.AddNetworkError("list", errors.New("boom"))
	e.AddValidationError("bucket", "Bad_Name", "lowercase only")
	if e.GetErrorCount() != 2 {
		t.Fatalf("count = %d, want 2", e.GetErrorCount())
	}
	e.AddNetworkError("list", nil)
	if e.GetErrorCount() != 3 {
		t.Fatal("nil network error was not recorded")
	}
}

func TestSpinnerLifecycle(t *testing.T) {
	s := NewSpinner("loading")
	if !s.IsActive() {
		t.Fatal("new spinner should be active")
	}
	if s.View() == "" {
		t.Fatal("active spinner rendered nothing")
	}

	s.Stop()
	if s.IsActive() || s.View() != "" {
		t.Fatal("stopped spinner still renders")
	}

	s.Start()
	if !s.IsActive() {
		t.Fatal("Start did not reactivate")
	}

	s.SetMessage("working")
	if !strings.Contains(s.View(), "working") {
		t.Fatal("SetMessage not reflected in View")
	}

	updated, cmd := s.Update(SpinnerTickMsg(time.Now()))
	if updated != s || cmd == nil {
		t.Fatal("active spinner tick did not reschedule")
	}
}

func TestSpinnerDotVariantAndSettings(t *testing.T) {
	s := NewDotSpinner("dots")
	s.SetFrameRate(30)
	if s.GetFrameRate() != 30 {
		t.Fatalf("frame rate = %d, want 30", s.GetFrameRate())
	}
	info := s.GetPerformanceInfo()
	if info["target_fps"] != 30 || info["active"] != true {
		t.Fatalf("performance info = %v", info)
	}
}

func TestProgressUpdateAndComplete(t *testing.T) {
	p := NewProgress("job", 100)
	p, _ = p.Update(ProgressMsg{Current: 50, Total: 100, Message: "half"})
	if !strings.Contains(p.View(), "50.0%") {
		t.Fatalf("View missing percentage: %q", p.View())
	}
	if p.IsCompleted() {
		t.Fatal("progress completed early")
	}

	p, _ = p.Update(CompletedMsg{Success: true, Message: "done"})
	if !p.IsCompleted() || !p.IsSuccess() {
		t.Fatal("CompletedMsg not applied")
	}
	if !strings.Contains(p.View(), "done") {
		t.Fatal("completion message not rendered")
	}

	p.Complete(false, "failed")
	if p.IsSuccess() {
		t.Fatal("failure marked as success")
	}
}

func TestProgressNarrowWidthAndOverflowDoNotPanic(t *testing.T) {
	// A very narrow terminal must still render a bar instead of feeding a
	// negative count to strings.Repeat.
	p := NewProgress("job", 100)
	p, _ = p.Update(tea.WindowSizeMsg{Width: 10, Height: 5})
	p.SetProgress(50, 100, "half")
	if p.View() == "" {
		t.Fatal("narrow progress rendered nothing")
	}

	// Overshooting the total must clamp at 100% instead of panicking.
	q := NewProgress("job", 100)
	q, _ = q.Update(ProgressMsg{Current: 250, Total: 100, Message: "over"})
	if !strings.Contains(q.View(), "100.0%") {
		t.Fatalf("overflowing progress not clamped: %q", q.View())
	}
}

func TestProgressSpeedSampling(t *testing.T) {
	p := NewProgress("job", 1000)
	p.SetProgress(10, 1000, "")
	time.Sleep(time.Millisecond)
	p.SetProgress(100, 1000, "")
	if p.GetCurrentSpeed() <= 0 {
		t.Fatal("speed was not calculated")
	}
	p.SetProgress(200, 1000, "")
	if p.GetAverageSpeed() <= 0 {
		t.Fatal("average speed was not calculated")
	}
}

func TestPreviewRendering(t *testing.T) {
	p := NewPreview()
	if !strings.Contains(p.View(), "No content") {
		t.Fatal("empty preview should say so")
	}

	p, _ = p.Update(PreviewMsg{Content: "hello", PreviewType: PreviewTypeText})
	if !strings.Contains(p.View(), "hello") {
		t.Fatal("text content not rendered")
	}

	p, _ = p.Update(PreviewMsg{Error: errors.New("nope")})
	if !strings.Contains(p.View(), "nope") {
		t.Fatal("error not rendered")
	}
}

func TestPreviewTruncatesToHeight(t *testing.T) {
	p := NewPreview()
	p, _ = p.Update(tea.WindowSizeMsg{Width: 80, Height: 6})
	content := strings.Repeat("line\n", 50)
	p, _ = p.Update(PreviewMsg{Content: content, PreviewType: PreviewTypeText})
	if !strings.Contains(p.View(), "truncated") {
		t.Fatal("long content was not truncated to height")
	}
}

func TestFormatBytesComponent(t *testing.T) {
	if got := formatBytes(2048); got != "2.0 KB" {
		t.Fatalf("formatBytes(2048) = %q", got)
	}
	if got := formatBytes(5); got != "5 B" {
		t.Fatalf("formatBytes(5) = %q", got)
	}
}
