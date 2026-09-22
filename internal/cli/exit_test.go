package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/seike460/s3ry/internal/s3"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: 0},
		{name: "context canceled", err: context.Canceled, want: 130},
		{name: "unknown", err: &s3.Error{Kind: s3.KindUnknown}, want: 1},
		{name: "not found", err: &s3.Error{Kind: s3.KindNotFound}, want: 3},
		{name: "access denied", err: &s3.Error{Kind: s3.KindAccessDenied}, want: 4},
		{name: "no credentials", err: &s3.Error{Kind: s3.KindNoCredentials}, want: 4},
		{name: "throttled", err: &s3.Error{Kind: s3.KindThrottled}, want: 1},
		{name: "canceled", err: &s3.Error{Kind: s3.KindCanceled}, want: 130},
		{name: "timeout", err: &s3.Error{Kind: s3.KindTimeout, Err: context.DeadlineExceeded}, want: 1},
		{name: "invalid", err: &s3.Error{Kind: s3.KindInvalid}, want: 1},
		{name: "exists", err: &s3.Error{Kind: s3.KindExists}, want: 1},
		{name: "unsupported", err: &s3.Error{Kind: s3.KindUnsupported}, want: 1},
		{name: "plain error", err: errors.New("plain error"), want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ExitCode(test.err); got != test.want {
				t.Fatalf("ExitCode(%v) = %d, want %d", test.err, got, test.want)
			}
		})
	}
}

func TestExitCodeWrappedCancellation(t *testing.T) {
	err := errors.Join(errors.New("operation stopped"), context.Canceled)
	if got := ExitCode(err); got != 130 {
		t.Fatalf("ExitCode(wrapped cancellation) = %d, want 130", got)
	}
}
