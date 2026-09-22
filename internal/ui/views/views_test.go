package views

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
)

// fakeS3 serves a minimal XML API: ListBuckets on / and ListObjectsV2 on
// /<bucket>?list-type=2.
func fakeS3(t *testing.T, objects []s3.Object) (*s3.Session, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Buckets>
    <Bucket><Name>test-bucket</Name><BucketRegion>us-east-1</BucketRegion></Bucket>
  </Buckets>
</ListAllMyBucketsResult>`))
		case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
			var body strings.Builder
			body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><IsTruncated>false</IsTruncated>`)
			for _, obj := range objects {
				fmt.Fprintf(&body, "<Contents><Key>%s</Key><Size>%d</Size><LastModified>%s</LastModified><ETag>\"e\"</ETag></Contents>",
					obj.Key, obj.Size, obj.LastModified.UTC().Format(time.RFC3339))
			}
			body.WriteString("</ListBucketResult>")
			_, _ = w.Write([]byte(body.String()))
		case r.Method == http.MethodHead:
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut:
			w.Header().Set("ETag", `"e"`)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Query().Has("delete"):
			var body strings.Builder
			body.WriteString(`<?xml version="1.0" encoding="UTF-8"?><DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
			data, _ := io.ReadAll(r.Body)
			for _, m := range regexp.MustCompile(`<Key>([^<]+)</Key>`).FindAllStringSubmatch(string(data), -1) {
				fmt.Fprintf(&body, "<Deleted><Key>%s</Key></Deleted>", m[1])
			}
			body.WriteString("</DeleteResult>")
			_, _ = w.Write([]byte(body.String()))
		case r.Method == http.MethodGet && r.URL.Path != "/":
			body := []byte("hello world")
			var start, end int
			if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err == nil {
				if end == 0 || end >= len(body) {
					end = len(body) - 1
				}
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(body)))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(body[start : end+1])
				return
			}
			_, _ = w.Write(body)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	session, err := s3.NewSession(t.Context(), s3.Options{
		Region:        "us-east-1",
		EndpointURL:   server.URL,
		NoSignRequest: true,
		HTTPClient:    server.Client(),
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return session, server
}

func testDeps(t *testing.T, objects []s3.Object) Deps {
	t.Helper()
	session, _ := fakeS3(t, objects)
	return Deps{Session: session, Config: config.Default(), Timeout: 5 * time.Second}
}

func TestDepsSessionErr(t *testing.T) {
	if err := (Deps{}).sessionErr(); !errors.Is(err, errNoSession) {
		t.Fatalf("sessionErr = %v, want errNoSession", err)
	}
	if err := (Deps{Session: &s3.Session{}}).sessionErr(); err != nil {
		t.Fatalf("sessionErr with session = %v, want nil", err)
	}
}

func TestDepsRegion(t *testing.T) {
	if got := (Deps{}).region(); got != "-" {
		t.Fatalf("region without session = %q, want -", got)
	}
	deps := testDeps(t, nil)
	if got := deps.region(); got != "us-east-1" {
		t.Fatalf("region = %q, want us-east-1", got)
	}
}

func TestDepsListContext(t *testing.T) {
	ctx, cancel := (Deps{Timeout: time.Second}).listContext(context.Background())
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("listContext without timeout produced no deadline")
	}

	ctx, cancel = (Deps{}).listContext(context.Background())
	defer cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("listContext with zero timeout produced a deadline")
	}
}

func TestProgressBrokerDeliversDone(t *testing.T) {
	broker := newProgressBroker()
	defer broker.close()

	go func() {
		for i := 0; i < 100; i++ {
			broker.callback(s3.Progress{Transferred: int64(i)})
		}
		broker.callback(s3.Progress{Done: true})
	}()

	for i := 0; i < 200; i++ {
		msg := broker.wait()()
		progress, ok := msg.(transferProgressMsg)
		if !ok {
			t.Fatalf("wait returned %T, want transferProgressMsg", msg)
		}
		if progress.event.Done {
			return
		}
	}
	t.Fatal("Done event was never delivered")
}

func TestProgressBrokerCloseUnblocksWait(t *testing.T) {
	broker := newProgressBroker()
	broker.close()

	msg := broker.wait()()
	if _, ok := msg.(brokerClosedMsg); !ok {
		t.Fatalf("wait after close returned %T, want brokerClosedMsg", msg)
	}

	// callback after close must not block or panic.
	broker.callback(s3.Progress{Done: true})
}

func TestBucketViewListsBuckets(t *testing.T) {
	view := NewBucketView(testDeps(t, nil))

	msg, ok := view.loadBuckets()().(BucketsLoadedMsg)
	if !ok {
		t.Fatalf("loadBuckets returned %T", msg)
	}
	if msg.Err != nil {
		t.Fatalf("loadBuckets error: %v", msg.Err)
	}
	if len(msg.Buckets) != 1 || msg.Buckets[0].Name != "test-bucket" {
		t.Fatalf("buckets = %#v", msg.Buckets)
	}

	model, _ := view.Update(msg)
	updated := model.(*BucketView)
	if updated.state.loading {
		t.Fatal("view still loading after BucketsLoadedMsg")
	}
	if updated.state.list == nil {
		t.Fatal("list was not built")
	}
}

func TestBucketViewNoSession(t *testing.T) {
	view := NewBucketView(Deps{})

	msg, ok := view.loadBuckets()().(BucketsLoadedMsg)
	if !ok {
		t.Fatalf("loadBuckets returned %T", msg)
	}
	if !errors.Is(msg.Err, errNoSession) {
		t.Fatalf("loadBuckets error = %v, want errNoSession", msg.Err)
	}

	model, _ := view.Update(msg)
	updated := model.(*BucketView)
	if updated.state.loading || updated.state.list == nil {
		t.Fatal("error path did not stop loading and build an error list")
	}
}

func TestOperationViewNavigation(t *testing.T) {
	view := NewOperationView(testDeps(t, nil), "test-bucket")

	cases := []struct {
		key  string
		want string
	}{
		{"d", "*views.ObjectView"},
		{"u", "*views.UploadView"},
		{"delete", "*views.ObjectView"},
		{"?", "*views.HelpView"},
		{"s", "*views.SettingsView"},
		{"esc", "*views.BucketView"},
	}
	for _, tc := range cases {
		model, _ := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
		if got := fmt.Sprintf("%T", model); got != tc.want {
			t.Errorf("key %q navigated to %s, want %s", tc.key, got, tc.want)
		}
	}
}

func TestOperationViewEnterSelectsList(t *testing.T) {
	view := NewOperationView(testDeps(t, nil), "test-bucket")
	// Move the cursor to "Create object list" (index 3).
	for i := 0; i < 3; i++ {
		view.list, _ = view.list.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	model, _ := view.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := fmt.Sprintf("%T", model); got != "*views.ListGeneratorView" {
		t.Fatalf("enter on List = %s, want *views.ListGeneratorView", got)
	}
}

func TestObjectViewListsObjects(t *testing.T) {
	objects := []s3.Object{
		{Key: "a.txt", Size: 10, LastModified: time.Now()},
		{Key: "dir/b.txt", Size: 20, LastModified: time.Now()},
	}
	view := NewObjectView(testDeps(t, objects), "test-bucket", ModeDownload)

	msg, ok := view.loadObjects()().(ObjectsLoadedMsg)
	if !ok {
		t.Fatalf("loadObjects returned %T", msg)
	}
	if msg.Err != nil {
		t.Fatalf("loadObjects error: %v", msg.Err)
	}
	if len(msg.Objects) != 2 {
		t.Fatalf("objects = %#v", msg.Objects)
	}

	model, _ := view.Update(msg)
	updated := model.(*ObjectView)
	if updated.state.loading || updated.state.list == nil {
		t.Fatal("object list was not built")
	}
}

func TestObjectViewDeleteRequiresConfirmation(t *testing.T) {
	objects := []s3.Object{{Key: "a.txt", Size: 1, LastModified: time.Now()}}
	view := NewObjectView(testDeps(t, objects), "test-bucket", ModeDelete)
	model, _ := view.Update(ObjectsLoadedMsg{Objects: objects})
	view = model.(*ObjectView)

	// Select the first object: the view must not start deleting yet.
	model, _ = view.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view = model.(*ObjectView)
	if view.confirm == nil {
		t.Fatal("delete started without confirmation")
	}
	if view.transfer.active {
		t.Fatal("delete ran without a y answer")
	}

	// "n" cancels the prompt.
	model, _ = view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	view = model.(*ObjectView)
	if view.confirm != nil {
		t.Fatal("confirm prompt was not cleared by n")
	}
}

func TestObjectViewDownloadSkipsFolderMarker(t *testing.T) {
	folder := s3.Object{Key: "dir/", Size: 0, LastModified: time.Now()}
	view := NewObjectView(testDeps(t, nil), "test-bucket", ModeDownload)

	model, cmd := view.selectObject(folder)
	if cmd != nil {
		t.Fatal("folder marker selection started a transfer")
	}
	if model.(*ObjectView).transfer.active {
		t.Fatal("folder marker selection entered processing state")
	}
}

func TestUploadViewFilesLoaded(t *testing.T) {
	view := NewUploadView(testDeps(t, nil), "test-bucket")

	model, _ := view.Update(FilesLoadedMsg{Files: []FileInfo{
		{Path: "one.txt", RelativePath: "one.txt", Size: 5, ModTime: time.Now()},
	}})
	updated := model.(*UploadView)
	if updated.state.loading || updated.state.list == nil {
		t.Fatal("file list was not built")
	}
}

func TestUploadViewFilesLoadedError(t *testing.T) {
	view := NewUploadView(testDeps(t, nil), "test-bucket")

	model, _ := view.Update(FilesLoadedMsg{Err: errors.New("scan failed")})
	updated := model.(*UploadView)
	if updated.state.loading || updated.state.list == nil {
		t.Fatal("error path did not build the error list")
	}
	if updated.state.errors.GetErrorCount() == 0 {
		t.Fatal("error was not recorded")
	}
}

func TestHelpAndSettingsGoBack(t *testing.T) {
	deps := testDeps(t, nil)

	for name, view := range map[string]tea.Model{
		"help":     NewHelpView(deps),
		"settings": NewSettingsView(deps),
	} {
		model, _ := view.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if got := fmt.Sprintf("%T", model); got != "*views.BucketView" {
			t.Errorf("%s esc = %s, want *views.BucketView", name, got)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:        "0 B",
		512:      "512 B",
		1024:     "1.0 KB",
		1048576:  "1.0 MB",
		12345678: "11.8 MB",
	}
	for input, want := range cases {
		if got := components.FormatBytes(input); got != want {
			t.Errorf("components.FormatBytes(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestTruncateShort(t *testing.T) {
	if got := truncateShort("short", 10); got != "short" {
		t.Errorf("truncateShort passthrough = %q", got)
	}
	if got := truncateShort("abcdefghij", 7); got != "abcd..." {
		t.Errorf("truncateShort = %q, want abcd...", got)
	}
}

func TestProgressMessage(t *testing.T) {
	withTotal := progressMessage(s3.Progress{Transferred: 10, Total: 100})
	if !strings.Contains(withTotal, "/") {
		t.Errorf("progressMessage with total = %q", withTotal)
	}
	withoutTotal := progressMessage(s3.Progress{Transferred: 10, Total: -1})
	if strings.Contains(withoutTotal, "/") {
		t.Errorf("progressMessage without total = %q", withoutTotal)
	}
}

func TestListGeneratorWritesFile(t *testing.T) {
	t.Chdir(t.TempDir())
	objects := []s3.Object{
		{Key: "a.txt", Size: 5, LastModified: time.Now()},
		{Key: "dir/b.txt", Size: 7, LastModified: time.Now()},
	}
	view := NewListGeneratorView(testDeps(t, objects), "test-bucket")

	batch, ok := view.generateList()().(tea.BatchMsg)
	if !ok {
		t.Fatal("generateList did not return a batch")
	}
	var done transferDoneMsg
	for _, cmd := range batch {
		if msg := cmd(); msg != nil {
			if m, isDone := msg.(transferDoneMsg); isDone {
				done = m
			}
		}
	}
	if done.err != nil {
		t.Fatalf("generateList error: %v", done.err)
	}
	if !strings.Contains(done.summary, "2 objects") {
		t.Fatalf("summary = %q, want 2 objects", done.summary)
	}

	matches, err := filepath.Glob("ObjectList-*.txt")
	if err != nil || len(matches) != 1 {
		t.Fatalf("generated files = %v, err = %v", matches, err)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "./a.txt,5") || !strings.Contains(string(data), "./dir/b.txt,7") {
		t.Fatalf("file contents = %q", data)
	}
}

func TestListGeneratorNoSession(t *testing.T) {
	view := NewListGeneratorView(Deps{}, "test-bucket")
	msg, ok := view.generateList()().(transferDoneMsg)
	if !ok {
		t.Fatalf("generateList returned %T", msg)
	}
	if !errors.Is(msg.err, errNoSession) {
		t.Fatalf("error = %v, want errNoSession", msg.err)
	}
}

func TestListGeneratorCancelOnEsc(t *testing.T) {
	view := NewListGeneratorView(testDeps(t, nil), "test-bucket")
	canceled := false
	view.transfer.cancel = func() { canceled = true }
	model, _ := view.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model != view || !canceled {
		t.Fatal("esc during processing did not cancel")
	}
}

func TestTransferFinishDropsStaleBroker(t *testing.T) {
	view := NewListGeneratorView(testDeps(t, nil), "test-bucket")
	view.transfer.begin(testDeps(t, nil), "job", 1)
	defer view.transfer.abort()

	// A completion tagged with a foreign broker must be ignored.
	if cmd := view.transfer.finish(transferDoneMsg{broker: newProgressBroker()}); cmd != nil {
		t.Fatal("stale done message was not dropped")
	}
	if !view.transfer.active {
		t.Fatal("stale done message ended the active transfer")
	}
}

func TestBackToOperationRejectsStaleTick(t *testing.T) {
	view := NewListGeneratorView(testDeps(t, nil), "test-bucket")

	// A tick that does not name the last finished broker must not navigate.
	model, _ := view.Update(backToOperationMsg{broker: newProgressBroker()})
	if model != view {
		t.Fatal("stale tick navigated away")
	}
}

func TestBackToOperationAcceptsMatchingTick(t *testing.T) {
	view := NewListGeneratorView(testDeps(t, nil), "test-bucket")
	view.transfer.begin(testDeps(t, nil), "job", 1)
	finished := view.transfer.broker
	cmd := view.transfer.finish(transferDoneMsg{broker: finished})
	if cmd == nil {
		t.Fatal("legitimate done message was dropped")
	}
	if view.transfer.active {
		t.Fatal("transfer still active after finish")
	}
	model, _ := view.Update(backToOperationMsg{broker: finished})
	if got := fmt.Sprintf("%T", model); got != "*views.OperationView" {
		t.Fatalf("matching tick navigated to %s, want *views.OperationView", got)
	}
}

func TestTransferAbortReleasesBlockedDone(t *testing.T) {
	view := NewListGeneratorView(testDeps(t, nil), "test-bucket")
	view.transfer.begin(testDeps(t, nil), "job", 1)
	broker := view.transfer.broker

	// Saturate the channel so the final Done send blocks, then prove abort
	// releases it via broker.close.
	for i := 0; i < 64; i++ {
		broker.callback(s3.Progress{Transferred: int64(i)})
	}
	released := make(chan struct{})
	go func() {
		broker.callback(s3.Progress{Done: true})
		close(released)
	}()
	time.Sleep(20 * time.Millisecond)

	view.transfer.abort()
	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("abort did not release a blocked Done callback")
	}
	if view.transfer.active || view.transfer.broker != nil {
		t.Fatal("abort left the transfer marked active")
	}
}

func TestObjectViewQuitAbortsTransfer(t *testing.T) {
	view := NewObjectView(testDeps(t, nil), "test-bucket", ModeDownload)
	view.transfer.begin(testDeps(t, nil), "job", 1)
	broker := view.transfer.broker

	model, cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if model != view || cmd == nil {
		t.Fatal("q during transfer did not quit")
	}
	if view.transfer.broker != nil || view.transfer.active {
		t.Fatal("quit did not abort the transfer")
	}
	// The closed broker must answer waiters with brokerClosedMsg.
	if _, ok := broker.wait()().(brokerClosedMsg); !ok {
		t.Fatal("aborted broker did not close")
	}
}

func TestSettingsViewMasksSecrets(t *testing.T) {
	t.Setenv("AWS_SECRET_ACCESS_KEY", "AKIAEXAMPLESECRET123")
	view := NewSettingsView(Deps{Config: config.Default()})
	rendered := view.View()
	if strings.Contains(rendered, "AKIAEXAMPLESECRET123") {
		t.Fatal("secret access key leaked into settings view")
	}
	if !strings.Contains(rendered, "AKIA") {
		t.Fatal("masked prefix missing from settings view")
	}
}

func TestSettingsViewNilConfig(t *testing.T) {
	view := NewSettingsView(Deps{})
	if view.config == nil {
		t.Fatal("nil config was not replaced by defaults")
	}
	if view.View() == "" {
		t.Fatal("settings view rendered nothing")
	}
}

func TestObjectViewCurrentObject(t *testing.T) {
	view := NewObjectView(testDeps(t, nil), "test-bucket", ModeDownload)
	if got := view.currentObject(); got != nil {
		t.Fatalf("currentObject on empty list = %#v, want nil", got)
	}

	obj := s3.Object{Key: "a.txt", Size: 1, LastModified: time.Now()}
	model, _ := view.Update(ObjectsLoadedMsg{Objects: []s3.Object{obj}})
	view = model.(*ObjectView)

	got := view.currentObject()
	if got == nil || got.Key != "a.txt" {
		t.Fatalf("currentObject = %#v, want a.txt", got)
	}
}

func TestObjectViewAbortTransferIdle(t *testing.T) {
	view := NewObjectView(testDeps(t, nil), "test-bucket", ModeDownload)
	view.AbortTransfer()
	if view.transfer.active || view.transfer.broker != nil {
		t.Fatal("AbortTransfer left transfer state marked active")
	}
}

func TestObjectViewPreviewObject(t *testing.T) {
	view := NewObjectView(testDeps(t, nil), "test-bucket", ModeDownload)
	msg := view.previewObject(s3.Object{
		Key:          "a.txt",
		Size:         3,
		LastModified: time.Now(),
		ETag:         "0123456789abcdef",
	})()
	preview, ok := msg.(components.PreviewMsg)
	if !ok {
		t.Fatalf("previewObject returned %T, want components.PreviewMsg", msg)
	}
	if !strings.Contains(preview.Content, "a.txt") {
		t.Fatalf("preview content = %q, want object key included", preview.Content)
	}
}

func TestListStateOnTick(t *testing.T) {
	view := NewBucketView(testDeps(t, nil))
	if !view.state.loading {
		t.Fatal("new bucket view is not loading")
	}
	if cmd := view.state.onTick(components.SpinnerTickMsg{}); cmd == nil {
		t.Fatal("onTick while loading returned nil, want next tick command")
	}
	view.state.loading = false
	if cmd := view.state.onTick(components.SpinnerTickMsg{}); cmd != nil {
		t.Fatal("onTick while idle returned a command")
	}
}

func TestTransferStateResize(t *testing.T) {
	view := NewObjectView(testDeps(t, nil), "test-bucket", ModeDownload)
	// Without a progress widget the resize is a no-op.
	view.transfer.resize(tea.WindowSizeMsg{Width: 80, Height: 24})

	_, _ = view.transfer.begin(view.deps, "job", 10)
	view.transfer.resize(tea.WindowSizeMsg{Width: 40, Height: 10})
}

func TestObjectViewFolderDeleteUsesPrefix(t *testing.T) {
	objects := []s3.Object{
		{Key: "dir/"},
		{Key: "dir/a.txt", Size: 1, LastModified: time.Now()},
		{Key: "dir/b.txt", Size: 1, LastModified: time.Now()},
	}
	view := NewObjectView(testDeps(t, objects), "test-bucket", ModeDelete)

	// Selecting a folder marker opens the prefix prompt and starts the
	// dry-run count instead of deleting the marker alone.
	model, cmd := view.selectObject(s3.Object{Key: "dir/"})
	view = model.(*ObjectView)
	if view.confirm == nil || view.confirm.kind != confirmDeletePrefix {
		t.Fatalf("confirm = %+v, want a prefix-delete prompt", view.confirm)
	}
	if cmd == nil {
		t.Fatal("dry-run count command was not issued")
	}

	count, ok := cmd().(prefixCountMsg)
	if !ok {
		t.Fatalf("count command returned %T, want prefixCountMsg", cmd())
	}
	if count.err != nil {
		t.Fatalf("dry-run count failed: %v", count.err)
	}
	if count.count != 3 {
		t.Fatalf("count = %d, want 3", count.count)
	}

	model, _ = view.Update(count)
	view = model.(*ObjectView)
	if view.confirm.count != 3 {
		t.Fatalf("prompt count = %d, want 3", view.confirm.count)
	}

	// "y" starts the real prefix delete through the transfer pipeline.
	model, deleteCmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	view = model.(*ObjectView)
	defer view.AbortTransfer()
	if !view.transfer.active || deleteCmd == nil {
		t.Fatal("prefix delete did not start")
	}
}

func TestObjectViewFolderDeleteCancelKeepsMarker(t *testing.T) {
	view := NewObjectView(testDeps(t, nil), "test-bucket", ModeDelete)
	model, _ := view.selectObject(s3.Object{Key: "dir/"})
	view = model.(*ObjectView)

	model, _ = view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	view = model.(*ObjectView)
	if view.confirm != nil || view.transfer.active {
		t.Fatal("n did not cancel the prefix delete")
	}
}

func TestObjectViewPresignFlow(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	session, err := s3.NewSession(t.Context(), s3.Options{
		Region:      "us-east-1",
		EndpointURL: "http://localhost",
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	deps := Deps{Session: session, Config: config.Default(), Timeout: 5 * time.Second}
	view := NewObjectView(deps, "test-bucket", ModeDownload)

	model, _ := view.Update(ObjectsLoadedMsg{Objects: []s3.Object{
		{Key: "a.txt", Size: 1, LastModified: time.Now()},
	}})
	view = model.(*ObjectView)

	// "P" opens the expiry chooser for the object under the cursor.
	model, _ = view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	view = model.(*ObjectView)
	if view.confirm == nil || view.confirm.kind != confirmPresign {
		t.Fatalf("confirm = %+v, want a presign prompt", view.confirm)
	}

	// "2" picks the 24-hour expiry; the command signs locally.
	model, cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	view = model.(*ObjectView)
	if view.confirm != nil {
		t.Fatal("presign prompt was not cleared")
	}
	result, ok := cmd().(presignResultMsg)
	if !ok {
		t.Fatalf("presign command returned %T, want presignResultMsg", cmd())
	}
	if result.err != nil {
		t.Fatalf("PresignGet: %v", result.err)
	}
	if !strings.Contains(result.url, "X-Amz-Expires=86400") {
		t.Fatalf("url = %q, want 24h expiry", result.url)
	}

	model, _ = view.Update(result)
	view = model.(*ObjectView)
	if !strings.Contains(view.notice, result.url) {
		t.Fatalf("notice = %q, want the presigned URL", view.notice)
	}
}

func TestObjectViewPresignCancel(t *testing.T) {
	view := NewObjectView(testDeps(t, nil), "test-bucket", ModeDownload)
	view.confirm = &confirmPrompt{kind: confirmPresign, object: s3.Object{Key: "a.txt"}}

	model, _ := view.Update(tea.KeyMsg{Type: tea.KeyEscape})
	view = model.(*ObjectView)
	if view.confirm != nil {
		t.Fatal("esc did not cancel the presign prompt")
	}
}
