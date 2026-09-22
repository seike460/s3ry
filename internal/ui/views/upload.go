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
		state:  newListState(deps.T("Scanning local files...")),
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
	case tea.WindowSizeMsg, transferProgressMsg, transferDoneMsg, components.SpinnerTickMsg:
		if cmd := v.onEvent(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case FilesLoadedMsg:
		return v.onFilesLoaded(msg)

	case backToOperationMsg, brokerClosedMsg:
		if msg, ok := msg.(backToOperationMsg); ok && v.transfer.backToOperation(msg) {
			return NewOperationView(v.deps, v.bucket), nil
		}

	case tea.KeyMsg:
		return v.onKey(msg)
	}

	return v, tea.Batch(cmds...)
}

// onEvent routes resize, transfer events, and spinner ticks to their
// sub-components.
func (v *UploadView) onEvent(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.state.resize(msg)
		v.transfer.resize(msg)
		return nil
	case components.SpinnerTickMsg:
		return v.state.onTick(msg)
	default:
		return v.onTransfer(msg)
	}
}

// onTransfer feeds progress and completion events into the transfer state.
func (v *UploadView) onTransfer(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case transferProgressMsg:
		return v.transfer.onProgress(msg, progressMessage(msg.event))
	case transferDoneMsg:
		return v.transfer.finish(msg)
	}
	return nil
}

func (v *UploadView) onFilesLoaded(msg FilesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		v.state.fail(v.deps, v.deps.T("Error Scanning Files"), v.deps.T("Failed to scan local files"), msg.Err)
		return v, nil
	}

	items := make([]components.ListItem, 0, len(msg.Files))
	for _, file := range msg.Files {
		items = append(items, components.ListItem{
			Title: file.RelativePath,
			Description: fmt.Sprintf("%s %s | %s %s",
				v.deps.T("Size:"), components.FormatBytes(file.Size),
				v.deps.T("Modified:"), formatModified(file.ModTime)),
			Tag:  "File",
			Data: file,
		})
	}
	v.state.loaded(v.deps.T("Select File to Upload"), items)
	return v, nil
}

func (v *UploadView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

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
		return v, v.state.startLoading(v.deps.T("Retrying to scan local files..."), v.loadFiles())
	}

	return v.onReadyKey(msg)
}

// onReadyKey handles input on the file list.
func (v *UploadView) onReadyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if v.state.filterActive() {
		v.state.routeFilterKey(msg)
		return v, nil
	}
	if v.transfer.quitRequested(msg.String()) {
		return v, tea.Quit
	}
	switch msg.String() {
	case "esc":
		return NewOperationView(v.deps, v.bucket), nil
	case "enter", " ":
		if file := v.selectedFile(); file != nil {
			return v.startUpload(*file)
		}
	}

	if v.state.list != nil {
		v.state.list, _ = v.state.list.Update(msg)
	}
	return v, nil
}

// selectedFile returns the FileInfo under the cursor, if any.
func (v *UploadView) selectedFile() *FileInfo {
	item := v.state.currentItem()
	if item == nil {
		return nil
	}
	file, ok := item.Data.(FileInfo)
	if !ok {
		return nil
	}
	return &file
}

// startUpload runs Session.Upload on a command goroutine. The S3 key is the
// file's slash-separated path relative to the working directory.
func (v *UploadView) startUpload(file FileInfo) (tea.Model, tea.Cmd) {
	ctx, wait := v.transfer.begin(v.deps, v.deps.T("Uploading %s", file.RelativePath), file.Size)
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
				summary: v.deps.T("Uploaded %s (%s)", file.RelativePath, components.FormatBytes(file.Size)),
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
		return headerStyle.Render(v.deps.T("Local Files")) + "\n\n" + v.state.spinner.View()
	}

	if v.state.list == nil {
		return errorStyle.Render(v.deps.T("Failed to scan local files"))
	}

	context := contextStyle.Render(fmt.Sprintf("%s %s | %s %s",
		v.deps.T("Region:"), v.deps.region(), v.deps.T("Bucket:"), v.bucket))
	footer := footerStyle.Render(v.deps.T("↑↓: navigate • enter: select • r: refresh • esc: back • q: quit"))

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
