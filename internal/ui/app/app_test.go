package app

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/ui/views"
)

func testDeps() views.Deps {
	return views.Deps{Config: config.Default(), Timeout: time.Second}
}

func TestNewStartsAtBucketView(t *testing.T) {
	app := New(testDeps())
	if got := fmt.Sprintf("%T", app.view); got != "*views.BucketView" {
		t.Fatalf("initial view = %s, want *views.BucketView", got)
	}
	if app.Init() == nil {
		t.Fatal("Init returned nil; bucket view should start loading")
	}
}

func TestUpdateGlobalKeys(t *testing.T) {
	app := New(testDeps())

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if model != app || cmd == nil {
		t.Fatal("ctrl+c did not quit")
	}

	app.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	if got := fmt.Sprintf("%T", app.view); got != "*views.HelpView" {
		t.Fatalf("ctrl+h view = %s, want *views.HelpView", got)
	}

	app.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if got := fmt.Sprintf("%T", app.view); got != "*views.SettingsView" {
		t.Fatalf("ctrl+s view = %s, want *views.SettingsView", got)
	}
}

func TestUpdateInitsOnViewTransition(t *testing.T) {
	app := New(testDeps())

	// esc inside the settings view goes back to the bucket view; the app
	// must then call Init so the bucket list reloads.
	app.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := fmt.Sprintf("%T", app.view); got != "*views.BucketView" {
		t.Fatalf("esc view = %s, want *views.BucketView", got)
	}
	if cmd == nil {
		t.Fatal("view transition did not schedule the new view's Init")
	}
}

func TestViewDelegates(t *testing.T) {
	app := New(testDeps())
	if app.View() == "" {
		t.Fatal("View rendered nothing")
	}
}

func TestRunRejectsBadConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Performance.PartSize = 1 // below the 5 MiB minimum
	if err := Run(t.Context(), cfg); err == nil {
		t.Fatal("Run accepted an invalid part size")
	}
}
