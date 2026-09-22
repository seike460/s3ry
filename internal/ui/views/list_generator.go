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

// generationDoneDelay leaves the generated list visible a little longer than
// a normal transfer outcome, since the path text is the result itself.
const generationDoneDelay = 3 * time.Second

// listProgressFlushInterval reports progress every N walked objects.
const listProgressFlushInterval = 100

// NewListGeneratorView creates a new list generator view.
func NewListGeneratorView(deps Deps, bucket string) *ListGeneratorView {
	return &ListGeneratorView{
		deps:     deps,
		bucket:   bucket,
		spinner:  components.NewSpinner(deps.T("Generating object list...")),
		transfer: transferState{active: true, doneDelay: generationDoneDelay},
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
		message := fmt.Sprintf(v.deps.T("%d objects written"), msg.event.Transferred)
		if cmd := v.transfer.onProgress(msg, message); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case transferDoneMsg:
		v.spinner.Stop()
		if cmd := v.transfer.finish(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case backToOperationMsg:
		if v.transfer.backToOperation(msg) {
			return NewOperationView(v.deps, v.bucket), nil
		}

	case brokerClosedMsg:
		// The broker was closed while a read was in flight; nothing to do.

	case components.SpinnerTickMsg:
		if v.transfer.active && v.spinner.IsActive() {
			v.spinner, _ = v.spinner.Update(msg)
			cmds = append(cmds, v.spinner.Start())
		}

	case tea.KeyMsg:
		if v.transfer.quitRequested(msg.String()) {
			return v, tea.Quit
		}
		switch msg.String() {
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
		v.deps.T("Region:"), v.deps.region(), v.deps.T("Bucket:"), v.bucket))

	var content string
	if v.transfer.progress != nil {
		content = v.transfer.progress.View()
	} else {
		content = headerStyle.Render(v.deps.T("Generating Object List")) + "\n\n" + v.spinner.View()
	}

	help := ""
	if !v.transfer.active {
		help = "\n\n" + footerStyle.Render(v.deps.T("esc: back • q: quit"))
	}
	return context + "\n\n" + content + help
}

// generateList walks the bucket with the session's concurrency and writes
// each object as it is discovered, so memory use stays flat.
func (v *ListGeneratorView) generateList() tea.Cmd {
	if err := v.deps.sessionErr(); err != nil {
		return func() tea.Msg { return transferDoneMsg{err: err} }
	}

	ctx, wait := v.transfer.begin(v.deps, v.deps.T("Generating object list"), -1)
	broker := v.transfer.broker

	session, bucket := v.deps.Session, v.bucket
	concurrency := session.Options().Concurrency

	return tea.Batch(
		func() tea.Msg {
			filename := fmt.Sprintf("ObjectList-%s.txt", time.Now().Format("2006-01-02-15-04-05"))
			file, err := os.Create(filename)
			if err != nil {
				return transferDoneMsg{err: err, broker: broker}
			}
			defer func() { _ = file.Close() }()

			writer := bufio.NewWriter(file)
			var count int64
			walkErr := session.Walk(ctx, bucket, "", s3.WalkOptions{Concurrency: concurrency}, func(object s3.Object) error {
				if _, err := fmt.Fprintf(writer, "./%s,%d\n", object.Key, object.Size); err != nil {
					return err
				}
				count++
				if count%listProgressFlushInterval == 0 {
					broker.callback(s3.Progress{Op: s3.OpDownload, Bucket: bucket, Transferred: count, Total: -1})
				}
				return nil
			})
			if walkErr == nil {
				walkErr = writer.Flush()
			}
			broker.callback(s3.Progress{Op: s3.OpDownload, Bucket: bucket, Transferred: count, Total: count, Done: true, Err: walkErr})
			return transferDoneMsg{
				err:     walkErr,
				summary: v.deps.T("Object list created: %s (%d objects)", filename, count),
				broker:  broker,
			}
		},
		wait,
	)
}

// AbortTransfer cancels the running list generation and releases the
// progress broker. The app calls it when replacing the view or quitting.
func (v *ListGeneratorView) AbortTransfer() {
	v.transfer.abort()
}
