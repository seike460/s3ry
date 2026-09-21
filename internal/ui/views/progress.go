package views

import (
	"context"
	"errors"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
)

// transferProgressMsg delivers one progress event to the update loop.
type transferProgressMsg struct{ event s3.Progress }

// transferDoneMsg reports that a transfer goroutine returned.
type transferDoneMsg struct {
	err     error
	summary string
}

// brokerClosedMsg is emitted when a view stops listening to a broker.
type brokerClosedMsg struct{}

// backToOperationMsg asks a view to return to the operation menu after a
// finished operation has been shown briefly.
type backToOperationMsg struct{}

// progressBroker adapts concurrent s3.ProgressFunc callbacks to the
// Bubble Tea message loop. Intermediate events may be dropped when the UI
// falls behind; the final Done event is always queued.
type progressBroker struct {
	ch   chan s3.Progress
	done chan struct{}
	once sync.Once
}

func newProgressBroker() *progressBroker {
	return &progressBroker{
		ch:   make(chan s3.Progress, 64),
		done: make(chan struct{}),
	}
}

// callback implements s3.ProgressFunc and is safe to call from any goroutine.
func (b *progressBroker) callback(event s3.Progress) {
	if event.Done {
		select {
		case b.ch <- event:
		case <-b.done:
		}
		return
	}
	select {
	case b.ch <- event:
	case <-b.done:
	default:
	}
}

// wait returns a command resolving with the next progress event, or
// brokerClosedMsg once the broker is closed.
func (b *progressBroker) wait() tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-b.ch:
			return transferProgressMsg{event: event}
		case <-b.done:
			return brokerClosedMsg{}
		}
	}
}

// close releases any blocked callback or waiter.
func (b *progressBroker) close() {
	b.once.Do(func() { close(b.done) })
}

// transferState tracks the lifecycle of one running S3 transfer inside a
// view: the cancelable context, the progress broker feeding the UI, and the
// progress widget itself. Views embed it so every transfer behaves the same.
type transferState struct {
	broker   *progressBroker
	cancel   context.CancelFunc
	progress *components.Progress
	active   bool
	// doneDelay customizes how long the outcome stays visible before the
	// view returns to the operation menu. Zero uses two seconds.
	doneDelay time.Duration
}

// begin starts a transfer: it creates the cancelable context, the broker,
// and the progress widget. The returned context must be passed to the S3
// call, and the returned command starts progress delivery.
func (t *transferState) begin(title string, total int64) (context.Context, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.broker = newProgressBroker()
	t.active = true
	t.progress = components.NewProgress(title, total)
	return ctx, t.broker.wait()
}

// callback returns the progress function to pass to s3 transfer options.
func (t *transferState) callback() s3.ProgressFunc {
	return t.broker.callback
}

// onProgress forwards one broker event to the progress widget and returns
// the command that waits for the next event.
func (t *transferState) onProgress(event s3.Progress, message string) tea.Cmd {
	if t.broker == nil || t.progress == nil {
		return nil
	}
	t.progress, _ = t.progress.Update(components.ProgressMsg{
		Current: event.Transferred,
		Total:   event.Total,
		Message: message,
	})
	return t.broker.wait()
}

// finish ends the transfer, renders the outcome, and schedules the return
// to the operation view after a short pause.
func (t *transferState) finish(msg transferDoneMsg) tea.Cmd {
	if t.broker != nil {
		t.broker.close()
		t.broker = nil
	}
	t.cancel = nil
	t.active = false

	text := msg.summary
	success := msg.err == nil
	if msg.err != nil {
		text = msg.err.Error()
		if errors.Is(msg.err, s3.ErrCanceled) {
			text = T("Canceled")
		}
	}
	if t.progress == nil {
		t.progress = components.NewProgress("", 0)
	}
	t.progress, _ = t.progress.Update(components.CompletedMsg{Success: success, Message: text})
	delay := t.doneDelay
	if delay <= 0 {
		delay = 2 * time.Second
	}
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return backToOperationMsg{}
	})
}

// cancelTransfer aborts the running transfer, if any.
func (t *transferState) cancelTransfer() {
	if t.cancel != nil {
		t.cancel()
	}
}

// resize forwards window size changes to the progress widget.
func (t *transferState) resize(msg tea.WindowSizeMsg) {
	if t.progress != nil {
		t.progress, _ = t.progress.Update(msg)
	}
}
