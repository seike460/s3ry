package i18n

import (
	"fmt"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"os"
	"strings"
)

var (
	// Printer is the global message printer instance
	Printer *message.Printer
	// currentLanguage keeps track of the current language
	currentLanguage language.Tag
)

// Init initializes the i18n system with the appropriate language
func Init() {
	lang := detectLanguage()
	currentLanguage = lang
	Printer = message.NewPrinter(lang)
}

// InitWithLanguage initializes the i18n system with a specific language
func InitWithLanguage(languageCode string) {
	lang := parseLanguageCode(languageCode)
	currentLanguage = lang
	Printer = message.NewPrinter(lang)
}

// detectLanguage detects the system language or returns English as default
func detectLanguage() language.Tag {
	// Check environment variables for language preference
	if lang := os.Getenv("LANG"); lang != "" {
		if strings.HasPrefix(lang, "ja") {
			return language.Japanese
		}
	}

	if lang := os.Getenv("LANGUAGE"); lang != "" {
		if strings.HasPrefix(lang, "ja") {
			return language.Japanese
		}
	}

	// Default to English
	return language.English
}

// parseLanguageCode converts a language code string to language.Tag
func parseLanguageCode(languageCode string) language.Tag {
	if languageCode == "" {
		return detectLanguage()
	}

	// Normalize the language code
	languageCode = strings.ToLower(strings.TrimSpace(languageCode))

	switch languageCode {
	case "ja", "japanese", "jp":
		return language.Japanese
	case "en", "english":
		return language.English
	default:
		// Try to parse the language code
		if tag, err := language.Parse(languageCode); err == nil {
			// Check if we support this language
			if tag == language.Japanese {
				return language.Japanese
			}
		}
		// Default to English for unsupported languages
		return language.English
	}
}

// Sprintf returns a localized string formatted with the given arguments
func Sprintf(format string, args ...interface{}) string {
	if Printer == nil {
		Init()
	}
	return Printer.Sprintf(format, args...)
}

// Printf prints a localized string formatted with the given arguments
func Printf(format string, args ...interface{}) {
	if Printer == nil {
		Init()
	}
	Printer.Printf(format, args...)
}

// Print prints a localized string
func Print(args ...interface{}) {
	if Printer == nil {
		Init()
	}
	fmt.Print(args...)
}

// Println prints a localized string with a newline
func Println(args ...interface{}) {
	if Printer == nil {
		Init()
	}
	fmt.Println(args...)
}

// SetLanguage changes the current language
func SetLanguage(languageCode string) {
	InitWithLanguage(languageCode)
}

// GetCurrentLanguage returns the current language tag
func GetCurrentLanguage() language.Tag {
	if Printer == nil {
		Init()
	}
	return currentLanguage
}

// GetSupportedLanguages returns a list of supported languages
func GetSupportedLanguages() []string {
	return []string{"en", "ja"}
}
