package views

import (
	"bufio"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
)

// ListGeneratorView streams the bucket's object list into a timestamped
// local file.
type ListGeneratorView struct {
	deps     Deps
	bucket   string
	spinner  *components.Spinner
	transfer transferState
}

// NewListGeneratorView creates a new list generator view.
func NewListGeneratorView(deps Deps, bucket string) *ListGeneratorView {
	return &ListGeneratorView{
		deps:     deps,
		bucket:   bucket,
		spinner:  components.NewSpinner(T("Generating object list...")),
		transfer: transferState{active: true, doneDelay: 3 * time.Second},
	}
}

// Init starts the spinner and the generation.
func (v *ListGeneratorView) Init() tea.Cmd {
	return tea.Batch(v.spinner.Start(), v.generateList())
}

// Update handles messages for the list generator view.
func (v *ListGeneratorView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.transfer.resize(msg)

	case transferProgressMsg:
		message := fmt.Sprintf(T("%d objects written"), msg.event.Transferred)
		if cmd := v.transfer.onProgress(msg.event, message); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case transferDoneMsg:
		v.spinner.Stop()
		if cmd := v.transfer.finish(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case backToOperationMsg:
		return NewOperationView(v.deps, v.bucket), nil

	case brokerClosedMsg:
		// The broker was closed while a read was in flight; nothing to do.

	case components.SpinnerTickMsg:
		if v.transfer.active && v.spinner.IsActive() {
			v.spinner, _ = v.spinner.Update(msg)
			cmds = append(cmds, v.spinner.Start())
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			if v.transfer.active {
				v.transfer.cancelTransfer()
				return v, nil
			}
			return NewOperationView(v.deps, v.bucket), nil
		}
	}

	return v, tea.Batch(cmds...)
}

// View renders the list generator view.
func (v *ListGeneratorView) View() string {
	context := contextStyle.Render(fmt.Sprintf("%s %s | %s %s",
		T("Region:"), v.deps.region(), T("Bucket:"), v.bucket))

	var content string
	if v.transfer.progress != nil {
		content = v.transfer.progress.View()
	} else {
		content = headerStyle.Render(T("Generating Object List")) + "\n\n" + v.spinner.View()
	}

	help := ""
	if !v.transfer.active {
		help = "\n\n" + footerStyle.Render(T("esc: back • q: quit"))
	}
	return context + "\n\n" + content + help
}

// generateList walks the bucket with the session's concurrency and writes
// each object as it is discovered, so memory use stays flat.
func (v *ListGeneratorView) generateList() tea.Cmd {
	if err := v.deps.sessionErr(); err != nil {
		return func() tea.Msg { return transferDoneMsg{err: err} }
	}

	ctx, wait := v.transfer.begin(T("Generating object list"), -1)
	progressFn := v.transfer.callback()

	session, bucket := v.deps.Session, v.bucket
	concurrency := session.Options().Concurrency

	return tea.Batch(
		func() tea.Msg {
			filename := fmt.Sprintf("ObjectList-%s.txt", time.Now().Format("2006-01-02-15-04-05"))
			file, err := os.Create(filename)
			if err != nil {
				return transferDoneMsg{err: err}
			}
			defer func() { _ = file.Close() }()

			writer := bufio.NewWriter(file)
			var count int64
			walkErr := session.Walk(ctx, bucket, "", s3.WalkOptions{Concurrency: concurrency}, func(object s3.Object) error {
				if _, err := fmt.Fprintf(writer, "./%s,%d\n", object.Key, object.Size); err != nil {
					return err
				}
				count++
				if count%100 == 0 {
					progressFn(s3.Progress{Op: s3.OpDownload, Bucket: bucket, Transferred: count, Total: -1})
				}
				return nil
			})
			if walkErr == nil {
				walkErr = writer.Flush()
			}
			progressFn(s3.Progress{Op: s3.OpDownload, Bucket: bucket, Transferred: count, Total: count, Done: true, Err: walkErr})
			return transferDoneMsg{
				err:     walkErr,
				summary: T("Object list created: %s (%d objects)", filename, count),
			}
		},
		wait,
	)
}
