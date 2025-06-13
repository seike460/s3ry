package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigError_Error(t *testing.T) {
	err := &ConfigError{
		Field:   "language",
		Value:   "invalid",
		Message: "unsupported language",
	}
	
	expected := "config error in field 'language' with value 'invalid': unsupported language"
	assert.Equal(t, expected, err.Error())
}

func TestConfigError_WithDifferentFields(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		value    string
		message  string
		expected string
	}{
		{
			name:     "region error",
			field:    "region",
			value:    "invalid-region",
			message:  "region not found",
			expected: "config error in field 'region' with value 'invalid-region': region not found",
		},
		{
			name:     "mode error",
			field:    "mode",
			value:    "unknown",
			message:  "unsupported UI mode",
			expected: "config error in field 'mode' with value 'unknown': unsupported UI mode",
		},
		{
			name:     "empty values",
			field:    "",
			value:    "",
			message:  "",
			expected: "config error in field '' with value '': ",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ConfigError{
				Field:   tt.field,
				Value:   tt.value,
				Message: tt.message,
			}
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestChangeLanguage_ValidLanguages(t *testing.T) {
	cfg := Default()
	
	// Test valid languages
	validLanguages := []string{"en", "ja", "english", "japanese"}
	
	for _, lang := range validLanguages {
		err := cfg.ChangeLanguage(lang)
		assert.NoError(t, err, "Language %s should be valid", lang)
		
		// Check that language was normalized and set
		normalized := cfg.NormalizeLanguage(lang)
		assert.Equal(t, normalized, cfg.UI.Language, "Language should be normalized and set")
	}
}

func TestChangeLanguage_InvalidLanguages(t *testing.T) {
	cfg := Default()
	
	// Test invalid languages
	invalidLanguages := []string{"fr", "de", "es", "invalid", ""}
	
	for _, lang := range invalidLanguages {
		err := cfg.ChangeLanguage(lang)
		assert.Error(t, err, "Language %s should be invalid", lang)
		
		// Check that error is ConfigError
		configErr, ok := err.(*ConfigError)
		assert.True(t, ok, "Error should be ConfigError type")
		assert.Equal(t, "language", configErr.Field)
		assert.Equal(t, lang, configErr.Value)
		assert.Equal(t, "unsupported language", configErr.Message)
	}
}

func TestChangeLanguage_Normalization(t *testing.T) {
	cfg := Default()
	
	// Test that normalization works in ChangeLanguage
	err := cfg.ChangeLanguage("japanese")
	assert.NoError(t, err)
	assert.Equal(t, "ja", cfg.UI.Language)
	
	err = cfg.ChangeLanguage("english")
	assert.NoError(t, err)
	assert.Equal(t, "en", cfg.UI.Language)
	
	err = cfg.ChangeLanguage("jp")
	assert.NoError(t, err)
	assert.Equal(t, "ja", cfg.UI.Language)
}

func TestChangeLanguage_WithI18nIntegration(t *testing.T) {
	cfg := Default()
	
	// Test changing language updates both config and i18n
	// Note: This test assumes i18n package exists and has SetLanguage function
	err := cfg.ChangeLanguage("en")
	assert.NoError(t, err)
	assert.Equal(t, "en", cfg.UI.Language)
	
	err = cfg.ChangeLanguage("ja")
	assert.NoError(t, err)
	assert.Equal(t, "ja", cfg.UI.Language)
}

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

func TestSyncI18nLanguage_Integration(t *testing.T) {
	cfg := Default()
	
	// Test that SyncI18nLanguage doesn't panic
	assert.NotPanics(t, func() {
		cfg.SyncI18nLanguage()
	})
	
	// The actual sync behavior depends on i18n implementation
	// Here we just test that the method exists and doesn't crash
}

func TestI18nIntegration_LanguageFlow(t *testing.T) {
	cfg := Default()
	
	// Test complete language change flow
	originalLang := cfg.UI.Language
	
	// Change to English
	err := cfg.ChangeLanguage("english")
	assert.NoError(t, err)
	assert.Equal(t, "en", cfg.UI.Language)
	assert.NotEqual(t, originalLang, cfg.UI.Language)
	
	// Initialize i18n with new language
	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
	})
	
	// Sync language state
	assert.NotPanics(t, func() {
		cfg.SyncI18nLanguage()
	})
	
	// Change back to Japanese
	err = cfg.ChangeLanguage("ja")
	assert.NoError(t, err)
	assert.Equal(t, "ja", cfg.UI.Language)
}

func TestConfigError_AsError(t *testing.T) {
	// Test that ConfigError implements error interface
	var err error = &ConfigError{
		Field:   "test",
		Value:   "test",
		Message: "test",
	}
	
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "config error")
}

func TestI18nIntegration_EdgeCases(t *testing.T) {
	cfg := Default()
	
	// Test with nil config (shouldn't happen, but defensive programming)
	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
		cfg.SyncI18nLanguage()
	})
	
	// Test multiple initializations
	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
		cfg.InitializeI18n()
		cfg.SyncI18nLanguage()
		cfg.SyncI18nLanguage()
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

// Benchmark tests for i18n integration
func BenchmarkChangeLanguage(b *testing.B) {
	cfg := Default()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.ChangeLanguage("en")
		cfg.ChangeLanguage("ja")
	}
}

func BenchmarkInitializeI18n(b *testing.B) {
	cfg := Default()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.InitializeI18n()
	}
}

func BenchmarkSyncI18nLanguage(b *testing.B) {
	cfg := Default()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.SyncI18nLanguage()
	}
}

func BenchmarkConfigError_Error(b *testing.B) {
	err := &ConfigError{
		Field:   "language",
		Value:   "invalid",
		Message: "unsupported language",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}