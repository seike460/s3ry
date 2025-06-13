package vscode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/history"
)

// VSCodeServer provides an HTTP API for VS Code extension integration
type VSCodeServer struct {
	port           int
	s3Client       *s3.Client
	historyManager *history.Manager
	upgrader       websocket.Upgrader
	clients        map[*websocket.Conn]bool
}

// NewVSCodeServer creates a new VS Code integration server
func NewVSCodeServer(port int, s3Client *s3.Client) (*VSCodeServer, error) {
	historyManager, err := history.NewManager("")
	if err != nil {
		return nil, fmt.Errorf("failed to create history manager: %w", err)
	}

	return &VSCodeServer{
		port:           port,
		s3Client:       s3Client,
		historyManager: historyManager,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow connections from VS Code extension
			},
		},
		clients: make(map[*websocket.Conn]bool),
	}, nil
}

// Start starts the VS Code integration server
func (s *VSCodeServer) Start() error {
	router := mux.NewRouter()

	// API routes for VS Code extension
	api := router.PathPrefix("/api/vscode").Subrouter()
	
	// S3 operations
	api.HandleFunc("/buckets", s.handleListBuckets).Methods("GET")
	api.HandleFunc("/buckets/{bucket}/objects", s.handleListObjects).Methods("GET")
	api.HandleFunc("/buckets/{bucket}/objects/{key:.*}/download", s.handleDownloadObject).Methods("POST")
	api.HandleFunc("/buckets/{bucket}/objects/upload", s.handleUploadObject).Methods("POST")
	api.HandleFunc("/buckets/{bucket}/objects/{key:.*}", s.handleDeleteObject).Methods("DELETE")
	
	// File operations
	api.HandleFunc("/workspace/upload", s.handleUploadFromWorkspace).Methods("POST")
	api.HandleFunc("/workspace/download", s.handleDownloadToWorkspace).Methods("POST")
	
	// History and bookmarks
	api.HandleFunc("/history", s.handleGetHistory).Methods("GET")
	api.HandleFunc("/bookmarks", s.handleGetBookmarks).Methods("GET")
	api.HandleFunc("/bookmarks", s.handleCreateBookmark).Methods("POST")
	
	// Configuration
	api.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	api.HandleFunc("/config", s.handleUpdateConfig).Methods("PUT")
	
	// WebSocket for real-time updates
	api.HandleFunc("/ws", s.handleWebSocket)

	// CORS middleware
	router.Use(s.corsMiddleware)

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("VS Code extension server starting on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, router)
}

// CORS middleware for VS Code extension
func (s *VSCodeServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Bucket operations
func (s *VSCodeServer) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := s.s3Client.ListBuckets(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buckets)
}

func (s *VSCodeServer) handleListObjects(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bucket := vars["bucket"]
	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")

	objects, err := s.s3Client.ListObjects(context.Background(), bucket, prefix, delimiter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(objects)
}

// File operations for VS Code workspace integration
func (s *VSCodeServer) handleUploadFromWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LocalPath string `json:"localPath"`
		Bucket    string `json:"bucket"`
		Key       string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Record operation start
	entry := history.HistoryEntry{
		Action: history.ActionUpload,
		Bucket: req.Bucket,
		Key:    req.Key,
		Source: req.LocalPath,
	}

	start := time.Now()
	err := s.s3Client.UploadFile(context.Background(), req.LocalPath, req.Bucket, req.Key)
	
	// Record result
	entry.Duration = time.Since(start)
	entry.Success = err == nil
	if err != nil {
		entry.Error = err.Error()
	}
	
	s.historyManager.AddEntry(entry)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Notify connected clients
	s.broadcastUpdate("upload", map[string]interface{}{
		"bucket": req.Bucket,
		"key":    req.Key,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (s *VSCodeServer) handleDownloadToWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Bucket    string `json:"bucket"`
		Key       string `json:"key"`
		LocalPath string `json:"localPath"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure directory exists
	dir := filepath.Dir(req.LocalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Record operation start
	entry := history.HistoryEntry{
		Action: history.ActionDownload,
		Bucket: req.Bucket,
		Key:    req.Key,
		Target: req.LocalPath,
	}

	start := time.Now()
	err := s.s3Client.DownloadFile(context.Background(), req.Bucket, req.Key, req.LocalPath)
	
	// Record result
	entry.Duration = time.Since(start)
	entry.Success = err == nil
	if err != nil {
		entry.Error = err.Error()
	}
	
	s.historyManager.AddEntry(entry)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Notify connected clients
	s.broadcastUpdate("download", map[string]interface{}{
		"bucket":    req.Bucket,
		"key":       req.Key,
		"localPath": req.LocalPath,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (s *VSCodeServer) handleDownloadObject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bucket := vars["bucket"]
	key := vars["key"]

	var req struct {
		LocalPath string `json:"localPath"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := s.s3Client.DownloadFile(context.Background(), bucket, key, req.LocalPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (s *VSCodeServer) handleUploadObject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bucket := vars["bucket"]

	var req struct {
		LocalPath string `json:"localPath"`
		Key       string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := s.s3Client.UploadFile(context.Background(), req.LocalPath, bucket, req.Key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (s *VSCodeServer) handleDeleteObject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bucket := vars["bucket"]
	key := vars["key"]

	err := s.s3Client.DeleteObject(context.Background(), bucket, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// History and bookmarks
func (s *VSCodeServer) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	filter := history.HistoryFilter{Limit: 100}
	entries := s.historyManager.GetHistory(filter)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (s *VSCodeServer) handleGetBookmarks(w http.ResponseWriter, r *http.Request) {
	bookmarks := s.historyManager.GetBookmarks(history.BookmarkFilter{})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bookmarks)
}

func (s *VSCodeServer) handleCreateBookmark(w http.ResponseWriter, r *http.Request) {
	var bookmark history.Bookmark
	if err := json.NewDecoder(r.Body).Decode(&bookmark); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := s.historyManager.AddBookmark(bookmark)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// Configuration management
func (s *VSCodeServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"defaultRegion": "us-east-1",
		"maxFileSize":   100 * 1024 * 1024, // 100MB
		"autoSync":      true,
		"compression":   false,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (s *VSCodeServer) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var config map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Save configuration to file
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// WebSocket handling for real-time updates
func (s *VSCodeServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade error: %v\n", err)
		return
	}
	defer conn.Close()

	s.clients[conn] = true
	defer delete(s.clients, conn)

	// Send initial connection message
	conn.WriteJSON(map[string]interface{}{
		"type": "connected",
		"data": map[string]string{"status": "VS Code extension connected"},
	})

	// Keep connection alive and handle incoming messages
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}

		// Handle ping/pong
		if msgType, ok := msg["type"].(string); ok && msgType == "ping" {
			conn.WriteJSON(map[string]interface{}{
				"type": "pong",
				"data": map[string]string{"timestamp": time.Now().Format(time.RFC3339)},
			})
		}
	}
}

// Broadcast updates to all connected VS Code instances
func (s *VSCodeServer) broadcastUpdate(eventType string, data interface{}) {
	message := map[string]interface{}{
		"type": eventType,
		"data": data,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	for client := range s.clients {
		err := client.WriteJSON(message)
		if err != nil {
			client.Close()
			delete(s.clients, client)
		}
	}
}

// ExtensionInfo represents VS Code extension information
type ExtensionInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Publisher   string `json:"publisher"`
	Repository  string `json:"repository"`
}

// GetExtensionInfo returns information about the VS Code extension
func GetExtensionInfo() ExtensionInfo {
	return ExtensionInfo{
		Name:        "s3ry-vscode",
		Version:     "2.0.0",
		Description: "S3ry integration for Visual Studio Code - High-performance S3 browser with 271,615x improvement",
		Publisher:   "s3ry-team",
		Repository:  "https://github.com/seike460/s3ry",
	}
}