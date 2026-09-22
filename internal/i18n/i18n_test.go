package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestNewPrinter(t *testing.T) {
	p := NewPrinter("en")
	assert.Equal(t, language.English, p.Language())
}

func TestDetectLanguage_English(t *testing.T) {
	for _, key := range []string{"S3RY_LANGUAGE", "LANG", "LANGUAGE", "LC_ALL"} {
		t.Setenv(key, "")
	}

	lang := detectLanguage()
	assert.Equal(t, language.English, lang)
}

func TestDetectLanguage_Japanese_LANG(t *testing.T) {
	for _, key := range []string{"S3RY_LANGUAGE", "LANGUAGE", "LC_ALL"} {
		t.Setenv(key, "")
	}
	t.Setenv("LANG", "ja_JP.UTF-8")

	lang := detectLanguage()
	assert.Equal(t, language.Japanese, lang)
}

func TestDetectLanguage_Japanese_LANGUAGE(t *testing.T) {
	for _, key := range []string{"S3RY_LANGUAGE", "LANG", "LC_ALL"} {
		t.Setenv(key, "")
	}
	t.Setenv("LANGUAGE", "ja")

	lang := detectLanguage()
	assert.Equal(t, language.Japanese, lang)
}

func TestSprintf(t *testing.T) {
	p := NewPrinter("en")

	result := p.Sprintf("Hello %s", "World")

	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Hello")
	assert.Contains(t, result, "World")
}

func TestSprintf_Japanese(t *testing.T) {
	p := NewPrinter("ja")

	result := p.Sprintf("Hello %s", "World")

	assert.NotEmpty(t, result)
}

func TestJapaneseCatalog(t *testing.T) {
	p := NewPrinter("ja")

	if got := p.Sprintf("Canceled"); got != "キャンセルしました" {
		t.Fatalf("ja catalog = %q, want キャンセルしました", got)
	}
	if got := p.Sprintf("Delete %s? [y/N]", "a.txt"); got != "a.txt を削除しますか？ [y/N]" {
		t.Fatalf("ja formatted catalog = %q", got)
	}
	if got := p.Sprintf("key without a translation"); got != "key without a translation" {
		t.Fatalf("untranslated key = %q, want the key itself", got)
	}
}

func TestPrintersAreIndependent(t *testing.T) {
	en := NewPrinter("en")
	ja := NewPrinter("ja")

	if got := ja.Sprintf("Canceled"); got != "キャンセルしました" {
		t.Fatalf("ja printer = %q, want キャンセルしました", got)
	}
	if got := en.Sprintf("Canceled"); got != "Canceled" {
		t.Fatalf("en printer = %q, want the untranslated key", got)
	}
}

func BenchmarkSprintf(b *testing.B) {
	p := NewPrinter("en")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := p.Sprintf("Benchmark test %d", i)
		if result == "" {
			b.Fatal("Sprintf returned empty string")
		}
	}
}

func TestNewPrinterFallbacks(t *testing.T) {
	assert.Equal(t, language.Japanese, NewPrinter("ja").Language())
	assert.Equal(t, language.English, NewPrinter("invalid").Language())
	assert.NotEqual(t, language.Und, NewPrinter("").Language())
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

func TestSupportedLanguages(t *testing.T) {
	langs := SupportedLanguages()

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
