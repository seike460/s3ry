package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestDownloadPrefix(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)

	keys := []string{
		"src/a/x.txt",
		"src/b/x.txt",
		"src/a/one.txt",
		"src/a/two.txt",
		"src/b/one.txt",
		"src/b/two.txt",
		"src/deep/one/two.txt",
		"src/deep/three/four.txt",
		"src/root.txt",
		"src/root2.txt",
		"src/root3.txt",
		"src/empty/",
	}
	putObjects(t, client, transferTestBucket, keys...)

	destDir := t.TempDir()
	n, err := session.DownloadPrefix(t.Context(), transferTestBucket, "src/", destDir, BulkOptions{
		Parallel:  4,
		Overwrite: OverwriteAlways,
	})
	if err != nil {
		t.Fatalf("DownloadPrefix: %v", err)
	}
	if n != 11 {
		t.Fatalf("DownloadPrefix count = %d, want 11", n)
	}

	aPath, err := LocalPath(destDir, "src/", "src/a/x.txt")
	if err != nil {
		t.Fatalf("LocalPath a/x.txt: %v", err)
	}
	bPath, err := LocalPath(destDir, "src/", "src/b/x.txt")
	if err != nil {
		t.Fatalf("LocalPath b/x.txt: %v", err)
	}
	if aPath == bPath {
		t.Fatalf("a/x.txt and b/x.txt resolved to the same path %q", aPath)
	}
	if got, readErr := os.ReadFile(aPath); readErr != nil || string(got) != "src/a/x.txt" {
		t.Fatalf("a/x.txt = %q, read error = %v", got, readErr)
	}
	if got, readErr := os.ReadFile(bPath); readErr != nil || string(got) != "src/b/x.txt" {
		t.Fatalf("b/x.txt = %q, read error = %v", got, readErr)
	}

	if _, statErr := os.Lstat(filepath.Join(destDir, "empty")); !os.IsNotExist(statErr) {
		t.Fatalf("folder marker destination = %v, want file or directory absent", statErr)
	}
}

func TestDownloadPrefixTraversalContinueModes(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)

	goodKeys := []string{
		"src/a/x.txt",
		"src/b/x.txt",
		"src/a/one.txt",
		"src/a/two.txt",
		"src/b/one.txt",
		"src/b/two.txt",
		"src/deep/one/two.txt",
		"src/deep/three/four.txt",
		"src/root.txt",
		"src/root2.txt",
		"src/root3.txt",
	}
	putObjects(t, client, transferTestBucket, goodKeys...)
	putTransferTestObject(t, client, transferTestBucket, "src/../evil", []byte("evil"))

	continueDest := t.TempDir()
	continueOutside := filepath.Join(filepath.Dir(continueDest), "evil")
	n, err := session.DownloadPrefix(t.Context(), transferTestBucket, "src/", continueDest, BulkOptions{
		Parallel:        4,
		Overwrite:       OverwriteAlways,
		ContinueOnError: true,
	})
	if n != 11 {
		t.Fatalf("DownloadPrefix ContinueOnError count = %d, want 11 (err = %v)", n, err)
	}
	var bulkErr *BulkError
	if !errors.As(err, &bulkErr) {
		t.Fatalf("DownloadPrefix ContinueOnError error = %v, want *BulkError", err)
	}
	var invalidErr *Error
	foundInvalid := false
	for _, item := range bulkErr.Errors {
		if item.Key != "src/../evil" {
			continue
		}
		foundInvalid = true
		if !errors.As(item.Err, &invalidErr) || invalidErr.Kind != KindInvalid {
			t.Fatalf("traversal KeyError = %#v, want KindInvalid", item)
		}
	}
	if !foundInvalid {
		t.Fatalf("BulkError = %#v, want a KeyError for src/../evil", bulkErr.Errors)
	}
	if _, statErr := os.Lstat(continueOutside); !os.IsNotExist(statErr) {
		t.Fatalf("outside canary stat = %v, want absent", statErr)
	}

	failDest := t.TempDir()
	failOutside := filepath.Join(filepath.Dir(failDest), "evil")
	n, err = session.DownloadPrefix(t.Context(), transferTestBucket, "src/", failDest, BulkOptions{
		Parallel:  4,
		Overwrite: OverwriteAlways,
	})
	if err == nil {
		t.Fatal("DownloadPrefix without ContinueOnError error = nil, want traversal error")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("DownloadPrefix without ContinueOnError error = %v, want ErrInvalid", err)
	}
	if _, statErr := os.Lstat(failOutside); !os.IsNotExist(statErr) {
		t.Fatalf("outside fail canary stat = %v, want absent (n = %d)", statErr, n)
	}
}

func TestDownloadPrefixOverwriteSkip(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	putTransferTestObject(t, client, transferTestBucket, "src/keep.txt", []byte("remote keep"))
	putTransferTestObject(t, client, transferTestBucket, "src/new.txt", []byte("remote new"))

	destDir := t.TempDir()
	keepPath := filepath.Join(destDir, "keep.txt")
	if err := os.WriteFile(keepPath, []byte("local keep"), 0o644); err != nil {
		t.Fatalf("WriteFile existing destination: %v", err)
	}

	n, err := session.DownloadPrefix(t.Context(), transferTestBucket, "src/", destDir, BulkOptions{
		Parallel:  2,
		Overwrite: OverwriteSkip,
	})
	if err != nil {
		t.Fatalf("DownloadPrefix OverwriteSkip: %v", err)
	}
	if n != 1 {
		t.Fatalf("DownloadPrefix OverwriteSkip count = %d, want 1", n)
	}
	if got, readErr := os.ReadFile(keepPath); readErr != nil || string(got) != "local keep" {
		t.Fatalf("existing destination = %q, read error = %v", got, readErr)
	}
	if got, readErr := os.ReadFile(filepath.Join(destDir, "new.txt")); readErr != nil || string(got) != "remote new" {
		t.Fatalf("new destination = %q, read error = %v", got, readErr)
	}
}

func TestDownloadPrefixTransferContinueOnError(t *testing.T) {
	const deniedKey = "src/denied.txt"
	keys := []string{
		"src/one.txt",
		"src/two.txt",
		deniedKey,
		"src/three.txt",
	}
	session, client, _ := newCountingFake(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/"+deniedKey) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	for _, key := range keys {
		putTransferTestObject(t, client, transferTestBucket, key, []byte(key))
	}

	continueDest := t.TempDir()
	n, err := session.DownloadPrefix(t.Context(), transferTestBucket, "src/", continueDest, BulkOptions{
		Parallel:        2,
		Overwrite:       OverwriteAlways,
		ContinueOnError: true,
	})
	if n != len(keys)-1 {
		t.Fatalf("DownloadPrefix ContinueOnError count = %d, want %d (err = %v)", n, len(keys)-1, err)
	}
	var bulkErr *BulkError
	if !errors.As(err, &bulkErr) {
		t.Fatalf("DownloadPrefix ContinueOnError error = %v, want *BulkError", err)
	}
	if len(bulkErr.Errors) != 1 {
		t.Fatalf("DownloadPrefix ContinueOnError errors = %#v, want exactly one error", bulkErr.Errors)
	}
	item := bulkErr.Errors[0]
	if item.Key != deniedKey {
		t.Fatalf("DownloadPrefix failed key = %q, want %q", item.Key, deniedKey)
	}
	var deniedErr *Error
	if !errors.As(item.Err, &deniedErr) || deniedErr.Kind != KindAccessDenied {
		t.Fatalf("DownloadPrefix failed error = %#v, want KindAccessDenied", item.Err)
	}
	for _, key := range keys {
		if key == deniedKey {
			continue
		}
		path, pathErr := LocalPath(continueDest, "src/", key)
		if pathErr != nil {
			t.Fatalf("LocalPath %q: %v", key, pathErr)
		}
		if got, readErr := os.ReadFile(path); readErr != nil || string(got) != key {
			t.Fatalf("downloaded %q = %q, read error = %v", key, got, readErr)
		}
	}

	failDest := t.TempDir()
	n, err = session.DownloadPrefix(t.Context(), transferTestBucket, "src/", failDest, BulkOptions{
		Parallel:  2,
		Overwrite: OverwriteAlways,
	})
	if n >= len(keys) {
		t.Fatalf("DownloadPrefix fail-fast count = %d, want less than %d", n, len(keys))
	}
	var accessErr *Error
	if !errors.As(err, &accessErr) || accessErr.Kind != KindAccessDenied {
		t.Fatalf("DownloadPrefix fail-fast error = %v, want KindAccessDenied", err)
	}
	if errors.Is(err, ErrCanceled) {
		t.Fatalf("DownloadPrefix fail-fast error = %v, want access denied rather than canceled", err)
	}
}

func TestDownloadPrefixCancellation(t *testing.T) {
	setTransferProgressInterval(t, 0)
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)

	payload := bytes.Repeat([]byte("download-cancel"), 4096)
	keys := make([]string, 24)
	for i := range keys {
		keys[i] = filepath.ToSlash(filepath.Join("src", "object", string(rune('a'+i)), "file.bin"))
		putTransferTestObject(t, client, transferTestBucket, keys[i], payload)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var cancelOnce sync.Once
	n, err := session.DownloadPrefix(ctx, transferTestBucket, "src/", t.TempDir(), BulkOptions{
		Parallel: 4,
		Progress: func(event Progress) {
			if event.Done {
				cancelOnce.Do(cancel)
			}
		},
	})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("DownloadPrefix cancellation error = %v, want ErrCanceled", err)
	}
	if n >= len(keys) {
		t.Fatalf("DownloadPrefix cancellation count = %d, want less than %d", n, len(keys))
	}
}

func TestUploadDirRoundTrip(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)

	sourceDir := t.TempDir()
	files := map[string][]byte{
		"top.txt":                        []byte("top level"),
		"level1/level2/level3/data.json": []byte(`{"ok":true}`),
		"level1/level2/level3/data.bin":  []byte("binary data"),
		"level1/level2/another.txt":      []byte("another file"),
	}
	for relative, payload := range files {
		path := filepath.Join(sourceDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll %q: %v", path, err)
		}
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			t.Fatalf("WriteFile %q: %v", path, err)
		}
	}

	symlink := filepath.Join(sourceDir, "level1", "skip.txt")
	if err := os.Symlink(filepath.Join(sourceDir, "top.txt"), symlink); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	n, err := session.UploadDir(t.Context(), sourceDir, transferTestBucket, "uploaded", BulkOptions{
		Parallel: 4,
	})
	if err != nil {
		t.Fatalf("UploadDir: %v", err)
	}
	if n != len(files) {
		t.Fatalf("UploadDir count = %d, want %d", n, len(files))
	}

	wantKeys := make([]string, 0, len(files))
	for relative := range files {
		wantKeys = append(wantKeys, "uploaded/"+filepath.ToSlash(relative))
	}
	sort.Strings(wantKeys)
	gotKeys := listCopyTestKeys(t, client, transferTestBucket, "uploaded/")
	if !equalCopyTestStrings(gotKeys, wantKeys) {
		t.Fatalf("uploaded keys = %v, want %v", gotKeys, wantKeys)
	}

	jsonKey := "uploaded/level1/level2/level3/data.json"
	head, err := client.HeadObject(t.Context(), &awss3.HeadObjectInput{
		Bucket: aws.String(transferTestBucket),
		Key:    aws.String(jsonKey),
	})
	if err != nil {
		t.Fatalf("HeadObject %q: %v", jsonKey, err)
	}
	if got := aws.ToString(head.ContentType); got != "application/json" {
		t.Fatalf("JSON ContentType = %q, want application/json", got)
	}

	roundTripDir := t.TempDir()
	n, err = session.DownloadPrefix(t.Context(), transferTestBucket, "uploaded/", roundTripDir, BulkOptions{
		Parallel:  4,
		Overwrite: OverwriteAlways,
	})
	if err != nil {
		t.Fatalf("DownloadPrefix round trip: %v", err)
	}
	if n != len(files) {
		t.Fatalf("DownloadPrefix round trip count = %d, want %d", n, len(files))
	}
	for relative, want := range files {
		path := filepath.Join(roundTripDir, filepath.FromSlash(strings.TrimPrefix("uploaded/"+filepath.ToSlash(relative), "uploaded/")))
		if got, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("round-trip %q = %q, read error = %v, want %q", relative, got, readErr, want)
		}
	}
	if _, err := os.Lstat(filepath.Join(roundTripDir, "level1", "skip.txt")); !os.IsNotExist(err) {
		t.Fatalf("round-trip symlink stat = %v, want absent", err)
	}
}

func TestUploadDirRootSymlink(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)

	realDir := t.TempDir()
	files := map[string][]byte{
		"one.txt":         []byte("one"),
		"nested/two.json": []byte("two"),
	}
	for relative, payload := range files {
		path := filepath.Join(realDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll %q: %v", path, err)
		}
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			t.Fatalf("WriteFile %q: %v", path, err)
		}
	}
	insideSymlink := filepath.Join(realDir, "nested", "skip.txt")
	rootSymlink := filepath.Join(t.TempDir(), "source-link")
	if err := os.Symlink(filepath.Join(realDir, "one.txt"), insideSymlink); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if err := os.Symlink(realDir, rootSymlink); err != nil {
		t.Skipf("root symlink is unavailable: %v", err)
	}

	n, err := session.UploadDir(t.Context(), rootSymlink, transferTestBucket, "linked", BulkOptions{
		Parallel: 2,
	})
	if err != nil {
		t.Fatalf("UploadDir root symlink: %v", err)
	}
	if n != len(files) {
		t.Fatalf("UploadDir root symlink count = %d, want %d", n, len(files))
	}
	wantKeys := []string{"linked/nested/two.json", "linked/one.txt"}
	sort.Strings(wantKeys)
	gotKeys := listCopyTestKeys(t, client, transferTestBucket, "linked/")
	if !equalCopyTestStrings(gotKeys, wantKeys) {
		t.Fatalf("root symlink uploaded keys = %v, want %v", gotKeys, wantKeys)
	}
}

func TestUploadDirTransferContinueOnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks are ineffective for root")
	}

	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	sourceDir := t.TempDir()
	goodFiles := map[string][]byte{
		"one.txt": []byte("one"),
		"two.txt": []byte("two"),
	}
	for relative, payload := range goodFiles {
		writeTransferTestFile(t, filepath.Join(sourceDir, relative), payload)
	}
	deniedPath := filepath.Join(sourceDir, "denied.txt")
	writeTransferTestFile(t, deniedPath, []byte("denied"))
	if err := os.Chmod(deniedPath, 0); err != nil {
		t.Fatalf("Chmod %q: %v", deniedPath, err)
	}
	t.Cleanup(func() { _ = os.Chmod(deniedPath, 0o644) })

	n, err := session.UploadDir(t.Context(), sourceDir, transferTestBucket, "uploaded", BulkOptions{
		Parallel:        2,
		ContinueOnError: true,
	})
	if n != len(goodFiles) {
		t.Fatalf("UploadDir ContinueOnError count = %d, want %d (err = %v)", n, len(goodFiles), err)
	}
	var bulkErr *BulkError
	if !errors.As(err, &bulkErr) {
		t.Fatalf("UploadDir ContinueOnError error = %v, want *BulkError", err)
	}
	if len(bulkErr.Errors) != 1 {
		t.Fatalf("UploadDir ContinueOnError errors = %#v, want exactly one error", bulkErr.Errors)
	}
	item := bulkErr.Errors[0]
	if item.Key != "uploaded/denied.txt" {
		t.Fatalf("UploadDir failed key = %q, want uploaded/denied.txt", item.Key)
	}
	var deniedErr *Error
	if !errors.As(item.Err, &deniedErr) || deniedErr.Kind != KindAccessDenied {
		t.Fatalf("UploadDir failed error = %#v, want KindAccessDenied", item.Err)
	}
	wantKeys := []string{"uploaded/one.txt", "uploaded/two.txt"}
	sort.Strings(wantKeys)
	gotKeys := listCopyTestKeys(t, client, transferTestBucket, "uploaded/")
	if !equalCopyTestStrings(gotKeys, wantKeys) {
		t.Fatalf("UploadDir ContinueOnError keys = %v, want %v", gotKeys, wantKeys)
	}
}

func TestUploadDirCancellation(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	sourceDir := t.TempDir()
	const totalFiles = 4
	for i := 0; i < totalFiles; i++ {
		path := filepath.Join(sourceDir, fmt.Sprintf("file-%d.txt", i))
		writeTransferTestFile(t, path, []byte(path))
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var cancelOnce sync.Once
	n, err := session.UploadDir(ctx, sourceDir, transferTestBucket, "uploaded", BulkOptions{
		Parallel: 1,
		Progress: func(event Progress) {
			if event.Done {
				cancelOnce.Do(cancel)
			}
		},
	})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("UploadDir cancellation error = %v, want ErrCanceled", err)
	}
	if n >= totalFiles {
		t.Fatalf("UploadDir cancellation count = %d, want less than %d", n, totalFiles)
	}
}

func TestUploadDirProgressIsConcurrentSafe(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	sourceDir := t.TempDir()
	const totalFiles = 4
	for i := 0; i < totalFiles; i++ {
		path := filepath.Join(sourceDir, fmt.Sprintf("file-%d.txt", i))
		writeTransferTestFile(t, path, []byte(path))
	}

	var progressMu sync.Mutex
	done := 0
	n, err := session.UploadDir(t.Context(), sourceDir, transferTestBucket, "progress", BulkOptions{
		Parallel: totalFiles,
		Progress: func(event Progress) {
			if !event.Done {
				return
			}
			progressMu.Lock()
			done++
			progressMu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("UploadDir progress: %v", err)
	}
	if n != totalFiles {
		t.Fatalf("UploadDir progress count = %d, want %d", n, totalFiles)
	}
	progressMu.Lock()
	gotDone := done
	progressMu.Unlock()
	if gotDone != totalFiles {
		t.Fatalf("Progress Done events = %d, want %d", gotDone, totalFiles)
	}
}

func TestDownloadPrefixRequiresDirectory(t *testing.T) {
	var listCalls atomic.Int64
	session, _, _ := newCountingFake(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
				listCalls.Add(1)
			}
			next.ServeHTTP(w, r)
		})
	}, transferTestOptions)
	destination := filepath.Join(t.TempDir(), "destination")
	if err := os.WriteFile(destination, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("WriteFile destination: %v", err)
	}

	n, err := session.DownloadPrefix(t.Context(), transferTestBucket, "src/", destination, BulkOptions{})
	if n != 0 {
		t.Fatalf("DownloadPrefix file destination count = %d, want 0", n)
	}
	var invalidErr *Error
	if !errors.As(err, &invalidErr) || invalidErr.Kind != KindInvalid || invalidErr.Op != "download-prefix" {
		t.Fatalf("DownloadPrefix file destination error = %#v, want KindInvalid Op download-prefix", err)
	}
	if got := listCalls.Load(); got != 0 {
		t.Fatalf("DownloadPrefix list requests = %d, want 0", got)
	}
}

func TestUploadDirRequiresDirectory(t *testing.T) {
	session := &Session{}
	sourceFile := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(sourceFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	n, err := session.UploadDir(t.Context(), sourceFile, transferTestBucket, "prefix", BulkOptions{})
	if n != 0 {
		t.Fatalf("UploadDir file count = %d, want 0", n)
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("UploadDir file error = %v, want ErrInvalid", err)
	}
}

func listCopyTestKeys(t *testing.T, client *awss3.Client, bucket, prefix string) []string {
	t.Helper()
	out, err := client.ListObjectsV2(t.Context(), &awss3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 %q: %v", prefix, err)
	}
	keys := make([]string, 0, len(out.Contents))
	for _, object := range out.Contents {
		keys = append(keys, aws.ToString(object.Key))
	}
	sort.Strings(keys)
	return keys
}

func equalCopyTestStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
