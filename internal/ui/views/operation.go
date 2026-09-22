package views

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/ui/components"
)

// operation describes one bucket action: its menu entry, its shortcut key,
// and the view it opens. Adding an operation is a one-entry change here.
type operation struct {
	// tag identifies the operation; it is stored as the list item's Tag so
	// selection can find this entry again.
	tag string
	// title and desc render the menu row.
	title string
	desc  string
	// shortcut is the key that jumps straight to this operation; empty means
	// the operation is reachable only through the list.
	shortcut string
	// view builds the screen for the operation.
	view func(Deps, string) tea.Model
}

// operations is the menu table. Order defines the list order.
func operations(d Deps) []operation {
	return []operation{
		{
			tag:      "Download",
			title:    d.T("Download files"),
			desc:     d.T("Download an object from the bucket (shortcut: d)"),
			shortcut: "d",
			view:     func(d Deps, b string) tea.Model { return NewObjectView(d, b, ModeDownload) },
		},
		{
			tag:      "Upload",
			title:    d.T("Upload files"),
			desc:     d.T("Upload a local file to the bucket (shortcut: u)"),
			shortcut: "u",
			view:     func(d Deps, b string) tea.Model { return NewUploadView(d, b) },
		},
		{
			tag:      "Delete",
			title:    d.T("Delete objects"),
			desc:     d.T("Delete an object from the bucket (shortcut: delete)"),
			shortcut: "delete",
			view:     func(d Deps, b string) tea.Model { return NewObjectView(d, b, ModeDelete) },
		},
		{
			tag:   "List",
			title: d.T("Create object list"),
			desc:  d.T("Export the bucket's object list to a local file"),
			view:  func(d Deps, b string) tea.Model { return NewListGeneratorView(d, b) },
		},
	}
}

// findOperation resolves a list item tag or shortcut key to its operation.
func findOperation(ops []operation, match func(operation) bool) *operation {
	for _, op := range ops {
		if match(op) {
			return &op
		}
	}
	return nil
}

// OperationView is the per-bucket operation selection view.
type OperationView struct {
	deps   Deps
	bucket string
	ops    []operation
	list   *components.List
}

// NewOperationView creates a new operation view.
func NewOperationView(deps Deps, bucket string) *OperationView {
	ops := operations(deps)
	items := make([]components.ListItem, len(ops))
	for i, op := range ops {
		items[i] = components.ListItem{
			Title:       op.title,
			Description: op.desc,
			Tag:         op.tag,
		}
	}

	return &OperationView{
		deps:   deps,
		bucket: bucket,
		ops:    ops,
		list:   components.NewList(deps.T("Select Operation"), items),
	}
}

// Init initializes the operation view.
func (v *OperationView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the operation view.
func (v *OperationView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.list, _ = v.list.Update(msg)

	case tea.KeyMsg:
		return v.onKey(msg)
	}

	return v, nil
}

// onKey handles keyboard input on the operation list.
func (v *OperationView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if quitKey(key) {
		return v, tea.Quit
	}
	switch key {
	case "esc":
		return NewBucketView(v.deps), nil
	case "?":
		return NewHelpView(v.deps), nil
	case "s":
		return NewSettingsView(v.deps), nil
	case "enter", " ":
		if op := v.selectedOperation(); op != nil {
			return op.view(v.deps, v.bucket), nil
		}
	default:
		if op := v.shortcutOperation(key); op != nil {
			return op.view(v.deps, v.bucket), nil
		}
	}

	v.list, _ = v.list.Update(msg)
	return v, nil
}

// selectedOperation returns the operation under the cursor, if any.
func (v *OperationView) selectedOperation() *operation {
	item := v.list.GetCurrentItem()
	if item == nil {
		return nil
	}
	return findOperation(v.ops, func(o operation) bool { return o.tag == item.Tag })
}

// shortcutOperation resolves key to the operation with that shortcut.
func (v *OperationView) shortcutOperation(key string) *operation {
	return findOperation(v.ops, func(o operation) bool { return o.shortcut == key && key != "" })
}

// View renders the operation view.
func (v *OperationView) View() string {
	header := headerStyle.Render(v.deps.T("Select Operation"))
	context := contextStyle.Render(fmt.Sprintf("%s %s | %s %s",
		v.deps.T("Region:"), v.deps.region(), v.deps.T("Bucket:"), v.bucket))
	footer := footerStyle.Render(v.deps.T("d: download • u: upload • delete: delete • ?: help • s: settings • esc: back • q: quit"))

	return header + "\n" + context + "\n\n" + v.list.View() + "\n\n" + footer
}
