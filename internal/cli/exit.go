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

// ExitCode maps command errors to the process codes used by s3ry.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if isUsageError(err) || isCobraUsageMessage(err.Error()) {
		return 2
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}

	var s3Err *s3.Error
	if errors.As(err, &s3Err) && s3Err != nil {
		switch s3Err.Kind {
		case s3.KindCanceled:
			return 130
		case s3.KindNotFound:
			return 3
		case s3.KindAccessDenied, s3.KindNoCredentials:
			return 4
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
