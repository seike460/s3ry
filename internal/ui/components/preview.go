package components

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PreviewType represents different file types for preview
type PreviewType int

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
			Foreground(lipgloss.Color("#7D56F4")).
			PaddingLeft(1).
			PaddingRight(1),
		
		contentStyle: lipgloss.NewStyle().
			Padding(1).
			Foreground(lipgloss.Color("#FAFAFA")),
		
		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")).
			Padding(1),
		
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")),
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
	
	var title string
	switch p.previewType {
	case PreviewTypeText:
		title = "📄 Text Preview"
	case PreviewTypeImage:
		title = "🖼️ Image Preview"
	case PreviewTypePDF:
		title = "📊 PDF Preview"
	case PreviewTypeBinary:
		title = "🔍 Binary Preview"
	default:
		title = "❓ Preview"
	}
	
	// Truncate content if too large
	displayContent := p.content
	maxLines := p.height - 4 // Account for title and border
	if maxLines > 0 {
		lines := strings.Split(displayContent, "\n")
		if len(lines) > maxLines {
			lines = lines[:maxLines-1]
			lines = append(lines, "... (content truncated)")
			displayContent = strings.Join(lines, "\n")
		}
	}
	
	return p.borderStyle.Render(
		p.titleStyle.Render(title) + "\n" +
		p.contentStyle.Render(displayContent),
	)
}

// PreviewFile generates a preview for a file
func PreviewFile(filename string) tea.Cmd {
	return func() tea.Msg {
		previewType := getPreviewType(filename)
		
		switch previewType {
		case PreviewTypeText:
			return previewTextFile(filename)
		case PreviewTypeImage:
			return previewImageFile(filename)
		case PreviewTypePDF:
			return previewPDFFile(filename)
		case PreviewTypeBinary:
			return previewBinaryFile(filename)
		default:
			return PreviewMsg{
				Content:     fmt.Sprintf("File type not supported for preview: %s", filepath.Ext(filename)),
				PreviewType: PreviewTypeUnsupported,
			}
		}
	}
}

// getPreviewType determines the preview type based on file extension
func getPreviewType(filename string) PreviewType {
	ext := strings.ToLower(filepath.Ext(filename))
	
	switch ext {
	case ".txt", ".md", ".yaml", ".yml", ".json", ".xml", ".html", ".css", ".js", ".ts", ".go", ".py", ".sh", ".conf", ".cfg", ".ini", ".log":
		return PreviewTypeText
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return PreviewTypeImage
	case ".pdf":
		return PreviewTypePDF
	default:
		// Check if it's a text file by trying to read it
		if isTextFile(filename) {
			return PreviewTypeText
		}
		return PreviewTypeBinary
	}
}

// isTextFile checks if a file is likely a text file
func isTextFile(filename string) bool {
	file, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer file.Close()
	
	// Read first 512 bytes to check for binary content
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false
	}
	
	// Check for null bytes which indicate binary content
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return false
		}
	}
	
	return true
}

// previewTextFile previews a text file
func previewTextFile(filename string) PreviewMsg {
	content, err := os.ReadFile(filename)
	if err != nil {
		return PreviewMsg{
			Error: fmt.Errorf("failed to read text file: %w", err),
		}
	}
	
	// Limit content size
	const maxSize = 8192 // 8KB
	if len(content) > maxSize {
		content = content[:maxSize]
		return PreviewMsg{
			Content:     string(content) + "\n... (file truncated, showing first 8KB)",
			PreviewType: PreviewTypeText,
		}
	}
	
	return PreviewMsg{
		Content:     string(content),
		PreviewType: PreviewTypeText,
	}
}

// previewImageFile previews an image file
func previewImageFile(filename string) PreviewMsg {
	file, err := os.Open(filename)
	if err != nil {
		return PreviewMsg{
			Error: fmt.Errorf("failed to open image file: %w", err),
		}
	}
	defer file.Close()
	
	// Decode image to get dimensions
	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return PreviewMsg{
			Error: fmt.Errorf("failed to decode image: %w", err),
		}
	}
	
	// Get file info
	stat, err := file.Stat()
	if err != nil {
		return PreviewMsg{
			Error: fmt.Errorf("failed to get file info: %w", err),
		}
	}
	
	content := fmt.Sprintf(`Image Information:
┌─────────────────────────────────┐
│ Format:     %-20s │
│ Dimensions: %dx%-16d │
│ File Size:  %-20s │
└─────────────────────────────────┘

📸 Image preview is available in terminal
   that supports image display.
   
   File: %s`, 
		strings.ToUpper(format),
		config.Width, config.Height,
		formatBytesPreview(stat.Size()),
		filename)
	
	return PreviewMsg{
		Content:     content,
		PreviewType: PreviewTypeImage,
	}
}

// previewPDFFile previews a PDF file
func previewPDFFile(filename string) PreviewMsg {
	stat, err := os.Stat(filename)
	if err != nil {
		return PreviewMsg{
			Error: fmt.Errorf("failed to get PDF file info: %w", err),
		}
	}
	
	content := fmt.Sprintf(`PDF Document Information:
┌─────────────────────────────────┐
│ File Size: %-20s │
│ Modified:  %-20s │
└─────────────────────────────────┘

📊 PDF Preview
   Full PDF viewing requires external tools.
   
   Suggested actions:
   • Download and open with PDF viewer
   • Use 'open %s' (macOS) or 'xdg-open %s' (Linux)
   
   File: %s`,
		formatBytesPreview(stat.Size()),
		stat.ModTime().Format("2006-01-02 15:04:05"),
		filename, filename, filename)
	
	return PreviewMsg{
		Content:     content,
		PreviewType: PreviewTypePDF,
	}
}

// previewBinaryFile previews a binary file
func previewBinaryFile(filename string) PreviewMsg {
	stat, err := os.Stat(filename)
	if err != nil {
		return PreviewMsg{
			Error: fmt.Errorf("failed to get binary file info: %w", err),
		}
	}
	
	// Read first few bytes for hex preview
	file, err := os.Open(filename)
	if err != nil {
		return PreviewMsg{
			Error: fmt.Errorf("failed to open binary file: %w", err),
		}
	}
	defer file.Close()
	
	buffer := make([]byte, 64) // Read first 64 bytes
	n, _ := file.Read(buffer)
	
	var hexPreview strings.Builder
	hexPreview.WriteString("Hex dump (first 64 bytes):\n")
	for i := 0; i < n; i += 16 {
		end := i + 16
		if end > n {
			end = n
		}
		
		// Offset
		hexPreview.WriteString(fmt.Sprintf("%08x  ", i))
		
		// Hex values
		for j := i; j < end; j++ {
			hexPreview.WriteString(fmt.Sprintf("%02x ", buffer[j]))
		}
		
		// Padding for alignment
		for j := end; j < i+16; j++ {
			hexPreview.WriteString("   ")
		}
		
		// ASCII representation
		hexPreview.WriteString(" |")
		for j := i; j < end; j++ {
			if buffer[j] >= 32 && buffer[j] <= 126 {
				hexPreview.WriteByte(buffer[j])
			} else {
				hexPreview.WriteByte('.')
			}
		}
		hexPreview.WriteString("|\n")
	}
	
	content := fmt.Sprintf(`Binary File Information:
┌─────────────────────────────────┐
│ File Size: %-20s │
│ Modified:  %-20s │
└─────────────────────────────────┘

🔍 Binary File Preview
   This appears to be a binary file.
   
%s

   File: %s`,
		formatBytesPreview(stat.Size()),
		stat.ModTime().Format("2006-01-02 15:04:05"),
		hexPreview.String(),
		filename)
	
	return PreviewMsg{
		Content:     content,
		PreviewType: PreviewTypeBinary,
	}
}

// formatBytesPreview formats byte count as human readable string
func formatBytesPreview(bytes int64) string {
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