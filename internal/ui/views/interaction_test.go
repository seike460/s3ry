package views

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/s3"
)

// batchCmd unwraps a tea.Batch command so tests can run each inner command
// synchronously.
func batchCmd(t *testing.T, cmd tea.Cmd) tea.BatchMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command batch, got nil")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("command returned %T, want tea.BatchMsg", cmd())
	}
	return batch
}

func TestViewInitsReturnCommands(t *testing.T) {
	deps := testDeps(t, nil)
	views := map[string]tea.Model{
		"bucket":    NewBucketView(deps),
		"object":    NewObjectView(deps, "test-bucket", ModeDownload),
		"upload":    NewUploadView(deps, "test-bucket"),
		"operation": NewOperationView(deps, "test-bucket"),
		"help":      NewHelpView(deps),
		"settings":  NewSettingsView(deps),
		"listgen":   NewListGeneratorView(deps, "test-bucket"),
	}
	for name, view := range views {
		if cmd := view.Init(); cmd == nil {
			switch name {
			case "operation", "help", "settings":
				// Static views have no startup command.
			default:
				t.Errorf("%s.Init() returned nil", name)
			}
		}
		if view.View() == "" {
			t.Errorf("%s.View() rendered an empty frame", name)
		}
	}
}

func TestViewRenderingAfterLoad(t *testing.T) {
	deps := testDeps(t, nil)

	bucket := NewBucketView(deps)
	model, _ := bucket.Update(BucketsLoadedMsg{Buckets: []s3.Bucket{{Name: "b", Region: "us-east-1"}}})
	if out := model.(*BucketView).View(); !strings.Contains(out, "b") {
		t.Error("bucket view did not render the loaded bucket")
	}

	object := NewObjectView(deps, "test-bucket", ModeDownload)
	model, _ = object.Update(ObjectsLoadedMsg{Page: &s3.Page{Objects: []s3.Object{{Key: "k", LastModified: time.Now()}}}})
	if out := model.(*ObjectView).View(); !strings.Contains(out, "k") {
		t.Error("object view did not render the loaded object")
	}

	upload := NewUploadView(deps, "test-bucket")
	model, _ = upload.Update(FilesLoadedMsg{Files: []FileInfo{{Path: "f", RelativePath: "f", ModTime: time.Now()}}})
	if out := model.(*UploadView).View(); !strings.Contains(out, "f") {
		t.Error("upload view did not render the loaded file")
	}
}

func TestViewKeyNavigation(t *testing.T) {
	deps := testDeps(t, nil)

	key := func(s string) tea.KeyMsg {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}

	bucket := NewBucketView(deps)
	bucket.Update(BucketsLoadedMsg{Buckets: []s3.Bucket{{Name: "b"}}})
	if model, _ := bucket.Update(key("?")); model.(*HelpView) == nil {
		t.Error("? did not open help from the bucket view")
	}
	if model, _ := bucket.Update(key("s")); model.(*SettingsView) == nil {
		t.Error("s did not open settings from the bucket view")
	}
	if _, cmd := bucket.Update(key("q")); cmd == nil {
		t.Error("q did not produce a quit command")
	}

	object := NewObjectView(deps, "test-bucket", ModeDownload)
	object.Update(ObjectsLoadedMsg{Page: &s3.Page{Objects: []s3.Object{{Key: "k"}}}})
	if model, _ := object.Update(tea.KeyMsg{Type: tea.KeyEsc}); model.(*OperationView) == nil {
		t.Error("esc did not return to the operation view")
	}
	if model, _ := object.Update(key("?")); model.(*HelpView) == nil {
		t.Error("? did not open help from the object view")
	}

	upload := NewUploadView(deps, "test-bucket")
	upload.Update(FilesLoadedMsg{Files: []FileInfo{{Path: "f", RelativePath: "f"}}})
	if model, _ := upload.Update(tea.KeyMsg{Type: tea.KeyEsc}); model.(*OperationView) == nil {
		t.Error("esc did not return to the operation view")
	}
}

func TestBucketViewRetryAfterError(t *testing.T) {
	view := NewBucketView(testDeps(t, nil))
	model, _ := view.Update(BucketsLoadedMsg{Err: errTest})
	view = model.(*BucketView)
	if view.state.loading || view.state.list == nil {
		t.Fatal("error path did not show the retryable error list")
	}

	// r on the error list reloads.
	model, cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	view = model.(*BucketView)
	if !view.state.loading || cmd == nil {
		t.Fatal("r did not restart the load")
	}
}

func TestObjectViewDownloadEndToEnd(t *testing.T) {
	obj := s3.Object{Key: "hello.txt", Size: 11, LastModified: time.Now()}
	view := NewObjectView(testDeps(t, []s3.Object{obj}), "test-bucket", ModeDownload)
	t.Chdir(t.TempDir())

	model, cmd := view.startDownload(obj, s3.OverwriteAlways)
	batch := batchCmd(t, cmd)
	done, ok := batch[0]().(transferDoneMsg)
	if !ok {
		t.Fatalf("download command returned %T", batch[0]())
	}
	if done.err != nil {
		t.Fatalf("download failed: %v", done.err)
	}
	content, err := os.ReadFile("hello.txt")
	if err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}
	if string(content) != "hello world" {
		t.Fatalf("content = %q, want %q", content, "hello world")
	}

	// The completion message finishes the transfer and arms the return tick.
	updated, tick := model.(*ObjectView).Update(done)
	if updated.(*ObjectView).transfer.active {
		t.Fatal("transfer still active after completion")
	}
	if tick == nil {
		t.Fatal("completion did not schedule the return tick")
	}
}

func TestObjectViewDeleteEndToEnd(t *testing.T) {
	obj := s3.Object{Key: "victim.txt", Size: 1, LastModified: time.Now()}
	view := NewObjectView(testDeps(t, []s3.Object{obj}), "test-bucket", ModeDelete)

	model, _ := view.Update(ObjectsLoadedMsg{Page: &s3.Page{Objects: []s3.Object{obj}}})
	view = model.(*ObjectView)

	model, _ = view.selectObject(obj)
	view = model.(*ObjectView)
	if view.confirm == nil {
		t.Fatal("delete did not ask for confirmation")
	}

	model, cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	view = model.(*ObjectView)
	batch := batchCmd(t, cmd)
	done, ok := batch[0]().(transferDoneMsg)
	if !ok {
		t.Fatalf("delete command returned %T", batch[0]())
	}
	if done.err != nil {
		t.Fatalf("delete failed: %v", done.err)
	}
	if !view.transfer.active {
		t.Fatal("delete did not enter the transfer state")
	}
}

func TestUploadViewEndToEnd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "up.txt"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	view := NewUploadView(testDeps(t, nil), "test-bucket")
	file := FileInfo{Path: "up.txt", RelativePath: "up.txt", Size: 7, ModTime: time.Now()}

	model, cmd := view.startUpload(file)
	batch := batchCmd(t, cmd)
	done, ok := batch[0]().(transferDoneMsg)
	if !ok {
		t.Fatalf("upload command returned %T", batch[0]())
	}
	if done.err != nil {
		t.Fatalf("upload failed: %v", done.err)
	}
	if !model.(*UploadView).transfer.active {
		t.Fatal("upload did not enter the transfer state")
	}
}

func TestListGeneratorAbortAndKeys(t *testing.T) {
	view := NewListGeneratorView(testDeps(t, nil), "test-bucket")

	// esc while generating cancels instead of navigating.
	model, cmd := view.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("esc during generation produced a command")
	}
	view = model.(*ListGeneratorView)
	view.AbortTransfer()
	if view.transfer.active {
		t.Fatal("abort left the transfer active")
	}

	// esc after finishing returns to the operation view.
	model, _ = view.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model.(*OperationView) == nil {
		t.Error("esc after generation did not return to the operation view")
	}
}

func TestTransferProgressUpdatesWidget(t *testing.T) {
	view := NewUploadView(testDeps(t, nil), "test-bucket")
	_, _ = view.transfer.begin(testDeps(t, nil), "upload", 100)
	broker := view.transfer.broker

	model, cmd := view.Update(transferProgressMsg{
		event:  s3.Progress{Transferred: 40, Total: 100},
		broker: broker,
	})
	if cmd == nil {
		t.Fatal("progress update did not re-arm the broker wait")
	}
	view = model.(*UploadView)
	if view.transfer.progress == nil {
		t.Fatal("progress widget was not updated")
	}
	view.AbortTransfer()
}

var errTest = errTestError("test failure")

type errTestError string

func (e errTestError) Error() string { return string(e) }
