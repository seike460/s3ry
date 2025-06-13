package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/ui/web"
)

func main() {
	var (
		port       = flag.String("port", "8080", "Port to run the web server on")
		configPath = flag.String("config", "", "Path to configuration file")
		version    = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *version {
		fmt.Println("s3ry-web v2.0.0")
		fmt.Println("Modern S3 browser with web interface")
		os.Exit(0)
	}

	// Load configuration
	var cfg *config.Config
	var err error

	if *configPath != "" {
		cfg, err = config.LoadFromFile(*configPath)
	} else {
		cfg, err = config.Load()
	}

	if err != nil {
		log.Printf("Warning: Failed to load configuration: %v", err)
		log.Println("Using default configuration")
		cfg = config.Default()
	}

	// Validate AWS configuration
	if cfg.AWS.Region == "" {
		log.Println("Warning: No AWS region configured. Some features may not work.")
		log.Println("Please set AWS_DEFAULT_REGION environment variable or configure ~/.aws/config")
	}

	// Start web server
	log.Printf("Starting S3ry Web UI...")
	log.Printf("Configuration loaded from: %s", cfg.GetConfigPath())
	log.Printf("AWS Region: %s", cfg.AWS.Region)
	
	if err := web.RunWebUI(cfg, *port); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}
}