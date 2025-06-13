package web

// getEmbeddedCSS returns the embedded CSS for the web UI
func getEmbeddedCSS() string {
	return `
/* S3ry Web UI Styles */
* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    line-height: 1.6;
    color: #333;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    min-height: 100vh;
}

.app {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
}

/* Header */
.header {
    background: rgba(255, 255, 255, 0.95);
    backdrop-filter: blur(10px);
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    padding: 1rem 2rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    box-shadow: 0 2px 20px rgba(0, 0, 0, 0.1);
}

.header h1 {
    font-size: 1.5rem;
    font-weight: 700;
    background: linear-gradient(135deg, #667eea, #764ba2);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
}

.header nav {
    display: flex;
    gap: 2rem;
}

.header nav a {
    text-decoration: none;
    color: #666;
    font-weight: 500;
    padding: 0.5rem 1rem;
    border-radius: 8px;
    transition: all 0.3s ease;
}

.header nav a:hover,
.header nav a.active {
    background: rgba(102, 126, 234, 0.1);
    color: #667eea;
}

/* Main content */
.main {
    flex: 1;
    padding: 2rem;
    max-width: 1200px;
    margin: 0 auto;
    width: 100%;
}

/* Welcome section */
.welcome {
    background: rgba(255, 255, 255, 0.95);
    backdrop-filter: blur(10px);
    border-radius: 20px;
    padding: 3rem;
    text-align: center;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.1);
}

.welcome h2 {
    font-size: 2.5rem;
    margin-bottom: 1rem;
    background: linear-gradient(135deg, #667eea, #764ba2);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
}

.welcome p {
    font-size: 1.2rem;
    color: #666;
    margin-bottom: 3rem;
}

/* Features grid */
.features {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 2rem;
    margin-bottom: 3rem;
}

.feature {
    padding: 2rem;
    background: rgba(255, 255, 255, 0.7);
    border-radius: 15px;
    border: 1px solid rgba(255, 255, 255, 0.2);
}

.feature h3 {
    font-size: 1.3rem;
    margin-bottom: 1rem;
    color: #667eea;
}

/* Buttons */
.actions {
    display: flex;
    gap: 1rem;
    justify-content: center;
    flex-wrap: wrap;
}

.btn {
    display: inline-block;
    padding: 1rem 2rem;
    border: none;
    border-radius: 12px;
    font-size: 1rem;
    font-weight: 600;
    text-decoration: none;
    cursor: pointer;
    transition: all 0.3s ease;
    box-shadow: 0 4px 15px rgba(0, 0, 0, 0.1);
}

.btn-primary {
    background: linear-gradient(135deg, #667eea, #764ba2);
    color: white;
}

.btn-primary:hover {
    transform: translateY(-2px);
    box-shadow: 0 6px 20px rgba(102, 126, 234, 0.4);
}

.btn-secondary {
    background: rgba(255, 255, 255, 0.9);
    color: #667eea;
    border: 1px solid rgba(102, 126, 234, 0.3);
}

.btn-secondary:hover {
    background: rgba(102, 126, 234, 0.1);
    transform: translateY(-2px);
}

/* Page header */
.page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2rem;
    background: rgba(255, 255, 255, 0.95);
    backdrop-filter: blur(10px);
    padding: 1.5rem 2rem;
    border-radius: 15px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
}

.page-header h2 {
    font-size: 1.8rem;
    background: linear-gradient(135deg, #667eea, #764ba2);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
}

.controls {
    display: flex;
    gap: 1rem;
    align-items: center;
}

.controls select {
    padding: 0.5rem 1rem;
    border: 1px solid rgba(102, 126, 234, 0.3);
    border-radius: 8px;
    background: white;
    font-size: 0.9rem;
}

/* Loading */
.loading {
    text-align: center;
    padding: 3rem;
    color: #667eea;
    font-size: 1.1rem;
}

/* Buckets grid */
.buckets-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 1.5rem;
}

.bucket-card {
    background: rgba(255, 255, 255, 0.95);
    backdrop-filter: blur(10px);
    border-radius: 15px;
    padding: 1.5rem;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
    transition: all 0.3s ease;
    cursor: pointer;
}

.bucket-card:hover {
    transform: translateY(-5px);
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.15);
}

.bucket-card h3 {
    color: #667eea;
    margin-bottom: 0.5rem;
    font-size: 1.2rem;
}

.bucket-card .bucket-info {
    color: #666;
    font-size: 0.9rem;
}

/* File browser */
.file-browser {
    background: rgba(255, 255, 255, 0.95);
    backdrop-filter: blur(10px);
    border-radius: 15px;
    overflow: hidden;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
}

.breadcrumb {
    padding: 1rem 1.5rem;
    background: rgba(102, 126, 234, 0.1);
    border-bottom: 1px solid rgba(102, 126, 234, 0.2);
    font-family: monospace;
    color: #667eea;
}

.objects-table {
    overflow-x: auto;
}

.objects-table table {
    width: 100%;
    border-collapse: collapse;
}

.objects-table th,
.objects-table td {
    padding: 1rem 1.5rem;
    text-align: left;
    border-bottom: 1px solid rgba(0, 0, 0, 0.05);
}

.objects-table th {
    background: rgba(102, 126, 234, 0.05);
    font-weight: 600;
    color: #667eea;
}

.objects-table tr:hover {
    background: rgba(102, 126, 234, 0.05);
}

/* Modal */
.modal {
    display: none;
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(0, 0, 0, 0.5);
    backdrop-filter: blur(5px);
    z-index: 1000;
}

.modal.active {
    display: flex;
    align-items: center;
    justify-content: center;
}

.modal-content {
    background: white;
    border-radius: 20px;
    padding: 2rem;
    max-width: 500px;
    width: 90%;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}

.modal-content h3 {
    margin-bottom: 1.5rem;
    color: #667eea;
}

/* Drop zone */
.drop-zone {
    border: 2px dashed rgba(102, 126, 234, 0.3);
    border-radius: 15px;
    padding: 3rem;
    text-align: center;
    margin-bottom: 1.5rem;
    transition: all 0.3s ease;
    cursor: pointer;
}

.drop-zone:hover,
.drop-zone.dragover {
    border-color: #667eea;
    background: rgba(102, 126, 234, 0.05);
}

.drop-zone input[type="file"] {
    display: none;
}

.modal-actions {
    display: flex;
    gap: 1rem;
    justify-content: flex-end;
}

/* Settings */
.settings {
    background: rgba(255, 255, 255, 0.95);
    backdrop-filter: blur(10px);
    border-radius: 20px;
    padding: 2rem;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.1);
}

.settings h2 {
    margin-bottom: 2rem;
    background: linear-gradient(135deg, #667eea, #764ba2);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
}

.settings-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 2rem;
}

.settings-section {
    background: rgba(255, 255, 255, 0.7);
    border-radius: 15px;
    padding: 1.5rem;
    border: 1px solid rgba(102, 126, 234, 0.1);
}

.settings-section h3 {
    color: #667eea;
    margin-bottom: 1rem;
    font-size: 1.2rem;
}

.setting {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
    padding: 0.5rem 0;
}

.setting label {
    font-weight: 500;
    color: #555;
}

.setting span,
.setting select {
    color: #667eea;
    font-weight: 600;
}

.setting select {
    padding: 0.25rem 0.5rem;
    border: 1px solid rgba(102, 126, 234, 0.3);
    border-radius: 6px;
    background: white;
}

/* Responsive */
@media (max-width: 768px) {
    .header {
        flex-direction: column;
        gap: 1rem;
    }
    
    .page-header {
        flex-direction: column;
        gap: 1rem;
        text-align: center;
    }
    
    .controls {
        flex-direction: column;
        width: 100%;
    }
    
    .welcome {
        padding: 2rem 1rem;
    }
    
    .welcome h2 {
        font-size: 2rem;
    }
    
    .features {
        grid-template-columns: 1fr;
    }
}
`
}

// getEmbeddedJS returns the embedded JavaScript for the web UI
func getEmbeddedJS(filename string) string {
	switch filename {
	case "app.js":
		return `
// S3ry Web UI - Main Application JavaScript
console.log('🚀 S3ry Web UI loaded');

// Global app state
window.S3ryApp = {
    currentTheme: 'dark',
    wsConnection: null,
    init() {
        this.setupWebSocket();
        this.setupTheme();
    },
    
    setupWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = protocol + '//' + window.location.host + '/api/ws';
        
        this.wsConnection = new WebSocket(wsUrl);
        
        this.wsConnection.onopen = () => {
            console.log('WebSocket connected');
        };
        
        this.wsConnection.onmessage = (event) => {
            const data = JSON.parse(event.data);
            console.log('WebSocket message:', data);
        };
        
        this.wsConnection.onclose = () => {
            console.log('WebSocket disconnected');
            // Attempt to reconnect after 5 seconds
            setTimeout(() => this.setupWebSocket(), 5000);
        };
    },
    
    setupTheme() {
        const savedTheme = localStorage.getItem('s3ry-theme') || 'dark';
        this.setTheme(savedTheme);
    },
    
    setTheme(theme) {
        this.currentTheme = theme;
        document.body.setAttribute('data-theme', theme);
        localStorage.setItem('s3ry-theme', theme);
    }
};

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.S3ryApp.init();
});
`
	
	case "buckets.js":
		return `
// S3ry Web UI - Buckets Page JavaScript
let currentRegion = '';
let buckets = [];

document.addEventListener('DOMContentLoaded', async () => {
    await loadRegions();
    await loadBuckets();
    
    // Event listeners
    document.getElementById('region-select').addEventListener('change', onRegionChange);
    document.getElementById('refresh-btn').addEventListener('click', () => loadBuckets());
});

async function loadRegions() {
    try {
        const response = await fetch('/api/regions');
        const regions = await response.json();
        
        const select = document.getElementById('region-select');
        regions.forEach(region => {
            const option = document.createElement('option');
            option.value = region;
            option.textContent = region;
            select.appendChild(option);
        });
    } catch (error) {
        console.error('Failed to load regions:', error);
    }
}

async function loadBuckets() {
    const loading = document.getElementById('loading');
    const grid = document.getElementById('buckets-grid');
    
    loading.style.display = 'block';
    grid.innerHTML = '';
    
    try {
        const url = '/api/buckets' + (currentRegion ? '?region=' + currentRegion : '');
        const response = await fetch(url);
        buckets = await response.json();
        
        loading.style.display = 'none';
        renderBuckets();
    } catch (error) {
        console.error('Failed to load buckets:', error);
        loading.textContent = 'Failed to load buckets: ' + error.message;
    }
}

function renderBuckets() {
    const grid = document.getElementById('buckets-grid');
    
    if (buckets.length === 0) {
        grid.innerHTML = '<p>No buckets found</p>';
        return;
    }
    
    buckets.forEach(bucket => {
        const card = document.createElement('div');
        card.className = 'bucket-card';
        card.innerHTML = ` + "`" + `
            <h3>📁 \${bucket.name}</h3>
            <div class="bucket-info">
                <p>Created: \${new Date(bucket.creationDate).toLocaleDateString()}</p>
                <p>Region: \${bucket.region || 'Unknown'}</p>
            </div>
        ` + "`" + `;
        
        card.addEventListener('click', () => {
            window.location.href = '/buckets/' + bucket.name;
        });
        
        grid.appendChild(card);
    });
}

function onRegionChange(event) {
    currentRegion = event.target.value;
    loadBuckets();
}
`
	
	case "bucket.js":
		return `
// S3ry Web UI - Bucket Page JavaScript
let currentPrefix = '';
let objects = [];

document.addEventListener('DOMContentLoaded', async () => {
    await loadObjects();
    
    // Event listeners
    document.getElementById('refresh-btn').addEventListener('click', () => loadObjects());
    document.getElementById('upload-btn').addEventListener('click', showUploadModal);
    document.getElementById('upload-cancel').addEventListener('click', hideUploadModal);
    document.getElementById('upload-confirm').addEventListener('click', uploadFiles);
    
    // Drag and drop
    const dropZone = document.getElementById('drop-zone');
    const fileInput = document.getElementById('file-input');
    
    dropZone.addEventListener('click', () => fileInput.click());
    dropZone.addEventListener('dragover', handleDragOver);
    dropZone.addEventListener('drop', handleDrop);
});

async function loadObjects() {
    const loading = document.getElementById('loading');
    const table = document.getElementById('objects-table');
    
    loading.style.display = 'block';
    table.innerHTML = '';
    
    try {
        const url = '/api/buckets/' + window.bucketName + '/objects' + 
                   (currentPrefix ? '?prefix=' + currentPrefix : '');
        const response = await fetch(url);
        objects = await response.json();
        
        loading.style.display = 'none';
        renderObjects();
        updateBreadcrumb();
    } catch (error) {
        console.error('Failed to load objects:', error);
        loading.textContent = 'Failed to load objects: ' + error.message;
    }
}

function renderObjects() {
    const table = document.getElementById('objects-table');
    
    if (objects.length === 0) {
        table.innerHTML = '<p>No objects found</p>';
        return;
    }
    
    let html = ` + "`" + `
        <table>
            <thead>
                <tr>
                    <th>Name</th>
                    <th>Size</th>
                    <th>Modified</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
    ` + "`" + `;
    
    objects.forEach(obj => {
        html += ` + "`" + `
            <tr>
                <td>\${obj.key}</td>
                <td>\${formatBytes(obj.size)}</td>
                <td>\${new Date(obj.lastModified).toLocaleString()}</td>
                <td>
                    <button onclick="downloadObject('\${obj.key}')" class="btn btn-secondary">Download</button>
                    <button onclick="deleteObject('\${obj.key}')" class="btn btn-secondary">Delete</button>
                </td>
            </tr>
        ` + "`" + `;
    });
    
    html += '</tbody></table>';
    table.innerHTML = html;
}

function updateBreadcrumb() {
    const breadcrumb = document.getElementById('breadcrumb');
    if (currentPrefix) {
        breadcrumb.textContent = '/' + window.bucketName + '/' + currentPrefix;
    } else {
        breadcrumb.textContent = '/' + window.bucketName;
    }
}

async function downloadObject(key) {
    try {
        const response = await fetch('/api/buckets/' + window.bucketName + '/objects/' + key);
        const data = await response.json();
        
        // Open download URL in new tab
        window.open(data.download_url, '_blank');
    } catch (error) {
        console.error('Failed to download object:', error);
        alert('Failed to download object: ' + error.message);
    }
}

async function deleteObject(key) {
    if (!confirm('Are you sure you want to delete ' + key + '?')) {
        return;
    }
    
    try {
        const response = await fetch('/api/buckets/' + window.bucketName + '/objects/' + key, {
            method: 'DELETE'
        });
        
        if (response.ok) {
            await loadObjects(); // Reload objects
        } else {
            throw new Error('Failed to delete object');
        }
    } catch (error) {
        console.error('Failed to delete object:', error);
        alert('Failed to delete object: ' + error.message);
    }
}

function showUploadModal() {
    document.getElementById('upload-modal').classList.add('active');
}

function hideUploadModal() {
    document.getElementById('upload-modal').classList.remove('active');
}

function handleDragOver(event) {
    event.preventDefault();
    event.currentTarget.classList.add('dragover');
}

function handleDrop(event) {
    event.preventDefault();
    event.currentTarget.classList.remove('dragover');
    
    const files = event.dataTransfer.files;
    handleFiles(files);
}

function handleFiles(files) {
    const fileInput = document.getElementById('file-input');
    fileInput.files = files;
    
    // Update drop zone text
    const dropZone = document.getElementById('drop-zone');
    dropZone.innerHTML = ` + "`" + `<p>\${files.length} file(s) selected</p>` + "`" + `;
}

async function uploadFiles() {
    const fileInput = document.getElementById('file-input');
    const files = fileInput.files;
    
    if (files.length === 0) {
        alert('Please select files to upload');
        return;
    }
    
    // For MVP, just show a message
    // In a full implementation, this would handle file uploads
    alert('Upload functionality will be implemented in the full version');
    hideUploadModal();
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}
`
	
	case "settings.js":
		return `
// S3ry Web UI - Settings Page JavaScript
document.addEventListener('DOMContentLoaded', () => {
    // Load saved settings
    loadSettings();
    
    // Event listeners
    document.getElementById('theme-select').addEventListener('change', onThemeChange);
    document.getElementById('language-select').addEventListener('change', onLanguageChange);
});

function loadSettings() {
    // Load theme setting
    const savedTheme = localStorage.getItem('s3ry-theme') || 'dark';
    document.getElementById('theme-select').value = savedTheme;
    
    // Load language setting
    const savedLanguage = localStorage.getItem('s3ry-language') || 'en';
    document.getElementById('language-select').value = savedLanguage;
}

function onThemeChange(event) {
    const theme = event.target.value;
    localStorage.setItem('s3ry-theme', theme);
    
    if (window.S3ryApp) {
        window.S3ryApp.setTheme(theme);
    }
    
    // Show success message
    showMessage('Theme updated successfully');
}

function onLanguageChange(event) {
    const language = event.target.value;
    localStorage.setItem('s3ry-language', language);
    
    // Show success message
    showMessage('Language updated successfully (requires page refresh)');
}

function showMessage(text) {
    // Create and show a temporary message
    const message = document.createElement('div');
    message.textContent = text;
    message.style.cssText = ` + "`" + `
        position: fixed;
        top: 20px;
        right: 20px;
        background: #667eea;
        color: white;
        padding: 1rem 2rem;
        border-radius: 8px;
        box-shadow: 0 4px 20px rgba(0,0,0,0.1);
        z-index: 1000;
        animation: slideIn 0.3s ease;
    ` + "`" + `;
    
    document.body.appendChild(message);
    
    setTimeout(() => {
        message.style.animation = 'slideOut 0.3s ease';
        setTimeout(() => message.remove(), 300);
    }, 3000);
}

// Add CSS animations
const style = document.createElement('style');
style.textContent = ` + "`" + `
    @keyframes slideIn {
        from { transform: translateX(100%); opacity: 0; }
        to { transform: translateX(0); opacity: 1; }
    }
    @keyframes slideOut {
        from { transform: translateX(0); opacity: 1; }
        to { transform: translateX(100%); opacity: 0; }
    }
` + "`" + `;
document.head.appendChild(style);
`
	
	default:
		return "// Unknown JavaScript file"
	}
}