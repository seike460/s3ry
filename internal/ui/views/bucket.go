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
	deps    Deps
	list    *components.List
	spinner *components.Spinner
	errors  *components.ErrorDisplay
	loading bool
}

// NewBucketView creates the bucket selection view.
func NewBucketView(deps Deps) *BucketView {
	return &BucketView{
		deps:    deps,
		loading: true,
		spinner: components.NewSpinner(T("Loading S3 buckets...")),
		errors:  components.NewErrorDisplay(),
	}
}

// Init starts the spinner and the first bucket load.
func (v *BucketView) Init() tea.Cmd {
	return tea.Batch(v.spinner.Start(), v.loadBuckets())
}

// Update handles messages for the bucket view.
func (v *BucketView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}

	case BucketsLoadedMsg:
		v.loading = false
		v.spinner.Stop()

		if msg.Err != nil {
			v.errors.AddAWSError(msg.Err)
			v.list = components.NewList(T("Error Loading Buckets"), []components.ListItem{{
				Title:       T("Failed to load S3 buckets"),
				Description: T("Press 'r' to retry, 'esc' to go back, or 'q' to quit"),
				Tag:         "Error",
			}})
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
		v.list = components.NewList(T("Select S3 Bucket"), items)
		return v, nil

	case tea.KeyMsg:
		if v.loading {
			break
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "?":
			return NewHelpView(v.deps), nil
		case "s":
			return NewSettingsView(v.deps), nil
		case "r":
			v.loading = true
			v.spinner = components.NewSpinner(T("Retrying to load S3 buckets..."))
			return v, tea.Batch(v.spinner.Start(), v.loadBuckets())
		case "enter", " ":
			item := v.currentItem()
			if item == nil {
				break
			}
			if item.Tag == "Error" {
				v.loading = true
				v.spinner = components.NewSpinner(T("Retrying to load S3 buckets..."))
				return v, tea.Batch(v.spinner.Start(), v.loadBuckets())
			}
			bucket, ok := item.Data.(s3.Bucket)
			if !ok {
				break
			}
			return NewOperationView(v.deps, bucket.Name), nil
		}

		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}

	case components.SpinnerTickMsg:
		if v.loading {
			v.spinner, _ = v.spinner.Update(msg)
			cmds = append(cmds, v.spinner.Start())
		}
	}

	return v, tea.Batch(cmds...)
}

func (v *BucketView) currentItem() *components.ListItem {
	if v.list == nil {
		return nil
	}
	return v.list.GetCurrentItem()
}

// View renders the bucket view.
func (v *BucketView) View() string {
	if v.loading {
		return headerStyle.Render(T("s3ry - S3 file manager")) + "\n\n" + v.spinner.View()
	}
	if v.list == nil {
		return errorStyle.Render(T("Failed to load S3 buckets"))
	}

	var result strings.Builder
	result.WriteString(contextStyle.Render(
		fmt.Sprintf("%s %s", T("Region:"), v.deps.region())))
	result.WriteString("\n\n")
	result.WriteString(v.list.View())

	if v.errors.GetErrorCount() > 0 {
		result.WriteString("\n\n")
		result.WriteString(v.errors.View())
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
