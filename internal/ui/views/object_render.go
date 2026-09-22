package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
)

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
