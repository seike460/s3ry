// Package i18n wraps golang.org/x/text/message with the small surface s3ry
// needs: process-wide language selection and a printf-style lookup.
package i18n

import (
	"os"
	"strings"
	"sync"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	// printer is the global message printer. It is guarded because the TUI
	// reads strings while startup code may still be selecting the language.
	printerMu sync.RWMutex
	printer   = message.NewPrinter(language.English)
	current   = language.English
)

// Init selects the language from the environment.
func Init() {
	SetLanguage(detectLanguage().String())
}

// InitWithLanguage selects a specific language. An empty or unsupported code
// falls back to locale detection and then to English.
func InitWithLanguage(languageCode string) {
	SetLanguage(languageCode)
}

// SetLanguage changes the current language.
func SetLanguage(languageCode string) {
	tag := parseLanguageCode(languageCode)
	printerMu.Lock()
	printer = message.NewPrinter(tag)
	current = tag
	printerMu.Unlock()
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

// Sprintf returns a localized string formatted with the given arguments.
// Strings that have no catalog entry are returned unchanged.
func Sprintf(format string, args ...any) string {
	printerMu.RLock()
	p := printer
	printerMu.RUnlock()
	return p.Sprintf(format, args...)
}

// CurrentLanguage returns the tag of the active printer.
func CurrentLanguage() language.Tag {
	printerMu.RLock()
	defer printerMu.RUnlock()
	return current
}

// SupportedLanguages lists the languages with a registered catalog.
func SupportedLanguages() []string {
	return []string{"en", "ja"}
}
