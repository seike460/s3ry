package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestInit(t *testing.T) {
	Init()

	assert.NotEqual(t, language.Und, CurrentLanguage())
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
	SetLanguage("en")

	result := Sprintf("Hello %s", "World")

	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Hello")
	assert.Contains(t, result, "World")
}

func TestSprintf_Japanese(t *testing.T) {
	SetLanguage("ja")
	t.Cleanup(func() { SetLanguage("en") })

	result := Sprintf("Hello %s", "World")

	assert.NotEmpty(t, result)
}

func TestJapaneseCatalog(t *testing.T) {
	SetLanguage("ja")
	t.Cleanup(func() { SetLanguage("en") })

	if got := Sprintf("Canceled"); got != "キャンセルしました" {
		t.Fatalf("ja catalog = %q, want キャンセルしました", got)
	}
	if got := Sprintf("Delete %s? [y/N]", "a.txt"); got != "a.txt を削除しますか？ [y/N]" {
		t.Fatalf("ja formatted catalog = %q", got)
	}
	if got := Sprintf("key without a translation"); got != "key without a translation" {
		t.Fatalf("untranslated key = %q, want the key itself", got)
	}
}

func TestMultipleInitCalls(t *testing.T) {
	Init()
	first := CurrentLanguage()

	Init()
	second := CurrentLanguage()

	assert.Equal(t, first, second)
}

func BenchmarkSprintf(b *testing.B) {
	SetLanguage("en")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := Sprintf("Benchmark test %d", i)
		if result == "" {
			b.Fatal("Sprintf returned empty string")
		}
	}
}

func TestInitWithLanguage(t *testing.T) {
	InitWithLanguage("ja")
	assert.Equal(t, language.Japanese, CurrentLanguage())

	InitWithLanguage("invalid")
	assert.Equal(t, language.English, CurrentLanguage())

	InitWithLanguage("")
	assert.NotEqual(t, language.Und, CurrentLanguage())
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
	SetLanguage("ja")
	assert.Equal(t, language.Japanese, CurrentLanguage())

	SetLanguage("en")
	assert.Equal(t, language.English, CurrentLanguage())
}

func TestCurrentLanguage(t *testing.T) {
	lang := CurrentLanguage()
	assert.NotEqual(t, language.Und, lang)
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
