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
		state: newListState(T("Loading S3 buckets...")),
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
		if msg.Err != nil {
			v.state.fail(T("Error Loading Buckets"), T("Failed to load S3 buckets"), msg.Err)
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
				Description: fmt.Sprintf("%s %s", T("Region:"), region),
				Tag:         "Bucket",
				Data:        bucket,
			}
		}
		v.state.loaded(T("Select S3 Bucket"), items)
		return v, nil

	case tea.KeyMsg:
		if v.state.loading {
			break
		}

		if v.state.retryRequested(msg.String()) {
			return v, v.state.startLoading(T("Retrying to load S3 buckets..."), v.loadBuckets())
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "?":
			return NewHelpView(v.deps), nil
		case "s":
			return NewSettingsView(v.deps), nil
		case "enter", " ":
			item := v.state.currentItem()
			if item == nil {
				break
			}
			bucket, ok := item.Data.(s3.Bucket)
			if !ok {
				break
			}
			return NewOperationView(v.deps, bucket.Name), nil
		}

		if v.state.list != nil {
			v.state.list, _ = v.state.list.Update(msg)
		}

	case components.SpinnerTickMsg:
		cmds = append(cmds, v.state.onTick(msg))
	}

	return v, tea.Batch(cmds...)
}

// View renders the bucket view.
func (v *BucketView) View() string {
	if v.state.loading {
		return headerStyle.Render(T("s3ry - S3 file manager")) + "\n\n" + v.state.spinner.View()
	}
	if v.state.list == nil {
		return errorStyle.Render(T("Failed to load S3 buckets"))
	}

	var result strings.Builder
	result.WriteString(contextStyle.Render(
		fmt.Sprintf("%s %s", T("Region:"), v.deps.region())))
	result.WriteString("\n\n")
	result.WriteString(v.state.list.View())

	if v.state.errors.GetErrorCount() > 0 {
		result.WriteString("\n\n")
		result.WriteString(v.state.errors.View())
	}

	result.WriteString("\n\n")
	result.WriteString(footerStyle.Render(T("↑↓: navigate • enter: select • r: retry • ?: help • s: settings • q: quit")))
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
