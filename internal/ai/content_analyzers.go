package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// TextAnalyzer analyzes text content
type TextAnalyzer struct{}

func (ta *TextAnalyzer) CanAnalyze(contentType string) bool {
	return strings.HasPrefix(contentType, "text/") || 
		   contentType == "application/json" ||
		   contentType == "application/xml"
}

func (ta *TextAnalyzer) Analyze(ctx context.Context, content []byte) (*ContentAnalysisResult, error) {
	result := &ContentAnalysisResult{
		Metadata: make(map[string]string),
	}

	text := string(content)
	
	// Detect encoding
	if utf8.Valid(content) {
		result.Encoding = "UTF-8"
	} else {
		result.Encoding = "unknown"
	}

	// Basic text statistics
	result.Metadata["line_count"] = fmt.Sprintf("%d", strings.Count(text, "\n")+1)
	result.Metadata["word_count"] = fmt.Sprintf("%d", len(strings.Fields(text)))
	result.Metadata["char_count"] = fmt.Sprintf("%d", len(text))

	// Language detection (simplified)
	language := ta.detectLanguage(text)
	if language != "" {
		result.Language = language
	}

	// Generate text preview
	if len(text) > 200 {
		result.TextPreview = text[:200] + "..."
	} else {
		result.TextPreview = text
	}

	// Pattern analysis
	patterns := map[string]*regexp.Regexp{
		"has_code":       regexp.MustCompile(`(?i)(function|class|import|def|var|let|const)\s+\w+`),
		"has_urls":       regexp.MustCompile(`https?://[^\s]+`),
		"has_emails":     regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
		"has_numbers":    regexp.MustCompile(`\d+`),
		"has_dates":      regexp.MustCompile(`\d{4}-\d{2}-\d{2}|\d{2}/\d{2}/\d{4}`),
	}

	for pattern, regex := range patterns {
		if regex.MatchString(text) {
			result.Metadata[pattern] = "true"
		}
	}

	return result, nil
}

func (ta *TextAnalyzer) GetSupportedTypes() []string {
	return []string{"text/*", "application/json", "application/xml"}
}

// detectLanguage performs simple language detection
func (ta *TextAnalyzer) detectLanguage(text string) string {
	// Simple heuristic-based language detection
	text = strings.ToLower(text)
	
	// English indicators
	englishWords := []string{"the", "and", "for", "are", "but", "not", "you", "all", "can", "had", "her", "was", "one", "our", "out", "day", "get", "has", "him", "his", "how", "man", "new", "now", "old", "see", "two", "way", "who", "boy", "did", "its", "let", "put", "say", "she", "too", "use"}
	englishCount := 0
	for _, word := range englishWords {
		if strings.Contains(text, " "+word+" ") {
			englishCount++
		}
	}

	// Japanese indicators (hiragana/katakana)
	japanesePattern := regexp.MustCompile(`[\p{Hiragana}\p{Katakana}\p{Han}]`)
	if japanesePattern.MatchString(text) {
		return "ja"
	}

	// French indicators
	frenchWords := []string{"le", "de", "et", "à", "un", "il", "être", "et", "en", "avoir", "que", "pour", "dans", "ce", "son", "une", "sur", "avec", "ne", "se", "pas", "tout", "plus", "par", "grand", "en", "une", "être", "et", "à", "il", "avoir", "ne", "je", "son", "que", "se", "qui", "ce", "dans", "un", "sur", "avec", "ne", "se", "pas", "tout", "plus", "par", "grand", "en", "une", "être", "et", "à", "il", "avoir", "ne", "je", "son", "que", "se", "qui", "ce", "dans", "un", "sur", "avec", "ne", "se", "pas", "tout", "plus", "par", "grand", "en"}
	frenchCount := 0
	for _, word := range frenchWords {
		if strings.Contains(text, " "+word+" ") {
			frenchCount++
		}
	}

	// Spanish indicators
	spanishWords := []string{"el", "la", "de", "que", "y", "a", "en", "un", "ser", "se", "no", "te", "lo", "le", "da", "su", "por", "son", "con", "para", "al", "una", "sur", "les", "todo", "pero", "más", "hacer", "o", "poder", "su", "año", "vez", "tener", "él", "estar", "ver", "ir", "me", "ya", "sobre", "tiempo", "muy", "cuando", "sin", "entre", "cada", "tanto", "hasta", "donde", "mientras", "aunque", "través", "durante", "contra", "sin", "sobre", "entre", "hasta", "hacia", "durante", "antes", "según", "bajo", "tras", "encima", "cerca", "dentro", "fuera", "alrededor"}
	spanishCount := 0
	for _, word := range spanishWords {
		if strings.Contains(text, " "+word+" ") {
			spanishCount++
		}
	}

	// Return language with highest count
	if englishCount >= frenchCount && englishCount >= spanishCount && englishCount > 2 {
		return "en"
	} else if frenchCount >= spanishCount && frenchCount > 2 {
		return "fr"
	} else if spanishCount > 2 {
		return "es"
	}

	return "unknown"
}

// ImageAnalyzer analyzes image content
type ImageAnalyzer struct{}

func (ia *ImageAnalyzer) CanAnalyze(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

func (ia *ImageAnalyzer) Analyze(ctx context.Context, content []byte) (*ContentAnalysisResult, error) {
	result := &ContentAnalysisResult{
		Metadata: make(map[string]string),
	}

	// Basic image analysis (simplified - in production would use image libraries)
	imageProps := &ImageProperties{}
	
	// Detect image format from content
	if len(content) > 10 {
		// PNG signature
		if content[0] == 0x89 && content[1] == 0x50 && content[2] == 0x4E && content[3] == 0x47 {
			imageProps.Format = "PNG"
			imageProps.HasAlpha = true // PNG typically supports alpha
		}
		// JPEG signature
		if content[0] == 0xFF && content[1] == 0xD8 && content[2] == 0xFF {
			imageProps.Format = "JPEG"
			imageProps.HasAlpha = false
		}
		// GIF signature
		if string(content[0:6]) == "GIF87a" || string(content[0:6]) == "GIF89a" {
			imageProps.Format = "GIF"
			imageProps.HasAlpha = true
		}
		// WebP signature
		if string(content[0:4]) == "RIFF" && string(content[8:12]) == "WEBP" {
			imageProps.Format = "WebP"
			imageProps.HasAlpha = true
		}
	}

	// Try to extract basic dimensions (simplified parsing)
	width, height := ia.extractDimensions(content, imageProps.Format)
	if width > 0 && height > 0 {
		imageProps.Width = width
		imageProps.Height = height
		
		// Determine orientation
		if width > height {
			imageProps.Orientation = "landscape"
		} else if height > width {
			imageProps.Orientation = "portrait"
		} else {
			imageProps.Orientation = "square"
		}

		// Classify image size
		totalPixels := width * height
		if totalPixels < 100*100 {
			result.Metadata["image_size"] = "thumbnail"
		} else if totalPixels < 800*600 {
			result.Metadata["image_size"] = "small"
		} else if totalPixels < 1920*1080 {
			result.Metadata["image_size"] = "medium"
		} else if totalPixels < 4096*2160 {
			result.Metadata["image_size"] = "large"
		} else {
			result.Metadata["image_size"] = "huge"
		}

		// Aspect ratio classification
		aspectRatio := float64(width) / float64(height)
		if aspectRatio > 1.7 {
			result.Metadata["aspect_ratio"] = "wide"
		} else if aspectRatio < 0.6 {
			result.Metadata["aspect_ratio"] = "tall"
		} else {
			result.Metadata["aspect_ratio"] = "standard"
		}
	}

	result.ImageProperties = imageProps
	
	return result, nil
}

func (ia *ImageAnalyzer) GetSupportedTypes() []string {
	return []string{"image/*"}
}

// extractDimensions attempts to extract image dimensions (simplified implementation)
func (ia *ImageAnalyzer) extractDimensions(content []byte, format string) (int, int) {
	switch format {
	case "PNG":
		return ia.extractPNGDimensions(content)
	case "JPEG":
		return ia.extractJPEGDimensions(content)
	case "GIF":
		return ia.extractGIFDimensions(content)
	}
	return 0, 0
}

func (ia *ImageAnalyzer) extractPNGDimensions(content []byte) (int, int) {
	// PNG IHDR chunk starts at byte 16
	if len(content) < 24 {
		return 0, 0
	}
	
	// Width is bytes 16-19, height is bytes 20-23 (big-endian)
	width := int(content[16])<<24 | int(content[17])<<16 | int(content[18])<<8 | int(content[19])
	height := int(content[20])<<24 | int(content[21])<<16 | int(content[22])<<8 | int(content[23])
	
	return width, height
}

func (ia *ImageAnalyzer) extractJPEGDimensions(content []byte) (int, int) {
	// Simplified JPEG parsing - look for SOF0 marker (0xFFC0)
	for i := 0; i < len(content)-9; i++ {
		if content[i] == 0xFF && content[i+1] == 0xC0 {
			// Height is at offset +5,+6 and width at +7,+8 (big-endian)
			if i+8 < len(content) {
				height := int(content[i+5])<<8 | int(content[i+6])
				width := int(content[i+7])<<8 | int(content[i+8])
				return width, height
			}
		}
	}
	return 0, 0
}

func (ia *ImageAnalyzer) extractGIFDimensions(content []byte) (int, int) {
	// GIF dimensions are at bytes 6-9 (little-endian)
	if len(content) < 10 {
		return 0, 0
	}
	
	width := int(content[6]) | int(content[7])<<8
	height := int(content[8]) | int(content[9])<<8
	
	return width, height
}

// JSONAnalyzer analyzes JSON content
type JSONAnalyzer struct{}

func (ja *JSONAnalyzer) CanAnalyze(contentType string) bool {
	return contentType == "application/json" || strings.Contains(contentType, "json")
}

func (ja *JSONAnalyzer) Analyze(ctx context.Context, content []byte) (*ContentAnalysisResult, error) {
	result := &ContentAnalysisResult{
		Metadata: make(map[string]string),
	}

	// Parse JSON to analyze structure
	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		result.Metadata["json_valid"] = "false"
		result.Metadata["json_error"] = err.Error()
		return result, nil
	}

	result.Metadata["json_valid"] = "true"

	// Analyze JSON structure
	structure := ja.analyzeJSONStructure(data)
	for k, v := range structure {
		result.Metadata[k] = v
	}

	// Generate preview
	if len(content) > 200 {
		result.TextPreview = string(content[:200]) + "..."
	} else {
		result.TextPreview = string(content)
	}

	return result, nil
}

func (ja *JSONAnalyzer) GetSupportedTypes() []string {
	return []string{"application/json"}
}

func (ja *JSONAnalyzer) analyzeJSONStructure(data interface{}) map[string]string {
	metadata := make(map[string]string)

	switch v := data.(type) {
	case map[string]interface{}:
		metadata["json_type"] = "object"
		metadata["json_keys"] = fmt.Sprintf("%d", len(v))
		
		// Analyze common keys
		commonConfigKeys := []string{"name", "version", "config", "settings", "api", "database", "server"}
		hasConfigKeys := 0
		for _, key := range commonConfigKeys {
			if _, exists := v[key]; exists {
				hasConfigKeys++
			}
		}
		if hasConfigKeys >= 2 {
			metadata["json_appears_config"] = "true"
		}

		// Check for API response patterns
		apiKeys := []string{"data", "status", "error", "message", "success"}
		hasAPIKeys := 0
		for _, key := range apiKeys {
			if _, exists := v[key]; exists {
				hasAPIKeys++
			}
		}
		if hasAPIKeys >= 2 {
			metadata["json_appears_api_response"] = "true"
		}

	case []interface{}:
		metadata["json_type"] = "array"
		metadata["json_length"] = fmt.Sprintf("%d", len(v))
		
		if len(v) > 0 {
			// Analyze first element to determine array content type
			switch v[0].(type) {
			case map[string]interface{}:
				metadata["json_array_type"] = "objects"
			case []interface{}:
				metadata["json_array_type"] = "arrays"
			case string:
				metadata["json_array_type"] = "strings"
			case float64:
				metadata["json_array_type"] = "numbers"
			case bool:
				metadata["json_array_type"] = "booleans"
			}
		}

	case string:
		metadata["json_type"] = "string"
		metadata["json_length"] = fmt.Sprintf("%d", len(v))

	case float64:
		metadata["json_type"] = "number"

	case bool:
		metadata["json_type"] = "boolean"

	case nil:
		metadata["json_type"] = "null"
	}

	return metadata
}