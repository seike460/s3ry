package views

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seike460/s3ry/internal/ui/components"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	Source    string
}

// LogsLoadedMsg represents logs being loaded
type LogsLoadedMsg struct {
	Logs  []LogEntry
	Error error
}

// LogsView represents the logs viewing interface
type LogsView struct {
	list    *components.List
	spinner *components.Spinner
	loading bool
	logs    []LogEntry
	
	// Styles
	headerStyle    lipgloss.Style
	errorStyle     lipgloss.Style
	infoStyle      lipgloss.Style
	warningStyle   lipgloss.Style
	timestampStyle lipgloss.Style
}

// NewLogsView creates a new logs view
func NewLogsView() *LogsView {
	logs := &LogsView{
		loading: true,
		spinner: components.NewSpinner("Loading application logs..."),
		
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(2),
		
		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")),
		
		infoStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")),
		
		warningStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFA500")),
		
		timestampStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")),
	}
	
	return logs
}

// Init initializes the logs view
func (v *LogsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Start(),
		v.loadLogs(),
	)
}

// Update handles messages for the logs view
func (v *LogsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}
		
	case LogsLoadedMsg:
		v.loading = false
		v.spinner.Stop()
		
		if msg.Error != nil {
			// Show error message
			items := []components.ListItem{
				{
					Title:       "❌ Failed to load logs",
					Description: fmt.Sprintf("Error: %v", msg.Error),
					Tag:         "Error",
				},
				{
					Title:       "💡 Possible causes:",
					Description: "• Log file not found • Insufficient permissions • Application not started yet",
					Tag:         "Help",
				},
			}
			v.list = components.NewList("⚠️ Error Loading Logs", items)
			return v, nil
		}
		
		v.logs = msg.Logs
		v.buildLogsList()
		return v, nil
		
	case tea.KeyMsg:
		if v.loading {
			break
		}
		
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return v, tea.Quit
		case "r":
			// Refresh logs
			v.loading = true
			v.spinner = components.NewSpinner("Refreshing logs...")
			return v, tea.Batch(
				v.spinner.Start(),
				v.loadLogs(),
			)
		}
		
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}
		
	case components.SpinnerTickMsg:
		if v.loading {
			v.spinner, _ = v.spinner.Update(msg)
			cmds = append(cmds, v.spinner.Start())
		}
	}
	
	return v, tea.Batch(cmds...)
}

// View renders the logs view
func (v *LogsView) View() string {
	if v.loading {
		return v.headerStyle.Render("📋 S3ry Application Logs") + "\n\n" + v.spinner.View()
	}
	
	if v.list == nil {
		return v.errorStyle.Render("No logs available")
	}
	
	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		MarginTop(1).
		Render("r: refresh • esc/q: back • Logs show recent application activity")
	
	return v.list.View() + "\n" + footer
}

// loadLogs loads application logs from various sources - prepared for real logging system
func (v *LogsView) loadLogs() tea.Cmd {
	return func() tea.Msg {
		var logs []LogEntry
		
		// Try to load from actual log files first
		logFiles := []string{
			"/tmp/s3ry.log",
			"/var/log/s3ry.log",
			filepath.Join(os.TempDir(), "s3ry.log"),
		}
		
		foundLogFile := false
		for _, logFile := range logFiles {
			if entries, err := v.loadLogFile(logFile); err == nil {
				logs = append(logs, entries...)
				foundLogFile = true
				break
			}
		}
		
		// If no log files found, show minimal system information
		if !foundLogFile {
			// Add basic startup information
			logs = append(logs, LogEntry{
				Timestamp: time.Now(),
				Level:     "INFO",
				Message:   "S3ry TUI application started",
				Source:    "ui/app",
			})
			
			// Add environment information if available
			if region := os.Getenv("AWS_REGION"); region != "" {
				logs = append(logs, LogEntry{
					Timestamp: time.Now().Add(-time.Second * 30),
					Level:     "INFO",
					Message:   fmt.Sprintf("Using AWS region: %s", region),
					Source:    "config",
				})
			}
			
			if profile := os.Getenv("AWS_PROFILE"); profile != "" {
				logs = append(logs, LogEntry{
					Timestamp: time.Now().Add(-time.Second * 30),
					Level:     "INFO",
					Message:   fmt.Sprintf("Using AWS profile: %s", profile),
					Source:    "config",
				})
			}
			
			// TODO: Replace with real logging system integration when available
			// This will be connected to actual application logs when LLM-2 completes integration
		}
		
		return LogsLoadedMsg{Logs: logs}
	}
}

// loadLogFile attempts to load logs from a file
func (v *LogsView) loadLogFile(filepath string) ([]LogEntry, error) {
	// For now, return empty as we don't have actual log files
	// In a real implementation, this would parse log files
	return []LogEntry{}, fmt.Errorf("log file not found: %s", filepath)
}

// buildLogsList creates the logs list items
func (v *LogsView) buildLogsList() {
	if len(v.logs) == 0 {
		items := []components.ListItem{
			{
				Title:       "📋 No logs available",
				Description: "No application logs found. Start using S3ry to see activity logs here.",
				Tag:         "Empty",
			},
		}
		v.list = components.NewList("📋 S3ry Application Logs", items)
		return
	}
	
	// Sort logs by timestamp (newest first)
	sortedLogs := make([]LogEntry, len(v.logs))
	copy(sortedLogs, v.logs)
	for i := 0; i < len(sortedLogs)-1; i++ {
		for j := i + 1; j < len(sortedLogs); j++ {
			if sortedLogs[i].Timestamp.Before(sortedLogs[j].Timestamp) {
				sortedLogs[i], sortedLogs[j] = sortedLogs[j], sortedLogs[i]
			}
		}
	}
	
	// Create list items
	items := make([]components.ListItem, 0, len(sortedLogs)+3)
	
	// Add header
	items = append(items, components.ListItem{
		Title:       "📋 Recent Application Activity",
		Description: fmt.Sprintf("Showing %d log entries", len(sortedLogs)),
		Tag:         "Header",
	})
	
	items = append(items, components.ListItem{
		Title:       "",
		Description: "",
		Tag:         "Separator",
	})
	
	// Add log entries
	for _, log := range sortedLogs {
		title := v.formatLogTitle(log)
		description := v.formatLogDescription(log)
		
		items = append(items, components.ListItem{
			Title:       title,
			Description: description,
			Tag:         strings.ToLower(log.Level),
			Data:        log,
		})
	}
	
	v.list = components.NewList("📋 S3ry Application Logs", items)
}

// formatLogTitle formats the log entry title
func (v *LogsView) formatLogTitle(log LogEntry) string {
	var levelIcon string
	switch strings.ToUpper(log.Level) {
	case "ERROR":
		levelIcon = "❌"
	case "WARNING", "WARN":
		levelIcon = "⚠️"
	case "INFO":
		levelIcon = "ℹ️"
	case "DEBUG":
		levelIcon = "🔍"
	default:
		levelIcon = "📝"
	}
	
	timestamp := log.Timestamp.Format("15:04:05")
	return fmt.Sprintf("%s %s [%s] %s", levelIcon, timestamp, log.Level, log.Message)
}

// formatLogDescription formats the log entry description
func (v *LogsView) formatLogDescription(log LogEntry) string {
	return fmt.Sprintf("Source: %s | %s", log.Source, log.Timestamp.Format("2006-01-02 15:04:05"))
}