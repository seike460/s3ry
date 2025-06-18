package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seike460/s3ry/internal/ui/components"
)

// CloudService represents different AWS services
type CloudService struct {
	Name        string
	Status      string
	Description string
	Details     interface{}
}

// CloudMetrics represents metrics for a service
type CloudMetrics struct {
	RequestCount    int64
	ErrorRate       float64
	AvgResponseTime float64
	LastUpdated     time.Time
}

// CloudIntegrationView represents the cloud services integration view
type CloudIntegrationView struct {
	list     *components.List
	spinner  *components.Spinner
	loading  bool
	region   string
	bucket   string
	session  *session.Session
	services []CloudService

	// Styles
	headerStyle  lipgloss.Style
	metricStyle  lipgloss.Style
	statusStyle  lipgloss.Style
	healthyStyle lipgloss.Style
	warningStyle lipgloss.Style
	errorStyle   lipgloss.Style
}

// NewCloudIntegrationView creates a new cloud integration view
func NewCloudIntegrationView(region, bucket string) *CloudIntegrationView {
	// Create AWS session
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	}))

	return &CloudIntegrationView{
		region:  region,
		bucket:  bucket,
		session: sess,
		loading: true,
		spinner: components.NewSpinner("Loading cloud services..."),

		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(2),

		metricStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			PaddingLeft(4),

		statusStyle: lipgloss.NewStyle().
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1),

		healthyStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF87")).
			Background(lipgloss.Color("#1A1A1A")),

		warningStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD700")).
			Background(lipgloss.Color("#1A1A1A")),

		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")).
			Background(lipgloss.Color("#1A1A1A")),
	}
}

// Init initializes the cloud integration view
func (v *CloudIntegrationView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Start(),
		v.loadCloudServices(),
	)
}

// Update handles messages for the cloud integration view
func (v *CloudIntegrationView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}

	case CloudServicesLoadedMsg:
		v.loading = false
		v.spinner.Stop()
		v.services = msg.Services

		// Convert services to list items
		items := make([]components.ListItem, len(v.services))
		for i, service := range v.services {
			statusIcon := v.getStatusIcon(service.Status)
			items[i] = components.ListItem{
				Title:       fmt.Sprintf("%s %s", statusIcon, service.Name),
				Description: service.Description,
				Tag:         service.Status,
				Data:        service,
			}
		}

		v.list = components.NewList("☁️ Cloud Services Integration", items)
		return v, nil

	case tea.KeyMsg:
		if v.loading {
			break
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			// Go back to operation selection
			return NewOperationView(v.region, v.bucket), nil
		case "r":
			// Refresh services
			v.loading = true
			v.spinner = components.NewSpinner("Refreshing cloud services...")
			return v, tea.Batch(
				v.spinner.Start(),
				v.loadCloudServices(),
			)
		case "enter", " ":
			if v.list != nil {
				selectedItem := v.list.GetCurrentItem()
				if selectedItem != nil {
					service := selectedItem.Data.(CloudService)
					return v.showServiceDetails(service)
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

// View renders the cloud integration view
func (v *CloudIntegrationView) View() string {
	if v.loading {
		return v.headerStyle.Render("☁️ Cloud Services") + "\n\n" + v.spinner.View()
	}

	if v.list == nil {
		return v.errorStyle.Render("Failed to load cloud services")
	}

	context := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888")).
		Render("Region: " + v.region + " | Bucket: " + v.bucket)

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("r: refresh • enter: details • esc: back • q: quit")

	return context + "\n\n" + v.list.View() + "\n\n" + help
}

// CloudServicesLoadedMsg represents cloud services being loaded
type CloudServicesLoadedMsg struct {
	Services []CloudService
	Error    error
}

// loadCloudServices loads information about related AWS services
func (v *CloudIntegrationView) loadCloudServices() tea.Cmd {
	return func() tea.Msg {
		var services []CloudService

		// Check S3 service
		s3Status := v.checkS3Service()
		services = append(services, s3Status)

		// Check CloudFormation stacks
		cfStatus := v.checkCloudFormationStacks()
		services = append(services, cfStatus)

		// Check IAM roles and policies
		iamStatus := v.checkIAMResources()
		services = append(services, iamStatus)

		// Check CloudWatch metrics
		cwStatus := v.checkCloudWatchMetrics()
		services = append(services, cwStatus)

		return CloudServicesLoadedMsg{Services: services}
	}
}

// checkS3Service checks the status of S3 service
func (v *CloudIntegrationView) checkS3Service() CloudService {
	svc := s3.New(v.session)

	// Try to head the bucket
	_, err := svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(v.bucket),
	})

	status := "Healthy"
	description := fmt.Sprintf("S3 bucket '%s' is accessible", v.bucket)

	if err != nil {
		status = "Error"
		description = fmt.Sprintf("S3 bucket '%s' is not accessible: %v", v.bucket, err)
	}

	return CloudService{
		Name:        "Amazon S3",
		Status:      status,
		Description: description,
	}
}

// checkCloudFormationStacks checks for related CloudFormation stacks
func (v *CloudIntegrationView) checkCloudFormationStacks() CloudService {
	cfSvc := cloudformation.New(v.session)

	// List stacks that might be related to S3
	result, err := cfSvc.ListStacks(&cloudformation.ListStacksInput{
		StackStatusFilter: []*string{
			aws.String("CREATE_COMPLETE"),
			aws.String("UPDATE_COMPLETE"),
			aws.String("UPDATE_ROLLBACK_COMPLETE"),
		},
	})

	status := "Healthy"
	description := "No related CloudFormation stacks found"

	if err != nil {
		status = "Warning"
		description = fmt.Sprintf("Cannot access CloudFormation: %v", err)
	} else if len(result.StackSummaries) > 0 {
		description = fmt.Sprintf("Found %d active CloudFormation stacks", len(result.StackSummaries))
	}

	return CloudService{
		Name:        "CloudFormation",
		Status:      status,
		Description: description,
		Details:     result.StackSummaries,
	}
}

// checkIAMResources checks for IAM roles and policies
func (v *CloudIntegrationView) checkIAMResources() CloudService {
	iamSvc := iam.New(v.session)

	// List roles that might be related to S3
	roles, err := iamSvc.ListRoles(&iam.ListRolesInput{})

	status := "Healthy"
	description := "IAM service is accessible"

	if err != nil {
		status = "Warning"
		description = fmt.Sprintf("Cannot access IAM: %v", err)
	} else {
		s3Roles := 0
		for _, role := range roles.Roles {
			if role.RoleName != nil &&
				(containsIgnoreCase(*role.RoleName, "s3") ||
					containsIgnoreCase(*role.RoleName, v.bucket)) {
				s3Roles++
			}
		}
		description = fmt.Sprintf("Found %d S3-related IAM roles", s3Roles)
	}

	return CloudService{
		Name:        "IAM",
		Status:      status,
		Description: description,
	}
}

// checkCloudWatchMetrics checks CloudWatch metrics for the bucket
func (v *CloudIntegrationView) checkCloudWatchMetrics() CloudService {
	cwSvc := cloudwatch.New(v.session)

	// Get S3 metrics for the bucket
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/S3"),
		MetricName: aws.String("NumberOfObjects"),
		Dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("BucketName"),
				Value: aws.String(v.bucket),
			},
		},
		StartTime:  aws.Time(startTime),
		EndTime:    aws.Time(endTime),
		Period:     aws.Int64(3600), // 1 hour
		Statistics: []*string{aws.String("Average")},
	}

	_, err := cwSvc.GetMetricStatistics(input)

	status := "Healthy"
	description := "CloudWatch metrics available"

	if err != nil {
		status = "Warning"
		description = fmt.Sprintf("CloudWatch metrics not available: %v", err)
	}

	return CloudService{
		Name:        "CloudWatch",
		Status:      status,
		Description: description,
	}
}

// getStatusIcon returns an appropriate icon for the service status
func (v *CloudIntegrationView) getStatusIcon(status string) string {
	switch status {
	case "Healthy":
		return "✅"
	case "Warning":
		return "⚠️"
	case "Error":
		return "❌"
	default:
		return "❓"
	}
}

// showServiceDetails shows detailed information about a service
func (v *CloudIntegrationView) showServiceDetails(service CloudService) (tea.Model, tea.Cmd) {
	// For now, just return to the same view
	// In a full implementation, this would show a detailed view
	return v, nil
}

// containsIgnoreCase checks if a string contains a substring (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
