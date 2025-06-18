package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalPrinter(t *testing.T) {
	assert.NotNil(t, GlobalPrinter)
}

func TestPrinterWrapper_Sprintf(t *testing.T) {
	wrapper := &PrinterWrapper{}

	result := wrapper.Sprintf("Test %s %d", "message", 42)

	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Test")
	assert.Contains(t, result, "message")
	assert.Contains(t, result, "42")
}

func TestGlobalPrinter_Sprintf(t *testing.T) {
	result := GlobalPrinter.Sprintf("Global test %s", "message")

	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Global test")
	assert.Contains(t, result, "message")
}

func TestGlobalPrinter_Integration_WithMainPackage(t *testing.T) {
	// This tests that GlobalPrinter works as expected for backward compatibility
	// when used from the main package (simulating legacy i18nPrinter usage)

	// Test various format strings
	testCases := []struct {
		format   string
		args     []interface{}
		expected string
	}{
		{"Simple message", nil, "Simple message"},
		{"Hello %s", []interface{}{"World"}, "Hello World"},
		{"Number: %d", []interface{}{42}, "Number: 42"},
		{"Float: %.2f", []interface{}{3.14159}, "Float: 3.14"},
	}

	for _, tc := range testCases {
		var result string
		if tc.args == nil {
			result = GlobalPrinter.Sprintf(tc.format)
		} else {
			result = GlobalPrinter.Sprintf(tc.format, tc.args...)
		}

		assert.Contains(t, result, tc.expected)
	}
}

func BenchmarkGlobalPrinter_Sprintf(b *testing.B) {
	for i := 0; i < b.N; i++ {
		result := GlobalPrinter.Sprintf("Benchmark test %d", i)
		if result == "" {
			b.Fatal("GlobalPrinter.Sprintf returned empty string")
		}
	}
}

func BenchmarkPrinterWrapper_Sprintf(b *testing.B) {
	wrapper := &PrinterWrapper{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := wrapper.Sprintf("Benchmark test %d", i)
		if result == "" {
			b.Fatal("PrinterWrapper.Sprintf returned empty string")
		}
	}
}
