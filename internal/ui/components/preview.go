package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PreviewType represents different file types for preview
type PreviewType int

// Preview content categories.
const (
	PreviewTypeUnsupported PreviewType = iota
	PreviewTypeText
	PreviewTypeImage
	PreviewTypePDF
	PreviewTypeBinary
)

// PreviewMsg represents a preview update message
type PreviewMsg struct {
	Content     string
	PreviewType PreviewType
	Error       error
}

// Preview represents a file preview component
type Preview struct {
	content     string
	previewType PreviewType
	width       int
	height      int
	error       error

	// Styles
	titleStyle   lipgloss.Style
	contentStyle lipgloss.Style
	errorStyle   lipgloss.Style
	borderStyle  lipgloss.Style
}

// NewPreview creates a new preview component
func NewPreview() *Preview {
	return &Preview{
		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorAccent)).
			PaddingLeft(1).
			PaddingRight(1),

		contentStyle: lipgloss.NewStyle().
			Padding(1).
			Foreground(lipgloss.Color(ColorText)),

		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorDanger)).
			Padding(1),

		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorAccent)),
	}
}

// Update updates the preview component
func (p *Preview) Update(msg tea.Msg) (*Preview, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height

	case PreviewMsg:
		p.content = msg.Content
		p.previewType = msg.PreviewType
		p.error = msg.Error
	}

	return p, nil
}

// View renders the preview component
func (p *Preview) View() string {
	if p.error != nil {
		return p.borderStyle.Render(
			p.titleStyle.Render("❌ Preview Error") + "\n" +
				p.errorStyle.Render(p.error.Error()),
		)
	}

	if p.content == "" {
		return p.borderStyle.Render(
			p.titleStyle.Render("📄 File Preview") + "\n" +
				p.contentStyle.Render("No content to preview"),
		)
	}

	return p.borderStyle.Render(
		p.titleStyle.Render(p.title()) + "\n" +
			p.contentStyle.Render(p.fitContent()),
	)
}

// title returns the pane title for the detected preview type.
func (p *Preview) title() string {
	switch p.previewType {
	case PreviewTypeText:
		return "📄 Text Preview"
	case PreviewTypeImage:
		return "🖼️ Image Preview"
	case PreviewTypePDF:
		return "📊 PDF Preview"
	case PreviewTypeBinary:
		return "🔍 Binary Preview"
	default:
		return "❓ Preview"
	}
}

// fitContent truncates the preview body to the pane height.
func (p *Preview) fitContent() string {
	maxLines := p.height - 4 // Account for title and border
	if maxLines <= 0 {
		return p.content
	}
	lines := strings.Split(p.content, "\n")
	if len(lines) <= maxLines {
		return p.content
	}
	lines = lines[:maxLines-1]
	lines = append(lines, "... (content truncated)")
	return strings.Join(lines, "\n")
}
