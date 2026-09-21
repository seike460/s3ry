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
	list        *components.List
	spinner     *components.Spinner
	preview     *components.Preview
	errors      *components.ErrorDisplay
	loading     bool
	showPreview bool
	width       int
	confirm     *confirmPrompt
	transfer    transferState
}

// NewObjectView creates a new object view.
func NewObjectView(deps Deps, bucket string, mode ObjectMode) *ObjectView {
	var spinnerMessage string
	if mode == ModeDelete {
		spinnerMessage = T("Loading S3 objects for delete...")
	} else {
		spinnerMessage = T("Loading S3 objects for download...")
	}

	return &ObjectView{
		deps:    deps,
		bucket:  bucket,
		mode:    mode,
		loading: true,
		spinner: components.NewSpinner(spinnerMessage),
		preview: components.NewPreview(),
		errors:  components.NewErrorDisplay(),
	}
}

// Init starts the spinner and the first object listing.
func (v *ObjectView) Init() tea.Cmd {
	return tea.Batch(v.spinner.Start(), v.loadObjects())
}

// Update handles messages for the object view.
func (v *ObjectView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}
		v.transfer.resize(msg)
		if v.preview != nil {
			v.preview, _ = v.preview.Update(msg)
		}

	case ObjectsLoadedMsg:
		return v.onObjectsLoaded(msg)

	case transferProgressMsg:
		if cmd := v.transfer.onProgress(msg.event, progressMessage(msg.event)); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case transferDoneMsg:
		if cmd := v.transfer.finish(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case backToOperationMsg:
		return NewOperationView(v.deps, v.bucket), nil

	case brokerClosedMsg:
		// The broker was closed while a read was in flight; nothing to do.

	case components.SpinnerTickMsg:
		if v.loading {
			v.spinner, _ = v.spinner.Update(msg)
			cmds = append(cmds, v.spinner.Start())
		}

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
	v.loading = false
	v.spinner.Stop()

	if msg.Err != nil {
		v.errors.AddAWSError(msg.Err)
		v.list = components.NewList(T("Error Loading Objects"), []components.ListItem{{
			Title:       T("Failed to load S3 objects"),
			Description: T("Press 'r' to retry, 'esc' to go back, or 'q' to quit"),
			Tag:         "Error",
		}})
		return v, nil
	}

	items := make([]components.ListItem, 0, len(msg.Objects))
	for _, obj := range msg.Objects {
		tag := "Object"
		description := fmt.Sprintf("%s %s | %s %s",
			T("Size:"), formatBytes(obj.Size),
			T("Modified:"), formatModified(obj.LastModified))
		if strings.HasSuffix(obj.Key, "/") {
			tag = "Folder"
			description = T("Folder marker")
		}
		items = append(items, components.ListItem{
			Title:       obj.Key,
			Description: description,
			Tag:         tag,
			Data:        obj,
		})
	}

	title := T("Select Object")
	switch v.mode {
	case ModeDownload:
		title = T("Select Object to Download")
	case ModeDelete:
		title = T("Select Object to Delete")
	}
	v.list = components.NewList(title, items)
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
			return v, tea.Quit
		}
		return v, nil
	}

	if v.transfer.active {
		switch key {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			v.transfer.cancelTransfer()
		}
		return v, nil
	}

	if v.loading {
		if key == "ctrl+c" || key == "q" {
			return v, tea.Quit
		}
		return v, nil
	}

	var cmds []tea.Cmd
	switch key {
	case "ctrl+c", "q":
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
				cmds = append(cmds, previewObject(*obj))
			}
		}
	case "r":
		v.loading = true
		v.errors.ClearErrors()
		v.spinner = components.NewSpinner(T("Retrying to load S3 objects..."))
		return v, tea.Batch(v.spinner.Start(), v.loadObjects())
	case "enter", " ":
		item := v.currentItem()
		if item == nil {
			break
		}
		if item.Tag == "Error" {
			v.loading = true
			v.spinner = components.NewSpinner(T("Retrying to load S3 objects..."))
			return v, tea.Batch(v.spinner.Start(), v.loadObjects())
		}
		obj, ok := item.Data.(s3.Object)
		if !ok {
			break
		}
		return v.selectObject(obj)
	}

	if v.list != nil {
		v.list, _ = v.list.Update(msg)
		if v.showPreview {
			if obj := v.currentObject(); obj != nil {
				cmds = append(cmds, previewObject(*obj))
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

	if v.loading {
		return headerStyle.Render(T("S3 Objects")) + "\n\n" + v.spinner.View()
	}

	if v.list == nil {
		return errorStyle.Render(T("Failed to load S3 objects"))
	}

	context := contextStyle.Render(fmt.Sprintf("%s %s | %s %s",
		T("Region:"), v.deps.region(), T("Bucket:"), v.bucket))

	if v.confirm != nil {
		var question string
		if v.confirm.kind == confirmDelete {
			question = T("Delete %s? [y/N]", v.confirm.object.Key)
		} else {
			question = T("The file exists. Overwrite %s? [y/N]", v.confirm.localPath)
		}
		return context + "\n\n" + errorStyle.Render(question)
	}

	footer := footerStyle.Render(T("↑↓: navigate • enter: select • r: refresh • p: preview • ?: help • s: settings • esc: back • q: quit"))

	var body string
	if v.showPreview && v.preview != nil {
		listWidth := v.width / 2
		if listWidth < 20 {
			listWidth = 20
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(listWidth).Render(v.list.View()),
			lipgloss.NewStyle().Width(v.width-listWidth).Render(v.preview.View()),
		)
	} else {
		body = v.list.View()
	}

	result := context + "\n\n" + body
	if v.errors.GetErrorCount() > 0 {
		result += "\n\n" + v.errors.View()
	}
	return result + "\n\n" + footer
}

func (v *ObjectView) currentItem() *components.ListItem {
	if v.list == nil {
		return nil
	}
	return v.list.GetCurrentItem()
}

func (v *ObjectView) currentObject() *s3.Object {
	item := v.currentItem()
	if item == nil || item.Tag != "Object" {
		return nil
	}
	obj, ok := item.Data.(s3.Object)
	if !ok {
		return nil
	}
	return &obj
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
func previewObject(obj s3.Object) tea.Cmd {
	return func() tea.Msg {
		modified := formatModified(obj.LastModified)
		content := fmt.Sprintf("%s\n\n%s %s\n%s %s\n%s %s\n%s %s",
			T("S3 Object Information"),
			T("Key:"), obj.Key,
			T("Size:"), formatBytes(obj.Size),
			T("Modified:"), modified,
			T("ETag:"), truncateShort(obj.ETag, 40),
		)
		return components.PreviewMsg{Content: content, PreviewType: components.PreviewTypeText}
	}
}

// progressMessage renders the text line shown under the progress bar.
func progressMessage(event s3.Progress) string {
	if event.Total > 0 {
		return fmt.Sprintf("%s / %s", formatBytes(event.Transferred), formatBytes(event.Total))
	}
	return formatBytes(event.Transferred)
}

// formatModified renders a timestamp, substituting a dash for the zero time.
func formatModified(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}
