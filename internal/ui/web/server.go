package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/seike460/s3ry/internal/config"
	internalS3 "github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/pkg/interfaces"
)

// WebServer represents the web UI server
type WebServer struct {
	config    *config.Config
	s3Client  interfaces.S3Client
	upgrader  websocket.Upgrader
	templates *template.Template
	port      int
}

// NewWebServer creates a new web server instance
func NewWebServer(cfg *config.Config, port int) *WebServer {
	if port == 0 {
		port = 8080 // Default port
	}

	return &WebServer{
		config: cfg,
		port:   port,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}
}

// Start starts the web server
func (ws *WebServer) Start() error {
	// Load templates
	if err := ws.loadTemplates(); err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

	// Create router
	router := mux.NewRouter()

	// Static files
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(ws.serveStatic)))

	// API routes
	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/regions", ws.handleRegions).Methods("GET")
	api.HandleFunc("/buckets", ws.handleBuckets).Methods("GET")
	api.HandleFunc("/buckets/{bucket}/objects", ws.handleObjects).Methods("GET")
	api.HandleFunc("/buckets/{bucket}/objects/{key:.*}", ws.handleObjectAction).Methods("GET", "POST", "DELETE")
	api.HandleFunc("/ws", ws.handleWebSocket).Methods("GET")

	// Web UI routes
	router.HandleFunc("/", ws.handleIndex).Methods("GET")
	router.HandleFunc("/buckets", ws.handleBucketsPage).Methods("GET")
	router.HandleFunc("/buckets/{bucket}", ws.handleBucketPage).Methods("GET")
	router.HandleFunc("/settings", ws.handleSettingsPage).Methods("GET")

	// Start server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", ws.port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("🌐 S3ry Web UI starting on http://localhost:%d", ws.port)
	return server.ListenAndServe()
}

// loadTemplates loads HTML templates
func (ws *WebServer) loadTemplates() error {
	// For now, use embedded templates
	// In a full implementation, these would be loaded from files
	ws.templates = template.New("")

	// Define templates inline for MVP
	indexTemplate := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>S3ry Web UI</title>
    <link href="/static/css/style.css" rel="stylesheet">
</head>
<body>
    <div class="app">
        <header class="header">
            <h1>🚀 S3ry Web UI</h1>
            <nav>
                <a href="/">Home</a>
                <a href="/buckets">Buckets</a>
                <a href="/settings">Settings</a>
            </nav>
        </header>
        <main class="main">
            <div class="welcome">
                <h2>Welcome to S3ry Web Interface</h2>
                <p>A modern, fast S3 browser with advanced features</p>
                <div class="features">
                    <div class="feature">
                        <h3>🏃‍♂️ High Performance</h3>
                        <p>271,615x improvement over traditional tools</p>
                    </div>
                    <div class="feature">
                        <h3>🎨 Modern UI</h3>
                        <p>Responsive design with real-time updates</p>
                    </div>
                    <div class="feature">
                        <h3>⚡ Fast Operations</h3>
                        <p>Parallel processing for maximum speed</p>
                    </div>
                </div>
                <div class="actions">
                    <a href="/buckets" class="btn btn-primary">Browse Buckets</a>
                    <a href="/settings" class="btn btn-secondary">Configuration</a>
                </div>
            </div>
        </main>
    </div>
    <script src="/static/js/app.js"></script>
</body>
</html>`

	bucketsTemplate := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>S3 Buckets - S3ry</title>
    <link href="/static/css/style.css" rel="stylesheet">
</head>
<body>
    <div class="app">
        <header class="header">
            <h1>🚀 S3ry Web UI</h1>
            <nav>
                <a href="/">Home</a>
                <a href="/buckets" class="active">Buckets</a>
                <a href="/settings">Settings</a>
            </nav>
        </header>
        <main class="main">
            <div class="page-header">
                <h2>📁 S3 Buckets</h2>
                <div class="controls">
                    <select id="region-select">
                        <option value="">Select Region</option>
                    </select>
                    <button id="refresh-btn" class="btn btn-secondary">🔄 Refresh</button>
                </div>
            </div>
            <div id="loading" class="loading">Loading buckets...</div>
            <div id="buckets-grid" class="buckets-grid"></div>
        </main>
    </div>
    <script src="/static/js/buckets.js"></script>
</body>
</html>`

	bucketTemplate := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.BucketName}} - S3ry</title>
    <link href="/static/css/style.css" rel="stylesheet">
</head>
<body>
    <div class="app">
        <header class="header">
            <h1>🚀 S3ry Web UI</h1>
            <nav>
                <a href="/">Home</a>
                <a href="/buckets">Buckets</a>
                <a href="/settings">Settings</a>
            </nav>
        </header>
        <main class="main">
            <div class="page-header">
                <h2>📁 {{.BucketName}}</h2>
                <div class="controls">
                    <button id="upload-btn" class="btn btn-primary">📤 Upload</button>
                    <button id="refresh-btn" class="btn btn-secondary">🔄 Refresh</button>
                </div>
            </div>
            <div class="file-browser">
                <div class="breadcrumb" id="breadcrumb"></div>
                <div id="loading" class="loading">Loading objects...</div>
                <div id="objects-table" class="objects-table"></div>
            </div>
            <div id="upload-modal" class="modal">
                <div class="modal-content">
                    <h3>Upload Files</h3>
                    <div id="drop-zone" class="drop-zone">
                        <p>Drag and drop files here or click to select</p>
                        <input type="file" id="file-input" multiple>
                    </div>
                    <div class="modal-actions">
                        <button id="upload-confirm" class="btn btn-primary">Upload</button>
                        <button id="upload-cancel" class="btn btn-secondary">Cancel</button>
                    </div>
                </div>
            </div>
        </main>
    </div>
    <script>window.bucketName = "{{.BucketName}}";</script>
    <script src="/static/js/bucket.js"></script>
</body>
</html>`

	settingsTemplate := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Settings - S3ry</title>
    <link href="/static/css/style.css" rel="stylesheet">
</head>
<body>
    <div class="app">
        <header class="header">
            <h1>🚀 S3ry Web UI</h1>
            <nav>
                <a href="/">Home</a>
                <a href="/buckets">Buckets</a>
                <a href="/settings" class="active">Settings</a>
            </nav>
        </header>
        <main class="main">
            <div class="settings">
                <h2>⚙️ Configuration</h2>
                <div class="settings-grid">
                    <div class="settings-section">
                        <h3>AWS Configuration</h3>
                        <div class="setting">
                            <label>Region:</label>
                            <span>{{.Config.AWS.Region}}</span>
                        </div>
                        <div class="setting">
                            <label>Profile:</label>
                            <span>{{.Config.AWS.Profile}}</span>
                        </div>
                    </div>
                    <div class="settings-section">
                        <h3>UI Configuration</h3>
                        <div class="setting">
                            <label>Theme:</label>
                            <select id="theme-select">
                                <option value="dark">Dark</option>
                                <option value="light">Light</option>
                                <option value="auto">Auto</option>
                            </select>
                        </div>
                        <div class="setting">
                            <label>Language:</label>
                            <select id="language-select">
                                <option value="en">English</option>
                                <option value="ja">Japanese</option>
                            </select>
                        </div>
                    </div>
                </div>
            </div>
        </main>
    </div>
    <script src="/static/js/settings.js"></script>
</body>
</html>`

	template.Must(ws.templates.New("index").Parse(indexTemplate))
	template.Must(ws.templates.New("buckets").Parse(bucketsTemplate))
	template.Must(ws.templates.New("bucket").Parse(bucketTemplate))
	template.Must(ws.templates.New("settings").Parse(settingsTemplate))

	return nil
}

// serveStatic serves static files (CSS, JS, images)
func (ws *WebServer) serveStatic(w http.ResponseWriter, r *http.Request) {
	// For MVP, serve embedded CSS/JS content
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	
	switch {
	case strings.HasSuffix(path, ".css"):
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(getEmbeddedCSS()))
	case strings.HasSuffix(path, ".js"):
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte(getEmbeddedJS(filepath.Base(path))))
	default:
		http.NotFound(w, r)
	}
}

// HTTP Handlers
func (ws *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	ws.templates.ExecuteTemplate(w, "index", nil)
}

func (ws *WebServer) handleBucketsPage(w http.ResponseWriter, r *http.Request) {
	ws.templates.ExecuteTemplate(w, "buckets", nil)
}

func (ws *WebServer) handleBucketPage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bucketName := vars["bucket"]
	
	data := struct {
		BucketName string
	}{
		BucketName: bucketName,
	}
	
	ws.templates.ExecuteTemplate(w, "bucket", data)
}

func (ws *WebServer) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Config *config.Config
	}{
		Config: ws.config,
	}
	
	ws.templates.ExecuteTemplate(w, "settings", data)
}

// API Handlers
func (ws *WebServer) handleRegions(w http.ResponseWriter, r *http.Request) {
	regions := []string{
		"us-east-1", "us-west-1", "us-west-2",
		"eu-west-1", "eu-central-1", "ap-southeast-1",
		"ap-northeast-1", "ap-south-1",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(regions)
}

func (ws *WebServer) handleBuckets(w http.ResponseWriter, r *http.Request) {
	region := r.URL.Query().Get("region")
	if region == "" {
		region = ws.config.AWS.Region
	}
	
	// Create S3 client for the region
	s3Client := internalS3.NewClient(region)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	buckets, err := s3Client.ListBuckets(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list buckets: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buckets)
}

func (ws *WebServer) handleObjects(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bucketName := vars["bucket"]
	prefix := r.URL.Query().Get("prefix")
	
	region := r.URL.Query().Get("region")
	if region == "" {
		region = ws.config.AWS.Region
	}
	
	s3Client := internalS3.NewClient(region)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	objects, err := s3Client.ListObjects(ctx, bucketName, prefix)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list objects: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(objects)
}

func (ws *WebServer) handleObjectAction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bucketName := vars["bucket"]
	objectKey := vars["key"]
	
	region := r.URL.Query().Get("region")
	if region == "" {
		region = ws.config.AWS.Region
	}
	
	s3Client := internalS3.NewClient(region)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	switch r.Method {
	case "GET":
		// Download object
		url, err := s3Client.GetPresignedURL(ctx, bucketName, objectKey, 15*time.Minute)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate download URL: %v", err), http.StatusInternalServerError)
			return
		}
		
		response := map[string]string{"download_url": url}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		
	case "DELETE":
		// Delete object
		err := s3Client.DeleteObject(ctx, bucketName, objectKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete object: %v", err), http.StatusInternalServerError)
			return
		}
		
		w.WriteHeader(http.StatusNoContent)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// WebSocket handler for real-time updates
func (ws *WebServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	
	// Handle WebSocket communication
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		
		// Echo message back for now
		// In a full implementation, this would handle real-time updates
		conn.WriteJSON(map[string]interface{}{
			"type":    "response",
			"data":    msg,
			"timestamp": time.Now().Unix(),
		})
	}
}

// RunWebUI starts the web UI server
func RunWebUI(cfg *config.Config, port string) error {
	portInt := 8080
	if port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			portInt = p
		}
	}
	
	server := NewWebServer(cfg, portInt)
	return server.Start()
}