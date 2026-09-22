package s3

import (
	"strings"
	"testing"
)

func TestWalkPanicError(t *testing.T) {
	err := walkPanic{value: "boom"}
	if got := err.Error(); !strings.Contains(got, "boom") {
		t.Fatalf("walkPanic.Error() = %q, want panic value included", got)
	}
}
