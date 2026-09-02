package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitializeI18n_ValidLanguage(t *testing.T) {
	cfg := Default()
	cfg.UI.Language = "en"

	// Test that InitializeI18n doesn't panic with valid language
	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
	})
}

func TestInitializeI18n_InvalidLanguage(t *testing.T) {
	cfg := Default()
	cfg.UI.Language = "invalid"

	// Test that InitializeI18n falls back gracefully with invalid language
	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
	})
}

func TestInitializeI18n_EmptyLanguage(t *testing.T) {
	cfg := Default()
	cfg.UI.Language = ""

	// Test that InitializeI18n falls back gracefully with empty language
	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
	})
}

func TestI18nIntegration_EdgeCases(t *testing.T) {
	cfg := Default()

	// Test repeated initialization
	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
	})

	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
		cfg.InitializeI18n()
	})
}

func TestLanguageValidation_Comprehensive(t *testing.T) {
	cfg := Default()

	// Test all documented supported languages
	supportedLanguages := []string{"en", "ja", "english", "japanese"}

	for _, lang := range supportedLanguages {
		assert.True(t, cfg.ValidateLanguage(lang), "Language %s should be supported", lang)
	}

	// Test case sensitivity
	caseSensitiveTests := []string{"EN", "JA", "English", "Japanese", "ENGLISH", "JAPANESE"}

	for _, lang := range caseSensitiveTests {
		// Current implementation is case-sensitive, so these should fail
		assert.False(t, cfg.ValidateLanguage(lang), "Language %s should be case-sensitive", lang)
	}
}

func TestNormalization_Comprehensive(t *testing.T) {
	cfg := Default()

	// Test all normalization cases
	normalizations := map[string]string{
		"japanese": "ja",
		"jp":       "ja",
		"english":  "en",
		"en":       "en",
		"ja":       "ja",
		"fr":       "fr", // Pass-through
		"":         "",   // Pass-through
	}

	for input, expected := range normalizations {
		actual := cfg.NormalizeLanguage(input)
		assert.Equal(t, expected, actual, "Normalization of %s should be %s", input, expected)
	}
}

func BenchmarkInitializeI18n(b *testing.B) {
	cfg := Default()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.InitializeI18n()
	}
}
