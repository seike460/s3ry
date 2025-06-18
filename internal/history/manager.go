package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ActionType represents the type of action performed
type ActionType string

const (
	ActionDownload     ActionType = "download"
	ActionUpload       ActionType = "upload"
	ActionDelete       ActionType = "delete"
	ActionCopy         ActionType = "copy"
	ActionMove         ActionType = "move"
	ActionList         ActionType = "list"
	ActionView         ActionType = "view"
	ActionBrowse       ActionType = "browse"
	ActionCreateBucket ActionType = "create_bucket"
	ActionDeleteBucket ActionType = "delete_bucket"
)

// HistoryEntry represents a single history entry
type HistoryEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Action    ActionType             `json:"action"`
	Bucket    string                 `json:"bucket"`
	Key       string                 `json:"key,omitempty"`
	Source    string                 `json:"source,omitempty"`
	Target    string                 `json:"target,omitempty"`
	Size      int64                  `json:"size,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Region    string                 `json:"region,omitempty"`
}

// Bookmark represents a saved location or operation
type Bookmark struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Type        BookmarkType           `json:"type"`
	Bucket      string                 `json:"bucket"`
	Prefix      string                 `json:"prefix,omitempty"`
	Region      string                 `json:"region,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	LastUsed    time.Time              `json:"last_used"`
	UseCount    int                    `json:"use_count"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// BookmarkType represents the type of bookmark
type BookmarkType string

const (
	BookmarkLocation  BookmarkType = "location"  // S3 bucket/prefix location
	BookmarkOperation BookmarkType = "operation" // Saved operation/workflow
	BookmarkQuery     BookmarkType = "query"     // Saved search query
)

// Manager manages history and bookmarks
type Manager struct {
	historyFile  string
	bookmarkFile string
	maxHistory   int
	history      []HistoryEntry
	bookmarks    []Bookmark
	autoSave     bool
}

// NewManager creates a new history and bookmark manager
func NewManager(dataDir string) (*Manager, error) {
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		dataDir = filepath.Join(homeDir, ".s3ry")
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	manager := &Manager{
		historyFile:  filepath.Join(dataDir, "history.json"),
		bookmarkFile: filepath.Join(dataDir, "bookmarks.json"),
		maxHistory:   1000, // Keep last 1000 entries
		autoSave:     true,
	}

	// Load existing data
	if err := manager.loadHistory(); err != nil {
		return nil, fmt.Errorf("failed to load history: %w", err)
	}

	if err := manager.loadBookmarks(); err != nil {
		return nil, fmt.Errorf("failed to load bookmarks: %w", err)
	}

	return manager, nil
}

// AddEntry adds a new history entry
func (m *Manager) AddEntry(entry HistoryEntry) error {
	// Generate ID if not provided
	if entry.ID == "" {
		entry.ID = generateID()
	}

	// Set timestamp if not provided
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Add to history
	m.history = append(m.history, entry)

	// Keep only the most recent entries
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}

	// Auto-save if enabled
	if m.autoSave {
		return m.saveHistory()
	}

	return nil
}

// GetHistory returns filtered history entries
func (m *Manager) GetHistory(filter HistoryFilter) []HistoryEntry {
	var filtered []HistoryEntry

	for _, entry := range m.history {
		if filter.matches(entry) {
			filtered = append(filtered, entry)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// Apply limit
	if filter.Limit > 0 && len(filtered) > filter.Limit {
		filtered = filtered[:filter.Limit]
	}

	return filtered
}

// GetRecentBuckets returns recently accessed buckets
func (m *Manager) GetRecentBuckets(limit int) []string {
	bucketMap := make(map[string]time.Time)

	// Find the most recent access time for each bucket
	for _, entry := range m.history {
		if entry.Bucket != "" {
			if lastTime, exists := bucketMap[entry.Bucket]; !exists || entry.Timestamp.After(lastTime) {
				bucketMap[entry.Bucket] = entry.Timestamp
			}
		}
	}

	// Convert to slice and sort
	type bucketTime struct {
		bucket string
		time   time.Time
	}

	var buckets []bucketTime
	for bucket, time := range bucketMap {
		buckets = append(buckets, bucketTime{bucket, time})
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].time.After(buckets[j].time)
	})

	// Return bucket names
	result := make([]string, 0, min(limit, len(buckets)))
	for i, bt := range buckets {
		if i >= limit {
			break
		}
		result = append(result, bt.bucket)
	}

	return result
}

// GetFrequentLocations returns frequently accessed locations
func (m *Manager) GetFrequentLocations(limit int) []LocationStats {
	locationMap := make(map[string]*LocationStats)

	for _, entry := range m.history {
		if entry.Bucket != "" {
			location := entry.Bucket
			if entry.Key != "" {
				// Use the directory part of the key
				dir := filepath.Dir(entry.Key)
				if dir != "." {
					location = entry.Bucket + "/" + dir
				}
			}

			if stats, exists := locationMap[location]; exists {
				stats.AccessCount++
				if entry.Timestamp.After(stats.LastAccess) {
					stats.LastAccess = entry.Timestamp
				}
			} else {
				locationMap[location] = &LocationStats{
					Location:    location,
					AccessCount: 1,
					LastAccess:  entry.Timestamp,
				}
			}
		}
	}

	// Convert to slice and sort by access count
	var locations []LocationStats
	for _, stats := range locationMap {
		locations = append(locations, *stats)
	}

	sort.Slice(locations, func(i, j int) bool {
		if locations[i].AccessCount == locations[j].AccessCount {
			return locations[i].LastAccess.After(locations[j].LastAccess)
		}
		return locations[i].AccessCount > locations[j].AccessCount
	})

	// Apply limit
	if limit > 0 && len(locations) > limit {
		locations = locations[:limit]
	}

	return locations
}

// AddBookmark adds a new bookmark
func (m *Manager) AddBookmark(bookmark Bookmark) error {
	// Generate ID if not provided
	if bookmark.ID == "" {
		bookmark.ID = generateID()
	}

	// Set timestamps
	if bookmark.CreatedAt.IsZero() {
		bookmark.CreatedAt = time.Now()
	}
	if bookmark.LastUsed.IsZero() {
		bookmark.LastUsed = time.Now()
	}

	// Check for duplicate names
	for _, existing := range m.bookmarks {
		if existing.Name == bookmark.Name {
			return fmt.Errorf("bookmark with name '%s' already exists", bookmark.Name)
		}
	}

	m.bookmarks = append(m.bookmarks, bookmark)

	// Auto-save if enabled
	if m.autoSave {
		return m.saveBookmarks()
	}

	return nil
}

// GetBookmarks returns filtered bookmarks
func (m *Manager) GetBookmarks(filter BookmarkFilter) []Bookmark {
	var filtered []Bookmark

	for _, bookmark := range m.bookmarks {
		if filter.matches(bookmark) {
			filtered = append(filtered, bookmark)
		}
	}

	// Sort by last used (most recent first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].LastUsed.After(filtered[j].LastUsed)
	})

	return filtered
}

// UseBookmark updates bookmark usage statistics
func (m *Manager) UseBookmark(id string) error {
	for i, bookmark := range m.bookmarks {
		if bookmark.ID == id {
			m.bookmarks[i].UseCount++
			m.bookmarks[i].LastUsed = time.Now()

			if m.autoSave {
				return m.saveBookmarks()
			}
			return nil
		}
	}

	return fmt.Errorf("bookmark with ID '%s' not found", id)
}

// DeleteBookmark removes a bookmark
func (m *Manager) DeleteBookmark(id string) error {
	for i, bookmark := range m.bookmarks {
		if bookmark.ID == id {
			m.bookmarks = append(m.bookmarks[:i], m.bookmarks[i+1:]...)

			if m.autoSave {
				return m.saveBookmarks()
			}
			return nil
		}
	}

	return fmt.Errorf("bookmark with ID '%s' not found", id)
}

// UpdateBookmark updates an existing bookmark
func (m *Manager) UpdateBookmark(id string, updates Bookmark) error {
	for i, bookmark := range m.bookmarks {
		if bookmark.ID == id {
			// Preserve certain fields
			updates.ID = bookmark.ID
			updates.CreatedAt = bookmark.CreatedAt
			updates.UseCount = bookmark.UseCount

			m.bookmarks[i] = updates

			if m.autoSave {
				return m.saveBookmarks()
			}
			return nil
		}
	}

	return fmt.Errorf("bookmark with ID '%s' not found", id)
}

// GetBookmarkTags returns all unique tags used in bookmarks
func (m *Manager) GetBookmarkTags() []string {
	tagSet := make(map[string]bool)

	for _, bookmark := range m.bookmarks {
		for _, tag := range bookmark.Tags {
			tagSet[tag] = true
		}
	}

	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	sort.Strings(tags)
	return tags
}

// ClearHistory removes all history entries
func (m *Manager) ClearHistory() error {
	m.history = nil

	if m.autoSave {
		return m.saveHistory()
	}

	return nil
}

// ExportHistory exports history to a file
func (m *Manager) ExportHistory(filename string) error {
	data, err := json.MarshalIndent(m.history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// ExportBookmarks exports bookmarks to a file
func (m *Manager) ExportBookmarks(filename string) error {
	data, err := json.MarshalIndent(m.bookmarks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal bookmarks: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// ImportBookmarks imports bookmarks from a file
func (m *Manager) ImportBookmarks(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read bookmarks file: %w", err)
	}

	var imported []Bookmark
	if err := json.Unmarshal(data, &imported); err != nil {
		return fmt.Errorf("failed to unmarshal bookmarks: %w", err)
	}

	// Add imported bookmarks (with duplicate checking)
	for _, bookmark := range imported {
		// Generate new ID to avoid conflicts
		bookmark.ID = generateID()

		// Check for name conflicts
		originalName := bookmark.Name
		counter := 1
		for m.bookmarkNameExists(bookmark.Name) {
			bookmark.Name = fmt.Sprintf("%s (%d)", originalName, counter)
			counter++
		}

		m.bookmarks = append(m.bookmarks, bookmark)
	}

	if m.autoSave {
		return m.saveBookmarks()
	}

	return nil
}

// GetStats returns usage statistics
func (m *Manager) GetStats() Stats {
	stats := Stats{
		TotalHistoryEntries: len(m.history),
		TotalBookmarks:      len(m.bookmarks),
		ActionCounts:        make(map[ActionType]int),
		BucketCounts:        make(map[string]int),
	}

	// Count actions and buckets
	for _, entry := range m.history {
		stats.ActionCounts[entry.Action]++
		if entry.Bucket != "" {
			stats.BucketCounts[entry.Bucket]++
		}

		if entry.Timestamp.After(stats.LastActivity) {
			stats.LastActivity = entry.Timestamp
		}

		if stats.FirstActivity.IsZero() || entry.Timestamp.Before(stats.FirstActivity) {
			stats.FirstActivity = entry.Timestamp
		}
	}

	return stats
}

// Helper functions
func (m *Manager) loadHistory() error {
	if _, err := os.Stat(m.historyFile); os.IsNotExist(err) {
		m.history = []HistoryEntry{}
		return nil
	}

	data, err := os.ReadFile(m.historyFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.history)
}

func (m *Manager) saveHistory() error {
	data, err := json.MarshalIndent(m.history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.historyFile, data, 0644)
}

func (m *Manager) loadBookmarks() error {
	if _, err := os.Stat(m.bookmarkFile); os.IsNotExist(err) {
		m.bookmarks = []Bookmark{}
		return nil
	}

	data, err := os.ReadFile(m.bookmarkFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.bookmarks)
}

func (m *Manager) saveBookmarks() error {
	data, err := json.MarshalIndent(m.bookmarks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.bookmarkFile, data, 0644)
}

func (m *Manager) bookmarkNameExists(name string) bool {
	for _, bookmark := range m.bookmarks {
		if bookmark.Name == name {
			return true
		}
	}
	return false
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Filter types and helper methods
type HistoryFilter struct {
	Actions     []ActionType `json:"actions,omitempty"`
	Buckets     []string     `json:"buckets,omitempty"`
	TimeFrom    *time.Time   `json:"time_from,omitempty"`
	TimeTo      *time.Time   `json:"time_to,omitempty"`
	SuccessOnly bool         `json:"success_only,omitempty"`
	SearchText  string       `json:"search_text,omitempty"`
	Limit       int          `json:"limit,omitempty"`
}

func (f HistoryFilter) matches(entry HistoryEntry) bool {
	// Check actions
	if len(f.Actions) > 0 {
		found := false
		for _, action := range f.Actions {
			if entry.Action == action {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check buckets
	if len(f.Buckets) > 0 {
		found := false
		for _, bucket := range f.Buckets {
			if entry.Bucket == bucket {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check time range
	if f.TimeFrom != nil && entry.Timestamp.Before(*f.TimeFrom) {
		return false
	}
	if f.TimeTo != nil && entry.Timestamp.After(*f.TimeTo) {
		return false
	}

	// Check success only
	if f.SuccessOnly && !entry.Success {
		return false
	}

	// Check search text
	if f.SearchText != "" {
		searchLower := strings.ToLower(f.SearchText)
		if !strings.Contains(strings.ToLower(entry.Bucket), searchLower) &&
			!strings.Contains(strings.ToLower(entry.Key), searchLower) &&
			!strings.Contains(strings.ToLower(entry.Source), searchLower) &&
			!strings.Contains(strings.ToLower(entry.Target), searchLower) {
			return false
		}
	}

	return true
}

type BookmarkFilter struct {
	Types      []BookmarkType `json:"types,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Buckets    []string       `json:"buckets,omitempty"`
	SearchText string         `json:"search_text,omitempty"`
}

func (f BookmarkFilter) matches(bookmark Bookmark) bool {
	// Check types
	if len(f.Types) > 0 {
		found := false
		for _, bookmarkType := range f.Types {
			if bookmark.Type == bookmarkType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check tags
	if len(f.Tags) > 0 {
		for _, filterTag := range f.Tags {
			found := false
			for _, bookmarkTag := range bookmark.Tags {
				if bookmarkTag == filterTag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Check buckets
	if len(f.Buckets) > 0 {
		found := false
		for _, bucket := range f.Buckets {
			if bookmark.Bucket == bucket {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check search text
	if f.SearchText != "" {
		searchLower := strings.ToLower(f.SearchText)
		if !strings.Contains(strings.ToLower(bookmark.Name), searchLower) &&
			!strings.Contains(strings.ToLower(bookmark.Description), searchLower) &&
			!strings.Contains(strings.ToLower(bookmark.Bucket), searchLower) &&
			!strings.Contains(strings.ToLower(bookmark.Prefix), searchLower) {
			return false
		}
	}

	return true
}

// Additional types
type LocationStats struct {
	Location    string    `json:"location"`
	AccessCount int       `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
}

type Stats struct {
	TotalHistoryEntries int                `json:"total_history_entries"`
	TotalBookmarks      int                `json:"total_bookmarks"`
	ActionCounts        map[ActionType]int `json:"action_counts"`
	BucketCounts        map[string]int     `json:"bucket_counts"`
	FirstActivity       time.Time          `json:"first_activity"`
	LastActivity        time.Time          `json:"last_activity"`
}
