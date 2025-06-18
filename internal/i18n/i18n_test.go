package i18n

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestInit(t *testing.T) {
	// Reset global state
	Printer = nil

	Init()

	assert.NotNil(t, Printer)
}

func TestDetectLanguage_English(t *testing.T) {
	// Save original env vars
	originalLang := os.Getenv("LANG")
	originalLanguage := os.Getenv("LANGUAGE")

	// Clean environment
	os.Unsetenv("LANG")
	os.Unsetenv("LANGUAGE")

	defer func() {
		// Restore original env vars
		if originalLang != "" {
			os.Setenv("LANG", originalLang)
		}
		if originalLanguage != "" {
			os.Setenv("LANGUAGE", originalLanguage)
		}
	}()

	lang := detectLanguage()
	assert.Equal(t, language.English, lang)
}

func TestDetectLanguage_Japanese_LANG(t *testing.T) {
	// Save original env vars
	originalLang := os.Getenv("LANG")
	originalLanguage := os.Getenv("LANGUAGE")

	// Set Japanese language via LANG
	os.Setenv("LANG", "ja_JP.UTF-8")
	os.Unsetenv("LANGUAGE")

	defer func() {
		// Restore original env vars
		if originalLang != "" {
			os.Setenv("LANG", originalLang)
		} else {
			os.Unsetenv("LANG")
		}
		if originalLanguage != "" {
			os.Setenv("LANGUAGE", originalLanguage)
		}
	}()

	lang := detectLanguage()
	assert.Equal(t, language.Japanese, lang)
}

func TestDetectLanguage_Japanese_LANGUAGE(t *testing.T) {
	// Save original env vars
	originalLang := os.Getenv("LANG")
	originalLanguage := os.Getenv("LANGUAGE")

	// Set Japanese language via LANGUAGE
	os.Unsetenv("LANG")
	os.Setenv("LANGUAGE", "ja")

	defer func() {
		// Restore original env vars
		if originalLang != "" {
			os.Setenv("LANG", originalLang)
		}
		if originalLanguage != "" {
			os.Setenv("LANGUAGE", originalLanguage)
		} else {
			os.Unsetenv("LANGUAGE")
		}
	}()

	lang := detectLanguage()
	assert.Equal(t, language.Japanese, lang)
}

func TestSprintf(t *testing.T) {
	// Reset global state
	Printer = nil

	result := Sprintf("Hello %s", "World")

	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Hello")
	assert.Contains(t, result, "World")
	assert.NotNil(t, Printer) // Should be initialized
}

func TestSprintf_WithInitializedPrinter(t *testing.T) {
	// Initialize printer first
	Init()

	result := Sprintf("Test %d", 123)

	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Test")
	assert.Contains(t, result, "123")
}

func TestPrintf(t *testing.T) {
	// Reset global state
	Printer = nil

	// Printf should not panic and should initialize Printer
	Printf("Test printf %s", "message")

	assert.NotNil(t, Printer)
}

func TestPrint(t *testing.T) {
	// Reset global state
	Printer = nil

	// Print should not panic and should initialize Printer
	Print("Test print message")

	assert.NotNil(t, Printer)
}

func TestPrintln(t *testing.T) {
	// Reset global state
	Printer = nil

	// Println should not panic and should initialize Printer
	Println("Test println message")

	assert.NotNil(t, Printer)
}

func TestMultipleInitCalls(t *testing.T) {
	// Reset global state
	Printer = nil

	// Multiple calls to Init should be safe
	Init()
	firstPrinter := Printer

	Init()
	secondPrinter := Printer

	assert.NotNil(t, firstPrinter)
	assert.NotNil(t, secondPrinter)
	// Note: They might be different instances, but both should be valid
}

func BenchmarkSprintf(b *testing.B) {
	Init() // Initialize once

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := Sprintf("Benchmark test %d", i)
		if result == "" {
			b.Fatal("Sprintf returned empty string")
		}
	}
}

func TestInitWithLanguage(t *testing.T) {
	// Reset global state
	Printer = nil

	InitWithLanguage("ja")

	assert.NotNil(t, Printer)

	// Test with invalid language
	InitWithLanguage("invalid")
	assert.NotNil(t, Printer)
}

func TestParseLanguageCode(t *testing.T) {
	tests := []struct {
		input    string
		expected language.Tag
	}{
		{"ja", language.Japanese},
		{"japanese", language.Japanese},
		{"jp", language.Japanese},
		{"en", language.English},
		{"english", language.English},
		{"EN", language.English},
		{"JA", language.Japanese},
		{"", detectLanguage()}, // Should detect language
		{"invalid", language.English},
		{"zh", language.English}, // Unsupported, fallback to English
	}

	for _, test := range tests {
		result := parseLanguageCode(test.input)
		assert.Equal(t, test.expected, result, "Failed for input: %s", test.input)
	}
}

func TestSetLanguage(t *testing.T) {
	// Reset global state
	Printer = nil

	SetLanguage("ja")
	assert.NotNil(t, Printer)

	SetLanguage("en")
	assert.NotNil(t, Printer)
}

func TestGetCurrentLanguage(t *testing.T) {
	// Reset global state
	Printer = nil

	lang := GetCurrentLanguage()
	assert.NotEqual(t, language.Und, lang)
	assert.NotNil(t, Printer) // Should initialize
}

func TestGetSupportedLanguages(t *testing.T) {
	langs := GetSupportedLanguages()

	assert.NotEmpty(t, langs)
	assert.Contains(t, langs, "en")
	assert.Contains(t, langs, "ja")
	assert.Len(t, langs, 2)
}

func BenchmarkDetectLanguage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		lang := detectLanguage()
		if lang == language.Und {
			b.Fatal("detectLanguage returned undefined language")
		}
	}
}

func BenchmarkParseLanguageCode(b *testing.B) {
	testCodes := []string{"ja", "en", "japanese", "english", "invalid"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		code := testCodes[i%len(testCodes)]
		parseLanguageCode(code)
	}
}
