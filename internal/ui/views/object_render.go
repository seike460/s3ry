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
	// maxListPageKeys bounds one ListObjectsV2 request.
	maxListPageKeys = 1000
)

func (v *ObjectView) onObjectsLoaded(msg ObjectsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		v.state.fail(v.deps, v.deps.T("Error Loading Objects"), v.deps.T("Failed to load S3 objects"), msg.Err)
		return v, nil
	}
	pageItems := v.pageItems(msg.Page)
	if msg.Append && v.state.list != nil {
		if n := len(v.items); n > 0 && v.items[n-1].Tag == "More" {
			v.items = v.items[:n-1]
		}
		v.items = append(v.items, pageItems...)
		v.state.list.SetItems(v.items)
		return v, nil
	}
	v.items = pageItems
	v.state.loaded(v.listTitle(), v.items)
	return v, nil
}

// listTitle returns the list header for the view's mode.
func (v *ObjectView) listTitle() string {
	if v.mode == ModeDelete {
		return v.deps.T("Select Object to Delete")
	}
	return v.deps.T("Select Object to Download")
}

// pageItems builds list rows for one page: common prefixes as folder rows,
// then objects, then the "Load more" sentinel while the listing is
// truncated.
func (v *ObjectView) pageItems(page *s3.Page) []components.ListItem {
	if page == nil {
		return nil
	}
	items := make([]components.ListItem, 0, len(page.Prefixes)+len(page.Objects)+1)
	for _, prefix := range page.Prefixes {
		items = append(items, components.ListItem{
			Title:       strings.TrimPrefix(prefix, v.prefix),
			Description: v.deps.T("Folder"),
			Tag:         "Folder",
			Data:        prefix,
		})
	}
	for _, obj := range page.Objects {
		items = append(items, v.objectItem(obj))
	}
	if page.IsTruncated {
		items = append(items, components.ListItem{
			Title:       v.deps.T("Load more..."),
			Description: v.deps.T("Show the next page of results"),
			Tag:         "More",
			Data:        page.NextToken,
		})
	}
	return items
}

// objectItem builds one list row for an object. Titles show the key
// relative to the current prefix; folder markers become folder rows.
func (v *ObjectView) objectItem(obj s3.Object) components.ListItem {
	tag := "Object"
	description := fmt.Sprintf("%s %s | %s %s",
		v.deps.T("Size:"), components.FormatBytes(obj.Size),
		v.deps.T("Modified:"), formatModified(obj.LastModified))
	if strings.HasSuffix(obj.Key, "/") {
		tag = "Folder"
		description = v.deps.T("Folder marker")
	}
	return components.ListItem{
		Title:       strings.TrimPrefix(obj.Key, v.prefix),
		Description: description,
		Tag:         tag,
		Data:        obj,
	}
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

	return v.readyBody(context)
}

// readyBody renders the loaded list with its notice, error panel, and
// footer.
func (v *ObjectView) readyBody(context string) string {
	footer := footerStyle.Render(v.deps.T("↑↓: navigate • enter: select • /: filter • r: refresh • p: preview • P: presign • ?: help • s: settings • esc: back • q: quit"))

	result := context
	if v.notice != "" {
		result += "\n\n" + noticeStyle.Render(v.notice)
	}
	result += "\n\n" + v.listBody()
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
	switch v.confirm.kind {
	case confirmDelete:
		return v.deps.T("Delete %s? [y/N]", v.confirm.object.Key)
	case confirmDeletePrefix:
		return v.prefixDeleteQuestion()
	case confirmPresign:
		return v.deps.T("Presigned URL for %s — expiry: [1] 1 hour  [2] 24 hours  [3] 7 days (esc: cancel)", v.confirm.object.Key)
	default:
		return v.deps.T("The file exists. Overwrite %s? [y/N]", v.confirm.localPath)
	}
}

// prefixDeleteQuestion renders the folder-delete prompt. The dry-run count
// arrives asynchronously; until then the question notes that counting is in
// progress, and a count error falls back to a plain prompt.
func (v *ObjectView) prefixDeleteQuestion() string {
	key := v.confirm.object.Key
	if v.confirm.countErr != nil {
		return v.deps.T("Delete all objects under %s? [y/N]", key)
	}
	if v.confirm.count < 0 {
		return v.deps.T("Delete all objects under %s? (counting...) [y/N]", key)
	}
	return v.deps.T("Delete %d objects under %s? [y/N]", v.confirm.count, key)
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
