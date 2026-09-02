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
