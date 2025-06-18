package config

import (
	"github.com/seike460/s3ry/internal/i18n"
)

// InitializeI18n initializes the i18n system with the configured language
func (c *Config) InitializeI18n() {
	lang := c.GetLanguage()
	if lang != "" && c.ValidateLanguage(lang) {
		normalizedLang := c.NormalizeLanguage(lang)
		i18n.InitWithLanguage(normalizedLang)
	} else {
		// Fall back to default initialization
		i18n.Init()
	}
}

// SyncI18nLanguage synchronizes the current i18n language with config
func (c *Config) SyncI18nLanguage() {
	currentLang := i18n.GetCurrentLanguage()
	var langCode string

	switch currentLang.String() {
	case "ja":
		langCode = "ja"
	case "en":
		langCode = "en"
	default:
		langCode = "en"
	}

	if c.UI.Language != langCode {
		c.SetLanguage(langCode)
	}
}

// ChangeLanguage changes both config and i18n language
func (c *Config) ChangeLanguage(lang string) error {
	normalizedLang := c.NormalizeLanguage(lang)

	if !c.ValidateLanguage(normalizedLang) {
		return &ConfigError{
			Field:   "language",
			Value:   lang,
			Message: "unsupported language",
		}
	}

	// Update config
	c.SetLanguage(normalizedLang)

	// Update i18n system
	i18n.SetLanguage(normalizedLang)

	return nil
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Value   string
	Message string
}

func (e *ConfigError) Error() string {
	return "config error in field '" + e.Field + "' with value '" + e.Value + "': " + e.Message
}
