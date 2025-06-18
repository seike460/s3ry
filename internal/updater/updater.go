// Package updater provides automatic update checking and notification for s3ry
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Updater handles version checking and update notifications
type Updater struct {
	mu              sync.RWMutex
	currentVersion  string
	latestVersion   string
	checkURL        string
	downloadURL     string
	lastChecked     time.Time
	updateAvailable bool
	config          *UpdaterConfig
	httpClient      *http.Client
}

// UpdaterConfig configures the updater behavior
type UpdaterConfig struct {
	CheckInterval     time.Duration `json:"check_interval"`
	AutoCheck         bool          `json:"auto_check"`
	IncludePrerelease bool          `json:"include_prerelease"`
	NotifyOnStartup   bool          `json:"notify_on_startup"`
	LastCheckTime     time.Time     `json:"last_check_time"`
	SkippedVersion    string        `json:"skipped_version"`
	ReminderInterval  time.Duration `json:"reminder_interval"`
	LastReminder      time.Time     `json:"last_reminder"`
}

// ReleaseInfo contains information about a release
type ReleaseInfo struct {
	Version     string    `json:"version"`
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Assets      []Asset   `json:"assets"`
	HTMLURL     string    `json:"html_url"`
}

// Asset represents a downloadable asset
type Asset struct {
	Name          string `json:"name"`
	DownloadURL   string `json:"browser_download_url"`
	Size          int64  `json:"size"`
	ContentType   string `json:"content_type"`
	DownloadCount int64  `json:"download_count"`
}

// UpdateCheckResult contains the result of an update check
type UpdateCheckResult struct {
	UpdateAvailable bool           `json:"update_available"`
	CurrentVersion  string         `json:"current_version"`
	LatestVersion   string         `json:"latest_version"`
	ReleaseInfo     *ReleaseInfo   `json:"release_info,omitempty"`
	DownloadURL     string         `json:"download_url,omitempty"`
	InstallCommand  string         `json:"install_command,omitempty"`
	ReleaseNotes    string         `json:"release_notes,omitempty"`
	Severity        UpdateSeverity `json:"severity"`
}

// UpdateSeverity indicates the importance of an update
type UpdateSeverity string

const (
	SeverityMajor    UpdateSeverity = "major"    // Breaking changes, new major features
	SeverityMinor    UpdateSeverity = "minor"    // New features, backward compatible
	SeverityPatch    UpdateSeverity = "patch"    // Bug fixes, security patches
	SeverityCritical UpdateSeverity = "critical" // Security fixes, critical bugs
)

const (
	defaultCheckURL      = "https://api.github.com/repos/seike460/s3ry/releases/latest"
	defaultCheckInterval = 24 * time.Hour
	configFileName       = ".s3ry/updater.json"
)

// NewUpdater creates a new updater instance
func NewUpdater(currentVersion string) (*Updater, error) {
	config, err := loadConfig()
	if err != nil {
		// Create default config if none exists
		config = &UpdaterConfig{
			CheckInterval:     defaultCheckInterval,
			AutoCheck:         true,
			IncludePrerelease: false,
			NotifyOnStartup:   true,
			ReminderInterval:  7 * 24 * time.Hour, // Weekly reminders
		}
		_ = saveConfig(config) // Ignore error for default config
	}

	updater := &Updater{
		currentVersion: strings.TrimPrefix(currentVersion, "v"),
		checkURL:       defaultCheckURL,
		config:         config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	return updater, nil
}

// CheckForUpdates checks for available updates
func (u *Updater) CheckForUpdates(ctx context.Context) (*UpdateCheckResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.lastChecked = time.Now()
	u.config.LastCheckTime = u.lastChecked

	release, err := u.fetchLatestRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	// Skip prerelease if not enabled
	if release.Prerelease && !u.config.IncludePrerelease {
		return &UpdateCheckResult{
			UpdateAvailable: false,
			CurrentVersion:  u.currentVersion,
			LatestVersion:   u.currentVersion,
		}, nil
	}

	u.latestVersion = strings.TrimPrefix(release.TagName, "v")
	u.updateAvailable = u.isNewerVersion(u.latestVersion, u.currentVersion)

	result := &UpdateCheckResult{
		UpdateAvailable: u.updateAvailable,
		CurrentVersion:  u.currentVersion,
		LatestVersion:   u.latestVersion,
		ReleaseInfo:     release,
		ReleaseNotes:    release.Body,
		Severity:        u.determineSeverity(u.currentVersion, u.latestVersion),
	}

	if u.updateAvailable {
		// Find appropriate download asset
		asset := u.findAssetForPlatform(release.Assets)
		if asset != nil {
			result.DownloadURL = asset.DownloadURL
			result.InstallCommand = u.generateInstallCommand(asset)
		}
	}

	// Save updated config
	_ = saveConfig(u.config)

	return result, nil
}

// ShouldNotify determines if the user should be notified about updates
func (u *Updater) ShouldNotify() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()

	if !u.updateAvailable {
		return false
	}

	// Don't notify if user has skipped this version
	if u.config.SkippedVersion == u.latestVersion {
		return false
	}

	// Check if enough time has passed since last reminder
	if time.Since(u.config.LastReminder) < u.config.ReminderInterval {
		return false
	}

	return true
}

// NotifyUpdate displays update notification to user
func (u *Updater) NotifyUpdate(result *UpdateCheckResult) {
	if !result.UpdateAvailable {
		return
	}

	u.mu.Lock()
	u.config.LastReminder = time.Now()
	_ = saveConfig(u.config)
	u.mu.Unlock()

	severity := result.Severity
	icon := u.getSeverityIcon(severity)

	fmt.Printf("\n%s ═══════════════════════════════════════════════════════════\n", icon)
	fmt.Printf("   S3ry Update Available: v%s → v%s\n", result.CurrentVersion, result.LatestVersion)
	fmt.Printf("═══════════════════════════════════════════════════════════\n\n")

	// Display severity-specific message
	switch severity {
	case SeverityCritical:
		fmt.Printf("🚨 CRITICAL UPDATE: This update contains important security fixes.\n")
		fmt.Printf("   Please update immediately to ensure security.\n\n")
	case SeverityMajor:
		fmt.Printf("🎉 MAJOR UPDATE: This update includes significant new features.\n")
		fmt.Printf("   Review the changelog before updating.\n\n")
	case SeverityMinor:
		fmt.Printf("✨ NEW FEATURES: This update adds new functionality.\n")
		fmt.Printf("   Update when convenient.\n\n")
	case SeverityPatch:
		fmt.Printf("🔧 BUG FIXES: This update fixes bugs and improves stability.\n")
		fmt.Printf("   Consider updating for better reliability.\n\n")
	}

	// Show installation options
	fmt.Printf("Installation Options:\n")

	if result.InstallCommand != "" {
		fmt.Printf("  Quick Install: %s\n", result.InstallCommand)
	}

	if result.DownloadURL != "" {
		fmt.Printf("  Manual Download: %s\n", result.DownloadURL)
	}

	if result.ReleaseInfo != nil && result.ReleaseInfo.HTMLURL != "" {
		fmt.Printf("  Release Page: %s\n", result.ReleaseInfo.HTMLURL)
	}

	fmt.Printf("\nUpdate Management:\n")
	fmt.Printf("  s3ry update check       - Check for updates manually\n")
	fmt.Printf("  s3ry update install     - Install the latest version\n")
	fmt.Printf("  s3ry update skip        - Skip this version\n")
	fmt.Printf("  s3ry update config      - Configure update settings\n")

	// Show abbreviated release notes
	if result.ReleaseNotes != "" {
		fmt.Printf("\nWhat's New:\n")
		lines := strings.Split(result.ReleaseNotes, "\n")
		maxLines := 5
		for i, line := range lines {
			if i >= maxLines {
				fmt.Printf("  ... (see full changelog at release page)\n")
				break
			}
			if strings.TrimSpace(line) != "" {
				fmt.Printf("  %s\n", line)
			}
		}
	}

	fmt.Printf("\n═══════════════════════════════════════════════════════════\n")
}

// SkipVersion marks a version as skipped
func (u *Updater) SkipVersion(version string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.config.SkippedVersion = strings.TrimPrefix(version, "v")
	return saveConfig(u.config)
}

// EnableAutoUpdates enables automatic update checking
func (u *Updater) EnableAutoUpdates() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.config.AutoCheck = true
	return saveConfig(u.config)
}

// DisableAutoUpdates disables automatic update checking
func (u *Updater) DisableAutoUpdates() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.config.AutoCheck = false
	return saveConfig(u.config)
}

// GetConfig returns a copy of the current configuration
func (u *Updater) GetConfig() UpdaterConfig {
	u.mu.RLock()
	defer u.mu.RUnlock()

	return *u.config
}

// SetConfig updates the configuration
func (u *Updater) SetConfig(config UpdaterConfig) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.config = &config
	return saveConfig(u.config)
}

// AutoCheckForUpdates performs automatic update checking in the background
func (u *Updater) AutoCheckForUpdates(ctx context.Context) {
	if !u.config.AutoCheck {
		return
	}

	// Check immediately if enough time has passed
	if time.Since(u.config.LastCheckTime) >= u.config.CheckInterval {
		result, err := u.CheckForUpdates(ctx)
		if err == nil && u.ShouldNotify() {
			u.NotifyUpdate(result)
		}
	}

	// Set up periodic checking
	ticker := time.NewTicker(u.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := u.CheckForUpdates(ctx)
			if err == nil && u.ShouldNotify() {
				u.NotifyUpdate(result)
			}
		}
	}
}

// fetchLatestRelease fetches the latest release information from GitHub
func (u *Updater) fetchLatestRelease(ctx context.Context) (*ReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u.checkURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "s3ry-updater/1.0")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release ReleaseInfo
	err = json.NewDecoder(resp.Body).Decode(&release)
	if err != nil {
		return nil, err
	}

	return &release, nil
}

// isNewerVersion compares two semantic versions
func (u *Updater) isNewerVersion(latest, current string) bool {
	latestParts := u.parseVersion(latest)
	currentParts := u.parseVersion(current)

	for i := 0; i < 3; i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}

	return false
}

// parseVersion parses a semantic version string
func (u *Updater) parseVersion(version string) [3]int {
	parts := strings.Split(version, ".")
	result := [3]int{0, 0, 0}

	for i := 0; i < len(parts) && i < 3; i++ {
		if num, err := strconv.Atoi(parts[i]); err == nil {
			result[i] = num
		}
	}

	return result
}

// determineSeverity determines the severity of an update
func (u *Updater) determineSeverity(current, latest string) UpdateSeverity {
	currentParts := u.parseVersion(current)
	latestParts := u.parseVersion(latest)

	// Major version change
	if latestParts[0] > currentParts[0] {
		return SeverityMajor
	}

	// Minor version change
	if latestParts[1] > currentParts[1] {
		return SeverityMinor
	}

	// Patch version change
	if latestParts[2] > currentParts[2] {
		// Check if it's a security/critical patch (heuristic)
		if latestParts[2]-currentParts[2] > 2 {
			return SeverityCritical
		}
		return SeverityPatch
	}

	return SeverityPatch
}

// findAssetForPlatform finds the appropriate asset for the current platform
func (u *Updater) findAssetForPlatform(assets []Asset) *Asset {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// Convert Go arch names to common names
	switch archName {
	case "amd64":
		archName = "x86_64"
	case "386":
		archName = "i386"
	}

	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, osName) && strings.Contains(name, archName) {
			return &asset
		}
	}

	// Fallback: look for generic binary names
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, osName) {
			return &asset
		}
	}

	return nil
}

// generateInstallCommand generates platform-specific install commands
func (u *Updater) generateInstallCommand(asset *Asset) string {
	switch runtime.GOOS {
	case "darwin":
		if strings.Contains(asset.Name, ".tar.gz") {
			return "curl -sSL " + asset.DownloadURL + " | tar -xzv && sudo mv s3ry /usr/local/bin/"
		}
		return "brew upgrade s3ry"
	case "linux":
		if strings.Contains(asset.Name, ".deb") {
			return "curl -sSL " + asset.DownloadURL + " -o s3ry.deb && sudo dpkg -i s3ry.deb"
		}
		if strings.Contains(asset.Name, ".rpm") {
			return "curl -sSL " + asset.DownloadURL + " -o s3ry.rpm && sudo rpm -U s3ry.rpm"
		}
		return "curl -sSL " + asset.DownloadURL + " | tar -xzv && sudo mv s3ry /usr/local/bin/"
	case "windows":
		return "scoop update s3ry"
	default:
		return "curl -sSL " + asset.DownloadURL + " -o s3ry"
	}
}

// getSeverityIcon returns an icon for the update severity
func (u *Updater) getSeverityIcon(severity UpdateSeverity) string {
	switch severity {
	case SeverityCritical:
		return "🚨"
	case SeverityMajor:
		return "🎉"
	case SeverityMinor:
		return "✨"
	case SeverityPatch:
		return "🔧"
	default:
		return "📦"
	}
}

// Configuration management

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, configFileName), nil
}

func loadConfig() (*UpdaterConfig, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config UpdaterConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func saveConfig(config *UpdaterConfig) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
