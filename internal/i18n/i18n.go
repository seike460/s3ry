// Package i18n wraps golang.org/x/text/message with the small surface s3ry
// needs: an immutable printer per selected language.
package i18n

import (
	"os"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Printer formats localized messages. It is immutable once created, so a
// single printer can be shared by every view without synchronization.
type Printer struct {
	printer *message.Printer
	lang    language.Tag
}

// NewPrinter selects the language for languageCode. An empty or unsupported
// code falls back to locale detection and then to English.
func NewPrinter(languageCode string) *Printer {
	tag := parseLanguageCode(languageCode)
	return &Printer{printer: message.NewPrinter(tag), lang: tag}
}

// Sprintf returns a localized string formatted with the given arguments.
// Strings that have no catalog entry are returned unchanged.
func (p *Printer) Sprintf(format string, args ...any) string {
	return p.printer.Sprintf(format, args...)
}

// Language returns the tag of the printer.
func (p *Printer) Language() language.Tag {
	return p.lang
}

// detectLanguage returns the system language, defaulting to English.
func detectLanguage() language.Tag {
	for _, key := range []string{"S3RY_LANGUAGE", "LANGUAGE", "LANG", "LC_ALL"} {
		if strings.HasPrefix(os.Getenv(key), "ja") {
			return language.Japanese
		}
	}
	return language.English
}

// parseLanguageCode converts a language code string to a language.Tag.
func parseLanguageCode(languageCode string) language.Tag {
	switch strings.ToLower(strings.TrimSpace(languageCode)) {
	case "":
		return detectLanguage()
	case "ja", "japanese", "jp":
		return language.Japanese
	case "en", "english":
		return language.English
	}
	if tag, err := language.Parse(languageCode); err == nil && tag == language.Japanese {
		return language.Japanese
	}
	return language.English
}

// SupportedLanguages lists the languages with a registered catalog.
func SupportedLanguages() []string {
	return []string{"en", "ja"}
}
