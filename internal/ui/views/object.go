package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
		if v.state.list != nil {
			v.state.list, _ = v.state.list.Update(msg)
		}
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

// onObjectsLoaded renders the freshly fetched listing or its error.
func (v *ObjectView) onObjectsLoaded(msg ObjectsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		v.state.fail(v.deps, v.deps.T("Error Loading Objects"), v.deps.T("Failed to load S3 objects"), msg.Err)
		return v, nil
	}

	items := make([]components.ListItem, 0, len(msg.Objects))
	for _, obj := range msg.Objects {
		tag := "Object"
		description := fmt.Sprintf("%s %s | %s %s",
			v.deps.T("Size:"), components.FormatBytes(obj.Size),
			v.deps.T("Modified:"), formatModified(obj.LastModified))
		if strings.HasSuffix(obj.Key, "/") {
			tag = "Folder"
			description = v.deps.T("Folder marker")
		}
		items = append(items, components.ListItem{
			Title:       obj.Key,
			Description: description,
			Tag:         tag,
			Data:        obj,
		})
	}

	title := v.deps.T("Select Object")
	switch v.mode {
	case ModeDownload:
		title = v.deps.T("Select Object to Download")
	case ModeDelete:
		title = v.deps.T("Select Object to Delete")
	}
	v.state.loaded(title, items)
	return v, nil
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
		case "ctrl+c", "q":
			v.transfer.abort()
			return v, tea.Quit
		}
		return v, nil
	}

	if v.transfer.active {
		switch key {
		case "ctrl+c", "q":
			v.transfer.abort()
			return v, tea.Quit
		case "esc":
			v.transfer.cancelTransfer()
		}
		return v, nil
	}

	if v.state.loading {
		if key == "ctrl+c" || key == "q" {
			v.transfer.abort()
			return v, tea.Quit
		}
		return v, nil
	}

	if v.state.retryRequested(key) {
		return v, v.state.startLoading(v.deps.T("Retrying to load S3 objects..."), v.loadObjects())
	}

	var cmds []tea.Cmd
	switch key {
	case "ctrl+c", "q":
		v.transfer.abort()
		return v, tea.Quit
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

// View renders the object view.
func (v *ObjectView) View() string {
	if v.transfer.active && v.transfer.progress != nil {
		return v.transfer.progress.View()
	}

	if v.state.loading {
		return headerStyle.Render(v.deps.T("S3 Objects")) + "\n\n" + v.state.spinner.View()
	}

	if v.state.list == nil {
		return errorStyle.Render(v.deps.T("Failed to load S3 objects"))
	}

	context := contextStyle.Render(fmt.Sprintf("%s %s | %s %s",
		v.deps.T("Region:"), v.deps.region(), v.deps.T("Bucket:"), v.bucket))

	if v.confirm != nil {
		var question string
		if v.confirm.kind == confirmDelete {
			question = v.deps.T("Delete %s? [y/N]", v.confirm.object.Key)
		} else {
			question = v.deps.T("The file exists. Overwrite %s? [y/N]", v.confirm.localPath)
		}
		return context + "\n\n" + errorStyle.Render(question)
	}

	footer := footerStyle.Render(v.deps.T("↑↓: navigate • enter: select • r: refresh • p: preview • ?: help • s: settings • esc: back • q: quit"))

	var body string
	if v.showPreview && v.preview != nil {
		listWidth := v.width / 2
		if listWidth < 20 {
			listWidth = 20
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(listWidth).Render(v.state.list.View()),
			lipgloss.NewStyle().Width(v.width-listWidth).Render(v.preview.View()),
		)
	} else {
		body = v.state.list.View()
	}

	result := context + "\n\n" + body
	if v.state.errors.GetErrorCount() > 0 {
		result += "\n\n" + v.state.errors.View()
	}
	return result + "\n\n" + footer
}

func (v *ObjectView) currentObject() *s3.Object {
	item := v.state.currentItem()
	if item == nil || item.Tag != "Object" {
		return nil
	}
	obj, ok := item.Data.(s3.Object)
	if !ok {
		return nil
	}
	return &obj
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

// previewObject renders the object's metadata into the preview pane.
func (v *ObjectView) previewObject(obj s3.Object) tea.Cmd {
	return func() tea.Msg {
		modified := formatModified(obj.LastModified)
		content := fmt.Sprintf("%s\n\n%s %s\n%s %s\n%s %s\n%s %s",
			v.deps.T("S3 Object Information"),
			v.deps.T("Key:"), obj.Key,
			v.deps.T("Size:"), components.FormatBytes(obj.Size),
			v.deps.T("Modified:"), modified,
			v.deps.T("ETag:"), truncateShort(obj.ETag, 40),
		)
		return components.PreviewMsg{Content: content, PreviewType: components.PreviewTypeText}
	}
}

// progressMessage renders the text line shown under the progress bar.
func progressMessage(event s3.Progress) string {
	if event.Total > 0 {
		return fmt.Sprintf("%s / %s", components.FormatBytes(event.Transferred), components.FormatBytes(event.Total))
	}
	return components.FormatBytes(event.Transferred)
}

// formatModified renders a timestamp, substituting a dash for the zero time.
func formatModified(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}
