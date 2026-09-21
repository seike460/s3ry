// Package cli implements the s3ry command line: flag parsing, configuration
// loading, and startup of the interactive TUI.
package cli

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/ui/app"
	"github.com/spf13/cobra"
)

type rootFlags struct {
	Region     string
	Profile    string
	ConfigFile string
	Verbose    bool
	Language   string
	LogLevel   string
}

// runTUI is replaceable so command behavior can be tested without starting Bubble Tea.
var runTUI = func(ctx context.Context, cfg *config.Config) error {
	return app.Run(ctx, cfg)
}

// NewRootCommand builds a fresh command tree for one invocation.
func NewRootCommand(info BuildInfo) *cobra.Command {
	info = info.normalized()
	flags := &rootFlags{}

	root := &cobra.Command{
		Use:           "s3ry",
		Short:         "interactive terminal client for Amazon S3",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       info.Version,
		Annotations: map[string]string{
			"commit": info.Commit,
			"date":   info.Date,
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRoot(cmd, flags)
		},
	}

	root.SetVersionTemplate(versionTemplate)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})

	root.Flags().StringVar(&flags.Region, "region", "", "AWS region to use")
	root.Flags().StringVar(&flags.Profile, "profile", "", "AWS profile to use")
	root.Flags().StringVar(&flags.ConfigFile, "config", "", "Path to config file")
	root.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose logging")
	root.Flags().StringVar(&flags.LogLevel, "log-level", "", "Log level (debug, info, warn, error)")
	root.PersistentFlags().StringVar(&flags.Language, "lang", "", "Language (en, ja)")

	root.AddCommand(newVersionCommand(info))
	return root
}

// Run executes the CLI and returns its process exit code.
func Run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, info BuildInfo) int {
	if ctx == nil {
		ctx = context.Background()
	}

	cmd := NewRootCommand(info)
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

func runRoot(cmd *cobra.Command, flags *rootFlags) error {
	if err := cmd.Context().Err(); err != nil {
		return err
	}

	cfg, err := loadConfig(flags)
	if err != nil {
		return err
	}

	cfg.InitializeI18n()
	if flags.Verbose {
		cfg.Logging.Level = "debug"
	}
	setupLogging(cfg)

	if err := cmd.Context().Err(); err != nil {
		return err
	}
	if err := ensureInteractiveTerminal(cmd.InOrStdin(), cmd.OutOrStdout()); err != nil {
		return err
	}
	return runTUI(cmd.Context(), cfg)
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

	return cfg, nil
}

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
		file, err := os.OpenFile(cfg.Logging.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			log.Printf("Warning: Could not open log file %s: %v", cfg.Logging.File, err)
		} else {
			log.SetOutput(file)
		}
	}
}
