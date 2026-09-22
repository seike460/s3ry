package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seike460/s3ry/internal/config"
)

func TestVersionFlagMatchesVersionCommand(t *testing.T) {
	info := BuildInfo{Version: "1.2.3", Commit: "abc123", Date: "2026-09-04"}

	var flagOutput, flagError bytes.Buffer
	flagCode := Run(context.Background(), []string{"--version"}, strings.NewReader(""), &flagOutput, &flagError, info, RunDeps{})

	var commandOutput, commandError bytes.Buffer
	commandCode := Run(context.Background(), []string{"version"}, strings.NewReader(""), &commandOutput, &commandError, info, RunDeps{})

	if flagCode != 0 || commandCode != 0 {
		t.Fatalf("version commands returned codes %d and %d", flagCode, commandCode)
	}
	if got, want := commandOutput.String(), flagOutput.String(); got != want {
		t.Fatalf("version output differs:\ncommand: %q\nflag: %q", got, want)
	}
	if flagError.Len() != 0 || commandError.Len() != 0 {
		t.Fatalf("version commands wrote errors: %q and %q", flagError.String(), commandError.String())
	}
}

func TestRunUnknownFlagReturnsUsageCode(t *testing.T) {
	var output, errorOutput bytes.Buffer

	code := Run(context.Background(), []string{"--bogus"}, strings.NewReader(""), &output, &errorOutput, BuildInfo{}, RunDeps{})

	if code != 2 {
		t.Fatalf("Run returned code %d, want 2", code)
	}
	if got, want := errorOutput.String(), "s3ry: unknown flag: --bogus\n"; got != want {
		t.Fatalf("error output = %q, want %q", got, want)
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", output.String())
	}
}

func TestRunPositionalArgReturnsUsageCode(t *testing.T) {
	var output, errorOutput bytes.Buffer

	code := Run(context.Background(), []string{"extra-arg"}, strings.NewReader(""), &output, &errorOutput, BuildInfo{}, RunDeps{})

	if code != 2 {
		t.Fatalf("Run returned code %d, want 2", code)
	}
	if got, want := errorOutput.String(), "s3ry: unknown command \"extra-arg\" for \"s3ry\"\n"; got != want {
		t.Fatalf("error output = %q, want %q", got, want)
	}
}

func TestRunCompletionZsh(t *testing.T) {
	var output, errorOutput bytes.Buffer

	code := Run(context.Background(), []string{"completion", "zsh"}, strings.NewReader(""), &output, &errorOutput, BuildInfo{}, RunDeps{})

	if code != 0 {
		t.Fatalf("Run returned code %d, want 0", code)
	}
	if !strings.Contains(output.String(), "#compdef") {
		t.Fatalf("completion output does not contain #compdef")
	}
	if errorOutput.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", errorOutput.String())
	}
}

func TestRunTTYGuard(t *testing.T) {
	deps := RunDeps{
		IsTerminal: func(uintptr) bool { return false },
		RunTUI: func(context.Context, *config.Config) error {
			t.Fatal("runTUI was called for a non-terminal")
			return nil
		},
	}

	configPath := writeCLIConfig(t)
	var output, errorOutput bytes.Buffer
	code := Run(context.Background(), []string{"--config", configPath, "--lang", "en"}, strings.NewReader(""), &output, &errorOutput, BuildInfo{}, deps)

	if code != 1 {
		t.Fatalf("Run returned code %d, want 1", code)
	}
	want := "s3ry: " + interactiveTerminalMessage + "\n"
	if got := errorOutput.String(); got != want {
		t.Fatalf("error output = %q, want %q", got, want)
	}
}

func TestRunReachesTUIWhenTerminalIsAvailable(t *testing.T) {
	called := false
	deps := RunDeps{
		IsTerminal: func(uintptr) bool { return true },
		RunTUI: func(_ context.Context, cfg *config.Config) error {
			called = true
			if cfg.AWS.Region != "us-west-2" {
				t.Errorf("region = %q, want us-west-2", cfg.AWS.Region)
			}
			if cfg.UI.Language != "en" {
				t.Errorf("language = %q, want en", cfg.UI.Language)
			}
			return nil
		},
	}

	configPath := writeCLIConfig(t)
	var output, errorOutput bytes.Buffer
	code := Run(context.Background(), []string{
		"--config", configPath,
		"--region", "us-west-2",
		"--lang", "en",
	}, strings.NewReader(""), &output, &errorOutput, BuildInfo{}, deps)

	if code != 0 {
		t.Fatalf("Run returned code %d, want 0; stderr: %q", code, errorOutput.String())
	}
	if !called {
		t.Fatal("runTUI was not called")
	}
}

func writeCLIConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s3ry.yml")
	if err := os.WriteFile(path, []byte("ui:\n  language: en\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
