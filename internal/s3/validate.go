package s3

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// ValidateBucketName checks name against the S3 bucket naming rules.
func ValidateBucketName(name string) error {
	if len(name) < 3 || len(name) > 63 {
		return newInvalidError("validate", name, "", "bucket name must be between 3 and 63 characters")
	}
	if strings.HasPrefix(name, "xn--") {
		return newInvalidError("validate", name, "", "bucket name cannot start with xn--")
	}
	if strings.HasSuffix(name, "-s3alias") {
		return newInvalidError("validate", name, "", "bucket name cannot end with -s3alias")
	}
	if strings.HasSuffix(name, "--ol-s3") {
		return newInvalidError("validate", name, "", "bucket name cannot end with --ol-s3")
	}
	if strings.Contains(name, "..") {
		return newInvalidError("validate", name, "", "bucket name cannot contain consecutive dots")
	}
	if isIPv4DottedQuad(name) {
		return newInvalidError("validate", name, "", "bucket name cannot be an IPv4 address")
	}
	if !isLowerAlphaNumeric(name[0]) || !isLowerAlphaNumeric(name[len(name)-1]) {
		return newInvalidError("validate", name, "", "bucket name must start and end with a lowercase letter or digit")
	}
	for i := 0; i < len(name); i++ {
		if !isLowerAlphaNumeric(name[i]) && name[i] != '.' && name[i] != '-' {
			return newInvalidError("validate", name, "", "bucket name contains an invalid character")
		}
	}
	return nil
}

func isLowerAlphaNumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

func isIPv4DottedQuad(name string) bool {
	parts := strings.Split(name, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return false
		}
		value := 0
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return false
			}
			value = value*10 + int(part[i]-'0')
		}
		if value > 255 {
			return false
		}
	}
	return true
}

// ValidateKey checks that key is a non-empty, valid UTF-8 object key within
// the S3 length limit and without NUL bytes.
func ValidateKey(key string) error {
	if key == "" {
		return newInvalidError("validate", "", key, "object key must not be empty")
	}
	if len(key) > 1024 {
		return newInvalidError("validate", "", key, "object key must be at most 1024 bytes")
	}
	if !utf8.ValidString(key) {
		return newInvalidError("validate", "", key, "object key must be valid UTF-8")
	}
	if strings.IndexByte(key, 0) >= 0 {
		return newInvalidError("validate", "", key, "object key must not contain NUL")
	}
	return nil
}

// LocalPath maps an object key under prefix to a safe path below destDir.
// It rejects keys that escape the prefix or contain path traversal segments.
func LocalPath(destDir, prefix, key string) (string, error) {
	return localPath(destDir, prefix, key, runtime.GOOS == "windows")
}

func localPath(destDir, prefix, key string, windows bool) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}

	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if !strings.HasPrefix(key, prefix) {
		return "", newInvalidError("local_path", "", key, "object key is outside the requested prefix")
	}

	rel := strings.TrimPrefix(key, prefix)
	if rel == "" {
		return "", newInvalidError("local_path", "", key, "object key is a folder marker")
	}

	segments := make([]string, 0, strings.Count(rel, "/")+1)
	for _, segment := range strings.Split(rel, "/") {
		if segment == "" {
			continue
		}
		if segment == "." || segment == ".." {
			return "", newInvalidError("local_path", "", key, "object key contains a path traversal segment")
		}
		if windows && (strings.IndexByte(segment, '\\') >= 0 || strings.IndexByte(segment, ':') >= 0) {
			return "", newInvalidError("local_path", "", key, "object key contains a Windows path separator or volume marker")
		}
		segments = append(segments, segment)
	}

	pathParts := make([]string, 1, len(segments)+1)
	pathParts[0] = destDir
	pathParts = append(pathParts, segments...)
	path := filepath.Join(pathParts...)
	relative, err := filepath.Rel(destDir, path)
	if err != nil || relative == "." || strings.HasPrefix(relative, "..") {
		return "", newInvalidError("local_path", "", key, "resolved path escapes the destination directory")
	}
	return path, nil
}

func newInvalidError(op, bucket, key, reason string) *Error {
	return &Error{Kind: KindInvalid, Op: op, Bucket: bucket, Key: key, Err: errors.New(reason)}
}

// DetectContentType guesses the MIME type of path, first by extension and
// then by sniffing the file's first 512 bytes.
func DetectContentType(path string) string {
	if contentType := mime.TypeByExtension(filepath.Ext(path)); contentType != "" {
		return contentType
	}

	file, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer func() { _ = file.Close() }()

	buf := make([]byte, 512)
	read, err := io.ReadFull(file, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "application/octet-stream"
	}
	if read == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(buf[:read])
}
