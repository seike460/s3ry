package views

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
	"github.com/seike460/s3ry/pkg/interfaces"
)

// FilesLoadedMsg represents local files being loaded
type FilesLoadedMsg struct {
	Files []FileInfo
	Error error
}

// FileInfo represents local file information
type FileInfo struct {
	Path         string
	RelativePath string
	Size         int64
	ModTime      time.Time
	IsDir        bool
}

// UploadView represents the upload view
type UploadView struct {
	list       *components.List
	spinner    *components.Spinner
	progress   *components.Progress
	loading    bool
	processing bool
	region     string
	bucket     string
	s3Client   interfaces.S3Client

	// Styles
	headerStyle lipgloss.Style
	errorStyle  lipgloss.Style
}

// NewUploadView creates a new upload view
func NewUploadView(region, bucket string) *UploadView {
	// Create S3 client using the new architecture
	s3Client := s3.NewClient(region)

	return &UploadView{
		region:   region,
		bucket:   bucket,
		s3Client: s3Client,
		loading:  true,
		spinner:  components.NewSpinner("Scanning local files..."),

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

// Init initializes the upload view
func (v *UploadView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Start(),
		v.loadFiles(),
	)
}

// Update handles messages for the upload view
func (v *UploadView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}
		if v.progress != nil {
			v.progress, _ = v.progress.Update(msg)
		}

	case FilesLoadedMsg:
		v.loading = false
		v.spinner.Stop()

		if msg.Error != nil {
			// Enhanced error handling with user-friendly messages
			errorMsg := "Failed to scan local files"
			if strings.Contains(msg.Error.Error(), "permission") {
				errorMsg = "❌ Permission denied. Please check file/directory permissions."
			} else if strings.Contains(msg.Error.Error(), "no such file") {
				errorMsg = "❌ Directory not found. Please ensure the current directory exists."
			} else if strings.Contains(msg.Error.Error(), "too many open files") {
				errorMsg = "❌ Too many files open. Please close other applications and try again."
			} else {
				errorMsg = fmt.Sprintf("❌ Error: %v\n💡 Try: check current directory permissions or file access", msg.Error)
			}

			// Create a simple error display
			items := []components.ListItem{
				{
					Title:       errorMsg,
					Description: "Press 'r' to retry, 'esc' to go back, or 'q' to quit",
					Tag:         "Error",
				},
			}
			v.list = components.NewList("⚠️ Error Scanning Files", items)
			return v, nil
		}

		// Convert file info to list items
		items := make([]components.ListItem, 0, len(msg.Files))
		for _, file := range msg.Files {
			if !file.IsDir { // Only show files, not directories
				items = append(items, components.ListItem{
					Title:       file.RelativePath,
					Description: fmt.Sprintf("Size: %s | Modified: %s", formatBytes(file.Size), file.ModTime.Format("2006-01-02 15:04:05")),
					Tag:         "File",
					Data:        file,
				})
			}
		}

		v.list = components.NewList("⬆️ Select File to Upload", items)
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
		case "r":
			// Retry scanning files
			v.loading = true
			v.spinner = components.NewSpinner("Retrying to scan local files...")
			return v, tea.Batch(
				v.spinner.Start(),
				v.loadFiles(),
			)
		case "enter", " ":
			if v.list != nil {
				selectedItem := v.list.GetCurrentItem()
				if selectedItem != nil {
					// Check if it's an error item
					if selectedItem.Tag == "Error" {
						// On error item selection, retry
						v.loading = true
						v.spinner = components.NewSpinner("Retrying to scan local files...")
						return v, tea.Batch(
							v.spinner.Start(),
							v.loadFiles(),
						)
					}

					fileInfo := selectedItem.Data.(FileInfo)
					return v.uploadFile(fileInfo)
				}
			}
		}

		if v.list != nil {
			v.list, _ = v.list.Update(msg)
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
			// Wait a moment to show completion, then go back
			return v, tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
				return tea.KeyMsg{Type: tea.KeyEsc}
			})
		}
	}

	return v, tea.Batch(cmds...)
}

// View renders the upload view
func (v *UploadView) View() string {
	if v.processing && v.progress != nil {
		return v.progress.View()
	}

	if v.loading {
		return v.headerStyle.Render("📁 Local Files") + "\n\n" + v.spinner.View()
	}

	if v.list == nil {
		return v.errorStyle.Render("Failed to load files")
	}

	context := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888")).
		Render("Region: " + v.region + " | Bucket: " + v.bucket)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("↑↓: navigate • enter: select • r: refresh • esc: back • q: quit")

	return context + "\n\n" + v.list.View() + "\n\n" + footer
}

// loadFiles loads local files from current directory
func (v *UploadView) loadFiles() tea.Cmd {
	return func() tea.Msg {
		var files []FileInfo

		// Walk the current directory
		err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip hidden files and directories
			if filepath.Base(path)[0] == '.' {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			files = append(files, FileInfo{
				Path:         path,
				RelativePath: path,
				Size:         info.Size(),
				ModTime:      info.ModTime(),
				IsDir:        d.IsDir(),
			})

			return nil
		})

		if err != nil {
			return FilesLoadedMsg{Error: err}
		}

		return FilesLoadedMsg{Files: files}
	}
}

// uploadFile uploads the selected file using modern worker pool
func (v *UploadView) uploadFile(file FileInfo) (tea.Model, tea.Cmd) {
	v.processing = true
	v.progress = components.NewProgress("⬆️ Uploading "+file.RelativePath, file.Size)

	return v, func() tea.Msg {
		// Try to use modern uploader first (if available)
		if client, ok := v.s3Client.(*s3.Client); ok {
			// Create modern uploader with enhanced configuration
			config := s3.DefaultUploadConfig()
			config.ConcurrentUploads = 3
			config.OnProgress = func(uploaded, total int64) {
				// Update progress in real-time
				v.progress.SetProgress(uploaded, total,
					fmt.Sprintf("Uploaded %s of %s", formatBytes(uploaded), formatBytes(total)))
			}

			uploader := s3.NewUploader(client, config)
			defer uploader.Close()

			request := s3.UploadRequest{
				Bucket:   v.bucket,
				Key:      file.RelativePath,
				FilePath: file.Path,
			}

			ctx := context.Background()
			err := uploader.Upload(ctx, request, config)
			if err == nil {
				return components.CompletedMsg{
					Success: true,
					Message: fmt.Sprintf("Uploaded %s (%s) with worker pool", file.RelativePath, formatBytes(file.Size)),
				}
			}

			// If modern uploader fails, fall back to legacy
		}

		// Fallback to legacy uploader
		f, err := os.Open(file.Path)
		if err != nil {
			return components.CompletedMsg{
				Success: false,
				Message: fmt.Sprintf("Failed to open file: %v", err),
			}
		}
		defer f.Close()

		// Use interface method for MVP
		ctx := context.Background()
		err = v.s3Client.UploadFile(ctx, file.Path, v.bucket, file.RelativePath)

		if err != nil {
			return components.CompletedMsg{
				Success: false,
				Message: fmt.Sprintf("Upload failed: %v", err),
			}
		}

		return components.CompletedMsg{
			Success: true,
			Message: fmt.Sprintf("Uploaded %s (%s)", file.RelativePath, formatBytes(file.Size)),
		}
	}
}
