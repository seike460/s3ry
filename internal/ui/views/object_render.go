package views

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
)

// onObjectsLoaded renders the freshly fetched listing or its error.
const (
	// minPreviewListWidth keeps the object list usable when the preview pane
	// takes the right half of a narrow terminal.
	minPreviewListWidth = 20
	// etagDisplayLength truncates the ETag in the detail pane.
	etagDisplayLength = 40
)

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

	context := v.contextLine()

	if v.confirm != nil {
		return context + "\n\n" + errorStyle.Render(v.confirmQuestion())
	}

	footer := footerStyle.Render(v.deps.T("↑↓: navigate • enter: select • r: refresh • p: preview • ?: help • s: settings • esc: back • q: quit"))

	result := context + "\n\n" + v.listBody()
	if v.state.errors.GetErrorCount() > 0 {
		result += "\n\n" + v.state.errors.View()
	}
	return result + "\n\n" + footer
}

// contextLine renders the "Region | Bucket [| Prefix]" header line.
func (v *ObjectView) contextLine() string {
	line := fmt.Sprintf("%s %s | %s %s",
		v.deps.T("Region:"), v.deps.region(), v.deps.T("Bucket:"), v.bucket)
	if v.prefix != "" {
		line += fmt.Sprintf(" | %s %s", v.deps.T("Prefix:"), v.prefix)
	}
	return contextStyle.Render(line)
}

// confirmQuestion renders the open confirmation prompt's question.
func (v *ObjectView) confirmQuestion() string {
	if v.confirm.kind == confirmDelete {
		return v.deps.T("Delete %s? [y/N]", v.confirm.object.Key)
	}
	return v.deps.T("The file exists. Overwrite %s? [y/N]", v.confirm.localPath)
}

// listBody renders the object list, joined with the preview pane when the
// preview is open.
func (v *ObjectView) listBody() string {
	if !v.showPreview || v.preview == nil {
		return v.state.list.View()
	}
	listWidth := v.width / 2
	if listWidth < minPreviewListWidth {
		listWidth = minPreviewListWidth
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(listWidth).Render(v.state.list.View()),
		lipgloss.NewStyle().Width(v.width-listWidth).Render(v.preview.View()),
	)
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

// previewObject fetches the object's metadata via HeadObject and renders it
// into the preview pane.
func (v *ObjectView) previewObject(obj s3.Object) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := v.deps.listContext(context.Background())
		defer cancel()
		info, err := v.deps.Session.Stat(ctx, v.bucket, obj.Key)
		if err != nil {
			content := fmt.Sprintf("%s\n\n%s", v.deps.T("S3 Object Information"), err.Error())
			return components.PreviewMsg{Content: content, PreviewType: components.PreviewTypeText}
		}
		return components.PreviewMsg{
			Content:     v.renderObjectInfo(info),
			PreviewType: components.PreviewTypeText,
		}
	}
}

// renderObjectInfo formats every field of a HeadObject response.
func (v *ObjectView) renderObjectInfo(info *s3.ObjectInfo) string {
	var b strings.Builder
	b.WriteString(v.deps.T("S3 Object Information") + "\n\n")
	writeInfoField(&b, v.deps.T("Key:"), info.Key)
	writeInfoField(&b, v.deps.T("Size:"), components.FormatBytes(info.Size))
	writeInfoField(&b, v.deps.T("Modified:"), formatModified(info.LastModified))
	writeInfoField(&b, v.deps.T("ETag:"), truncateShort(info.ETag, etagDisplayLength))
	writeInfoField(&b, v.deps.T("Storage Class:"), info.StorageClass)
	writeInfoField(&b, v.deps.T("Content Type:"), info.ContentType)
	writeInfoField(&b, v.deps.T("Version ID:"), info.VersionID)
	writeMetadata(&b, v.deps, info.Metadata)
	return strings.TrimRight(b.String(), "\n")
}

// writeInfoField appends a "label value" line, skipping empty values.
func writeInfoField(b *strings.Builder, label, value string) {
	if value == "" {
		return
	}
	b.WriteString(label + " " + value + "\n")
}

// writeMetadata appends the user-defined metadata block in sorted key order.
func writeMetadata(b *strings.Builder, d Deps, metadata map[string]string) {
	if len(metadata) == 0 {
		return
	}
	b.WriteString(d.T("Metadata:") + "\n")
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString("  " + k + ": " + metadata[k] + "\n")
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
