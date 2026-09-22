package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
)

// ObjectsLoadedMsg carries one page of a hierarchical object listing.
// Append marks continuation pages requested through the "Load more" item;
// Prefix records which prefix the page belongs to so a stale response is
// dropped after the user navigates away.
type ObjectsLoadedMsg struct {
	Page   *s3.Page
	Append bool
	Prefix string
	Err    error
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
// It browses the bucket hierarchically: parents holds the prefixes above the
// current one and items accumulates the listing across "Load more" pages.
type ObjectView struct {
	deps        Deps
	bucket      string
	prefix      string
	parents     []string
	mode        ObjectMode
	state       listState
	items       []components.ListItem
	preview     *components.Preview
	showPreview bool
	previewKey  string // key whose HeadObject request is in flight or shown
	notice      string // transient status line (e.g. a presigned URL)
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
	v := &ObjectView{
		deps:    deps,
		bucket:  bucket,
		prefix:  prefix,
		mode:    mode,
		preview: components.NewPreview(),
	}
	v.state = newListState(v.loadMessage())
	return v
}

// loadMessage returns the spinner text used while the listing loads.
func (v *ObjectView) loadMessage() string {
	if v.mode == ModeDelete {
		return v.deps.T("Loading S3 objects for delete...")
	}
	return v.deps.T("Loading S3 objects for download...")
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

	case prefixCountMsg, presignResultMsg:
		v.onAsyncResult(msg)

	case backToOperationMsg, brokerClosedMsg:
		// brokerClosedMsg means the broker was closed while a read was in
		// flight; nothing to do.
		if v.backToOperation(msg) {
			return NewOperationView(v.deps, v.bucket), nil
		}

	case tea.KeyMsg:
		return v.onKey(msg)
	}

	return v, tea.Batch(cmds...)
}

// onAsyncResult stores asynchronous lookup results — the prefix delete
// count or a presigned URL — on the view.
func (v *ObjectView) onAsyncResult(msg tea.Msg) {
	switch msg := msg.(type) {
	case prefixCountMsg:
		v.onPrefixCount(msg)
	case presignResultMsg:
		v.onPresignResult(msg)
	}
}

// backToOperation reports whether msg is the post-transfer return tick for
// the most recent broker.
func (v *ObjectView) backToOperation(msg tea.Msg) bool {
	tick, ok := msg.(backToOperationMsg)
	return ok && v.transfer.backToOperation(tick)
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
	if v.confirm.kind == confirmPresign {
		return v.onPresignKey(key)
	}
	switch key {
	case "y", "Y":
		pending := *v.confirm
		v.confirm = nil
		switch pending.kind {
		case confirmDelete:
			return v.startDelete(pending.object)
		case confirmDeletePrefix:
			return v.startDeletePrefix(pending.object, max(int64(pending.count), 0))
		default:
			return v.startDownload(pending.object, s3.OverwriteAlways)
		}
	case "n", "N", "esc":
		v.confirm = nil
		return v, nil
	}
	if v.transfer.quitRequested(key) {
		return v, tea.Quit
	}
	return v, nil
}

// onPresignKey picks an expiry from the presign prompt or cancels it.
func (v *ObjectView) onPresignKey(key string) (tea.Model, tea.Cmd) {
	for _, choice := range presignExpirations {
		if key == choice.key {
			obj := v.confirm.object
			v.confirm = nil
			return v, v.startPresign(obj, choice.expires)
		}
	}
	if key == "esc" || key == "n" {
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
	if next, cmd := v.navView(key); next != nil {
		return next, cmd
	}

	if next, cmd := v.actionKey(msg); next != nil || cmd != nil {
		return next, cmd
	}

	var cmds []tea.Cmd
	if v.state.list != nil {
		v.state.list, _ = v.state.list.Update(msg)
		v.refreshPreview(&cmds)
	}
	return v, tea.Batch(cmds...)
}

// actionKey handles the ready-state action keys: preview toggle, presign,
// and item selection. It returns nil when the key is not an action key.
func (v *ObjectView) actionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg.String() {
	case "p":
		v.showPreview = !v.showPreview
		v.refreshPreview(&cmds)
		return v, tea.Batch(cmds...)
	case "P":
		if obj := v.currentObject(); obj != nil {
			v.confirm = &confirmPrompt{kind: confirmPresign, object: *obj}
		}
		return v, nil
	case "enter", " ":
		return v.enterItem()
	}
	return nil, nil
}

// enterItem dispatches enter/space on the item under the cursor: "Load
// more" fetches the next page, folders descend (or start a prefix delete),
// and objects run the view's operation.
func (v *ObjectView) enterItem() (tea.Model, tea.Cmd) {
	item := v.state.currentItem()
	if item == nil {
		return nil, nil
	}
	switch item.Tag {
	case "More":
		token, _ := item.Data.(string)
		return v, v.fetchPage(token)
	case "Folder":
		return v.enterFolder(item)
	default:
		if obj := v.selectedObject(); obj != nil {
			return v.selectObject(*obj)
		}
	}
	return nil, nil
}

// enterFolder opens a folder row. In delete mode it starts the prefix
// delete confirmation; otherwise it descends into the prefix.
func (v *ObjectView) enterFolder(item *components.ListItem) (tea.Model, tea.Cmd) {
	key := folderItemKey(item)
	if key == "" {
		return v, nil
	}
	if v.mode == ModeDelete {
		return v.selectObject(s3.Object{Key: key})
	}
	return v.descend(key)
}

// folderItemKey extracts the folder's full key from a row created for a
// common prefix (string) or a folder-marker object (s3.Object).
func folderItemKey(item *components.ListItem) string {
	switch data := item.Data.(type) {
	case string:
		return data
	case s3.Object:
		return data.Key
	}
	return ""
}

// descend moves the listing into a child prefix.
func (v *ObjectView) descend(prefix string) (tea.Model, tea.Cmd) {
	v.parents = append(v.parents, v.prefix)
	v.prefix = prefix
	return v.reload()
}

// ascend returns to the parent prefix.
func (v *ObjectView) ascend() (tea.Model, tea.Cmd) {
	v.prefix = v.parents[len(v.parents)-1]
	v.parents = v.parents[:len(v.parents)-1]
	return v.reload()
}

// reload clears per-location state and fetches the first page of the
// current prefix.
func (v *ObjectView) reload() (tea.Model, tea.Cmd) {
	v.previewKey = ""
	v.notice = ""
	return v, v.state.startLoading(v.loadMessage(), v.loadObjects())
}

// navView returns the destination for navigation keys — ascending a prefix
// for esc, or a sibling view — or nil when the key is not a navigation key.
func (v *ObjectView) navView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		if len(v.parents) > 0 {
			return v.ascend()
		}
		return NewOperationView(v.deps, v.bucket), nil
	case "?":
		return NewHelpView(v.deps), nil
	case "s":
		return NewSettingsView(v.deps), nil
	}
	return nil, nil
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
	key := ""
	if obj != nil {
		key = obj.Key
	}
	if key == v.previewKey {
		return
	}
	v.previewKey = key
	if obj == nil {
		// Folder or sentinel rows have no HeadObject; clear the pane.
		*cmds = append(*cmds, func() tea.Msg { return components.PreviewMsg{} })
		return
	}
	*cmds = append(*cmds, v.previewObject(*obj))
}

// AbortTransfer cancels any in-flight transfer and releases the progress
// broker. The app calls it when replacing the view or quitting.
func (v *ObjectView) AbortTransfer() {
	v.transfer.abort()
}

// loadObjects fetches the first page of the current prefix.
func (v *ObjectView) loadObjects() tea.Cmd {
	return v.fetchPage("")
}

// fetchPage requests one page of the hierarchical listing. A non-empty
// token continues the previous listing ("Load more" item). The prefix is
// captured now so a stale response can be dropped on arrival.
func (v *ObjectView) fetchPage(token string) tea.Cmd {
	prefix := v.prefix
	return func() tea.Msg {
		if err := v.deps.sessionErr(); err != nil {
			return ObjectsLoadedMsg{Err: err, Prefix: prefix}
		}
		ctx, cancel := v.deps.listContext(context.Background())
		defer cancel()
		page, err := v.deps.Session.ListPage(ctx, v.bucket, prefix, token, maxListPageKeys)
		if err != nil {
			return ObjectsLoadedMsg{Err: err, Prefix: prefix}
		}
		return ObjectsLoadedMsg{Page: page, Append: token != "", Prefix: prefix}
	}
}
