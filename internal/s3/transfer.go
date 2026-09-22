package s3

import (
	"errors"
	"fmt"
	"io/fs"
)

func localFileError(op, bucket, key, localPath string, err error) error {
	var kind Kind
	switch {
	case errors.Is(err, fs.ErrNotExist):
		kind = KindNotFound
	case errors.Is(err, fs.ErrPermission):
		kind = KindAccessDenied
	default:
		return Classify(op, bucket, key, err)
	}

	return &Error{
		Kind:   kind,
		Op:     op,
		Bucket: bucket,
		Key:    key,
		Err:    fmt.Errorf("local path %q: %w", localPath, err),
	}
}

func panicError(value any) error {
	if err, ok := value.(error); ok {
		return fmt.Errorf("transfer panic: %w", err)
	}
	return fmt.Errorf("transfer panic: %v", value)
}

func finishTransfer(m *meter, op, bucket, key string, err error) error {
	classified := Classify(op, bucket, key, err)
	m.finish(classified)
	return classified
}
