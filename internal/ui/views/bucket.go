package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	awsS3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
	"github.com/seike460/s3ry/pkg/interfaces"
)

// BucketsLoadedMsg represents buckets being loaded
type BucketsLoadedMsg struct {
	Buckets []BucketInfo
	Error   error
}

// BucketInfo represents S3 bucket information
type BucketInfo struct {
	Name   string
	Region string
}

// BucketView represents the bucket selection view with enhanced error handling
type BucketView struct {
	list         *components.List
	spinner      *components.Spinner
	errorDisplay *components.ErrorDisplay
	loading      bool
	region       string
	s3Client     interfaces.S3Client
	
	// Styles
	headerStyle lipgloss.Style
	errorStyle  lipgloss.Style
}

// NewBucketView creates a new bucket view
func NewBucketView(region string) *BucketView {
	// Create S3 client using the new architecture
	s3Client := s3.NewClient(region)
	
	return &BucketView{
		region:       region,
		s3Client:     s3Client,
		loading:      true,
		spinner:      components.NewSpinner("Loading S3 buckets..."),
		errorDisplay: components.NewErrorDisplay(),
		
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

// Init initializes the bucket view
func (v *BucketView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Start(),
		v.loadBuckets(),
	)
}

// Update handles messages for the bucket view
func (v *BucketView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}
		
	case BucketsLoadedMsg:
		v.loading = false
		v.spinner.Stop()
		
		if msg.Error != nil {
			// Enhanced error handling with intelligent error display
			v.loading = false
			v.spinner.Stop()
			
			// Use the new error display system
			v.errorDisplay.AddAWSError(msg.Error)
			
			// Create a simple error display in the list
			items := []components.ListItem{
				{
					Title:       "Failed to load S3 buckets",
					Description: "See error details below. Press 'r' to retry, 'esc' to go back, or 'q' to quit",
					Tag:         "Error",
				},
			}
			v.list = components.NewList("⚠️ Error Loading Buckets", items)
			return v, nil
		}
		
		// Convert bucket info to list items
		items := make([]components.ListItem, len(msg.Buckets))
		for i, bucket := range msg.Buckets {
			items[i] = components.ListItem{
				Title:       bucket.Name,
				Description: fmt.Sprintf("Region: %s", bucket.Region),
				Tag:         "Bucket",
				Data:        bucket,
			}
		}
		
		v.list = components.NewList("Select S3 Bucket", items)
		return v, nil
		
	case tea.KeyMsg:
		if v.loading {
			break
		}
		
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "?":
			// Show help
			return NewHelpView(), nil
		case "s":
			// Show settings
			return NewSettingsView(), nil
		case "l":
			// Show logs
			return NewLogsView(), nil
		case "r":
			// Retry loading buckets
			v.loading = true
			v.spinner = components.NewSpinner("Retrying to load S3 buckets...")
			return v, tea.Batch(
				v.spinner.Start(),
				v.loadBuckets(),
			)
			
		case "enter", " ":
			if v.list != nil {
				selectedItem := v.list.GetCurrentItem()
				if selectedItem != nil {
					// Check if it's an error item
					if selectedItem.Tag == "Error" {
						// On error item selection, retry
						v.loading = true
						v.spinner = components.NewSpinner("Retrying to load S3 buckets...")
						return v, tea.Batch(
							v.spinner.Start(),
							v.loadBuckets(),
						)
					}
					
					bucketInfo := selectedItem.Data.(BucketInfo)
					return NewOperationView(bucketInfo.Region, bucketInfo.Name), nil
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
	}
	
	return v, tea.Batch(cmds...)
}

// View renders the bucket view with enhanced error display
func (v *BucketView) View() string {
	if v.loading {
		return v.headerStyle.Render("🌏 S3ry - S3 File Manager") + "\n\n" + v.spinner.View()
	}
	
	if v.list == nil {
		return v.errorStyle.Render("Failed to load buckets")
	}
	
	// Combine list view with error display
	var result strings.Builder
	result.WriteString(v.list.View())
	
	// Show errors if any
	if v.errorDisplay.GetErrorCount() > 0 {
		result.WriteString("\n\n")
		result.WriteString("📋 Error Details:")
		result.WriteString("\n")
		result.WriteString(v.errorDisplay.View())
	}
	
	return result.String()
}

// loadBuckets loads the S3 buckets
func (v *BucketView) loadBuckets() tea.Cmd {
	return func() tea.Msg {
		// Add timeout context (shorter for faster feedback)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		// Debug: Check if S3 client is initialized
		if v.s3Client == nil {
			return BucketsLoadedMsg{Error: fmt.Errorf("S3 client not initialized")}
		}
		
		svc := v.s3Client.S3()
		if svc == nil {
			return BucketsLoadedMsg{Error: fmt.Errorf("S3 service not available")}
		}
		
		// List buckets with timeout
		result, err := svc.ListBucketsWithContext(ctx, &awsS3.ListBucketsInput{})
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return BucketsLoadedMsg{Error: fmt.Errorf("timeout listing buckets")}
			}
			return BucketsLoadedMsg{Error: fmt.Errorf("failed to list buckets: %w", err)}
		}
		
		// Simplified bucket list without region detection to avoid timeouts
		buckets := make([]BucketInfo, 0, len(result.Buckets))
		
		for _, bucket := range result.Buckets {
			buckets = append(buckets, BucketInfo{
				Name:   *bucket.Name,
				Region: v.region, // Use current region as default
			})
		}
		
		return BucketsLoadedMsg{Buckets: buckets}
	}
}