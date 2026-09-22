package s3

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

func TestValidateBucketName(t *testing.T) {
	valid63 := strings.Repeat("a", 63)
	tests := []struct {
		name    string
		bucket  string
		wantErr bool
	}{
		{name: "minimum length", bucket: "abc"},
		{name: "normal", bucket: "my-bucket.1"},
		{name: "maximum length", bucket: valid63},
		{name: "empty", bucket: "", wantErr: true},
		{name: "too short", bucket: "ab", wantErr: true},
		{name: "too long", bucket: strings.Repeat("a", 64), wantErr: true},
		{name: "uppercase", bucket: "My-bucket", wantErr: true},
		{name: "underscore", bucket: "my_bucket", wantErr: true},
		{name: "leading dot", bucket: ".my-bucket", wantErr: true},
		{name: "trailing hyphen", bucket: "my-bucket-", wantErr: true},
		{name: "consecutive dots", bucket: "my..bucket", wantErr: true},
		{name: "IPv4", bucket: "192.168.0.1", wantErr: true},
		{name: "xn prefix", bucket: "xn--bucket", wantErr: true},
		{name: "alias suffix", bucket: "my-s3alias", wantErr: true},
		{name: "ol suffix", bucket: "my--ol-s3", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBucketName(tt.bucket)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("ValidateBucketName(%q) = %v", tt.bucket, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateBucketName(%q) error = nil", tt.bucket)
			}
			var typed *Error
			if !errors.As(err, &typed) || typed.Kind != KindInvalid || typed.Op != "validate" || typed.Bucket != tt.bucket {
				t.Fatalf("ValidateBucketName(%q) = %#v, want invalid validate error", tt.bucket, err)
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("ValidateBucketName(%q) does not match ErrInvalid", tt.bucket)
			}
		})
	}
}

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "empty", key: "", wantErr: true},
		{name: "1025 bytes", key: strings.Repeat("a", 1025), wantErr: true},
		{name: "invalid UTF-8", key: string([]byte{0xff, 0xfe}), wantErr: true},
		{name: "NUL", key: "a\x00b", wantErr: true},
		{name: "normal UTF-8", key: "dir/日本語.txt"},
		{name: "1024 bytes", key: strings.Repeat("a", 1024)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("ValidateKey(%q) = %v", tt.key, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateKey(%q) error = nil", tt.key)
			}
			var typed *Error
			if !errors.As(err, &typed) || typed.Kind != KindInvalid || typed.Op != "validate" || typed.Key != tt.key {
				t.Fatalf("ValidateKey(%q) = %#v, want invalid validate error", tt.key, err)
			}
		})
	}
}

func TestLocalPath(t *testing.T) {
	destDir := t.TempDir()
	want := func(parts ...string) string { return filepath.Join(append([]string{destDir}, parts...)...) }
	tests := []struct {
		name    string
		prefix  string
		key     string
		windows bool
		want    string
		wantErr bool
	}{
		{name: "parent", key: "../x", wantErr: true},
		{name: "dotdot-like segment", key: "..backup/x", want: "..backup/x"},
		{name: "nested parent", key: "a/../../x", wantErr: true},
		{name: "dot segment", key: "a/./b", wantErr: true},
		{name: "empty segment", key: "a//b", want: "a/b"},
		{name: "leading slash", key: "/etc/passwd", want: "etc/passwd"},
		{name: "prefix a", prefix: "p/", key: "p/a/x.txt", want: "a/x.txt"},
		{name: "prefix b", prefix: "p/", key: "p/b/x.txt", want: "b/x.txt"},
		{name: "prefix normalized", prefix: "p", key: "p/a/x.txt", want: "a/x.txt"},
		{name: "folder marker", prefix: "p/", key: "p/", wantErr: true},
		{name: "prefix mismatch", prefix: "p/", key: "q/x", wantErr: true},
		{name: "windows separator", key: "a\\b", windows: true, wantErr: true},
		{name: "windows volume", key: "a:b", windows: true, wantErr: true},
		{name: "root only", key: "/", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := localPath(destDir, tt.prefix, tt.key, tt.windows)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("localPath(%q, %q) error = nil", tt.prefix, tt.key)
				}
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("localPath(%q, %q) error = %v, want invalid", tt.prefix, tt.key, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("localPath(%q, %q) error = %v", tt.prefix, tt.key, err)
			}
			if got != want(tt.want) {
				t.Fatalf("localPath(%q, %q) = %q, want %q", tt.prefix, tt.key, got, want(tt.want))
			}
		})
	}

	if got, err := LocalPath(destDir, "p/", "p/a/x.txt"); err != nil || got != want("a", "x.txt") {
		t.Fatalf("LocalPath() = %q, %v, want %q", got, err, want("a", "x.txt"))
	}
	if got, err := localPath(destDir, "", "safe/file", false); err != nil || got != want("safe", "file") {
		t.Fatalf("safe localPath() = %q, %v", got, err)
	}
}

func TestLocalPathPreservesValidationError(t *testing.T) {
	_, err := localPath(t.TempDir(), "", "", false)
	if err == nil {
		t.Fatal("localPath() error = nil, want validation error")
	}
	typed, ok := err.(*Error)
	if !ok || typed.Op != "validate" {
		t.Fatalf("localPath() error = %#v, want the original validate error", err)
	}
}

func TestDetectContentType(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "data.json")
	pngPath := filepath.Join(dir, "image.png")
	noExtension := filepath.Join(dir, "image")
	emptyPath := filepath.Join(dir, "empty")
	if err := os.WriteFile(noExtension, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(emptyPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "json extension", path: jsonPath, want: "application/json"},
		{name: "png extension", path: pngPath, want: "image/png"},
		{name: "PNG magic", path: noExtension, want: "image/png"},
		{name: "empty file", path: emptyPath, want: "application/octet-stream"},
		{name: "missing file", path: filepath.Join(dir, "missing"), want: "application/octet-stream"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectContentType(tt.path); got != tt.want {
				t.Fatalf("DetectContentType(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestObjectSize(t *testing.T) {
	if size := unsafe.Sizeof(Object{}); size > 96 {
		t.Fatalf("unsafe.Sizeof(Object{}) = %d, want <= 96", size)
	}
}
