package views

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
)

// confirmKind identifies which destructive step is being confirmed.
type confirmKind int

const (
	confirmDelete confirmKind = iota
	confirmOverwrite
)

// confirmPrompt holds the pending destructive action awaiting a y/n answer.
type confirmPrompt struct {
	kind      confirmKind
	object    s3.Object
	localPath string
}

// selectObject routes the selected object to the configured operation.
// Folder markers cannot be downloaded; deletes and overwrites ask first.
func (v *ObjectView) selectObject(obj s3.Object) (tea.Model, tea.Cmd) {
	switch v.mode {
	case ModeDelete:
		v.confirm = &confirmPrompt{kind: confirmDelete, object: obj}
		return v, nil
	case ModeDownload:
		if strings.HasSuffix(obj.Key, "/") {
			return v, nil
		}
		localPath := filepath.Base(obj.Key)
		if _, err := os.Stat(localPath); err == nil {
			v.confirm = &confirmPrompt{kind: confirmOverwrite, object: obj, localPath: localPath}
			return v, nil
		}
		return v.startDownload(obj, s3.OverwriteFail)
	}
	return v, nil
}

// startDownload runs Session.Download on a command goroutine and reports
// progress through the broker.
func (v *ObjectView) startDownload(obj s3.Object, overwrite s3.OverwriteMode) (tea.Model, tea.Cmd) {
	ctx, wait := v.transfer.begin(v.deps, v.deps.T("Downloading %s", obj.Key), obj.Size)
	broker := v.transfer.broker

	session, bucket, key := v.deps.Session, v.bucket, obj.Key
	localPath := filepath.Base(key)
	return v, tea.Batch(
		func() tea.Msg {
			err := session.Download(ctx, bucket, key, localPath, s3.DownloadOptions{
				Overwrite: overwrite,
				Progress:  broker.callback,
			})
			return transferDoneMsg{
				err:     err,
				summary: v.deps.T("Downloaded %s (%s)", localPath, components.FormatBytes(obj.Size)),
				broker:  broker,
			}
		},
		wait,
	)
}

// startDelete runs Session.DeleteKeys on a command goroutine.
func (v *ObjectView) startDelete(obj s3.Object) (tea.Model, tea.Cmd) {
	ctx, wait := v.transfer.begin(v.deps, v.deps.T("Deleting %s", obj.Key), 1)
	broker := v.transfer.broker

	session, bucket, key := v.deps.Session, v.bucket, obj.Key
	return v, tea.Batch(
		func() tea.Msg {
			result, err := session.DeleteKeys(ctx, bucket, []string{key}, s3.DeleteOptions{
				Progress: broker.callback,
			})
			if err == nil && len(result.Failed) > 0 {
				err = result.Failed[0].Err
			}
			return transferDoneMsg{err: err, summary: v.deps.T("Deleted %s", key), broker: broker}
		},
		wait,
	)
}
