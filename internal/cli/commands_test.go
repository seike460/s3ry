package cli

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

// newFakeS3Server starts an in-memory S3 endpoint for subcommand tests.
func newFakeS3Server(t *testing.T) (*httptest.Server, *s3mem.Backend) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_CONFIG_FILE", "/dev/null")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("AWS_ENDPOINT_URL_S3", "")

	backend := s3mem.New()
	server := httptest.NewServer(gofakes3.New(backend).Server())
	t.Cleanup(server.Close)
	return server, backend
}

// seedObject stores one object in the fake backend.
func seedObject(t *testing.T, backend *s3mem.Backend, bucket, key, body string) {
	t.Helper()
	if _, err := backend.PutObject(bucket, key, nil, strings.NewReader(body), int64(len(body)), nil); err != nil {
		t.Fatalf("PutObject %q/%q: %v", bucket, key, err)
	}
}

// runCommand invokes the CLI against the fake endpoint.
func runCommand(t *testing.T, server *httptest.Server, args ...string) (int, string, string) {
	t.Helper()
	args = append(args,
		"--endpoint", server.URL,
		"--path-style",
		"--region", "us-east-1",
	)
	var out, errOut strings.Builder
	code := Run(context.Background(), args, strings.NewReader(""), &out, &errOut, BuildInfo{}, RunDeps{})
	return code, out.String(), errOut.String()
}

func TestLsListsBuckets(t *testing.T) {
	server, backend := newFakeS3Server(t)
	if err := backend.CreateBucket("photos"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	code, out, errOut := runCommand(t, server, "ls")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "photos") {
		t.Fatalf("out = %q, want the bucket name", out)
	}
}

func TestLsListsObjects(t *testing.T) {
	server, backend := newFakeS3Server(t)
	if err := backend.CreateBucket("bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	seedObject(t, backend, "bucket", "a.txt", "a")
	seedObject(t, backend, "bucket", "dir/b.txt", "bucket")

	code, out, errOut := runCommand(t, server, "ls", "s3://bucket")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	for _, key := range []string{"a.txt", "dir/b.txt"} {
		if !strings.Contains(out, key) {
			t.Fatalf("out = %q, want %q", out, key)
		}
	}
}

func TestLsJSONOutput(t *testing.T) {
	server, backend := newFakeS3Server(t)
	if err := backend.CreateBucket("bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	seedObject(t, backend, "bucket", "a.txt", "hello")

	code, out, errOut := runCommand(t, server, "ls", "s3://bucket", "-o", "json")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	var rows []objectJSON
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("json.Unmarshal: %v (out = %q)", err, out)
	}
	if len(rows) != 1 || rows[0].Key != "a.txt" || rows[0].Size != 5 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestCatPrintsObject(t *testing.T) {
	server, backend := newFakeS3Server(t)
	if err := backend.CreateBucket("bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	seedObject(t, backend, "bucket", "a.txt", "file-body")

	code, out, errOut := runCommand(t, server, "cat", "s3://bucket/a.txt")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if out != "file-body" {
		t.Fatalf("out = %q, want file-body", out)
	}
}

func TestCatRejectsPrefixURL(t *testing.T) {
	server, _ := newFakeS3Server(t)
	code, _, _ := runCommand(t, server, "cat", "s3://bucket/dir/")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (usage)", code)
	}
}

func TestRmDryRunKeepsObject(t *testing.T) {
	server, backend := newFakeS3Server(t)
	if err := backend.CreateBucket("bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	seedObject(t, backend, "bucket", "a.txt", "a")

	code, out, errOut := runCommand(t, server, "rm", "s3://bucket/a.txt", "--dry-run")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "Would delete 1") {
		t.Fatalf("out = %q, want the dry-run summary", out)
	}
	if exists, _ := backend.BucketExists("bucket"); !exists {
		t.Fatal("bucket vanished")
	}
	if _, err := backend.HeadObject("bucket", "a.txt"); err != nil {
		t.Fatalf("dry-run deleted the object: %v", err)
	}
}

func TestRmDeletesObject(t *testing.T) {
	server, backend := newFakeS3Server(t)
	if err := backend.CreateBucket("bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	seedObject(t, backend, "bucket", "a.txt", "a")

	code, out, errOut := runCommand(t, server, "rm", "s3://bucket/a.txt")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "Deleted 1") {
		t.Fatalf("out = %q, want the delete summary", out)
	}
	if _, err := backend.HeadObject("bucket", "a.txt"); err == nil {
		t.Fatal("object still exists after rm")
	}
}

func TestRmPrefixDeletesAll(t *testing.T) {
	server, backend := newFakeS3Server(t)
	if err := backend.CreateBucket("bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	seedObject(t, backend, "bucket", "dir/a.txt", "a")
	seedObject(t, backend, "bucket", "dir/b.txt", "bucket")
	seedObject(t, backend, "bucket", "keep.txt", "k")

	code, out, errOut := runCommand(t, server, "rm", "s3://bucket/dir/")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "Deleted 2") {
		t.Fatalf("out = %q, want 2 deleted", out)
	}
	if _, err := backend.HeadObject("bucket", "keep.txt"); err != nil {
		t.Fatalf("unrelated object was deleted: %v", err)
	}
}

func TestRmRequiresKey(t *testing.T) {
	server, _ := newFakeS3Server(t)
	code, _, _ := runCommand(t, server, "rm", "s3://bucket")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (usage)", code)
	}
}

func TestPresignPrintsURL(t *testing.T) {
	server, backend := newFakeS3Server(t)
	if err := backend.CreateBucket("bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	seedObject(t, backend, "bucket", "a.txt", "a")

	code, out, errOut := runCommand(t, server, "presign", "s3://bucket/a.txt", "--expires", "2h")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "X-Amz-Expires=7200") {
		t.Fatalf("out = %q, want a 2h presigned URL", out)
	}
}

func TestLsRejectsUnknownFormat(t *testing.T) {
	server, _ := newFakeS3Server(t)
	code, _, _ := runCommand(t, server, "ls", "-o", "xml")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (usage error)", code)
	}
}
