// Command s3ry is the entry point of the interactive S3 terminal client.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/seike460/s3ry/internal/cli"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}, cli.RunDeps{}))
}
