package main

import (
	"flag"
	"fmt"
	"os"
)

// Flags represents command-line flags
type Flags struct {
	NewUI         bool
	LegacyUI      bool
	ModernBackend bool
	Region        string
	Profile       string
	ConfigFile    string
	Verbose       bool
	Version       bool
	Help          bool
	Language      string
	LogLevel      string
}

// parseFlags parses command-line flags
func parseFlags() *Flags {
	flags := &Flags{}

	flag.BoolVar(&flags.NewUI, "new-ui", false, "Explicitly use new Bubble Tea UI (enabled by default)")
	flag.BoolVar(&flags.NewUI, "bubbles", false, "Use new Bubble Tea UI (alias for --new-ui)")
	flag.BoolVar(&flags.LegacyUI, "legacy-ui", false, "Use legacy promptui interface instead of modern UI")
	flag.BoolVar(&flags.ModernBackend, "modern-backend", false, "Use modern S3 backend with worker pool for better performance")
	flag.StringVar(&flags.Region, "region", "", "AWS region to use")
	flag.StringVar(&flags.Profile, "profile", "", "AWS profile to use")
	flag.StringVar(&flags.ConfigFile, "config", "", "Path to config file")
	flag.BoolVar(&flags.Verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&flags.Verbose, "v", false, "Enable verbose logging (short)")
	flag.BoolVar(&flags.Version, "version", false, "Show version information")
	flag.StringVar(&flags.Language, "lang", "", "Language (en, ja)")
	flag.StringVar(&flags.LogLevel, "log-level", "", "Log level (debug, info, warn, error)")
	flag.BoolVar(&flags.Help, "help", false, "Show help")
	flag.BoolVar(&flags.Help, "h", false, "Show help (short)")

	// Custom usage function
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "S3ry - Modern S3 file manager\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s                      # Start with modern Bubble Tea UI (default)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --legacy-ui          # Use legacy promptui interface\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --modern-backend     # Use modern S3 backend\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --region us-west-2   # Use specific AWS region\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --profile dev      # Use specific AWS profile\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --lang en          # Use English language\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  AWS_REGION            # AWS region\n")
		fmt.Fprintf(os.Stderr, "  AWS_PROFILE           # AWS profile\n")
		fmt.Fprintf(os.Stderr, "  S3RY_UI_MODE          # UI mode (legacy, bubbles)\n")
		fmt.Fprintf(os.Stderr, "  S3RY_LANGUAGE         # Language (en, ja)\n")
		fmt.Fprintf(os.Stderr, "  S3RY_LOG_LEVEL        # Log level\n")
	}

	flag.Parse()

	// Handle help flag
	if flags.Help {
		flag.Usage()
		os.Exit(0)
	}

	// Handle version flag
	if flags.Version {
		showVersion()
		os.Exit(0)
	}

	return flags
}

// showVersion displays version information
func showVersion() {
	fmt.Printf("s3ry version %s\n", getVersion())
	fmt.Printf("Build commit: %s\n", getCommit())
	fmt.Printf("Build date: %s\n", getDate())
}

// Version information (will be set by ldflags during build)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func getVersion() string {
	if version == "" {
		return "dev"
	}
	return version
}

func getCommit() string {
	if commit == "" {
		return "unknown"
	}
	return commit
}

func getDate() string {
	if date == "" {
		return "unknown"
	}
	return date
}
