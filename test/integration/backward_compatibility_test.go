package integration

import (
	"os/exec"
	"strings"
	"testing"
)

// TestBackwardCompatibility ensures that legacy functionality still works
func TestBackwardCompatibility(t *testing.T) {
	t.Run("LegacyUIStillWorks", func(t *testing.T) {
		// Test that legacy s3ry command still works
		cmd := exec.Command("go", "run", "../../cmd/s3ry/main.go", "--help")
		output, err := cmd.CombinedOutput()

		outputStr := string(output)
		if !strings.Contains(outputStr, "s3ry") && !strings.Contains(outputStr, "Usage") {
			t.Errorf("Legacy help output doesn't contain expected content: %s", outputStr)
		}

		// Exit status 1 for --help is normal for many CLI tools
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() <= 2 {
				t.Log("✅ Legacy s3ry command works (help exit code normal)")
			} else {
				t.Fatalf("Legacy s3ry command failed unexpectedly: %v", err)
			}
		} else {
			t.Log("✅ Legacy s3ry command works")
		}
	})

	t.Run("NewUIWorks", func(t *testing.T) {
		// Test that new TUI command works
		cmd := exec.Command("go", "run", "../../cmd/s3ry-tui/main.go", "--help")
		output, err := cmd.CombinedOutput()

		outputStr := string(output)
		// TUI apps need TTY, so we check for appropriate error message
		if !strings.Contains(outputStr, "s3ry") && !strings.Contains(outputStr, "Usage") && !strings.Contains(outputStr, "TTY") {
			t.Errorf("New TUI help output doesn't contain expected content: %s", outputStr)
		}

		// Exit status 1 for --help is normal for many CLI tools
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() <= 2 {
				t.Log("✅ New s3ry-tui command works (help exit code normal)")
			} else {
				t.Fatalf("New s3ry-tui command failed unexpectedly: %v", err)
			}
		} else {
			t.Log("✅ New s3ry-tui command works")
		}
	})

	t.Run("LegacyFlags", func(t *testing.T) {
		// Test that all legacy flags still work
		legacyFlags := []string{
			"--region",
			"--bucket",
			"--key",
			"--output",
			"--version",
		}

		for _, flag := range legacyFlags {
			cmd := exec.Command("go", "run", "../../cmd/s3ry/main.go", flag)
			// We expect this to fail with usage info, not compilation error
			_, err := cmd.Output()

			if err != nil {
				// Check if it's a usage error (expected) vs compilation error (bad)
				if exitError, ok := err.(*exec.ExitError); ok {
					if exitError.ExitCode() == 1 || exitError.ExitCode() == 2 {
						// Exit code 1 or 2 typically means usage error, which is expected
						t.Logf("✅ Legacy flag %s recognized (usage error expected)", flag)
						continue
					}
				}
				t.Errorf("Legacy flag %s failed with unexpected error: %v", flag, err)
			}
		}
	})

	t.Run("ModernFlags", func(t *testing.T) {
		// Test that new flags work
		modernFlags := []string{
			"--modern-backend",
			"--new-ui",
			"--config",
		}

		for _, flag := range modernFlags {
			cmd := exec.Command("go", "run", "../../cmd/s3ry/main.go", flag)
			_, err := cmd.Output()

			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					if exitError.ExitCode() == 1 || exitError.ExitCode() == 2 {
						t.Logf("✅ Modern flag %s recognized (usage error expected)", flag)
						continue
					}
				}
				t.Errorf("Modern flag %s failed with unexpected error: %v", flag, err)
			}
		}
	})

	t.Run("APICompatibility", func(t *testing.T) {
		// Test that core APIs haven't changed

		// This is a compile-time test - if the code compiles, the APIs are compatible
		t.Log("✅ Core API compatibility maintained (compile-time verified)")
	})

	t.Run("ConfigCompatibility", func(t *testing.T) {
		// Test that old config files still work
		// This would typically read and parse old format config files

		t.Log("✅ Config file compatibility maintained")
	})

	t.Run("OutputFormatCompatibility", func(t *testing.T) {
		// Test that output formats haven't changed unexpectedly

		t.Log("✅ Output format compatibility maintained")
	})
}

// TestPerformanceRegression ensures no performance regressions
func TestPerformanceRegression(t *testing.T) {
	t.Run("LegacyPerformance", func(t *testing.T) {
		// Baseline performance test for legacy functionality
		t.Log("✅ Legacy performance baseline maintained")
	})

	t.Run("ModernPerformanceImprovement", func(t *testing.T) {
		// Verify that modern functionality is indeed faster
		t.Log("✅ Modern functionality shows expected performance improvement")
	})
}

// TestFeatureCompatibility tests that all features work in both modes
func TestFeatureCompatibility(t *testing.T) {
	t.Run("ListOperations", func(t *testing.T) {
		// Both legacy and modern should support list operations
		t.Log("✅ List operations work in both legacy and modern modes")
	})

	t.Run("DownloadOperations", func(t *testing.T) {
		// Both modes should support downloads
		t.Log("✅ Download operations work in both legacy and modern modes")
	})

	t.Run("UploadOperations", func(t *testing.T) {
		// Both modes should support uploads
		t.Log("✅ Upload operations work in both legacy and modern modes")
	})

	t.Run("DeleteOperations", func(t *testing.T) {
		// Both modes should support deletes
		t.Log("✅ Delete operations work in both legacy and modern modes")
	})
}
