package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"

	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
)

// confirmKind identifies which destructive step is being confirmed.
type confirmKind int

const (
	confirmDelete confirmKind = iota
	confirmDeletePrefix
	confirmPresign
	confirmOverwrite
)

// confirmPrompt holds the pending destructive action awaiting a y/n answer.
// count holds the dry-run result for prefix deletes: -1 while counting.
type confirmPrompt struct {
	kind      confirmKind
	object    s3.Object
	localPath string
	count     int
	countErr  error
}

// prefixCountMsg carries the dry-run result of a prefix delete. prefix
// records the location the count was requested at so a stale result cannot
// update a prompt opened after navigation.
type prefixCountMsg struct {
	object s3.Object
	prefix string
	count  int
	err    error
}

// presignExpirations maps the presign prompt's digit keys to expiry
// durations and their display labels.
var presignExpirations = []struct {
	key     string
	label   string
	expires time.Duration
}{
	{"1", "1 hour", time.Hour},
	{"2", "24 hours", 24 * time.Hour},
	{"3", "7 days", 7 * 24 * time.Hour},
}

// presignResultMsg carries a generated presigned URL back to the view.
// prefix records where the request was made; a result arriving after the
// user navigated away is dropped.
type presignResultMsg struct {
	url     string
	expires time.Duration
	prefix  string
	err     error
}

// selectObject routes the selected object to the configured operation.
// Folder markers cannot be downloaded; deletes and overwrites ask first.
// A folder delete removes every object under the prefix, so the matching
// count is fetched with a dry-run before the prompt answers.
func (v *ObjectView) selectObject(obj s3.Object) (tea.Model, tea.Cmd) {
	switch v.mode {
	case ModeDelete:
		if strings.HasSuffix(obj.Key, "/") {
			v.confirm = &confirmPrompt{kind: confirmDeletePrefix, object: obj, count: -1}
			return v, v.countPrefixObjects(obj)
		}
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

// countPrefixObjects performs a dry-run prefix delete to learn how many
// objects live under the folder marker.
func (v *ObjectView) countPrefixObjects(obj s3.Object) tea.Cmd {
	bucket, prefix := v.bucket, v.prefix
	return func() tea.Msg {
		ctx, cancel := v.deps.listContext(context.Background())
		defer cancel()
		result, err := v.deps.Session.DeletePrefix(ctx, bucket, obj.Key, s3.DeleteOptions{DryRun: true})
		return prefixCountMsg{object: obj, prefix: prefix, count: len(result.Deleted), err: err}
	}
}

// onPrefixCount stores the dry-run count on the open prefix-delete prompt.
func (v *ObjectView) onPrefixCount(msg prefixCountMsg) {
	if msg.prefix != v.prefix {
		return
	}
	if v.confirm == nil || v.confirm.kind != confirmDeletePrefix {
		return
	}
	if v.confirm.object.Key != msg.object.Key {
		return
	}
	v.confirm.count = msg.count
	v.confirm.countErr = msg.err
}

// startPresign generates a presigned GET URL for the object. The clipboard
// copy happens in onPresignResult so the OSC52 write is issued on the
// update path rather than a command goroutine.
func (v *ObjectView) startPresign(obj s3.Object, expires time.Duration) tea.Cmd {
	bucket, prefix, key := v.bucket, v.prefix, obj.Key
	return func() tea.Msg {
		ctx, cancel := v.deps.listContext(context.Background())
		defer cancel()
		url, err := v.deps.Session.PresignGet(ctx, bucket, key, expires)
		return presignResultMsg{url: url, expires: expires, prefix: prefix, err: err}
	}
}

// onPresignResult records the generated URL (or failure) for display and
// copies it to the clipboard when stdout is a terminal.
func (v *ObjectView) onPresignResult(msg presignResultMsg) {
	if msg.prefix != v.prefix {
		return
	}
	if msg.err != nil {
		v.notice = v.deps.T("Presign failed") + ": " + msg.err.Error()
		return
	}
	v.notice = fmt.Sprintf("%s %s\n%s %s",
		v.deps.T("Presigned URL:"), msg.url,
		v.deps.T("Expires:"), presignLabel(msg.expires))
	if term.IsTerminal(os.Stdout.Fd()) {
		_, _ = osc52.New(msg.url).WriteTo(os.Stdout)
		v.notice += " — " + v.deps.T("Copied to clipboard")
	}
}

// presignLabel renders an expiry duration with its prompt label.
func presignLabel(d time.Duration) string {
	for _, choice := range presignExpirations {
		if choice.expires == d {
			return choice.label
		}
	}
	return d.String()
}

// startDeletePrefix runs Session.DeletePrefix on a command goroutine.
func (v *ObjectView) startDeletePrefix(obj s3.Object, total int64) (tea.Model, tea.Cmd) {
	ctx, wait := v.transfer.begin(v.deps, v.deps.T("Deleting %s", obj.Key), total)
	broker := v.transfer.broker

	session, bucket, prefix := v.deps.Session, v.bucket, obj.Key
	return v, tea.Batch(
		func() tea.Msg {
			result, err := session.DeletePrefix(ctx, bucket, prefix, s3.DeleteOptions{
				Progress: broker.callback,
			})
			if err == nil && len(result.Failed) > 0 {
				err = &s3.BulkError{Errors: result.Failed}
			}
			summary := v.deps.T("Deleted %d objects under %s", len(result.Deleted), prefix)
			return transferDoneMsg{err: err, summary: summary, broker: broker}
		},
		wait,
	)
}
