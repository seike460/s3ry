package views

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
)

// BucketsLoadedMsg carries the result of a ListBuckets request.
type BucketsLoadedMsg struct {
	Buckets []s3.Bucket
	Err     error
}

// BucketView is the bucket selection view.
type BucketView struct {
	deps  Deps
	state listState
}

// NewBucketView creates the bucket selection view.
func NewBucketView(deps Deps) *BucketView {
	return &BucketView{
		deps:  deps,
		state: newListState(deps.T("Loading S3 buckets...")),
	}
}

// Init starts the spinner and the first bucket load.
func (v *BucketView) Init() tea.Cmd {
	return tea.Batch(v.state.spinner.Start(), v.loadBuckets())
}

// Update handles messages for the bucket view.
func (v *BucketView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if v.state.list != nil {
			v.state.list, _ = v.state.list.Update(msg)
		}

	case BucketsLoadedMsg:
		return v.onBucketsLoaded(msg)

	case tea.KeyMsg:
		return v.onKey(msg)

	case components.SpinnerTickMsg:
		cmds = append(cmds, v.state.onTick(msg))
	}

	return v, tea.Batch(cmds...)
}

// onBucketsLoaded fills the list with the fetched buckets.
func (v *BucketView) onBucketsLoaded(msg BucketsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		v.state.fail(v.deps, v.deps.T("Error Loading Buckets"), v.deps.T("Failed to load S3 buckets"), msg.Err)
		return v, nil
	}

	fallbackRegion := v.deps.region()
	items := make([]components.ListItem, len(msg.Buckets))
	for i, bucket := range msg.Buckets {
		region := bucket.Region
		if region == "" {
			region = fallbackRegion
		}
		items[i] = components.ListItem{
			Title:       bucket.Name,
			Description: fmt.Sprintf("%s %s", v.deps.T("Region:"), region),
			Tag:         "Bucket",
			Data:        bucket,
		}
	}
	v.state.loaded(v.deps.T("Select S3 Bucket"), items)
	return v, nil
}

// onKey handles keyboard input on the bucket list.
func (v *BucketView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if v.state.loading {
		return v, nil
	}

	if v.state.filterActive() {
		v.state.routeFilterKey(msg)
		return v, nil
	}

	if v.state.retryRequested(msg.String()) {
		return v, v.state.startLoading(v.deps.T("Retrying to load S3 buckets..."), v.loadBuckets())
	}

	return v.onListKey(msg)
}

// onListKey handles keys on the loaded bucket list.
func (v *BucketView) onListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if quitKey(msg.String()) {
		return v, tea.Quit
	}
	switch msg.String() {
	case "?":
		return NewHelpView(v.deps), nil
	case "s":
		return NewSettingsView(v.deps), nil
	case "enter", " ":
		if bucket := v.selectedBucket(); bucket != nil {
			return NewOperationView(v.deps, bucket.Name), nil
		}
	}

	if v.state.list != nil {
		v.state.list, _ = v.state.list.Update(msg)
	}
	return v, nil
}

// selectedBucket returns the s3.Bucket under the cursor, if any.
func (v *BucketView) selectedBucket() *s3.Bucket {
	item := v.state.currentItem()
	if item == nil {
		return nil
	}
	bucket, ok := item.Data.(s3.Bucket)
	if !ok {
		return nil
	}
	return &bucket
}

// View renders the bucket view.
func (v *BucketView) View() string {
	if v.state.loading {
		return headerStyle.Render(v.deps.T("s3ry - S3 file manager")) + "\n\n" + v.state.spinner.View()
	}
	if v.state.list == nil {
		return errorStyle.Render(v.deps.T("Failed to load S3 buckets"))
	}

	var result strings.Builder
	result.WriteString(contextStyle.Render(
		fmt.Sprintf("%s %s", v.deps.T("Region:"), v.deps.region())))
	result.WriteString("\n\n")
	result.WriteString(v.state.list.View())

	if v.state.errors.GetErrorCount() > 0 {
		result.WriteString("\n\n")
		result.WriteString(v.state.errors.View())
	}

	result.WriteString("\n\n")
	result.WriteString(footerStyle.Render(v.deps.T("↑↓: navigate • enter: select • r: retry • ?: help • s: settings • q: quit")))
	return result.String()
}

// loadBuckets issues ListBuckets with the configured timeout.
func (v *BucketView) loadBuckets() tea.Cmd {
	return func() tea.Msg {
		if err := v.deps.sessionErr(); err != nil {
			return BucketsLoadedMsg{Err: err}
		}
		ctx, cancel := v.deps.listContext(context.Background())
		defer cancel()
		buckets, err := v.deps.Session.ListBuckets(ctx)
		return BucketsLoadedMsg{Buckets: buckets, Err: err}
	}
}
