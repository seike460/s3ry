package cli

import (
	"context"
	"errors"
	"strings"

	"github.com/seike460/s3ry/internal/s3"
)

type usageError struct {
	err error
}

func (e *usageError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *usageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func normalizeUsageError(err error) error {
	if err == nil || isUsageError(err) {
		return err
	}
	if isCobraUsageMessage(err.Error()) {
		return &usageError{err: err}
	}
	return err
}

// exitCodeCanceled is the conventional shell exit status for SIGINT (128+2).
const exitCodeCanceled = 130

// kindExitCodes maps classified s3 errors to their process exit codes.
var kindExitCodes = map[s3.Kind]int{
	s3.KindCanceled:      exitCodeCanceled,
	s3.KindNotFound:      3,
	s3.KindAccessDenied:  4,
	s3.KindNoCredentials: 4,
}

// ExitCode maps command errors to the process codes used by s3ry.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if isUsageError(err) || isCobraUsageMessage(err.Error()) {
		return 2
	}
	if errors.Is(err, context.Canceled) {
		return exitCodeCanceled
	}

	var s3Err *s3.Error
	if errors.As(err, &s3Err) && s3Err != nil {
		if code, ok := kindExitCodes[s3Err.Kind]; ok {
			return code
		}
	}
	return 1
}

func isUsageError(err error) bool {
	var usageErr *usageError
	return errors.As(err, &usageErr) && usageErr != nil
}

func isCobraUsageMessage(message string) bool {
	for _, prefix := range []string{
		"unknown command ",
		"unknown flag:",
		"unknown shorthand flag:",
		"flag provided but not defined:",
		"flag needs an argument:",
		"invalid argument ",
		"requires at least ",
		"requires exactly ",
		"accepts at most ",
		"accepts exactly ",
		"accepts ",
	} {
		if strings.HasPrefix(message, prefix) {
			return true
		}
	}
	return false
}
