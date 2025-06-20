package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	internalS3 "github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
	"github.com/seike460/s3ry/pkg/interfaces"
)

// ObjectsLoadedMsg represents objects being loaded
type ObjectsLoadedMsg struct {
	Objects []interfaces.ObjectInfo
	Error   error
}

// ObjectView represents the object selection view
type ObjectView struct {
	list        *components.List
	spinner     *components.Spinner
	progress    *components.Progress
	preview     *components.Preview
	loading     bool
	processing  bool
	showPreview bool
	width       int
	height      int
	region      string
	bucket      string
	operation   string // "download" or "delete"
	s3Client    interfaces.S3Client

	// Styles
	headerStyle lipgloss.Style
	errorStyle  lipgloss.Style
}

// NewObjectView creates a new object view with enhanced performance
func NewObjectView(region, bucket, operation string) *ObjectView {
	// Create S3 client for basic S3/MinIO operations
	s3Client := internalS3.NewClient(region)

	var spinnerMsg string
	switch operation {
	case "download":
		spinnerMsg = "Loading S3 objects for download..."
	case "delete":
		spinnerMsg = "Loading S3 objects for delete..."
	default:
		spinnerMsg = "Loading S3 objects..."
	}

	return &ObjectView{
		region:    region,
		bucket:    bucket,
		operation: operation,
		s3Client:  s3Client,
		loading:   true,
		spinner:   components.NewSpinner(spinnerMsg),
		preview:   components.NewPreview(),

		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(2),

		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")).
			MarginTop(1),
	}
}

// Init initializes the object view
func (v *ObjectView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Start(),
		v.loadObjects(),
	)
}

// Update handles messages for the object view
func (v *ObjectView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}
		if v.progress != nil {
			v.progress, _ = v.progress.Update(msg)
		}
		if v.preview != nil {
			v.preview, _ = v.preview.Update(msg)
		}

	case ObjectsLoadedMsg:
		v.loading = false
		v.spinner.Stop()

		if msg.Error != nil {
			// Enhanced error handling with user-friendly messages
			errorMsg := "Failed to load S3 objects"
			if strings.Contains(msg.Error.Error(), "NoCredentialsErr") {
				errorMsg = "❌ AWS credentials not found. Please configure AWS CLI or set environment variables."
			} else if strings.Contains(msg.Error.Error(), "InvalidAccessKeyId") {
				errorMsg = "❌ Invalid AWS access key. Please check your credentials."
			} else if strings.Contains(msg.Error.Error(), "AccessDenied") {
				errorMsg = "❌ Access denied to bucket. Please check your permissions."
			} else if strings.Contains(msg.Error.Error(), "NoSuchBucket") {
				errorMsg = "❌ Bucket does not exist. Please verify the bucket name."
			} else if strings.Contains(msg.Error.Error(), "network") || strings.Contains(msg.Error.Error(), "timeout") {
				errorMsg = "🌐 Network error. Please check your internet connection and try again."
			} else {
				errorMsg = fmt.Sprintf("❌ Error: %v\n💡 Try: check bucket permissions or AWS credentials", msg.Error)
			}

			// Create a simple error display
			items := []components.ListItem{
				{
					Title:       errorMsg,
					Description: "Press 'r' to retry, 'esc' to go back, or 'q' to quit",
					Tag:         "Error",
				},
			}
			v.list = components.NewList("⚠️ Error Loading Objects", items)
			return v, nil
		}

		// Convert object info to list items
		items := make([]components.ListItem, len(msg.Objects))
		for i, obj := range msg.Objects {
			items[i] = components.ListItem{
				Title:       obj.Key,
				Description: fmt.Sprintf("Size: %s | Modified: %s", formatBytes(obj.Size), obj.LastModified.Format("2006-01-02 15:04:05")),
				Tag:         "Object",
				Data:        obj,
			}
		}

		var title string
		switch v.operation {
		case "download":
			title = "📁 Select Object to Download"
		case "delete":
			title = "🗑️ Select Object to Delete"
		default:
			title = "📁 Select Object"
		}

		v.list = components.NewList(title, items)
		return v, nil

	case tea.KeyMsg:
		if v.loading || v.processing {
			break
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			// Go back to operation selection
			return NewOperationView(v.region, v.bucket), nil
		case "?":
			// Show help
			return NewHelpView(), nil
		case "s":
			// Show settings
			return NewSettingsView(), nil
		case "l":
			// Show logs
			return NewLogsView(), nil
		case "p":
			// Toggle preview
			v.showPreview = !v.showPreview
			if v.showPreview && v.list != nil {
				selectedItem := v.list.GetCurrentItem()
				if selectedItem != nil && selectedItem.Tag == "Object" {
					objInfo := selectedItem.Data.(interfaces.ObjectInfo)
					return v, v.previewObject(objInfo)
				}
			}
		case "r":
			// Retry loading objects
			v.loading = true
			v.spinner = components.NewSpinner("Retrying to load S3 objects...")
			return v, tea.Batch(
				v.spinner.Start(),
				v.loadObjects(),
			)
		case "enter", " ":
			if v.list != nil {
				selectedItem := v.list.GetCurrentItem()
				if selectedItem != nil {
					// Check if it's an error item
					if selectedItem.Tag == "Error" {
						// On error item selection, retry
						v.loading = true
						v.spinner = components.NewSpinner("Retrying to load S3 objects...")
						return v, tea.Batch(
							v.spinner.Start(),
							v.loadObjects(),
						)
					}

					objInfo := selectedItem.Data.(interfaces.ObjectInfo)
					return v.processObject(objInfo)
				}
			}
		}

		if v.list != nil {
			v.list, _ = v.list.Update(msg)
			// Auto-preview on selection change if preview is enabled
			if v.showPreview {
				selectedItem := v.list.GetCurrentItem()
				if selectedItem != nil && selectedItem.Tag == "Object" {
					objInfo := selectedItem.Data.(interfaces.ObjectInfo)
					cmds = append(cmds, v.previewObject(objInfo))
				}
			}
		}

	case components.SpinnerTickMsg:
		if v.loading {
			v.spinner, _ = v.spinner.Update(msg)
			cmds = append(cmds, v.spinner.Start())
		}

	case components.ProgressMsg:
		if v.progress != nil {
			v.progress, _ = v.progress.Update(msg)
		}

	case components.CompletedMsg:
		if v.progress != nil {
			v.progress, _ = v.progress.Update(msg)
			v.processing = false
			// Wait a moment to show completion, then quit or go back
			return v, tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
				return tea.KeyMsg{Type: tea.KeyEsc}
			})
		}

	case components.PreviewMsg:
		if v.preview != nil {
			v.preview, _ = v.preview.Update(msg)
		}
	}

	return v, tea.Batch(cmds...)
}

// View renders the object view
func (v *ObjectView) View() string {
	if v.processing && v.progress != nil {
		return v.progress.View()
	}

	if v.loading {
		return v.headerStyle.Render("📁 S3 Objects") + "\n\n" + v.spinner.View()
	}

	if v.list == nil {
		return v.errorStyle.Render("Failed to load objects")
	}

	context := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888")).
		Render("Region: " + v.region + " | Bucket: " + v.bucket)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("↑↓: navigate • enter: select • r: refresh • p: preview • ?: help • s: settings • l: logs • esc: back • q: quit")

	if v.showPreview && v.preview != nil {
		// Split view: list on left, preview on right
		listWidth := v.width / 2
		previewWidth := v.width - listWidth

		listStyle := lipgloss.NewStyle().Width(listWidth)
		previewStyle := lipgloss.NewStyle().Width(previewWidth)

		return context + "\n\n" +
			lipgloss.JoinHorizontal(lipgloss.Top,
				listStyle.Render(v.list.View()),
				previewStyle.Render(v.preview.View()),
			) + "\n\n" + footer
	}

	return context + "\n\n" + v.list.View() + "\n\n" + footer
}

// loadObjects loads the S3 objects
func (v *ObjectView) loadObjects() tea.Cmd {
	return func() tea.Msg {
		// Even shorter timeout for debugging
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if v.s3Client == nil {
			return ObjectsLoadedMsg{Error: fmt.Errorf("S3 client is nil")}
		}

		// Use interface method for MVP with pagination limit
		objects, err := v.s3Client.ListObjects(ctx, v.bucket, "", "")
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return ObjectsLoadedMsg{Error: fmt.Errorf("timeout loading objects from bucket %s (waited 10s)", v.bucket)}
			}
			return ObjectsLoadedMsg{Error: fmt.Errorf("failed to load objects from bucket %s: %w", v.bucket, err)}
		}

		return ObjectsLoadedMsg{Objects: objects}
	}
}

// processObject handles the selected object based on operation
func (v *ObjectView) processObject(obj interfaces.ObjectInfo) (tea.Model, tea.Cmd) {
	switch v.operation {
	case "download":
		return v.downloadObject(obj)
	case "delete":
		return v.deleteObject(obj)
	default:
		return v, nil
	}
}

// downloadObject downloads the selected object using modern worker pool
func (v *ObjectView) downloadObject(obj interfaces.ObjectInfo) (tea.Model, tea.Cmd) {
	v.processing = true
	v.progress = components.NewProgress("⬇️ Downloading "+obj.Key, obj.Size)

	return v, func() tea.Msg {
		filename := filepath.Base(obj.Key)

		// Try to use modern downloader first (if available)
		if client, ok := v.s3Client.(*internalS3.Client); ok {
			// Create modern downloader with enhanced configuration
			config := internalS3.DefaultDownloadConfig()
			config.ConcurrentDownloads = 3
			config.OnProgress = func(downloaded, total int64) {
				// Update progress in real-time
				v.progress.SetProgress(downloaded, total,
					fmt.Sprintf("Downloaded %s of %s", formatBytes(downloaded), formatBytes(total)))
			}

			downloader := internalS3.NewDownloader(client, config)
			defer downloader.Close()

			request := internalS3.DownloadRequest{
				Bucket:   v.bucket,
				Key:      obj.Key,
				FilePath: filename,
			}

			ctx := context.Background()
			err := downloader.Download(ctx, request, config)
			if err == nil {
				return components.CompletedMsg{
					Success: true,
					Message: fmt.Sprintf("Downloaded %s (%s) with worker pool", filename, formatBytes(obj.Size)),
				}
			}

			// If modern downloader fails, fall back to legacy
		}

		// Fallback to legacy downloader
		file, err := os.Create(filename)
		if err != nil {
			return components.CompletedMsg{
				Success: false,
				Message: fmt.Sprintf("Failed to create file: %v", err),
			}
		}
		defer file.Close()

		// Use interface method for MVP
		ctx := context.Background()
		err = v.s3Client.DownloadFile(ctx, v.bucket, obj.Key, filename)

		if err != nil {
			return components.CompletedMsg{
				Success: false,
				Message: fmt.Sprintf("Download failed: %v", err),
			}
		}

		return components.CompletedMsg{
			Success: true,
			Message: fmt.Sprintf("Downloaded %s (%s)", filename, formatBytes(obj.Size)),
		}
	}
}

// deleteObject deletes the selected object
func (v *ObjectView) deleteObject(obj interfaces.ObjectInfo) (tea.Model, tea.Cmd) {
	v.processing = true
	v.progress = components.NewProgress("🗑️ Deleting "+obj.Key, 1)

	return v, func() tea.Msg {
		// Use interface method for MVP
		ctx := context.Background()
		err := v.s3Client.DeleteObject(ctx, v.bucket, obj.Key)

		if err != nil {
			return components.CompletedMsg{
				Success: false,
				Message: fmt.Sprintf("Delete failed: %v", err),
			}
		}

		return components.CompletedMsg{
			Success: true,
			Message: fmt.Sprintf("Deleted %s", obj.Key),
		}
	}
}

// previewObject generates a preview for the selected object
func (v *ObjectView) previewObject(obj interfaces.ObjectInfo) tea.Cmd {
	return func() tea.Msg {
		// For S3 objects, we need to download them first to preview
		// For now, we'll show object metadata as preview
		content := fmt.Sprintf(`S3 Object Information:
┌─────────────────────────────────┐
│ Key:        %-20s │
│ Size:       %-20s │
│ Modified:   %-20s │
│ ETag:       %-20s │
└─────────────────────────────────┘

📄 S3 Object Preview
   Object: %s
   
   To preview content:
   1. Download the object first
   2. Preview will be available for local files
   
   Suggested actions:
   • Press Enter to download
   • Press 'p' to toggle preview off
   
   Note: Direct S3 content preview would require
   downloading the object first.`,
			truncateString(obj.Key, 20),
			formatBytes(obj.Size),
			obj.LastModified.Format("2006-01-02 15:04"),
			truncateString(obj.ETag, 20),
			obj.Key)

		return components.PreviewMsg{
			Content:     content,
			PreviewType: components.PreviewTypeText,
		}
	}
}

// truncateString truncates a string to a specified length
func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}

// formatBytes formats byte count as human readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
