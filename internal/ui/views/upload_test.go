package views

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUploadLoadFilesSkipsHiddenEntries(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"regular-one.txt": "one",
		"regular-two.txt": "two",
		".hidden":         "hidden",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("make subdirectory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "nested.txt"), []byte("nested"), 0644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	t.Chdir(dir)

	msg, ok := (&UploadView{}).loadFiles()().(FilesLoadedMsg)
	if !ok {
		t.Fatal("loadFiles returned an unexpected message type")
	}
	if msg.Error != nil {
		t.Fatalf("loadFiles returned an error: %v", msg.Error)
	}

	candidates := make(map[string]bool)
	for _, file := range msg.Files {
		if !file.IsDir {
			candidates[file.RelativePath] = true
		}
	}

	want := []string{
		"regular-one.txt",
		"regular-two.txt",
		filepath.Join("subdir", "nested.txt"),
	}
	if len(candidates) != len(want) {
		t.Fatalf("got %d candidate files, want %d: %v", len(candidates), len(want), candidates)
	}
	for _, name := range want {
		if !candidates[name] {
			t.Errorf("candidate %q is missing from %v", name, candidates)
		}
	}
	if candidates[".hidden"] {
		t.Error("hidden file was included in upload candidates")
	}
}
