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
	mode        ObjectMode
	state       listState
	preview     *components.Preview
	showPreview bool
	width       int
	confirm     *confirmPrompt
	transfer    transferState
}

// NewObjectView creates a new object view.
func NewObjectView(deps Deps, bucket string, mode ObjectMode) *ObjectView {
	var spinnerMessage string
	if mode == ModeDelete {
		spinnerMessage = deps.T("Loading S3 objects for delete...")
	} else {
		spinnerMessage = deps.T("Loading S3 objects for download...")
	}

	return &ObjectView{
		deps:    deps,
		bucket:  bucket,
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
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.state.resize(msg)
		v.transfer.resize(msg)
		if v.preview != nil {
			v.preview, _ = v.preview.Update(msg)
		}

	case ObjectsLoadedMsg:
		return v.onObjectsLoaded(msg)

	case transferProgressMsg:
		if cmd := v.transfer.onProgress(msg, progressMessage(msg.event)); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case transferDoneMsg:
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
		cmds = append(cmds, v.state.onTick(msg))

	case components.PreviewMsg:
		if v.preview != nil {
			v.preview, _ = v.preview.Update(msg)
		}

	case tea.KeyMsg:
		return v.onKey(msg)
	}

	return v, tea.Batch(cmds...)
}

// onKey handles keyboard input across the ready, confirm, and processing
// states.
func (v *ObjectView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if v.confirm != nil {
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

	var cmds []tea.Cmd
	if v.transfer.quitRequested(key) {
		return v, tea.Quit
	}
	switch key {
	case "esc":
		return NewOperationView(v.deps, v.bucket), nil
	case "?":
		return NewHelpView(v.deps), nil
	case "s":
		return NewSettingsView(v.deps), nil
	case "p":
		v.showPreview = !v.showPreview
		if v.showPreview {
			if obj := v.currentObject(); obj != nil {
				cmds = append(cmds, v.previewObject(*obj))
			}
		}
	case "enter", " ":
		item := v.state.currentItem()
		if item == nil {
			break
		}
		obj, ok := item.Data.(s3.Object)
		if !ok {
			break
		}
		return v.selectObject(obj)
	}

	if v.state.list != nil {
		v.state.list, _ = v.state.list.Update(msg)
		if v.showPreview {
			if obj := v.currentObject(); obj != nil {
				cmds = append(cmds, v.previewObject(*obj))
			}
		}
	}
	return v, tea.Batch(cmds...)
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
		err := v.deps.Session.Walk(ctx, v.bucket, "", s3.WalkOptions{Concurrency: 1}, func(object s3.Object) error {
			objects = append(objects, object)
			return nil
		})
		return ObjectsLoadedMsg{Objects: objects, Err: err}
	}
}
