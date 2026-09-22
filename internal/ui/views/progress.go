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
// broker identifies the transfer so messages for a superseded or aborted
// transfer can be dropped instead of driving the wrong progress bar.
type transferProgressMsg struct {
	event  s3.Progress
	broker *progressBroker
}

// transferDoneMsg reports that a transfer goroutine returned.
type transferDoneMsg struct {
	err     error
	summary string
	broker  *progressBroker
}

// brokerClosedMsg is emitted when a view stops listening to a broker.
type brokerClosedMsg struct{}

// backToOperationMsg asks a view to return to the operation menu after a
// finished operation has been shown briefly. broker identifies the finished
// transfer so a delayed tick from an earlier run cannot force navigation in
// a view that has already moved on.
type backToOperationMsg struct{ broker *progressBroker }

// defaultDoneDelay keeps a completed transfer's outcome visible before the
// view returns to the operation menu.
const defaultDoneDelay = 2 * time.Second

// progressBroker adapts concurrent s3.ProgressFunc callbacks to the
// Bubble Tea message loop. Intermediate events may be dropped when the UI
// falls behind; the final Done event is always queued.
// brokerBufferSize bounds queued progress events; intermediate events are
// dropped when the UI falls behind, while the terminal Done event is always
// delivered.
const brokerBufferSize = 64

type progressBroker struct {
	ch   chan s3.Progress
	done chan struct{}
	once sync.Once
}

func newProgressBroker() *progressBroker {
	return &progressBroker{
		ch:   make(chan s3.Progress, brokerBufferSize),
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
			return transferProgressMsg{event: event, broker: b}
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
	// lastBroker is the most recently finished broker. The delayed
	// back-to-operation tick is accepted only while it still refers to this
	// broker, so stale ticks cannot hijack a view that started new work.
	lastBroker *progressBroker
	// doneDelay customizes how long the outcome stays visible before the
	// view returns to the operation menu. Zero uses two seconds.
	doneDelay time.Duration
	// localize formats status strings in the view's language; nil uses
	// English.
	localize func(string, ...any) string
}

// begin starts a transfer: it creates the cancelable context, the broker,
// and the progress widget. The returned context must be passed to the S3
// call, and the returned command starts progress delivery.
func (t *transferState) begin(d Deps, title string, total int64) (context.Context, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.broker = newProgressBroker()
	t.active = true
	t.localize = d.T
	t.progress = components.NewProgress(title, total)
	return ctx, t.broker.wait()
}

// onProgress forwards one broker event to the progress widget and returns
// the command that waits for the next event. Events tagged with a different
// broker belong to a superseded transfer and are dropped.
func (t *transferState) onProgress(msg transferProgressMsg, message string) tea.Cmd {
	if t.broker == nil || t.progress == nil || msg.broker != t.broker {
		return nil
	}
	t.progress, _ = t.progress.Update(components.ProgressMsg{
		Current: msg.event.Transferred,
		Total:   msg.event.Total,
		Message: message,
	})
	return t.broker.wait()
}

// finish ends the transfer, renders the outcome, and schedules the return
// to the operation view after a short pause. Completions tagged with a
// different broker belong to a superseded transfer and are dropped.
func (t *transferState) finish(msg transferDoneMsg) tea.Cmd {
	if msg.broker != t.broker {
		return nil
	}
	if t.broker != nil {
		t.broker.close()
		t.lastBroker = t.broker
		t.broker = nil
	}
	t.cancel = nil
	t.active = false

	localize := t.localize
	if localize == nil {
		localize = Deps{}.T
	}
	text := finishText(msg, localize)
	if t.progress == nil {
		t.progress = components.NewProgress("", 0)
	}
	t.progress, _ = t.progress.Update(components.CompletedMsg{Success: msg.err == nil, Message: text})
	delay := t.doneDelay
	if delay <= 0 {
		delay = defaultDoneDelay
	}
	finishedBroker := t.lastBroker
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return backToOperationMsg{broker: finishedBroker}
	})
}

// finishText renders the completed transfer's outcome line, preferring
// localized messages for known error kinds.
func finishText(msg transferDoneMsg, localize func(string, ...any) string) string {
	if msg.err == nil {
		return msg.summary
	}
	switch {
	case errors.Is(msg.err, s3.ErrCanceled):
		return localize("Canceled")
	case errors.Is(msg.err, s3.ErrTimeout):
		return localize("Timed out")
	default:
		return msg.err.Error()
	}
}

// backToOperation reports whether a delayed navigation tick is still valid:
// it must name the most recently finished broker while no transfer is active.
func (t *transferState) backToOperation(msg backToOperationMsg) bool {
	return msg.broker != nil && msg.broker == t.lastBroker && !t.active
}

// cancelTransfer aborts the running transfer, if any.
func (t *transferState) cancelTransfer() {
	if t.cancel != nil {
		t.cancel()
	}
}

// quitRequested aborts the transfer and reports whether key asks to quit
// the program.
func (t *transferState) quitRequested(key string) bool {
	if key != "ctrl+c" && key != "q" {
		return false
	}
	t.abort()
	return true
}

// activeKey handles the keys valid while a transfer runs and reports
// whether the program should quit.
func (t *transferState) activeKey(key string) (quit bool) {
	if t.quitRequested(key) {
		return true
	}
	if key == "esc" {
		t.cancelTransfer()
	}
	return false
}

// abort cancels the transfer and closes the broker so blocked waiters and
// Done callbacks are released. It is used when the owning view is being
// replaced or the program is quitting.
func (t *transferState) abort() {
	t.cancelTransfer()
	t.cancel = nil
	t.active = false
	if t.broker != nil {
		t.broker.close()
		t.broker = nil
	}
}

// resize forwards window size changes to the progress widget.
func (t *transferState) resize(msg tea.WindowSizeMsg) {
	if t.progress != nil {
		t.progress, _ = t.progress.Update(msg)
	}
}
