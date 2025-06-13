package telemetry

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ConsentManager handles user consent for telemetry
type ConsentManager struct {
	client *Client
}

// NewConsentManager creates a new consent manager
func NewConsentManager(client *Client) *ConsentManager {
	return &ConsentManager{
		client: client,
	}
}

// RequestConsent prompts the user for telemetry consent
func (cm *ConsentManager) RequestConsent() error {
	if cm.client.IsEnabled() {
		return nil // Already consented
	}

	fmt.Println(`
╭─────────────────────────────────────────────────────────────────────────────╮
│                          📊 Help Improve s3ry                              │
╰─────────────────────────────────────────────────────────────────────────────╯

s3ry would like to collect anonymous usage data to help improve the tool.

What we collect:
  ✅ Command usage patterns (which commands you use)
  ✅ Performance metrics (throughput, response times)
  ✅ Error rates and types (to fix bugs faster)
  ✅ System information (OS, architecture)

What we DON'T collect:
  ❌ File names, paths, or content
  ❌ AWS credentials or personal data
  ❌ Sensitive S3 bucket information
  ❌ Any personally identifiable information

Benefits:
  🚀 Helps us optimize performance for your use cases
  🐛 Enables faster bug fixes and improvements
  📈 Guides feature development priorities
  🔒 All data is anonymized and encrypted

You can:
  • View your data ID: s3ry telemetry status
  • Disable anytime: s3ry telemetry disable
  • See what's sent: s3ry telemetry debug

`)

	fmt.Print("Enable anonymous usage analytics? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response == "y" || response == "yes" {
		fmt.Println("\n✅ Thank you! Telemetry enabled.")
		fmt.Println("   You can disable it anytime with: s3ry telemetry disable")
		return cm.client.Enable()
	} else {
		fmt.Println("\n❌ Telemetry disabled.")
		fmt.Println("   You can enable it later with: s3ry telemetry enable")
		return cm.client.Disable()
	}
}

// ShowStatus displays current telemetry status
func (cm *ConsentManager) ShowStatus() {
	fmt.Println("📊 S3ry Telemetry Status")
	fmt.Println("═══════════════════════")
	
	if cm.client.IsEnabled() {
		fmt.Println("Status: ✅ ENABLED")
		fmt.Printf("User ID: %s\n", cm.client.userID)
		fmt.Printf("Session ID: %s\n", cm.client.sessionID)
		fmt.Printf("Endpoint: %s\n", cm.client.endpoint)
		fmt.Println("\nCommands:")
		fmt.Println("  s3ry telemetry disable  - Disable telemetry")
		fmt.Println("  s3ry telemetry debug    - Show debug info")
	} else {
		fmt.Println("Status: ❌ DISABLED")
		fmt.Println("\nTelemetry is currently disabled.")
		fmt.Println("No usage data is being collected.")
		fmt.Println("\nCommands:")
		fmt.Println("  s3ry telemetry enable   - Enable telemetry")
	}
	
	fmt.Println("\nPrivacy:")
	fmt.Println("  • No personal data is collected")
	fmt.Println("  • No file names or content")
	fmt.Println("  • No AWS credentials")
	fmt.Println("  • All data is anonymized")
}

// ShowDebugInfo displays debug information about telemetry
func (cm *ConsentManager) ShowDebugInfo() {
	if !cm.client.IsEnabled() {
		fmt.Println("❌ Telemetry is disabled. No data is being collected.")
		return
	}

	fmt.Println("🔍 S3ry Telemetry Debug Information")
	fmt.Println("═════════════════════════════════")
	
	system := getSystemInfo()
	
	fmt.Printf("Client Information:\n")
	fmt.Printf("  Version: %s\n", cm.client.version)
	fmt.Printf("  User ID: %s\n", cm.client.userID)
	fmt.Printf("  Session ID: %s\n", cm.client.sessionID)
	fmt.Printf("  Endpoint: %s\n", cm.client.endpoint)
	
	fmt.Printf("\nSystem Information:\n")
	fmt.Printf("  OS: %s\n", system.OS)
	fmt.Printf("  Architecture: %s\n", system.Arch)
	fmt.Printf("  Go Version: %s\n", system.GoVersion)
	fmt.Printf("  CPU Count: %d\n", system.NumCPU)
	fmt.Printf("  Container: %v\n", system.IsContainer)
	if system.CloudProvider != "" {
		fmt.Printf("  Cloud Provider: %s\n", system.CloudProvider)
	}
	
	fmt.Printf("\nExample Event (anonymized):\n")
	fmt.Println(`{
  "event_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "[YOUR_ANONYMOUS_ID]",
  "session_id": "[SESSION_ID]",
  "timestamp": "2024-01-01T12:00:00Z",
  "event_type": "command",
  "command": "list",
  "duration_ms": 1250,
  "success": true,
  "performance": {
    "objects_processed": 1000,
    "bytes_transferred": 1048576,
    "throughput_mbps": 15.2,
    "worker_pool_size": 10,
    "memory_usage_bytes": 52428800
  },
  "system": {
    "os": "linux",
    "arch": "amd64",
    "go_version": "go1.21.0",
    "num_cpu": 8,
    "is_container": false
  },
  "version": "2.0.0"
}`)
	
	fmt.Println("\n🔒 Privacy Notes:")
	fmt.Println("  • User ID is randomly generated and not linked to you")
	fmt.Println("  • No file names, paths, or content are collected")
	fmt.Println("  • No AWS credentials or sensitive data")
	fmt.Println("  • Data helps improve performance and fix bugs")
}

// EnableWithConsent enables telemetry after showing consent information
func (cm *ConsentManager) EnableWithConsent() error {
	fmt.Println("📊 Enabling s3ry telemetry...")
	err := cm.client.Enable()
	if err != nil {
		return fmt.Errorf("failed to enable telemetry: %w", err)
	}
	
	fmt.Println("✅ Telemetry enabled successfully!")
	fmt.Println("\nWhat happens now:")
	fmt.Println("  • Anonymous usage data will be collected")
	fmt.Println("  • Performance metrics will help optimize s3ry")
	fmt.Println("  • Error data will help fix bugs faster")
	fmt.Println("  • No personal or sensitive data is collected")
	fmt.Println("\nManage telemetry:")
	fmt.Println("  s3ry telemetry status   - View current status")
	fmt.Println("  s3ry telemetry disable  - Disable telemetry")
	fmt.Println("  s3ry telemetry debug    - View debug information")
	
	return nil
}

// DisableWithConfirmation disables telemetry after confirmation
func (cm *ConsentManager) DisableWithConfirmation() error {
	fmt.Println("📊 Disabling s3ry telemetry...")
	err := cm.client.Disable()
	if err != nil {
		return fmt.Errorf("failed to disable telemetry: %w", err)
	}
	
	fmt.Println("❌ Telemetry disabled successfully!")
	fmt.Println("\nWhat happens now:")
	fmt.Println("  • No usage data will be collected")
	fmt.Println("  • Existing data remains anonymous")
	fmt.Println("  • You can re-enable anytime")
	fmt.Println("\nTo re-enable:")
	fmt.Println("  s3ry telemetry enable   - Re-enable telemetry")
	
	return nil
}