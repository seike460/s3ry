package s3

import (
	"errors"
	"testing"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		want       URL
		wantErr    bool
		wantOp     string
		wantPrefix bool
	}{
		{name: "bucket", input: "s3://bucket", want: URL{Bucket: "bucket"}, wantPrefix: true},
		{name: "bucket slash", input: "s3://bucket/", want: URL{Bucket: "bucket"}, wantPrefix: true},
		{name: "object", input: "s3://bucket/a/b", want: URL{Bucket: "bucket", Key: "a/b"}},
		{name: "escaped key", input: "s3://bucket/a%20b", want: URL{Bucket: "bucket", Key: "a%20b"}},
		{name: "query in key", input: "s3://bucket/report?v=1", want: URL{Bucket: "bucket", Key: "report?v=1"}},
		{name: "fragment in key", input: "s3://bucket/notes#1", want: URL{Bucket: "bucket", Key: "notes#1"}},
		{name: "percent in key", input: "s3://bucket/50%off", want: URL{Bucket: "bucket", Key: "50%off"}},
		{name: "trailing prefix", input: "s3://bucket/a/b/", want: URL{Bucket: "bucket", Key: "a/b/"}, wantPrefix: true},
		{name: "missing scheme", input: "bucket/key", wantErr: true, wantOp: "parse"},
		{name: "empty URL", input: "", wantErr: true, wantOp: "parse"},
		{name: "empty bucket", input: "s3:///key", wantErr: true, wantOp: "parse"},
		{name: "short bucket", input: "s3://ab/key", wantErr: true, wantOp: "parse"},
		{name: "uppercase bucket", input: "s3://Bucket/key", wantErr: true, wantOp: "parse"},
		{name: "wrong scheme", input: "http://bucket/key", wantErr: true, wantOp: "parse"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseURL(%q) error = nil, want error", tt.input)
				}
				var typed *Error
				if !errors.As(err, &typed) {
					t.Fatalf("ParseURL(%q) error type = %T, want *Error", tt.input, err)
				}
				if typed.Kind != KindInvalid || typed.Op != tt.wantOp {
					t.Fatalf("ParseURL(%q) error = %#v, want invalid parse error", tt.input, typed)
				}
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("ParseURL(%q) error is not ErrInvalid", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseURL(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseURL(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
			if got.IsPrefix() != tt.wantPrefix {
				t.Fatalf("ParseURL(%q).IsPrefix() = %v, want %v", tt.input, got.IsPrefix(), tt.wantPrefix)
			}
		})
	}
}

func TestURLStringAndIsPrefix(t *testing.T) {
	tests := []struct {
		name       string
		url        URL
		wantString string
		wantPrefix bool
	}{
		{name: "bucket", url: URL{Bucket: "bucket"}, wantString: "s3://bucket", wantPrefix: true},
		{name: "object", url: URL{Bucket: "bucket", Key: "a/b"}, wantString: "s3://bucket/a/b"},
		{name: "trailing prefix", url: URL{Bucket: "bucket", Key: "a/"}, wantString: "s3://bucket/a/", wantPrefix: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.url.String(); got != tt.wantString {
				t.Fatalf("URL.String() = %q, want %q", got, tt.wantString)
			}
			if got := tt.url.IsPrefix(); got != tt.wantPrefix {
				t.Fatalf("URL.IsPrefix() = %v, want %v", got, tt.wantPrefix)
			}
		})
	}
}

func TestTypesAndConstants(t *testing.T) {
	if MaxDeleteBatch != 1000 {
		t.Fatalf("MaxDeleteBatch = %d, want 1000", MaxDeleteBatch)
	}
	if OpUpload != 1 || OpDownload != 2 || OpDelete != 3 {
		t.Fatalf("unexpected operation values: %d, %d, %d", OpUpload, OpDownload, OpDelete)
	}
	if OverwriteFail != 0 || OverwriteSkip != 1 || OverwriteAlways != 2 {
		t.Fatalf("unexpected overwrite values: %d, %d, %d", OverwriteFail, OverwriteSkip, OverwriteAlways)
	}
	if got := (Object{Key: "key", Size: 1}).Key; got != "key" {
		t.Fatalf("Object field access = %q, want key", got)
	}
	if got := (Bucket{Name: "bucket", Region: ""}).Region; got != "" {
		t.Fatalf("unknown bucket region = %q, want empty", got)
	}
	progress := Progress{Op: OpUpload, Total: -1, Done: true}
	if progress.Total != -1 || !progress.Done {
		t.Fatalf("unexpected progress value: %#v", progress)
	}
	var callback ProgressFunc = func(Progress) {}
	callback(progress)
	info := ObjectInfo{Object: Object{Key: "key"}, Metadata: map[string]string{"x": "y"}}
	if info.Key != "key" || info.Metadata["x"] != "y" {
		t.Fatalf("unexpected object info: %#v", info)
	}
	page := Page{Bucket: "bucket", Prefix: "p/", Prefixes: []string{"p/a/"}, Objects: []Object{info.Object}, NextToken: "next", IsTruncated: true}
	if page.Bucket != "bucket" || !page.IsTruncated || len(page.Objects) != 1 {
		t.Fatalf("unexpected page: %#v", page)
	}
}
