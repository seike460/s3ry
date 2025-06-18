package i18n

// Global variable for backward compatibility with existing code
// This allows the existing i18nPrinter references to work during migration
var GlobalPrinter *PrinterWrapper

type PrinterWrapper struct{}

func (p *PrinterWrapper) Sprintf(format string, args ...interface{}) string {
	return Sprintf(format, args...)
}

func init() {
	GlobalPrinter = &PrinterWrapper{}
}
