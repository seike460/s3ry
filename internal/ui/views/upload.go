package views

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
)

// FilesLoadedMsg carries the result of a local directory scan.
type FilesLoadedMsg struct {
	Files []FileInfo
	Err   error
}

// FileInfo describes one local file offered for upload.
type FileInfo struct {
	Path         string
	RelativePath string
	Size         int64
	ModTime      time.Time
}

// UploadView is the local file picker that uploads to the bucket root.
type UploadView struct {
	deps     Deps
	bucket   string
	state    listState
	transfer transferState
}

// NewUploadView creates a new upload view.
func NewUploadView(deps Deps, bucket string) *UploadView {
	return &UploadView{
		deps:   deps,
		bucket: bucket,
		state:  newListState(T("Scanning local files...")),
	}
}

// Init starts the spinner and the first scan.
func (v *UploadView) Init() tea.Cmd {
	return tea.Batch(v.state.spinner.Start(), v.loadFiles())
}

// Update handles messages for the upload view.
func (v *UploadView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if v.state.list != nil {
			v.state.list, _ = v.state.list.Update(msg)
		}
		v.transfer.resize(msg)

	case FilesLoadedMsg:
		return v.onFilesLoaded(msg)

	case transferProgressMsg:
		if cmd := v.transfer.onProgress(msg, progressMessage(msg.event)); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case transferDoneMsg:
		if cmd := v.transfer.finish(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case backToOperationMsg:
		if v.transfer.backToOperation(msg) {
			return NewOperationView(v.deps, v.bucket), nil
		}

	case brokerClosedMsg:

	case components.SpinnerTickMsg:
		cmds = append(cmds, v.state.onTick(msg))

	case tea.KeyMsg:
		return v.onKey(msg)
	}

	return v, tea.Batch(cmds...)
}

func (v *UploadView) onFilesLoaded(msg FilesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		v.state.fail(T("Error Scanning Files"), T("Failed to scan local files"), msg.Err)
		return v, nil
	}

	items := make([]components.ListItem, 0, len(msg.Files))
	for _, file := range msg.Files {
		items = append(items, components.ListItem{
			Title: file.RelativePath,
			Description: fmt.Sprintf("%s %s | %s %s",
				T("Size:"), components.FormatBytes(file.Size),
				T("Modified:"), formatModified(file.ModTime)),
			Tag:  "File",
			Data: file,
		})
	}
	v.state.loaded(T("Select File to Upload"), items)
	return v, nil
}

func (v *UploadView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if v.transfer.active {
		switch key {
		case "ctrl+c", "q":
			v.transfer.abort()
			return v, tea.Quit
		case "esc":
			v.transfer.cancelTransfer()
		}
		return v, nil
	}

	if v.state.loading {
		if key == "ctrl+c" || key == "q" {
			v.transfer.abort()
			return v, tea.Quit
		}
		return v, nil
	}

	if v.state.retryRequested(key) {
		return v, v.state.startLoading(T("Retrying to scan local files..."), v.loadFiles())
	}

	switch key {
	case "ctrl+c", "q":
		v.transfer.abort()
		return v, tea.Quit
	case "esc":
		return NewOperationView(v.deps, v.bucket), nil
	case "enter", " ":
		item := v.state.currentItem()
		if item == nil {
			break
		}
		file, ok := item.Data.(FileInfo)
		if !ok {
			break
		}
		return v.startUpload(file)
	}

	if v.state.list != nil {
		v.state.list, _ = v.state.list.Update(msg)
	}
	return v, nil
}

// startUpload runs Session.Upload on a command goroutine. The S3 key is the
// file's slash-separated path relative to the working directory.
func (v *UploadView) startUpload(file FileInfo) (tea.Model, tea.Cmd) {
	ctx, wait := v.transfer.begin(T("Uploading %s", file.RelativePath), file.Size)
	broker := v.transfer.broker

	session, bucket := v.deps.Session, v.bucket
	key := filepath.ToSlash(file.RelativePath)
	localPath := file.Path
	return v, tea.Batch(
		func() tea.Msg {
			err := session.Upload(ctx, localPath, bucket, key, s3.UploadOptions{
				Progress: broker.callback,
			})
			return transferDoneMsg{
				err:     err,
				summary: T("Uploaded %s (%s)", file.RelativePath, components.FormatBytes(file.Size)),
				broker:  broker,
			}
		},
		wait,
	)
}

// View renders the upload view.
func (v *UploadView) View() string {
	if v.transfer.active && v.transfer.progress != nil {
		return v.transfer.progress.View()
	}

	if v.state.loading {
		return headerStyle.Render(T("Local Files")) + "\n\n" + v.state.spinner.View()
	}

	if v.state.list == nil {
		return errorStyle.Render(T("Failed to scan local files"))
	}

	context := contextStyle.Render(fmt.Sprintf("%s %s | %s %s",
		T("Region:"), v.deps.region(), T("Bucket:"), v.bucket))
	footer := footerStyle.Render(T("↑↓: navigate • enter: select • r: refresh • esc: back • q: quit"))

	result := context + "\n\n" + v.state.list.View()
	if v.state.errors.GetErrorCount() > 0 {
		result += "\n\n" + v.state.errors.View()
	}
	return result + "\n\n" + footer
}

// AbortTransfer cancels any in-flight transfer and releases the progress
// broker. The app calls it when replacing the view or quitting.
func (v *UploadView) AbortTransfer() {
	v.transfer.abort()
}

// loadFiles walks the working directory and lists non-hidden regular files.
// Hidden entries and symlinks are skipped; directories are not selectable.
func (v *UploadView) loadFiles() tea.Cmd {
	return func() tea.Msg {
		var files []FileInfo
		err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			base := filepath.Base(path)
			if path != "." && strings.HasPrefix(base, ".") {
				if entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if entry.IsDir() || !entry.Type().IsRegular() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			files = append(files, FileInfo{
				Path:         path,
				RelativePath: path,
				Size:         info.Size(),
				ModTime:      info.ModTime(),
			})
			return nil
		})
		return FilesLoadedMsg{Files: files, Err: err}
	}
}
