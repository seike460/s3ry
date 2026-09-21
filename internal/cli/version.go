package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const versionTemplate = "s3ry version {{.Version}}\nBuild commit: {{index .Annotations \"commit\"}}\nBuild date: {{index .Annotations \"date\"}}\n"

// BuildInfo contains the values embedded into a s3ry binary at build time.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func (info BuildInfo) normalized() BuildInfo {
	if info.Version == "" {
		info.Version = "dev"
	}
	if info.Commit == "" {
		info.Commit = "unknown"
	}
	if info.Date == "" {
		info.Date = "unknown"
	}
	return info
}

func (info BuildInfo) output() string {
	info = info.normalized()
	return fmt.Sprintf("s3ry version %s\nBuild commit: %s\nBuild date: %s\n", info.Version, info.Commit, info.Date)
}

func newVersionCommand(info BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), info.output())
			return err
		},
	}
}
