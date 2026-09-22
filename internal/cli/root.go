// Package cli implements the s3ry command line: flag parsing, configuration
// loading, and startup of the interactive TUI.
package cli

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/i18n"
	"github.com/seike460/s3ry/internal/s3"
	"github.com/spf13/cobra"
)

type rootFlags struct {
	Region        string
	Profile       string
	ConfigFile    string
	Verbose       bool
	Language      string
	LogLevel      string
	Endpoint      string
	PathStyle     bool
	NoSignRequest bool
}

// RunDeps carries the replaceable runtime dependencies of a command run.
// Nil fields fall back to the production implementations.
type RunDeps struct {
	// RunTUI starts the interactive client; nil uses app.Run.
	RunTUI func(ctx context.Context, cfg *config.Config) error
	// IsTerminal reports whether a file descriptor is a terminal; nil uses
	// term.IsTerminal.
	IsTerminal func(fd uintptr) bool
}

// NewRootCommand builds a fresh command tree for one invocation.
func NewRootCommand(info BuildInfo, deps RunDeps) *cobra.Command {
	info = info.normalized()
	flags := &rootFlags{}

	root := &cobra.Command{
		Use:   "s3ry [s3://bucket/prefix]",
		Short: "interactive terminal client for Amazon S3",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most 1 arg(s), received %d", len(args))
			}
			if len(args) == 1 && !strings.HasPrefix(args[0], "s3://") {
				return fmt.Errorf("unknown command %q for \"s3ry\"", args[0])
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       info.Version,
		Annotations: map[string]string{
			"commit": info.Commit,
			"date":   info.Date,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(cmd, args, flags, deps)
		},
	}

	root.SetVersionTemplate(versionTemplate)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})

	root.Flags().StringVar(&flags.Region, "region", "", "AWS region to use")
	root.Flags().StringVar(&flags.Profile, "profile", "", "AWS profile to use")
	root.Flags().StringVar(&flags.ConfigFile, "config", "", "Path to config file")
	root.Flags().StringVar(&flags.Endpoint, "endpoint", "", "S3 endpoint URL for S3-compatible services")
	root.Flags().BoolVar(&flags.PathStyle, "path-style", false, "Force path-style S3 addressing (MinIO, LocalStack)")
	root.Flags().BoolVar(&flags.NoSignRequest, "no-sign-request", false, "Send unsigned requests for public endpoints")
	root.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose logging")
	root.Flags().StringVar(&flags.LogLevel, "log-level", "", "Log level (debug, info, warn, error)")
	root.PersistentFlags().StringVar(&flags.Language, "lang", "", "Language (en, ja)")

	root.AddCommand(newVersionCommand(info))
	return root
}

// Run executes the CLI and returns its process exit code.
func Run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, info BuildInfo, deps RunDeps) int {
	if ctx == nil {
		ctx = context.Background()
	}

	cmd := NewRootCommand(info, deps)
	if args == nil {
		args = []string{}
	}
	cmd.SetArgs(args)
	cmd.SetIn(in)
	cmd.SetOut(out)
	cmd.SetErr(errOut)

	err := cmd.ExecuteContext(ctx)
	err = normalizeUsageError(err)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "s3ry: %v\n", err)
	}
	return ExitCode(err)
}

func runRoot(cmd *cobra.Command, args []string, flags *rootFlags, deps RunDeps) error {
	if err := cmd.Context().Err(); err != nil {
		return err
	}

	cfg, err := loadConfig(flags)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		if _, err := s3.ParseURL(args[0]); err != nil {
			return &usageError{err: fmt.Errorf("invalid S3 URL %q: %w", args[0], err)}
		}
		cfg.StartURL = args[0]
	}

	printer := i18n.NewPrinter(cfg.UI.Language)
	if flags.Verbose {
		cfg.Logging.Level = "debug"
	}
	setupLogging(cfg)

	if err := cmd.Context().Err(); err != nil {
		return err
	}
	if err := ensureInteractiveTerminal(cmd.InOrStdin(), cmd.OutOrStdout(), printer, deps.terminalProbe()); err != nil {
		return err
	}
	return deps.tuiRunner()(cmd.Context(), cfg)
}

func loadConfig(flags *rootFlags) (*config.Config, error) {
	var (
		cfg *config.Config
		err error
	)

	if flags.ConfigFile != "" {
		cfg, err = config.LoadFromFile(flags.ConfigFile)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		return nil, err
	}

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
	if flags.Endpoint != "" {
		cfg.AWS.Endpoint = flags.Endpoint
	}
	if flags.PathStyle {
		cfg.AWS.PathStyle = true
	}
	if flags.NoSignRequest {
		cfg.AWS.NoSignRequest = true
	}

	return cfg, nil
}

func setupLogging(cfg *config.Config) {
	flags := log.LstdFlags
	if cfg.Logging.Level == "debug" {
		flags |= log.Lshortfile
	}
	log.SetFlags(flags)

	if cfg.Logging.File != "" {
		file, err := os.OpenFile(cfg.Logging.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			log.Printf("Warning: Could not open log file %s: %v", cfg.Logging.File, err)
		} else {
			log.SetOutput(file)
		}
	}
}
