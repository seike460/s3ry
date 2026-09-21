package cli

import (
	"errors"
	"io"

	"github.com/charmbracelet/x/term"
	"github.com/seike460/s3ry/internal/i18n"
)

const interactiveTerminalMessage = "s3ry needs an interactive terminal for the TUI. Run `s3ry --help` to see the non-interactive subcommands."

var isTerminal = func(fd uintptr) bool {
	return term.IsTerminal(fd)
}

func ensureInteractiveTerminal(in io.Reader, out io.Writer) error {
	if !isTerminal(fileDescriptor(in)) || !isTerminal(fileDescriptor(out)) {
		return errors.New(i18n.Sprintf(interactiveTerminalMessage))
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
