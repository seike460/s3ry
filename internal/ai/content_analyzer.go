package ai

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ContentAnalyzer provides AI/ML-powered content analysis for automatic tagging
type ContentAnalyzer struct {
	config           *AnalyzerConfig
	tagGenerators    []TagGenerator
	contentAnalyzers []ContentTypeAnalyzer
	patterns         *PatternMatcher
	logger           Logger
}

// AnalyzerConfig configures the content analyzer
type AnalyzerConfig struct {
	MaxFileSize              int64         `json:"max_file_size"`
	SupportedMimeTypes       []string      `json:"supported_mime_types"`
	EnableImageAnalysis      bool          `json:"enable_image_analysis"`
	EnableTextAnalysis       bool          `json:"enable_text_analysis"`
	EnableMetadataExtraction bool          `json:"enable_metadata_extraction"`
	TagConfidenceThreshold   float64       `json:"tag_confidence_threshold"`
	MaxTagsPerFile           int           `json:"max_tags_per_file"`
	AnalysisTimeout          time.Duration `json:"analysis_timeout"`
	CacheResults             bool          `json:"cache_results"`
}

// DefaultAnalyzerConfig returns default analyzer configuration
func DefaultAnalyzerConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		MaxFileSize:              100 * 1024 * 1024, // 100MB
		SupportedMimeTypes:       []string{"text/*", "image/*", "application/pdf", "application/json"},
		EnableImageAnalysis:      true,
		EnableTextAnalysis:       true,
		EnableMetadataExtraction: true,
		TagConfidenceThreshold:   0.7,
		MaxTagsPerFile:           10,
		AnalysisTimeout:          30 * time.Second,
		CacheResults:             true,
	}
}

// ContentAnalysisResult contains the results of content analysis
type ContentAnalysisResult struct {
	FileName        string            `json:"file_name"`
	FileSize        int64             `json:"file_size"`
	ContentType     string            `json:"content_type"`
	MD5Hash         string            `json:"md5_hash"`
	SHA256Hash      string            `json:"sha256_hash"`
	Tags            []Tag             `json:"tags"`
	Metadata        map[string]string `json:"metadata"`
	Language        string            `json:"language,omitempty"`
	Encoding        string            `json:"encoding,omitempty"`
	TextPreview     string            `json:"text_preview,omitempty"`
	ImageProperties *ImageProperties  `json:"image_properties,omitempty"`
	AnalysisTime    time.Duration     `json:"analysis_time"`
	Confidence      float64           `json:"confidence"`
}

// Tag represents an automatically generated tag
type Tag struct {
	Name       string  `json:"name"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"` // e.g., "content", "filename", "metadata"
}

// ImageProperties contains properties extracted from images
type ImageProperties struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Format      string `json:"format"`
	ColorSpace  string `json:"color_space"`
	HasAlpha    bool   `json:"has_alpha"`
	Orientation string `json:"orientation,omitempty"`
	DPI         int    `json:"dpi,omitempty"`
}

// TagGenerator interface for different tag generation strategies
type TagGenerator interface {
	GenerateTags(ctx context.Context, content []byte, metadata map[string]string) ([]Tag, error)
	GetName() string
	GetPriority() int
}

// ContentTypeAnalyzer interface for content-type specific analysis
type ContentTypeAnalyzer interface {
	CanAnalyze(contentType string) bool
	Analyze(ctx context.Context, content []byte) (*ContentAnalysisResult, error)
	GetSupportedTypes() []string
}

// Logger interface for logging
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// NewContentAnalyzer creates a new content analyzer
func NewContentAnalyzer(config *AnalyzerConfig, logger Logger) *ContentAnalyzer {
	if config == nil {
		config = DefaultAnalyzerConfig()
	}

	analyzer := &ContentAnalyzer{
		config:           config,
		tagGenerators:    make([]TagGenerator, 0),
		contentAnalyzers: make([]ContentTypeAnalyzer, 0),
		patterns:         NewPatternMatcher(),
		logger:           logger,
	}

	// Register default tag generators
	analyzer.RegisterTagGenerator(&FilenameTagGenerator{})
	analyzer.RegisterTagGenerator(&ContentTypeTagGenerator{})
	analyzer.RegisterTagGenerator(&SizeTagGenerator{})
	analyzer.RegisterTagGenerator(&DateTagGenerator{})

	// Register default content analyzers
	analyzer.RegisterContentAnalyzer(&TextAnalyzer{})
	analyzer.RegisterContentAnalyzer(&ImageAnalyzer{})
	analyzer.RegisterContentAnalyzer(&JSONAnalyzer{})

	return analyzer
}

// RegisterTagGenerator registers a tag generator
func (ca *ContentAnalyzer) RegisterTagGenerator(generator TagGenerator) {
	ca.tagGenerators = append(ca.tagGenerators, generator)

	// Sort by priority
	sort.Slice(ca.tagGenerators, func(i, j int) bool {
		return ca.tagGenerators[i].GetPriority() > ca.tagGenerators[j].GetPriority()
	})
}

// RegisterContentAnalyzer registers a content analyzer
func (ca *ContentAnalyzer) RegisterContentAnalyzer(analyzer ContentTypeAnalyzer) {
	ca.contentAnalyzers = append(ca.contentAnalyzers, analyzer)
}

// AnalyzeContent performs comprehensive content analysis
func (ca *ContentAnalyzer) AnalyzeContent(ctx context.Context, fileName string, content io.Reader) (*ContentAnalysisResult, error) {
	startTime := time.Now()

	// Read content into buffer
	buf := &bytes.Buffer{}
	_, err := io.Copy(buf, content)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	contentBytes := buf.Bytes()

	// Check file size limit
	if int64(len(contentBytes)) > ca.config.MaxFileSize {
		return nil, fmt.Errorf("file size exceeds limit: %d > %d", len(contentBytes), ca.config.MaxFileSize)
	}

	// Initialize result
	result := &ContentAnalysisResult{
		FileName:     fileName,
		FileSize:     int64(len(contentBytes)),
		Tags:         make([]Tag, 0),
		Metadata:     make(map[string]string),
		AnalysisTime: 0,
	}

	// Generate hashes
	result.MD5Hash = ca.generateMD5Hash(contentBytes)
	result.SHA256Hash = ca.generateSHA256Hash(contentBytes)

	// Detect content type
	result.ContentType = ca.detectContentType(fileName, contentBytes)

	// Check if content type is supported
	if !ca.isSupportedContentType(result.ContentType) {
		ca.logger.Debug("Unsupported content type: %s", result.ContentType)
		result.AnalysisTime = time.Since(startTime)
		return result, nil
	}

	// Perform content-specific analysis
	contentResult, err := ca.analyzeByContentType(ctx, result.ContentType, contentBytes)
	if err != nil {
		ca.logger.Warn("Content-specific analysis failed: %v", err)
	} else if contentResult != nil {
		// Merge results
		if contentResult.Language != "" {
			result.Language = contentResult.Language
		}
		if contentResult.Encoding != "" {
			result.Encoding = contentResult.Encoding
		}
		if contentResult.TextPreview != "" {
			result.TextPreview = contentResult.TextPreview
		}
		if contentResult.ImageProperties != nil {
			result.ImageProperties = contentResult.ImageProperties
		}
		for k, v := range contentResult.Metadata {
			result.Metadata[k] = v
		}
	}

	// Generate tags using all registered generators
	allTags := make([]Tag, 0)

	for _, generator := range ca.tagGenerators {
		tags, err := generator.GenerateTags(ctx, contentBytes, result.Metadata)
		if err != nil {
			ca.logger.Warn("Tag generator %s failed: %v", generator.GetName(), err)
			continue
		}
		allTags = append(allTags, tags...)
	}

	// Filter and deduplicate tags
	result.Tags = ca.filterTags(allTags)

	// Calculate overall confidence
	result.Confidence = ca.calculateConfidence(result.Tags)

	result.AnalysisTime = time.Since(startTime)

	ca.logger.Debug("Content analysis completed for %s: %d tags, confidence %.2f",
		fileName, len(result.Tags), result.Confidence)

	return result, nil
}

// generateMD5Hash generates MD5 hash of content
func (ca *ContentAnalyzer) generateMD5Hash(content []byte) string {
	hash := md5.Sum(content)
	return hex.EncodeToString(hash[:])
}

// generateSHA256Hash generates SHA256 hash of content
func (ca *ContentAnalyzer) generateSHA256Hash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// detectContentType detects the content type of the file
func (ca *ContentAnalyzer) detectContentType(fileName string, content []byte) string {
	// First try to detect from content
	contentType := http.DetectContentType(content)

	// If generic, try to detect from file extension
	if contentType == "application/octet-stream" || contentType == "text/plain; charset=utf-8" {
		ext := strings.ToLower(filepath.Ext(fileName))
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			contentType = mimeType
		}
	}

	return contentType
}

// isSupportedContentType checks if the content type is supported
func (ca *ContentAnalyzer) isSupportedContentType(contentType string) bool {
	for _, supported := range ca.config.SupportedMimeTypes {
		if strings.Contains(supported, "*") {
			// Wildcard matching
			prefix := strings.TrimSuffix(supported, "*")
			if strings.HasPrefix(contentType, prefix) {
				return true
			}
		} else if contentType == supported {
			return true
		}
	}
	return false
}

// analyzeByContentType performs content-type specific analysis
func (ca *ContentAnalyzer) analyzeByContentType(ctx context.Context, contentType string, content []byte) (*ContentAnalysisResult, error) {
	for _, analyzer := range ca.contentAnalyzers {
		if analyzer.CanAnalyze(contentType) {
			result, err := analyzer.Analyze(ctx, content)
			if err != nil {
				continue // Try next analyzer
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf("no analyzer found for content type: %s", contentType)
}

// filterTags filters and deduplicates tags based on confidence and limits
func (ca *ContentAnalyzer) filterTags(tags []Tag) []Tag {
	// Deduplicate tags by name
	tagMap := make(map[string]Tag)
	for _, tag := range tags {
		existing, exists := tagMap[tag.Name]
		if !exists || tag.Confidence > existing.Confidence {
			tagMap[tag.Name] = tag
		}
	}

	// Convert back to slice
	filteredTags := make([]Tag, 0, len(tagMap))
	for _, tag := range tagMap {
		if tag.Confidence >= ca.config.TagConfidenceThreshold {
			filteredTags = append(filteredTags, tag)
		}
	}

	// Sort by confidence
	sort.Slice(filteredTags, func(i, j int) bool {
		return filteredTags[i].Confidence > filteredTags[j].Confidence
	})

	// Limit number of tags
	if len(filteredTags) > ca.config.MaxTagsPerFile {
		filteredTags = filteredTags[:ca.config.MaxTagsPerFile]
	}

	return filteredTags
}

// calculateConfidence calculates overall confidence based on tags
func (ca *ContentAnalyzer) calculateConfidence(tags []Tag) float64 {
	if len(tags) == 0 {
		return 0.0
	}

	var totalConfidence float64
	for _, tag := range tags {
		totalConfidence += tag.Confidence
	}

	return totalConfidence / float64(len(tags))
}

// Default Tag Generators

// FilenameTagGenerator generates tags based on filename patterns
type FilenameTagGenerator struct{}

func (fg *FilenameTagGenerator) GenerateTags(ctx context.Context, content []byte, metadata map[string]string) ([]Tag, error) {
	fileName, ok := metadata["filename"]
	if !ok {
		return nil, fmt.Errorf("filename not provided")
	}

	var tags []Tag

	// Extract file extension
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext != "" {
		tags = append(tags, Tag{
			Name:       "ext:" + strings.TrimPrefix(ext, "."),
			Category:   "file_type",
			Confidence: 1.0,
			Source:     "filename",
		})
	}

	// Date patterns in filename
	datePatterns := []string{
		`\d{4}-\d{2}-\d{2}`, // YYYY-MM-DD
		`\d{4}_\d{2}_\d{2}`, // YYYY_MM_DD
		`\d{2}-\d{2}-\d{4}`, // MM-DD-YYYY
		`\d{8}`,             // YYYYMMDD
	}

	baseName := strings.ToLower(filepath.Base(fileName))
	for _, pattern := range datePatterns {
		if matched, _ := regexp.MatchString(pattern, baseName); matched {
			tags = append(tags, Tag{
				Name:       "has_date",
				Category:   "content",
				Confidence: 0.9,
				Source:     "filename",
			})
			break
		}
	}

	// Common filename indicators
	indicators := map[string]string{
		"backup":    "backup",
		"temp":      "temporary",
		"tmp":       "temporary",
		"log":       "log",
		"config":    "configuration",
		"readme":    "documentation",
		"license":   "legal",
		"changelog": "documentation",
		"test":      "testing",
		"spec":      "specification",
		"doc":       "documentation",
		"img":       "image",
		"pic":       "image",
		"photo":     "image",
	}

	for keyword, category := range indicators {
		if strings.Contains(baseName, keyword) {
			tags = append(tags, Tag{
				Name:       category,
				Category:   "purpose",
				Confidence: 0.8,
				Source:     "filename",
			})
		}
	}

	return tags, nil
}

func (fg *FilenameTagGenerator) GetName() string  { return "filename" }
func (fg *FilenameTagGenerator) GetPriority() int { return 100 }

// ContentTypeTagGenerator generates tags based on content type
type ContentTypeTagGenerator struct{}

func (ctg *ContentTypeTagGenerator) GenerateTags(ctx context.Context, content []byte, metadata map[string]string) ([]Tag, error) {
	contentType, ok := metadata["content_type"]
	if !ok {
		return nil, fmt.Errorf("content type not provided")
	}

	var tags []Tag

	// Main content type
	mainType := strings.Split(contentType, "/")[0]
	tags = append(tags, Tag{
		Name:       mainType,
		Category:   "content_type",
		Confidence: 1.0,
		Source:     "content",
	})

	// Specific mappings
	typeMapping := map[string][]string{
		"text/plain":       {"text", "document"},
		"text/html":        {"html", "web", "document"},
		"text/css":         {"css", "stylesheet", "web"},
		"text/javascript":  {"javascript", "code", "web"},
		"application/json": {"json", "data", "config"},
		"application/xml":  {"xml", "data", "config"},
		"application/pdf":  {"pdf", "document"},
		"image/jpeg":       {"photo", "image"},
		"image/png":        {"image", "graphics"},
		"image/gif":        {"image", "animation"},
		"image/svg+xml":    {"vector", "graphics", "web"},
		"video/mp4":        {"video", "media"},
		"audio/mpeg":       {"audio", "music", "media"},
		"application/zip":  {"archive", "compressed"},
		"application/gzip": {"archive", "compressed"},
	}

	if specificTags, exists := typeMapping[contentType]; exists {
		for _, tag := range specificTags {
			tags = append(tags, Tag{
				Name:       tag,
				Category:   "content",
				Confidence: 0.9,
				Source:     "content",
			})
		}
	}

	return tags, nil
}

func (ctg *ContentTypeTagGenerator) GetName() string  { return "content_type" }
func (ctg *ContentTypeTagGenerator) GetPriority() int { return 90 }

// SizeTagGenerator generates tags based on file size
type SizeTagGenerator struct{}

func (sg *SizeTagGenerator) GenerateTags(ctx context.Context, content []byte, metadata map[string]string) ([]Tag, error) {
	size := int64(len(content))

	var sizeTag string
	var confidence float64 = 0.7

	switch {
	case size < 1024: // < 1KB
		sizeTag = "tiny"
	case size < 1024*1024: // < 1MB
		sizeTag = "small"
	case size < 10*1024*1024: // < 10MB
		sizeTag = "medium"
	case size < 100*1024*1024: // < 100MB
		sizeTag = "large"
	default:
		sizeTag = "huge"
		confidence = 0.9
	}

	return []Tag{{
		Name:       sizeTag,
		Category:   "size",
		Confidence: confidence,
		Source:     "content",
	}}, nil
}

func (sg *SizeTagGenerator) GetName() string  { return "size" }
func (sg *SizeTagGenerator) GetPriority() int { return 50 }

// DateTagGenerator generates tags based on timestamps
type DateTagGenerator struct{}

func (dg *DateTagGenerator) GenerateTags(ctx context.Context, content []byte, metadata map[string]string) ([]Tag, error) {
	now := time.Now()
	var tags []Tag

	// Current year
	tags = append(tags, Tag{
		Name:       fmt.Sprintf("year_%d", now.Year()),
		Category:   "date",
		Confidence: 1.0,
		Source:     "metadata",
	})

	// Current month
	tags = append(tags, Tag{
		Name:       fmt.Sprintf("month_%02d", int(now.Month())),
		Category:   "date",
		Confidence: 1.0,
		Source:     "metadata",
	})

	// Day of week
	tags = append(tags, Tag{
		Name:       strings.ToLower(now.Weekday().String()),
		Category:   "date",
		Confidence: 0.8,
		Source:     "metadata",
	})

	return tags, nil
}

func (dg *DateTagGenerator) GetName() string  { return "date" }
func (dg *DateTagGenerator) GetPriority() int { return 30 }

// PatternMatcher provides pattern matching capabilities
type PatternMatcher struct {
	patterns map[string]*regexp.Regexp
}

// NewPatternMatcher creates a new pattern matcher
func NewPatternMatcher() *PatternMatcher {
	pm := &PatternMatcher{
		patterns: make(map[string]*regexp.Regexp),
	}

	// Compile common patterns
	commonPatterns := map[string]string{
		"email":       `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
		"url":         `https?://[^\s]+`,
		"ip_address":  `\b(?:\d{1,3}\.){3}\d{1,3}\b`,
		"phone":       `\b\d{3}-\d{3}-\d{4}\b`,
		"credit_card": `\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`,
		"ssn":         `\b\d{3}-\d{2}-\d{4}\b`,
	}

	for name, pattern := range commonPatterns {
		if regex, err := regexp.Compile(pattern); err == nil {
			pm.patterns[name] = regex
		}
	}

	return pm
}

// FindPatterns finds patterns in text content
func (pm *PatternMatcher) FindPatterns(content string) map[string]int {
	results := make(map[string]int)

	for name, regex := range pm.patterns {
		matches := regex.FindAllString(content, -1)
		if len(matches) > 0 {
			results[name] = len(matches)
		}
	}

	return results
}
