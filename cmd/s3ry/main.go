package main

import (
	"fmt"
	"log"
	"os"

	"github.com/seike460/s3ry"
	"github.com/seike460/s3ry/internal/config"
	ui "github.com/seike460/s3ry/internal/ui/app"
)

func main() {
	// Parse command-line flags
	flags := parseFlags()

	// Load configuration
	cfg, err := loadConfig(flags)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize i18n system with configured language
	cfg.InitializeI18n()

	// Set up logging if verbose mode is enabled
	if flags.Verbose {
		cfg.Logging.Level = "debug"
	}
	setupLogging(cfg)

	// Determine which UI to use
	if shouldUseNewUI(flags, cfg) {
		runNewUI(cfg, flags)
	} else {
		runLegacyUI(cfg, flags)
	}
}

// loadConfig loads configuration from file and applies flag overrides
func loadConfig(flags *Flags) (*config.Config, error) {
	var cfg *config.Config
	var err error

	if flags.ConfigFile != "" {
		// Load from specific file
		cfg = config.Default()
		if _, readErr := os.ReadFile(flags.ConfigFile); readErr == nil {
			// Parse as YAML
			// Note: This would need yaml parsing, for now use defaults
			// TODO: Implement specific file loading
		}
	} else {
		// Load from default locations
		cfg, err = config.Load()
		if err != nil {
			return nil, err
		}
	}

	// Apply flag overrides
	if flags.Region != "" {
		cfg.AWS.Region = flags.Region
	}
	if flags.Profile != "" {
		cfg.AWS.Profile = flags.Profile
	}
	if flags.Language != "" {
		cfg.UI.Language = flags.Language
	}
	if flags.LogLevel != "" {
		cfg.Logging.Level = flags.LogLevel
	}
	if flags.NewUI {
		cfg.UI.Mode = "bubbles"
	}
	// Store modern backend preference in config for later use
	if flags.ModernBackend {
		cfg.Performance.Workers = 5 // Enable worker pool
	}

	return cfg, nil
}

// shouldUseNewUI determines whether to use the new Bubble Tea UI
func shouldUseNewUI(flags *Flags, cfg *config.Config) bool {
	// --legacy-ui flag forces legacy UI
	if flags.LegacyUI {
		return false
	}

	// --new-ui flag forces new UI (explicit override)
	if flags.NewUI {
		return true
	}

	// Default: use new UI (modern default behavior)
	// Check configuration for user preference
	if cfg.IsNewUIEnabled() {
		return true
	}

	// Final fallback: default to new UI
	return true
}

// runNewUI starts the new Bubble Tea UI
func runNewUI(cfg *config.Config, flags *Flags) {
	fmt.Println("🚀 Starting new Bubble Tea UI...")
	
	// Check if new UI implementation is available
	if !isNewUIAvailable() {
		fmt.Println("❌ New UI not available in this environment (no TTY)")
		fmt.Println("💡 Falling back to legacy UI...")
		runLegacyUI(cfg, flags)
		return
	}
	
	// Start Bubble Tea application
	if err := ui.Run(cfg); err != nil {
		log.Fatalf("Failed to run new UI: %v", err)
	}
}

// runLegacyUI starts the legacy promptui-based UI
func runLegacyUI(cfg *config.Config, flags *Flags) {
	if cfg.Logging.Level == "debug" {
		fmt.Printf("🔧 Using legacy UI with region: %s\n", cfg.GetRegion())
	}

	// Use the configured region instead of hardcoded selection
	region := cfg.GetRegion()
	if cfg.AWS.Region != "" {
		region = cfg.AWS.Region
	}

	// Original legacy implementation
	selectedRegion, selectBucket := s3ry.SelectBucketAndRegion()
	
	// Override region if specified in config/flags
	if region != "ap-northeast-1" && region != selectedRegion {
		selectedRegion = region
		fmt.Printf("Using configured region: %s\n", region)
	}

	// Add modern backend support message
	if flags.ModernBackend {
		fmt.Printf("🚀 Modern backend enabled - enhanced performance and worker pool active\n")
	}

	s3ry.OperationsWithBackend(selectedRegion, selectBucket, flags.ModernBackend)
}

// isNewUIAvailable checks if the new UI implementation is ready
func isNewUIAvailable() bool {
	// Check if we're in a TTY environment
	if _, err := os.Stat("/dev/tty"); os.IsNotExist(err) {
		return false
	}
	// The new UI implementation is now ready!
	return true
}

// setupLogging configures logging based on configuration
func setupLogging(cfg *config.Config) {
	switch cfg.Logging.Level {
	case "debug":
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	case "info", "warn", "error":
		log.SetFlags(log.LstdFlags)
	default:
		log.SetFlags(log.LstdFlags)
	}

	if cfg.Logging.File != "" {
		file, err := os.OpenFile(cfg.Logging.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Printf("Warning: Could not open log file %s: %v", cfg.Logging.File, err)
		} else {
			log.SetOutput(file)
		}
	}
}