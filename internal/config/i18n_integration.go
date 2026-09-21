package config

import (
	"github.com/seike460/s3ry/internal/i18n"
)

// InitializeI18n initializes the i18n system with the configured language.
// An empty or unsupported language falls back to locale detection.
func (c *Config) InitializeI18n() {
	i18n.InitWithLanguage(NormalizeLanguage(c.UI.Language))
}
