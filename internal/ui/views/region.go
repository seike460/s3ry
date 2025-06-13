package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seike460/s3ry/internal/ui/components"
)

// RegionInfo represents AWS region information
type RegionInfo struct {
	Code string
	Name string
}

// RegionView represents the region selection view
type RegionView struct {
	list *components.List
	
	// Styles
	headerStyle lipgloss.Style
}

// NewRegionView creates a new region view
func NewRegionView() *RegionView {
	regions := []components.ListItem{
		{
			Title:       "ap-northeast-1",
			Description: "Asia Pacific (Tokyo)",
			Tag:         "Asia",
			Data:        RegionInfo{Code: "ap-northeast-1", Name: "Tokyo"},
		},
		{
			Title:       "us-east-1",
			Description: "US East (N. Virginia)",
			Tag:         "Americas",
			Data:        RegionInfo{Code: "us-east-1", Name: "N. Virginia"},
		},
		{
			Title:       "us-west-2",
			Description: "US West (Oregon)",
			Tag:         "Americas",
			Data:        RegionInfo{Code: "us-west-2", Name: "Oregon"},
		},
		{
			Title:       "eu-west-1",
			Description: "Europe (Ireland)",
			Tag:         "Europe",
			Data:        RegionInfo{Code: "eu-west-1", Name: "Ireland"},
		},
		{
			Title:       "eu-central-1",
			Description: "Europe (Frankfurt)",
			Tag:         "Europe",
			Data:        RegionInfo{Code: "eu-central-1", Name: "Frankfurt"},
		},
		{
			Title:       "ap-southeast-1",
			Description: "Asia Pacific (Singapore)",
			Tag:         "Asia",
			Data:        RegionInfo{Code: "ap-southeast-1", Name: "Singapore"},
		},
		{
			Title:       "ap-southeast-2",
			Description: "Asia Pacific (Sydney)",
			Tag:         "Asia",
			Data:        RegionInfo{Code: "ap-southeast-2", Name: "Sydney"},
		},
		{
			Title:       "ap-south-1",
			Description: "Asia Pacific (Mumbai)",
			Tag:         "Asia",
			Data:        RegionInfo{Code: "ap-south-1", Name: "Mumbai"},
		},
		{
			Title:       "sa-east-1",
			Description: "South America (São Paulo)",
			Tag:         "Americas",
			Data:        RegionInfo{Code: "sa-east-1", Name: "São Paulo"},
		},
		{
			Title:       "ca-central-1",
			Description: "Canada (Central)",
			Tag:         "Americas",
			Data:        RegionInfo{Code: "ca-central-1", Name: "Central"},
		},
	}
	
	return &RegionView{
		list: components.NewList("🌏 Select AWS Region", regions),
		
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1),
	}
}

// Init initializes the region view
func (v *RegionView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the region view
func (v *RegionView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.list, _ = v.list.Update(msg)
		
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "enter", " ":
			selectedItem := v.list.GetCurrentItem()
			if selectedItem != nil {
				regionInfo := selectedItem.Data.(RegionInfo)
				// Transition to bucket selection
				return NewBucketView(regionInfo.Code), nil
			}
		}
		
		v.list, _ = v.list.Update(msg)
	}
	
	return v, nil
}

// View renders the region view
func (v *RegionView) View() string {
	header := v.headerStyle.Render("S3ry - S3 File Manager")
	return header + "\n\n" + v.list.View()
}