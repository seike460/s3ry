package cli

import (
	"context"
	"errors"
	"io"

	"github.com/charmbracelet/x/term"
	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/i18n"
	"github.com/seike460/s3ry/internal/ui/app"
)

const interactiveTerminalMessage = "s3ry needs an interactive terminal for the TUI. Run `s3ry --help` to see the non-interactive subcommands."

func (d RunDeps) terminalProbe() func(uintptr) bool {
	if d.IsTerminal != nil {
		return d.IsTerminal
	}
	return term.IsTerminal
}

func (d RunDeps) tuiRunner() func(context.Context, *config.Config) error {
	if d.RunTUI != nil {
		return d.RunTUI
	}
	return app.Run
}

func ensureInteractiveTerminal(in io.Reader, out io.Writer, messages *i18n.Printer, isTTY func(uintptr) bool) error {
	if !isTTY(fileDescriptor(in)) || !isTTY(fileDescriptor(out)) {
		return errors.New(messages.Sprintf(interactiveTerminalMessage))
	}
	return nil
}

func fileDescriptor(value any) uintptr {
	fdProvider, ok := value.(interface{ Fd() uintptr })
	if !ok {
		return 0
	}
	return fdProvider.Fd()
}
