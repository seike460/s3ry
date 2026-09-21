package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitializeI18n_ValidLanguage(t *testing.T) {
	cfg := Default()
	cfg.UI.Language = "en"

	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
	})
}

func TestInitializeI18n_InvalidLanguage(t *testing.T) {
	cfg := Default()
	cfg.UI.Language = "invalid"

	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
	})
}

func TestInitializeI18n_EmptyLanguage(t *testing.T) {
	cfg := Default()
	cfg.UI.Language = ""

	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
	})
}

func TestI18nIntegration_EdgeCases(t *testing.T) {
	cfg := Default()

	assert.NotPanics(t, func() {
		cfg.InitializeI18n()
		cfg.InitializeI18n()
	})
}

func TestNormalization_Comprehensive(t *testing.T) {
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
		actual := NormalizeLanguage(input)
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
