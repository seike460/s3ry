package ai

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DuplicateDetector provides intelligent duplicate file detection and consolidation
type DuplicateDetector struct {
	config      *DetectorConfig
	hashCache   map[string]*FileFingerprint
	similarityCache map[string][]string
	mu          sync.RWMutex
	logger      Logger
}

// DetectorConfig configures the duplicate detector
type DetectorConfig struct {
	HashAlgorithms         []string      `json:"hash_algorithms"`
	EnableContentAnalysis  bool          `json:"enable_content_analysis"`
	EnableSimilarityCheck  bool          `json:"enable_similarity_check"`
	SimilarityThreshold    float64       `json:"similarity_threshold"`
	MaxFileSize           int64         `json:"max_file_size"`
	ChunkSize             int           `json:"chunk_size"`
	ConcurrentWorkers     int           `json:"concurrent_workers"`
	CacheEnabled          bool          `json:"cache_enabled"`
	CacheTTL              time.Duration `json:"cache_ttl"`
}

// DefaultDetectorConfig returns default detector configuration
func DefaultDetectorConfig() *DetectorConfig {
	return &DetectorConfig{
		HashAlgorithms:        []string{"md5", "sha256"},
		EnableContentAnalysis: true,
		EnableSimilarityCheck: true,
		SimilarityThreshold:   0.95,
		MaxFileSize:          1024 * 1024 * 1024, // 1GB
		ChunkSize:            8192,
		ConcurrentWorkers:    4,
		CacheEnabled:         true,
		CacheTTL:             24 * time.Hour,
	}
}

// FileFingerprint represents a unique fingerprint of a file
type FileFingerprint struct {
	FilePath      string            `json:"file_path"`
	FileName      string            `json:"file_name"`
	FileSize      int64             `json:"file_size"`
	ModifiedTime  time.Time         `json:"modified_time"`
	MD5Hash       string            `json:"md5_hash,omitempty"`
	SHA256Hash    string            `json:"sha256_hash,omitempty"`
	ContentType   string            `json:"content_type"`
	FirstBytes    string            `json:"first_bytes"`   // First 1KB as hex
	LastBytes     string            `json:"last_bytes"`    // Last 1KB as hex
	ChunkHashes   []string          `json:"chunk_hashes"`  // Hashes of file chunks
	TextContent   string            `json:"text_content,omitempty"`
	Metadata      map[string]string `json:"metadata"`
	CreatedAt     time.Time         `json:"created_at"`
}

// DuplicateGroup represents a group of duplicate files
type DuplicateGroup struct {
	GroupID        string             `json:"group_id"`
	Files          []FileInfo         `json:"files"`
	DuplicateType  DuplicateType      `json:"duplicate_type"`
	Confidence     float64            `json:"confidence"`
	TotalSize      int64              `json:"total_size"`
	PotentialSavings int64            `json:"potential_savings"`
	Recommendation *ConsolidationRec  `json:"recommendation"`
	CreatedAt      time.Time          `json:"created_at"`
}

// FileInfo contains information about a file in a duplicate group
type FileInfo struct {
	FilePath     string            `json:"file_path"`
	FileName     string            `json:"file_name"`
	FileSize     int64             `json:"file_size"`
	ModifiedTime time.Time         `json:"modified_time"`
	Bucket       string            `json:"bucket,omitempty"`
	IsRecommendedKeep bool          `json:"is_recommended_keep"`
	Reason       string            `json:"reason,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// DuplicateType represents the type of duplication detected
type DuplicateType int

const (
	ExactDuplicate DuplicateType = iota
	ContentSimilar
	NameSimilar
	PartialDuplicate
)

func (dt DuplicateType) String() string {
	switch dt {
	case ExactDuplicate:
		return "exact"
	case ContentSimilar:
		return "content_similar"
	case NameSimilar:
		return "name_similar"
	case PartialDuplicate:
		return "partial"
	default:
		return "unknown"
	}
}

// ConsolidationRec represents a recommendation for consolidating duplicates
type ConsolidationRec struct {
	Action          ConsolidationAction `json:"action"`
	KeepFile        string              `json:"keep_file"`
	RemoveFiles     []string            `json:"remove_files"`
	Reason          string              `json:"reason"`
	EstimatedSavings int64              `json:"estimated_savings"`
	RiskLevel       RiskLevel           `json:"risk_level"`
	AutoApproved    bool                `json:"auto_approved"`
}

// ConsolidationAction represents the type of consolidation action
type ConsolidationAction int

const (
	NoAction ConsolidationAction = iota
	DeleteDuplicates
	MoveToArchive
	CreateSymlinks
	MergeMetadata
)

func (ca ConsolidationAction) String() string {
	switch ca {
	case NoAction:
		return "no_action"
	case DeleteDuplicates:
		return "delete"
	case MoveToArchive:
		return "archive"
	case CreateSymlinks:
		return "symlink"
	case MergeMetadata:
		return "merge_metadata"
	default:
		return "unknown"
	}
}

// RiskLevel represents the risk level of a consolidation action
type RiskLevel int

const (
	LowRisk RiskLevel = iota
	MediumRisk
	HighRisk
)

func (rl RiskLevel) String() string {
	switch rl {
	case LowRisk:
		return "low"
	case MediumRisk:
		return "medium"
	case HighRisk:
		return "high"
	default:
		return "unknown"
	}
}

// NewDuplicateDetector creates a new duplicate detector
func NewDuplicateDetector(config *DetectorConfig, logger Logger) *DuplicateDetector {
	if config == nil {
		config = DefaultDetectorConfig()
	}

	return &DuplicateDetector{
		config:          config,
		hashCache:       make(map[string]*FileFingerprint),
		similarityCache: make(map[string][]string),
		logger:          logger,
	}
}

// AnalyzeFile creates a fingerprint for a file
func (dd *DuplicateDetector) AnalyzeFile(ctx context.Context, filePath string, content io.Reader) (*FileFingerprint, error) {
	fingerprint := &FileFingerprint{
		FilePath:    filePath,
		FileName:    filepath.Base(filePath),
		Metadata:    make(map[string]string),
		CreatedAt:   time.Now(),
	}

	// Read content into buffer
	contentBytes, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	fingerprint.FileSize = int64(len(contentBytes))

	// Check file size limit
	if fingerprint.FileSize > dd.config.MaxFileSize {
		return nil, fmt.Errorf("file size exceeds limit: %d > %d", fingerprint.FileSize, dd.config.MaxFileSize)
	}

	// Generate hashes
	if err := dd.generateHashes(fingerprint, contentBytes); err != nil {
		return nil, fmt.Errorf("failed to generate hashes: %w", err)
	}

	// Extract first and last bytes
	if err := dd.extractSignatureBytes(fingerprint, contentBytes); err != nil {
		return nil, fmt.Errorf("failed to extract signature bytes: %w", err)
	}

	// Generate chunk hashes for similarity detection
	if dd.config.EnableSimilarityCheck {
		if err := dd.generateChunkHashes(fingerprint, contentBytes); err != nil {
			dd.logger.Warn("Failed to generate chunk hashes: %v", err)
		}
	}

	// Content analysis for text files
	if dd.config.EnableContentAnalysis {
		if err := dd.analyzeTextContent(fingerprint, contentBytes); err != nil {
			dd.logger.Warn("Failed to analyze text content: %v", err)
		}
	}

	// Cache the fingerprint
	if dd.config.CacheEnabled {
		dd.mu.Lock()
		dd.hashCache[filePath] = fingerprint
		dd.mu.Unlock()
	}

	return fingerprint, nil
}

// FindDuplicates finds duplicate files from a list of fingerprints
func (dd *DuplicateDetector) FindDuplicates(ctx context.Context, fingerprints []*FileFingerprint) ([]DuplicateGroup, error) {
	var duplicateGroups []DuplicateGroup

	// Group by hash for exact duplicates
	exactGroups := dd.findExactDuplicates(fingerprints)
	duplicateGroups = append(duplicateGroups, exactGroups...)

	// Find similar files if enabled
	if dd.config.EnableSimilarityCheck {
		similarGroups := dd.findSimilarFiles(fingerprints)
		duplicateGroups = append(duplicateGroups, similarGroups...)
	}

	// Generate recommendations for each group
	for i := range duplicateGroups {
		duplicateGroups[i].Recommendation = dd.generateRecommendation(&duplicateGroups[i])
	}

	// Sort by potential savings (descending)
	sort.Slice(duplicateGroups, func(i, j int) bool {
		return duplicateGroups[i].PotentialSavings > duplicateGroups[j].PotentialSavings
	})

	dd.logger.Info("Found %d duplicate groups", len(duplicateGroups))
	return duplicateGroups, nil
}

// generateHashes generates various hashes for the file content
func (dd *DuplicateDetector) generateHashes(fingerprint *FileFingerprint, content []byte) error {
	for _, algorithm := range dd.config.HashAlgorithms {
		switch algorithm {
		case "md5":
			hash := md5.Sum(content)
			fingerprint.MD5Hash = hex.EncodeToString(hash[:])
		case "sha256":
			hash := sha256.Sum256(content)
			fingerprint.SHA256Hash = hex.EncodeToString(hash[:])
		default:
			dd.logger.Warn("Unsupported hash algorithm: %s", algorithm)
		}
	}
	return nil
}

// extractSignatureBytes extracts first and last bytes for quick comparison
func (dd *DuplicateDetector) extractSignatureBytes(fingerprint *FileFingerprint, content []byte) error {
	chunkSize := 1024 // 1KB

	// First bytes
	if len(content) > 0 {
		end := chunkSize
		if end > len(content) {
			end = len(content)
		}
		fingerprint.FirstBytes = hex.EncodeToString(content[:end])
	}

	// Last bytes
	if len(content) > chunkSize {
		start := len(content) - chunkSize
		fingerprint.LastBytes = hex.EncodeToString(content[start:])
	} else if len(content) > 0 {
		fingerprint.LastBytes = fingerprint.FirstBytes
	}

	return nil
}

// generateChunkHashes generates hashes of file chunks for similarity detection
func (dd *DuplicateDetector) generateChunkHashes(fingerprint *FileFingerprint, content []byte) error {
	chunkSize := dd.config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 8192
	}

	fingerprint.ChunkHashes = make([]string, 0)

	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}

		chunk := content[i:end]
		hash := md5.Sum(chunk)
		fingerprint.ChunkHashes = append(fingerprint.ChunkHashes, hex.EncodeToString(hash[:]))
	}

	return nil
}

// analyzeTextContent analyzes text content for similarity detection
func (dd *DuplicateDetector) analyzeTextContent(fingerprint *FileFingerprint, content []byte) error {
	// Check if content is text
	if !dd.isTextContent(content) {
		return nil
	}

	text := string(content)
	
	// Store normalized text content (for similarity analysis)
	normalizedText := dd.normalizeText(text)
	if len(normalizedText) > 1000 {
		fingerprint.TextContent = normalizedText[:1000] // Store first 1000 chars
	} else {
		fingerprint.TextContent = normalizedText
	}

	// Extract metadata
	fingerprint.Metadata["line_count"] = fmt.Sprintf("%d", strings.Count(text, "\n")+1)
	fingerprint.Metadata["word_count"] = fmt.Sprintf("%d", len(strings.Fields(text)))
	fingerprint.Metadata["char_count"] = fmt.Sprintf("%d", len(text))

	return nil
}

// isTextContent checks if content is text
func (dd *DuplicateDetector) isTextContent(content []byte) bool {
	// Simple heuristic: check if first 512 bytes are printable
	checkSize := 512
	if len(content) < checkSize {
		checkSize = len(content)
	}

	printableCount := 0
	for i := 0; i < checkSize; i++ {
		if content[i] >= 32 && content[i] <= 126 || content[i] == 9 || content[i] == 10 || content[i] == 13 {
			printableCount++
		}
	}

	return float64(printableCount)/float64(checkSize) > 0.95
}

// normalizeText normalizes text for comparison
func (dd *DuplicateDetector) normalizeText(text string) string {
	// Convert to lowercase
	text = strings.ToLower(text)
	
	// Remove extra whitespace
	text = strings.Join(strings.Fields(text), " ")
	
	// Remove common punctuation for comparison
	replacements := []string{
		".", "",
		",", "",
		";", "",
		":", "",
		"!", "",
		"?", "",
		"\"", "",
		"'", "",
	}
	
	for i := 0; i < len(replacements); i += 2 {
		text = strings.ReplaceAll(text, replacements[i], replacements[i+1])
	}

	return text
}

// findExactDuplicates finds files with identical hashes
func (dd *DuplicateDetector) findExactDuplicates(fingerprints []*FileFingerprint) []DuplicateGroup {
	var groups []DuplicateGroup

	// Group by primary hash (prefer SHA256, fallback to MD5)
	hashGroups := make(map[string][]*FileFingerprint)

	for _, fp := range fingerprints {
		var primaryHash string
		if fp.SHA256Hash != "" {
			primaryHash = "sha256:" + fp.SHA256Hash
		} else if fp.MD5Hash != "" {
			primaryHash = "md5:" + fp.MD5Hash
		} else {
			continue // Skip files without hashes
		}

		hashGroups[primaryHash] = append(hashGroups[primaryHash], fp)
	}

	// Create duplicate groups for hashes with multiple files
	for hash, fps := range hashGroups {
		if len(fps) < 2 {
			continue
		}

		group := DuplicateGroup{
			GroupID:       hash,
			Files:         make([]FileInfo, len(fps)),
			DuplicateType: ExactDuplicate,
			Confidence:    1.0,
			CreatedAt:     time.Now(),
		}

		totalSize := int64(0)
		for i, fp := range fps {
			group.Files[i] = FileInfo{
				FilePath:     fp.FilePath,
				FileName:     fp.FileName,
				FileSize:     fp.FileSize,
				ModifiedTime: fp.ModifiedTime,
				Metadata:     fp.Metadata,
			}
			totalSize += fp.FileSize
		}

		group.TotalSize = totalSize
		group.PotentialSavings = totalSize - fps[0].FileSize // Keep one, save the rest

		groups = append(groups, group)
	}

	return groups
}

// findSimilarFiles finds files with similar content
func (dd *DuplicateDetector) findSimilarFiles(fingerprints []*FileFingerprint) []DuplicateGroup {
	var groups []DuplicateGroup

	// Group by file size first (optimization)
	sizeGroups := make(map[int64][]*FileFingerprint)
	for _, fp := range fingerprints {
		sizeGroups[fp.FileSize] = append(sizeGroups[fp.FileSize], fp)
	}

	// Find similar files within each size group
	for size, fps := range sizeGroups {
		if len(fps) < 2 || size == 0 {
			continue
		}

		similarPairs := dd.findSimilarPairs(fps)
		for _, pair := range similarPairs {
			if len(pair) < 2 {
				continue
			}

			groupID := fmt.Sprintf("similar_%d_%s", size, pair[0].MD5Hash[:8])
			group := DuplicateGroup{
				GroupID:       groupID,
				Files:         make([]FileInfo, len(pair)),
				DuplicateType: ContentSimilar,
				Confidence:    dd.calculateSimilarity(pair[0], pair[1]),
				CreatedAt:     time.Now(),
			}

			totalSize := int64(0)
			for i, fp := range pair {
				group.Files[i] = FileInfo{
					FilePath:     fp.FilePath,
					FileName:     fp.FileName,
					FileSize:     fp.FileSize,
					ModifiedTime: fp.ModifiedTime,
					Metadata:     fp.Metadata,
				}
				totalSize += fp.FileSize
			}

			group.TotalSize = totalSize
			group.PotentialSavings = totalSize - pair[0].FileSize // Conservative estimate

			if group.Confidence >= dd.config.SimilarityThreshold {
				groups = append(groups, group)
			}
		}
	}

	return groups
}

// findSimilarPairs finds pairs of similar files
func (dd *DuplicateDetector) findSimilarPairs(fingerprints []*FileFingerprint) [][]*FileFingerprint {
	var pairs [][]*FileFingerprint

	for i := 0; i < len(fingerprints); i++ {
		for j := i + 1; j < len(fingerprints); j++ {
			similarity := dd.calculateSimilarity(fingerprints[i], fingerprints[j])
			if similarity >= dd.config.SimilarityThreshold {
				pairs = append(pairs, []*FileFingerprint{fingerprints[i], fingerprints[j]})
			}
		}
	}

	return pairs
}

// calculateSimilarity calculates similarity between two files
func (dd *DuplicateDetector) calculateSimilarity(fp1, fp2 *FileFingerprint) float64 {
	var totalScore float64
	var weightSum float64

	// File name similarity (weight: 0.2)
	nameScore := dd.calculateNameSimilarity(fp1.FileName, fp2.FileName)
	totalScore += nameScore * 0.2
	weightSum += 0.2

	// First/last bytes similarity (weight: 0.3)
	if fp1.FirstBytes != "" && fp2.FirstBytes != "" {
		if fp1.FirstBytes == fp2.FirstBytes {
			totalScore += 1.0 * 0.3
		}
		weightSum += 0.3
	}

	// Chunk similarity (weight: 0.4)
	if len(fp1.ChunkHashes) > 0 && len(fp2.ChunkHashes) > 0 {
		chunkScore := dd.calculateChunkSimilarity(fp1.ChunkHashes, fp2.ChunkHashes)
		totalScore += chunkScore * 0.4
		weightSum += 0.4
	}

	// Text content similarity (weight: 0.1)
	if fp1.TextContent != "" && fp2.TextContent != "" {
		textScore := dd.calculateTextSimilarity(fp1.TextContent, fp2.TextContent)
		totalScore += textScore * 0.1
		weightSum += 0.1
	}

	if weightSum == 0 {
		return 0.0
	}

	return totalScore / weightSum
}

// calculateNameSimilarity calculates filename similarity
func (dd *DuplicateDetector) calculateNameSimilarity(name1, name2 string) float64 {
	// Levenshtein distance-based similarity
	distance := dd.levenshteinDistance(strings.ToLower(name1), strings.ToLower(name2))
	maxLen := len(name1)
	if len(name2) > maxLen {
		maxLen = len(name2)
	}
	
	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - float64(distance)/float64(maxLen)
}

// calculateChunkSimilarity calculates similarity between chunk hashes
func (dd *DuplicateDetector) calculateChunkSimilarity(chunks1, chunks2 []string) float64 {
	if len(chunks1) == 0 || len(chunks2) == 0 {
		return 0.0
	}

	// Create a set of chunks from the first file
	chunkSet := make(map[string]bool)
	for _, chunk := range chunks1 {
		chunkSet[chunk] = true
	}

	// Count matches in the second file
	matches := 0
	for _, chunk := range chunks2 {
		if chunkSet[chunk] {
			matches++
		}
	}

	// Calculate Jaccard similarity
	union := len(chunks1) + len(chunks2) - matches
	if union == 0 {
		return 1.0
	}

	return float64(matches) / float64(union)
}

// calculateTextSimilarity calculates text content similarity
func (dd *DuplicateDetector) calculateTextSimilarity(text1, text2 string) float64 {
	// Simple word-based similarity
	words1 := strings.Fields(text1)
	words2 := strings.Fields(text2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	wordSet := make(map[string]bool)
	for _, word := range words1 {
		wordSet[word] = true
	}

	matches := 0
	for _, word := range words2 {
		if wordSet[word] {
			matches++
		}
	}

	union := len(words1) + len(words2) - matches
	if union == 0 {
		return 1.0
	}

	return float64(matches) / float64(union)
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func (dd *DuplicateDetector) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			if s1[i-1] == s2[j-1] {
				matrix[i][j] = matrix[i-1][j-1]
			} else {
				matrix[i][j] = min(
					matrix[i-1][j]+1,   // deletion
					matrix[i][j-1]+1,   // insertion
					matrix[i-1][j-1]+1, // substitution
				)
			}
		}
	}

	return matrix[len(s1)][len(s2)]
}

// generateRecommendation generates a consolidation recommendation for a duplicate group
func (dd *DuplicateDetector) generateRecommendation(group *DuplicateGroup) *ConsolidationRec {
	rec := &ConsolidationRec{
		Action:           NoAction,
		RemoveFiles:      make([]string, 0),
		EstimatedSavings: 0,
		RiskLevel:        MediumRisk,
		AutoApproved:     false,
	}

	if len(group.Files) < 2 {
		return rec
	}

	// Sort files by preference (newest, shortest path, etc.)
	sortedFiles := make([]FileInfo, len(group.Files))
	copy(sortedFiles, group.Files)
	
	sort.Slice(sortedFiles, func(i, j int) bool {
		// Prefer newer files
		if !sortedFiles[i].ModifiedTime.Equal(sortedFiles[j].ModifiedTime) {
			return sortedFiles[i].ModifiedTime.After(sortedFiles[j].ModifiedTime)
		}
		// Prefer shorter paths (likely more organized)
		return len(sortedFiles[i].FilePath) < len(sortedFiles[j].FilePath)
	})

	// Keep the best file
	rec.KeepFile = sortedFiles[0].FilePath
	
	// Remove the rest
	for i := 1; i < len(sortedFiles); i++ {
		rec.RemoveFiles = append(rec.RemoveFiles, sortedFiles[i].FilePath)
		rec.EstimatedSavings += sortedFiles[i].FileSize
	}

	// Determine action based on duplicate type and confidence
	switch group.DuplicateType {
	case ExactDuplicate:
		if group.Confidence == 1.0 {
			rec.Action = DeleteDuplicates
			rec.RiskLevel = LowRisk
			rec.AutoApproved = true
			rec.Reason = "Exact duplicates detected with identical content"
		}
	case ContentSimilar:
		if group.Confidence >= 0.98 {
			rec.Action = DeleteDuplicates
			rec.RiskLevel = MediumRisk
			rec.Reason = "Highly similar content detected"
		} else {
			rec.Action = MoveToArchive
			rec.RiskLevel = MediumRisk
			rec.Reason = "Similar content detected, recommend manual review"
		}
	case NameSimilar:
		rec.Action = MoveToArchive
		rec.RiskLevel = HighRisk
		rec.Reason = "Similar filenames detected, manual review recommended"
	}

	return rec
}

// Helper function for min
func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}