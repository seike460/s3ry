package main

import (
	"fmt"
	"log"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/seike460/s3ry/internal/config"
	ui "github.com/seike460/s3ry/internal/ui/app"
)

func main() {
	// Parse command-line flags
	flags := parseFlags()

	// Load configuration
	cfg, err := loadConfig(flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize i18n system with configured language
	cfg.InitializeI18n()

	// Set up logging if verbose mode is enabled
	if flags.Verbose {
		cfg.Logging.Level = "debug"
	}
	setupLogging(cfg)

	runUI(cfg)
}

// loadConfig loads configuration from file and applies flag overrides
func loadConfig(flags *Flags) (*config.Config, error) {
	var cfg *config.Config
	var err error

	if flags.ConfigFile != "" {
		cfg, err = config.LoadFromFile(flags.ConfigFile)
	} else {
		// Load from default locations
		cfg, err = config.Load()
	}
	if err != nil {
		return nil, err
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

	return cfg, nil
}

// runUI checks for an interactive terminal before starting the Bubble Tea UI.
func runUI(cfg *config.Config) {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "s3ry needs an interactive terminal. Non-interactive subcommands are planned; see README.")
		os.Exit(1)
	}

	if err := ui.Run(cfg); err != nil {
		log.Fatalf("Failed to run Bubble Tea UI: %v", err)
	}
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
