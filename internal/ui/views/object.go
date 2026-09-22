package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
)

// ObjectsLoadedMsg carries the result of an object listing.
type ObjectsLoadedMsg struct {
	Objects []s3.Object
	Err     error
}

// ObjectMode selects which operation the object view performs on selection.
type ObjectMode int

const (
	// ModeDownload downloads the selected object.
	ModeDownload ObjectMode = iota
	// ModeDelete deletes the selected object after confirmation.
	ModeDelete
)

// ObjectView is the object selection view used by downloads and deletes.
type ObjectView struct {
	deps        Deps
	bucket      string
	prefix      string
	mode        ObjectMode
	state       listState
	preview     *components.Preview
	showPreview bool
	previewKey  string // key whose HeadObject request is in flight or shown
	width       int
	confirm     *confirmPrompt
	transfer    transferState
}

// NewObjectView creates a new object view rooted at the bucket.
func NewObjectView(deps Deps, bucket string, mode ObjectMode) *ObjectView {
	return NewObjectViewAt(deps, bucket, "", mode)
}

// NewObjectViewAt creates an object view scoped to prefix.
func NewObjectViewAt(deps Deps, bucket, prefix string, mode ObjectMode) *ObjectView {
	var spinnerMessage string
	if mode == ModeDelete {
		spinnerMessage = deps.T("Loading S3 objects for delete...")
	} else {
		spinnerMessage = deps.T("Loading S3 objects for download...")
	}

	return &ObjectView{
		deps:    deps,
		bucket:  bucket,
		prefix:  prefix,
		mode:    mode,
		state:   newListState(spinnerMessage),
		preview: components.NewPreview(),
	}
}

// Init starts the spinner and the first object listing.
func (v *ObjectView) Init() tea.Cmd {
	return tea.Batch(v.state.spinner.Start(), v.loadObjects())
}

// Update handles messages for the object view.
func (v *ObjectView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg, transferProgressMsg, transferDoneMsg, components.SpinnerTickMsg, components.PreviewMsg:
		if cmd := v.onEvent(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case ObjectsLoadedMsg:
		return v.onObjectsLoaded(msg)

	case backToOperationMsg, brokerClosedMsg:
		// brokerClosedMsg means the broker was closed while a read was in
		// flight; nothing to do.
		if msg, ok := msg.(backToOperationMsg); ok && v.transfer.backToOperation(msg) {
			return NewOperationView(v.deps, v.bucket), nil
		}

	case tea.KeyMsg:
		return v.onKey(msg)
	}

	return v, tea.Batch(cmds...)
}

// onEvent routes resize, transfer, and passive events to their handlers.
func (v *ObjectView) onEvent(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.onResize(msg)
		return nil
	case transferProgressMsg, transferDoneMsg:
		return v.onTransfer(msg)
	default:
		return v.onPassive(msg)
	}
}

// onResize propagates the new terminal width to the list, transfer, and
// preview sub-components.
func (v *ObjectView) onResize(msg tea.WindowSizeMsg) {
	v.width = msg.Width
	v.state.resize(msg)
	v.transfer.resize(msg)
	if v.preview != nil {
		v.preview, _ = v.preview.Update(msg)
	}
}

// onTransfer feeds progress and completion events into the transfer state.
func (v *ObjectView) onTransfer(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case transferProgressMsg:
		return v.transfer.onProgress(msg, progressMessage(msg.event))
	case transferDoneMsg:
		return v.transfer.finish(msg)
	}
	return nil
}

// onPassive forwards spinner and preview messages to their components.
func (v *ObjectView) onPassive(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case components.SpinnerTickMsg:
		return v.state.onTick(msg)
	case components.PreviewMsg:
		if v.preview != nil {
			v.preview, _ = v.preview.Update(msg)
		}
	}
	return nil
}

// onKey handles keyboard input across the ready, confirm, and processing
// states.
func (v *ObjectView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if v.confirm != nil {
		return v.onConfirmKey(key)
	}

	if v.transfer.active {
		if v.transfer.activeKey(key) {
			return v, tea.Quit
		}
		return v, nil
	}

	if v.state.loading {
		if v.transfer.quitRequested(key) {
			return v, tea.Quit
		}
		return v, nil
	}

	if v.state.retryRequested(key) {
		return v, v.state.startLoading(v.deps.T("Retrying to load S3 objects..."), v.loadObjects())
	}

	return v.onReadyKey(msg)
}

// onConfirmKey handles input while a confirmation prompt is open.
func (v *ObjectView) onConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		pending := *v.confirm
		v.confirm = nil
		if pending.kind == confirmDelete {
			return v.startDelete(pending.object)
		}
		return v.startDownload(pending.object, s3.OverwriteAlways)
	case "n", "N", "esc":
		v.confirm = nil
		return v, nil
	}
	if v.transfer.quitRequested(key) {
		return v, tea.Quit
	}
	return v, nil
}

// onReadyKey handles input on the object list.
func (v *ObjectView) onReadyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if v.state.filterActive() {
		v.state.routeFilterKey(msg)
		return v, nil
	}
	key := msg.String()
	if v.transfer.quitRequested(key) {
		return v, tea.Quit
	}
	if next := v.navView(key); next != nil {
		return next, nil
	}

	var cmds []tea.Cmd
	switch key {
	case "p":
		v.showPreview = !v.showPreview
		v.refreshPreview(&cmds)
	case "enter", " ":
		if obj := v.selectedObject(); obj != nil {
			return v.selectObject(*obj)
		}
	}

	if v.state.list != nil {
		v.state.list, _ = v.state.list.Update(msg)
		v.refreshPreview(&cmds)
	}
	return v, tea.Batch(cmds...)
}

// navView returns the destination view for navigation keys, or nil when the
// key is not a navigation key.
func (v *ObjectView) navView(key string) tea.Model {
	switch key {
	case "esc":
		return NewOperationView(v.deps, v.bucket)
	case "?":
		return NewHelpView(v.deps)
	case "s":
		return NewSettingsView(v.deps)
	}
	return nil
}

// selectedObject returns the s3.Object under the cursor, if any.
func (v *ObjectView) selectedObject() *s3.Object {
	item := v.state.currentItem()
	if item == nil {
		return nil
	}
	obj, ok := item.Data.(s3.Object)
	if !ok {
		return nil
	}
	return &obj
}

// refreshPreview reloads the preview pane for the object under the cursor.
// HeadObject runs once per distinct key; repeat calls for the same object
// are skipped.
func (v *ObjectView) refreshPreview(cmds *[]tea.Cmd) {
	if !v.showPreview {
		return
	}
	obj := v.currentObject()
	if obj == nil || obj.Key == v.previewKey {
		return
	}
	v.previewKey = obj.Key
	*cmds = append(*cmds, v.previewObject(*obj))
}

// AbortTransfer cancels any in-flight transfer and releases the progress
// broker. The app calls it when replacing the view or quitting.
func (v *ObjectView) AbortTransfer() {
	v.transfer.abort()
}

// loadObjects walks the bucket sequentially, preserving lexical order.
func (v *ObjectView) loadObjects() tea.Cmd {
	return func() tea.Msg {
		if err := v.deps.sessionErr(); err != nil {
			return ObjectsLoadedMsg{Err: err}
		}
		ctx, cancel := v.deps.listContext(context.Background())
		defer cancel()

		var objects []s3.Object
		err := v.deps.Session.Walk(ctx, v.bucket, v.prefix, s3.WalkOptions{Concurrency: 1}, func(object s3.Object) error {
			objects = append(objects, object)
			return nil
		})
		return ObjectsLoadedMsg{Objects: objects, Err: err}
	}
}
