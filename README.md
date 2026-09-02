[![Build Status](https://github.com/seike460/s3ry/workflows/CI/badge.svg)](https://github.com/seike460/s3ry/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/seike460/s3ry)](https://goreportcard.com/report/github.com/seike460/s3ry)
[![codecov](https://codecov.io/gh/seike460/s3ry/branch/master/graph/badge.svg)](https://codecov.io/gh/seike460/s3ry)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/seike460/s3ry)](https://github.com/seike460/s3ry/releases)

# S3ry - AWS S3 Interactive Terminal Client

S3ry is a modern, interactive terminal-based AWS S3 management tool written in Go. It provides a Bubble Tea TUI for efficient S3 operations.

> 📚 **[ROADMAP](ROADMAP.md)** | 📋 **[RELEASE NOTES](RELEASE_NOTES.md)**

## ✨ Features

### 🎨 **Interactive Interface**
- **Modern Bubble Tea TUI** - Interactive terminal interface (default)
- **Terminal requirement** - stdin and stdout must both be interactive terminals

### ⚡ **Performance Options**  
- **Concurrent operations** - Configurable worker pool for bulk operations
- **Progress tracking** - Real-time feedback for long-running operations

### 🔄 **Core S3 Operations**
- **📥 Download** - Single and bulk file downloads  
- **📤 Upload** - File uploads with progress tracking
- **🗑️ Delete** - Object deletion with confirmation
- **📋 List** - Bucket and object browsing with search
- **📄 Export** - Generate object lists for analysis

### 🌍 **Multi-language Support**
- **English** - Default interface language
- **Japanese** - Full Japanese localization (`--lang ja`)
- **i18n framework** - Extensible internationalization system

![S3ry Modern TUI](doc/S3ry.png)

## 🚀 Installation

### Download Pre-built Binaries
```bash
# Linux
curl -LO https://github.com/seike460/s3ry/releases/latest/download/s3ry-linux-amd64
chmod +x s3ry-linux-amd64
sudo mv s3ry-linux-amd64 /usr/local/bin/s3ry

# macOS
curl -LO https://github.com/seike460/s3ry/releases/latest/download/s3ry-darwin-amd64
chmod +x s3ry-darwin-amd64
sudo mv s3ry-darwin-amd64 /usr/local/bin/s3ry

# Windows
# Download s3ry-windows-amd64.exe from releases and add to PATH
```

### Package Managers
```bash
# Arch Linux (AUR)
yay -S s3ry

# Windows (Chocolatey) 
choco install s3ry

# macOS (Homebrew)
brew install s3ry
```

### Build from Source
```bash
git clone https://github.com/seike460/s3ry.git
cd s3ry
go build -o s3ry ./cmd/s3ry
```

## 🎯 Usage

### Prerequisites
- AWS credentials configured via:
  - AWS CLI (`aws configure`)
  - Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
  - IAM roles or instance profiles

### Basic Usage
```bash
# Start interactive mode (modern TUI by default)
s3ry

# Specify AWS region and profile
s3ry --region us-west-2 --profile production

# Use a specific configuration file
s3ry --config ./s3ry.yml

# Enable Japanese interface
s3ry --lang ja

# Debug mode with verbose logging
s3ry --verbose --log-level debug
```

### Command Line Options
```bash
s3ry [OPTIONS]

OPTIONS:
  --region REGION      AWS region (overrides AWS_REGION)
  --profile PROFILE    AWS profile (overrides AWS_PROFILE)
  --lang LANGUAGE      Interface language: en (default) or ja
  --config FILE        Configuration file path
  --log-level LEVEL    Log level: debug, info, warn, error
  --verbose            Enable verbose/debug output
  --version            Show version information
  --help               Show help message
```

## ⚙️ Configuration

### Environment Variables
```bash
# AWS Configuration
export AWS_REGION=us-west-2
export AWS_PROFILE=production
export AWS_ENDPOINT_URL=https://custom.s3.endpoint

# S3ry Configuration  
export S3RY_LANGUAGE=en              # or "ja"
export S3RY_LOG_LEVEL=info           # debug, info, warn, error
```

### Configuration File
S3ry looks for configuration in these locations:
- `~/.s3ry.yml`
- `~/.config/s3ry/config.yml`
- `./s3ry.yml`

Basic configuration example:
```yaml
ui:
  language: "en"         # "en" or "ja"

aws:
  region: "us-west-2"
  profile: "default"

performance:
  workers: 5             # Worker pool size for modern backend

logging:
  level: "info"          # debug, info, warn, error
  file: ""               # Log file path (empty = console only)
```

## 🎮 Interface Overview

### Modern Bubble Tea TUI (Default)
When you run `s3ry` in an interactive terminal, you'll see an interface with:
- **Arrow keys** - Navigate buckets and objects
- **Enter** - Select and perform actions  
- **Tab** - Switch between panels
- **?** - Show help and keyboard shortcuts
- **q** - Quit application

### Non-interactive terminals
The CLI currently requires both stdin and stdout to be interactive terminals. Non-interactive subcommands are planned.

## 📁 Project Structure

The project includes several applications:

```
s3ry/
├── cmd/
│   ├── s3ry/          # Main CLI application
│   ├── s3ry-desktop/  # Desktop application (Wails)
│   ├── s3ry-web/      # Web server
│   ├── s3ry-tui/      # Standalone TUI
│   └── s3ry-vscode/   # VSCode extension backend
├── internal/          # Core implementation
│   ├── s3/           # S3 operations engine
│   ├── ui/           # UI components
│   ├── config/       # Configuration management
│   ├── worker/       # Worker pool implementation
│   └── i18n/         # Internationalization
├── vscode-extension/  # VSCode extension
└── test/             # Test suites
```

## 🔧 Development

### Requirements
- Go 1.23.0+
- Make (optional, for development tasks)

### Build from Source
```bash
git clone https://github.com/seike460/s3ry.git
cd s3ry

# Build main CLI
go build -o s3ry ./cmd/s3ry

# Build all applications
make build-all

# Run tests
go test ./...

# Run with race detection
go test -race ./...
```

### Architecture
S3ry provides dual backend implementations:
- **Legacy backend** - Uses AWS SDK v1 with traditional operations
- **Modern backend** - Enhanced performance with worker pools and optimized operations

Both backends support the same operations but the modern backend provides:
- Concurrent processing via worker pools
- Progress tracking for long operations
- Enhanced error handling and recovery
- Resource pooling for better performance

## 🤝 Contributing

We welcome contributions! Please follow these steps:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass (`go test ./...`)
6. Commit your changes (`git commit -m 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

### Code Quality Guidelines
- Follow Go best practices and conventions
- Maintain backward compatibility
- Add tests for new features
- Update documentation as needed
- Use meaningful commit messages

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **Go Team** - For the excellent programming language and ecosystem
- **Bubble Tea** - For the modern TUI framework  
- **AWS SDK** - For robust S3 integration capabilities
- **Community** - For feedback, contributions, and support
