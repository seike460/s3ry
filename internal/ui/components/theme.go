package components

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

// Theme represents a UI theme
type Theme struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Colors      ThemeColors          `json:"colors"`
	Styles      ThemeStyles          `json:"styles"`
	Custom      map[string]string    `json:"custom,omitempty"`
}

// ThemeColors represents the color palette for a theme
type ThemeColors struct {
	// Primary colors
	Primary     lipgloss.Color `json:"primary"`
	Secondary   lipgloss.Color `json:"secondary"`
	Accent      lipgloss.Color `json:"accent"`
	
	// Background colors
	Background  lipgloss.Color `json:"background"`
	Surface     lipgloss.Color `json:"surface"`
	
	// Text colors
	Text        lipgloss.Color `json:"text"`
	TextMuted   lipgloss.Color `json:"text_muted"`
	TextInverse lipgloss.Color `json:"text_inverse"`
	
	// Status colors
	Success     lipgloss.Color `json:"success"`
	Warning     lipgloss.Color `json:"warning"`
	Error       lipgloss.Color `json:"error"`
	Info        lipgloss.Color `json:"info"`
	
	// Interactive colors
	Border      lipgloss.Color `json:"border"`
	BorderFocus lipgloss.Color `json:"border_focus"`
	Selection   lipgloss.Color `json:"selection"`
	Hover       lipgloss.Color `json:"hover"`
}

// ThemeStyles represents style configurations
type ThemeStyles struct {
	BorderRadius int    `json:"border_radius"`
	BorderWidth  int    `json:"border_width"`
	Padding      int    `json:"padding"`
	Margin       int    `json:"margin"`
	FontWeight   string `json:"font_weight"`
}

// ThemeManager manages application themes
type ThemeManager struct {
	currentTheme *Theme
	themes       map[string]*Theme
	configPath   string
}

// NewThemeManager creates a new theme manager
func NewThemeManager() *ThemeManager {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".s3ry", "themes.json")
	
	tm := &ThemeManager{
		themes:     make(map[string]*Theme),
		configPath: configPath,
	}
	
	// Load built-in themes
	tm.loadBuiltinThemes()
	
	// Try to load custom themes
	tm.loadCustomThemes()
	
	// Set default theme
	tm.SetTheme("dark")
	
	return tm
}

// loadBuiltinThemes loads the built-in themes
func (tm *ThemeManager) loadBuiltinThemes() {
	// Dark theme (default)
	dark := &Theme{
		Name:        "dark",
		Description: "Dark theme with purple accents",
		Colors: ThemeColors{
			Primary:     lipgloss.Color("#7D56F4"),
			Secondary:   lipgloss.Color("#6F42C1"),
			Accent:      lipgloss.Color("#FF79C6"),
			Background:  lipgloss.Color("#282A36"),
			Surface:     lipgloss.Color("#44475A"),
			Text:        lipgloss.Color("#F8F8F2"),
			TextMuted:   lipgloss.Color("#6272A4"),
			TextInverse: lipgloss.Color("#282A36"),
			Success:     lipgloss.Color("#50FA7B"),
			Warning:     lipgloss.Color("#FFB86C"),
			Error:       lipgloss.Color("#FF5555"),
			Info:        lipgloss.Color("#8BE9FD"),
			Border:      lipgloss.Color("#6272A4"),
			BorderFocus: lipgloss.Color("#7D56F4"),
			Selection:   lipgloss.Color("#44475A"),
			Hover:       lipgloss.Color("#6272A4"),
		},
		Styles: ThemeStyles{
			BorderRadius: 1,
			BorderWidth:  1,
			Padding:      1,
			Margin:       1,
			FontWeight:   "normal",
		},
	}
	
	// Light theme
	light := &Theme{
		Name:        "light",
		Description: "Light theme with blue accents",
		Colors: ThemeColors{
			Primary:     lipgloss.Color("#0366D6"),
			Secondary:   lipgloss.Color("#586069"),
			Accent:      lipgloss.Color("#E36209"),
			Background:  lipgloss.Color("#FFFFFF"),
			Surface:     lipgloss.Color("#F6F8FA"),
			Text:        lipgloss.Color("#24292E"),
			TextMuted:   lipgloss.Color("#586069"),
			TextInverse: lipgloss.Color("#FFFFFF"),
			Success:     lipgloss.Color("#28A745"),
			Warning:     lipgloss.Color("#FFC107"),
			Error:       lipgloss.Color("#DC3545"),
			Info:        lipgloss.Color("#17A2B8"),
			Border:      lipgloss.Color("#E1E4E8"),
			BorderFocus: lipgloss.Color("#0366D6"),
			Selection:   lipgloss.Color("#F1F8FF"),
			Hover:       lipgloss.Color("#F6F8FA"),
		},
		Styles: ThemeStyles{
			BorderRadius: 1,
			BorderWidth:  1,
			Padding:      1,
			Margin:       1,
			FontWeight:   "normal",
		},
	}
	
	// High contrast theme
	highContrast := &Theme{
		Name:        "high-contrast",
		Description: "High contrast theme for accessibility",
		Colors: ThemeColors{
			Primary:     lipgloss.Color("#FFFF00"),
			Secondary:   lipgloss.Color("#00FFFF"),
			Accent:      lipgloss.Color("#FF00FF"),
			Background:  lipgloss.Color("#000000"),
			Surface:     lipgloss.Color("#333333"),
			Text:        lipgloss.Color("#FFFFFF"),
			TextMuted:   lipgloss.Color("#CCCCCC"),
			TextInverse: lipgloss.Color("#000000"),
			Success:     lipgloss.Color("#00FF00"),
			Warning:     lipgloss.Color("#FFFF00"),
			Error:       lipgloss.Color("#FF0000"),
			Info:        lipgloss.Color("#00FFFF"),
			Border:      lipgloss.Color("#FFFFFF"),
			BorderFocus: lipgloss.Color("#FFFF00"),
			Selection:   lipgloss.Color("#666666"),
			Hover:       lipgloss.Color("#444444"),
		},
		Styles: ThemeStyles{
			BorderRadius: 0,
			BorderWidth:  2,
			Padding:      1,
			Margin:       1,
			FontWeight:   "bold",
		},
	}
	
	// Minimal theme
	minimal := &Theme{
		Name:        "minimal",
		Description: "Minimal monochrome theme",
		Colors: ThemeColors{
			Primary:     lipgloss.Color("#888888"),
			Secondary:   lipgloss.Color("#666666"),
			Accent:      lipgloss.Color("#AAAAAA"),
			Background:  lipgloss.Color("#FAFAFA"),
			Surface:     lipgloss.Color("#F0F0F0"),
			Text:        lipgloss.Color("#333333"),
			TextMuted:   lipgloss.Color("#888888"),
			TextInverse: lipgloss.Color("#FFFFFF"),
			Success:     lipgloss.Color("#666666"),
			Warning:     lipgloss.Color("#888888"),
			Error:       lipgloss.Color("#AAAAAA"),
			Info:        lipgloss.Color("#777777"),
			Border:      lipgloss.Color("#DDDDDD"),
			BorderFocus: lipgloss.Color("#888888"),
			Selection:   lipgloss.Color("#EEEEEE"),
			Hover:       lipgloss.Color("#F5F5F5"),
		},
		Styles: ThemeStyles{
			BorderRadius: 0,
			BorderWidth:  1,
			Padding:      1,
			Margin:       0,
			FontWeight:   "normal",
		},
	}
	
	// Neon theme
	neon := &Theme{
		Name:        "neon",
		Description: "Cyberpunk neon theme",
		Colors: ThemeColors{
			Primary:     lipgloss.Color("#00FFFF"),
			Secondary:   lipgloss.Color("#FF00FF"),
			Accent:      lipgloss.Color("#FFFF00"),
			Background:  lipgloss.Color("#0A0A0A"),
			Surface:     lipgloss.Color("#1A1A2E"),
			Text:        lipgloss.Color("#00FFFF"),
			TextMuted:   lipgloss.Color("#0066CC"),
			TextInverse: lipgloss.Color("#000000"),
			Success:     lipgloss.Color("#00FF41"),
			Warning:     lipgloss.Color("#FFD700"),
			Error:       lipgloss.Color("#FF073A"),
			Info:        lipgloss.Color("#00BFFF"),
			Border:      lipgloss.Color("#00FFFF"),
			BorderFocus: lipgloss.Color("#FF00FF"),
			Selection:   lipgloss.Color("#16213E"),
			Hover:       lipgloss.Color("#0F4C75"),
		},
		Styles: ThemeStyles{
			BorderRadius: 0,
			BorderWidth:  2,
			Padding:      1,
			Margin:       1,
			FontWeight:   "bold",
		},
	}
	
	// Ocean theme
	ocean := &Theme{
		Name:        "ocean",
		Description: "Ocean blue theme",
		Colors: ThemeColors{
			Primary:     lipgloss.Color("#0077BE"),
			Secondary:   lipgloss.Color("#005577"),
			Accent:      lipgloss.Color("#FFA500"),
			Background:  lipgloss.Color("#001122"),
			Surface:     lipgloss.Color("#002244"),
			Text:        lipgloss.Color("#E6F3FF"),
			TextMuted:   lipgloss.Color("#99CCFF"),
			TextInverse: lipgloss.Color("#001122"),
			Success:     lipgloss.Color("#00CC88"),
			Warning:     lipgloss.Color("#FFAA00"),
			Error:       lipgloss.Color("#FF4444"),
			Info:        lipgloss.Color("#66DDFF"),
			Border:      lipgloss.Color("#0066AA"),
			BorderFocus: lipgloss.Color("#0099FF"),
			Selection:   lipgloss.Color("#003366"),
			Hover:       lipgloss.Color("#004477"),
		},
		Styles: ThemeStyles{
			BorderRadius: 1,
			BorderWidth:  1,
			Padding:      1,
			Margin:       1,
			FontWeight:   "normal",
		},
	}
	
	tm.themes["dark"] = dark
	tm.themes["light"] = light
	tm.themes["high-contrast"] = highContrast
	tm.themes["minimal"] = minimal
	tm.themes["neon"] = neon
	tm.themes["ocean"] = ocean
}

// loadCustomThemes loads custom themes from config file
func (tm *ThemeManager) loadCustomThemes() {
	if _, err := os.Stat(tm.configPath); os.IsNotExist(err) {
		return // No custom themes file
	}
	
	data, err := os.ReadFile(tm.configPath)
	if err != nil {
		return
	}
	
	var customThemes map[string]*Theme
	if err := json.Unmarshal(data, &customThemes); err != nil {
		return
	}
	
	// Add custom themes
	for name, theme := range customThemes {
		tm.themes[name] = theme
	}
}

// SaveCustomThemes saves custom themes to config file
func (tm *ThemeManager) SaveCustomThemes() error {
	// Filter out built-in themes
	customThemes := make(map[string]*Theme)
	builtinNames := []string{"dark", "light", "high-contrast", "minimal", "neon", "ocean"}
	
	for name, theme := range tm.themes {
		isBuiltin := false
		for _, builtinName := range builtinNames {
			if name == builtinName {
				isBuiltin = true
				break
			}
		}
		if !isBuiltin {
			customThemes[name] = theme
		}
	}
	
	// Ensure config directory exists
	dir := filepath.Dir(tm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	data, err := json.MarshalIndent(customThemes, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal themes: %w", err)
	}
	
	if err := os.WriteFile(tm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write themes file: %w", err)
	}
	
	return nil
}

// GetTheme returns the current theme
func (tm *ThemeManager) GetTheme() *Theme {
	return tm.currentTheme
}

// SetTheme sets the current theme by name
func (tm *ThemeManager) SetTheme(name string) error {
	theme, exists := tm.themes[name]
	if !exists {
		return fmt.Errorf("theme %q not found", name)
	}
	
	tm.currentTheme = theme
	return nil
}

// GetAvailableThemes returns a list of available theme names
func (tm *ThemeManager) GetAvailableThemes() []string {
	var names []string
	for name := range tm.themes {
		names = append(names, name)
	}
	return names
}

// AddCustomTheme adds a custom theme
func (tm *ThemeManager) AddCustomTheme(theme *Theme) error {
	if theme.Name == "" {
		return fmt.Errorf("theme name cannot be empty")
	}
	
	tm.themes[theme.Name] = theme
	return nil
}

// RemoveCustomTheme removes a custom theme
func (tm *ThemeManager) RemoveCustomTheme(name string) error {
	// Don't allow removing built-in themes
	builtinNames := []string{"dark", "light", "high-contrast", "minimal", "neon", "ocean"}
	for _, builtinName := range builtinNames {
		if name == builtinName {
			return fmt.Errorf("cannot remove built-in theme %q", name)
		}
	}
	
	if _, exists := tm.themes[name]; !exists {
		return fmt.Errorf("theme %q not found", name)
	}
	
	delete(tm.themes, name)
	return nil
}

// CreateThemedStyle creates a lipgloss style using current theme
func (tm *ThemeManager) CreateThemedStyle(styleType string) lipgloss.Style {
	theme := tm.currentTheme
	if theme == nil {
		// Fallback to default style
		return lipgloss.NewStyle()
	}
	
	style := lipgloss.NewStyle()
	
	switch styleType {
	case "title":
		style = style.
			Bold(true).
			Foreground(theme.Colors.Primary).
			Padding(theme.Styles.Padding)
			
	case "subtitle":
		style = style.
			Foreground(theme.Colors.Secondary).
			Padding(theme.Styles.Padding)
			
	case "text":
		style = style.
			Foreground(theme.Colors.Text)
			
	case "muted":
		style = style.
			Foreground(theme.Colors.TextMuted)
			
	case "success":
		style = style.
			Bold(true).
			Foreground(theme.Colors.Success)
			
	case "warning":
		style = style.
			Bold(true).
			Foreground(theme.Colors.Warning)
			
	case "error":
		style = style.
			Bold(true).
			Foreground(theme.Colors.Error)
			
	case "info":
		style = style.
			Foreground(theme.Colors.Info)
			
	case "border":
		style = style.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Colors.Border)
			
	case "border-focus":
		style = style.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Colors.BorderFocus)
			
	case "surface":
		style = style.
			Background(theme.Colors.Surface).
			Foreground(theme.Colors.Text).
			Padding(theme.Styles.Padding)
			
	case "selection":
		style = style.
			Background(theme.Colors.Selection).
			Foreground(theme.Colors.Text)
			
	case "hover":
		style = style.
			Background(theme.Colors.Hover).
			Foreground(theme.Colors.Text)
			
	case "accent":
		style = style.
			Bold(true).
			Foreground(theme.Colors.Accent)
			
	case "inverse":
		style = style.
			Background(theme.Colors.Text).
			Foreground(theme.Colors.TextInverse).
			Padding(theme.Styles.Padding)
	}
	
	return style
}

// GetThemePreview returns a preview string of the theme
func (tm *ThemeManager) GetThemePreview(themeName string) string {
	oldTheme := tm.currentTheme
	defer func() { tm.currentTheme = oldTheme }() // Restore original theme
	
	if err := tm.SetTheme(themeName); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	
	theme := tm.currentTheme
	
	// Create preview using theme colors
	preview := fmt.Sprintf(`%s
%s

%s  %s  %s  %s

%s
%s

%s
  %s
  %s
  %s`,
		tm.CreateThemedStyle("title").Render("■ "+theme.Name),
		tm.CreateThemedStyle("muted").Render(theme.Description),
		
		tm.CreateThemedStyle("success").Render("● Success"),
		tm.CreateThemedStyle("warning").Render("● Warning"), 
		tm.CreateThemedStyle("error").Render("● Error"),
		tm.CreateThemedStyle("info").Render("● Info"),
		
		tm.CreateThemedStyle("subtitle").Render("▓ Sample Content"),
		tm.CreateThemedStyle("text").Render("This is regular text in the current theme."),
		
		tm.CreateThemedStyle("border").Render(" Bordered Container "),
		tm.CreateThemedStyle("accent").Render("• Accent text"),
		tm.CreateThemedStyle("selection").Render("• Selected item"),
		tm.CreateThemedStyle("muted").Render("• Muted text"),
	)
	
	return preview
}