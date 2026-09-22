// Package cli implements the s3ry command line: flag parsing, configuration
// loading, the interactive TUI entrypoint, and the non-interactive
// subcommands (ls, cat, rm, presign). Subcommands share the root flag set
// through persistent flags and print to stdout so they compose with pipes.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/s3"
)

// newSession builds an AWS session from the resolved configuration.
func newSession(ctx context.Context, cfg *config.Config) (*s3.Session, error) {
	return s3.NewSession(ctx, s3.Options{
		Profile:       cfg.AWS.Profile,
		Region:        cfg.AWS.Region,
		EndpointURL:   cfg.AWS.Endpoint,
		PathStyle:     cfg.AWS.PathStyle,
		NoSignRequest: cfg.AWS.NoSignRequest,
		Concurrency:   cfg.Performance.Concurrency,
		PartSize:      cfg.Performance.PartSize,
	})
}

// commandContext bounds a subcommand's requests with the configured timeout.
func commandContext(cmd *cobra.Command, cfg *config.Config) (context.Context, context.CancelFunc) {
	if cfg.Performance.Timeout > 0 {
		return context.WithTimeout(cmd.Context(), time.Duration(cfg.Performance.Timeout)*time.Second)
	}
	return context.WithCancel(cmd.Context())
}

// openSession resolves the configuration, starts a bounded context, and
// opens the AWS session shared by every subcommand.
func openSession(cmd *cobra.Command, flags *rootFlags) (*s3.Session, context.Context, context.CancelFunc, error) {
	cfg, err := loadConfig(flags)
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := commandContext(cmd, cfg)
	session, err := newSession(ctx, cfg)
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}
	return session, ctx, cancel, nil
}

// parseObjectURL validates an s3:// URL that must name an object key.
func parseObjectURL(raw string) (s3.URL, error) {
	u, err := s3.ParseURL(raw)
	if err != nil {
		return u, &usageError{err: fmt.Errorf("invalid S3 URL %q: %w", raw, err)}
	}
	if u.IsPrefix() {
		return u, &usageError{err: fmt.Errorf("%q does not name an object", raw)}
	}
	return u, nil
}

// objectJSON is the JSON representation of one listed object.
type objectJSON struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"last_modified"`
	ETag         string `json:"etag,omitempty"`
	StorageClass string `json:"storage_class,omitempty"`
}

// bucketJSON is the JSON representation of one bucket.
type bucketJSON struct {
	Name         string `json:"name"`
	Region       string `json:"region,omitempty"`
	CreationDate string `json:"creation_date,omitempty"`
}

// newLsCommand lists buckets (no argument) or objects under a URL.
func newLsCommand(flags *rootFlags) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "ls [s3://bucket[/prefix]]",
		Short: "List buckets or objects without the TUI",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if output != "plain" && output != "json" {
				return &usageError{err: fmt.Errorf("unsupported output format %q (expected plain or json)", output)}
			}
			if len(args) == 0 {
				session, ctx, cancel, err := openSession(cmd, flags)
				if err != nil {
					return err
				}
				defer cancel()
				return printBuckets(ctx, cmd, session, output)
			}
			u, err := s3.ParseURL(args[0])
			if err != nil {
				return &usageError{err: fmt.Errorf("invalid S3 URL %q: %w", args[0], err)}
			}
			session, ctx, cancel, err := openSession(cmd, flags)
			if err != nil {
				return err
			}
			defer cancel()
			return printObjects(ctx, cmd, session, u, output)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "plain", "Output format (plain, json)")
	return cmd
}

// printBuckets writes the bucket list in the requested format.
func printBuckets(ctx context.Context, cmd *cobra.Command, session *s3.Session, output string) error {
	buckets, err := session.ListBuckets(ctx)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if output == "json" {
		rows := make([]bucketJSON, 0, len(buckets))
		for _, b := range buckets {
			rows = append(rows, bucketJSON{
				Name:         b.Name,
				Region:       b.Region,
				CreationDate: b.CreationDate.UTC().Format(time.RFC3339),
			})
		}
		return json.NewEncoder(out).Encode(rows)
	}
	for _, b := range buckets {
		_, _ = fmt.Fprintln(out, b.Name)
	}
	return nil
}

// printObjects walks the URL prefix and writes each object key. Plain
// output streams line by line — Concurrency 1 keeps Walk in listing order —
// while JSON must buffer the full result to encode the array.
func printObjects(ctx context.Context, cmd *cobra.Command, session *s3.Session, u s3.URL, output string) error {
	out := cmd.OutOrStdout()
	if output == "plain" {
		return session.Walk(ctx, u.Bucket, u.Key, s3.WalkOptions{Concurrency: 1}, func(o s3.Object) error {
			_, _ = fmt.Fprintln(out, o.Key)
			return nil
		})
	}
	var objects []s3.Object
	err := session.Walk(ctx, u.Bucket, u.Key, s3.WalkOptions{Concurrency: 1}, func(o s3.Object) error {
		objects = append(objects, o)
		return nil
	})
	if err != nil {
		return err
	}
	rows := make([]objectJSON, 0, len(objects))
	for _, o := range objects {
		rows = append(rows, objectJSON{
			Key:          o.Key,
			Size:         o.Size,
			LastModified: o.LastModified.UTC().Format(time.RFC3339),
			ETag:         o.ETag,
			StorageClass: o.StorageClass,
		})
	}
	return json.NewEncoder(out).Encode(rows)
}

// newCatCommand streams one object's body to stdout.
func newCatCommand(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "cat s3://bucket/key",
		Short: "Print an object's contents to stdout",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := parseObjectURL(args[0])
			if err != nil {
				return err
			}
			session, ctx, cancel, err := openSession(cmd, flags)
			if err != nil {
				return err
			}
			defer cancel()
			body, err := session.Open(ctx, u.Bucket, u.Key, "")
			if err != nil {
				return err
			}
			defer func() { _ = body.Close() }()
			_, err = io.Copy(cmd.OutOrStdout(), body)
			return err
		},
	}
}

// newRmCommand deletes one object or, with a trailing slash, a whole prefix.
func newRmCommand(flags *rootFlags) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "rm s3://bucket/key-or-prefix/",
		Short: "Delete an object or a prefix without the TUI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := s3.ParseURL(args[0])
			if err != nil {
				return &usageError{err: fmt.Errorf("invalid S3 URL %q: %w", args[0], err)}
			}
			if u.Key == "" {
				return &usageError{err: fmt.Errorf("rm requires an object or prefix URL, got %q", args[0])}
			}
			session, ctx, cancel, err := openSession(cmd, flags)
			if err != nil {
				return err
			}
			defer cancel()
			return removeTarget(ctx, cmd, session, u, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report what would be deleted without deleting")
	return cmd
}

// removeTarget dispatches to DeleteKeys or DeletePrefix and prints a
// one-line summary. Partial failures surface as a BulkError.
func removeTarget(ctx context.Context, cmd *cobra.Command, session *s3.Session, u s3.URL, dryRun bool) error {
	var result s3.DeleteResult
	var err error
	if strings.HasSuffix(u.Key, "/") {
		result, err = session.DeletePrefix(ctx, u.Bucket, u.Key, s3.DeleteOptions{DryRun: dryRun})
	} else {
		result, err = session.DeleteKeys(ctx, u.Bucket, []string{u.Key}, s3.DeleteOptions{DryRun: dryRun})
	}
	if err != nil {
		return err
	}
	return reportDeleteResult(cmd, result, dryRun)
}

// reportDeleteResult writes the delete summary and maps per-key failures to
// a BulkError for the exit-code machinery.
func reportDeleteResult(cmd *cobra.Command, result s3.DeleteResult, dryRun bool) error {
	verb := "Deleted"
	if dryRun {
		verb = "Would delete"
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %d object(s)\n", verb, len(result.Deleted))
	if len(result.Failed) > 0 {
		return &s3.BulkError{Errors: result.Failed}
	}
	return nil
}

// newPresignCommand prints a presigned GET URL for one object.
func newPresignCommand(flags *rootFlags) *cobra.Command {
	var expires time.Duration
	cmd := &cobra.Command{
		Use:   "presign s3://bucket/key",
		Short: "Print a presigned GET URL for an object",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := parseObjectURL(args[0])
			if err != nil {
				return err
			}
			session, ctx, cancel, err := openSession(cmd, flags)
			if err != nil {
				return err
			}
			defer cancel()
			url, err := session.PresignGet(ctx, u.Bucket, u.Key, expires)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), url)
			return nil
		},
	}
	cmd.Flags().DurationVar(&expires, "expires", time.Hour, "URL expiry (1s to 7 days)")
	return cmd
}
