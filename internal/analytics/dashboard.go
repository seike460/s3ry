// Package analytics provides usage statistics dashboard and reporting
package analytics

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"
)

// Dashboard provides real-time analytics visualization
type Dashboard struct {
	mu      sync.RWMutex
	stats   *Statistics
	server  *http.Server
	enabled bool
}

// Statistics holds aggregated usage statistics
type Statistics struct {
	StartTime        time.Time              `json:"start_time"`
	LastUpdate       time.Time              `json:"last_update"`
	TotalCommands    int64                  `json:"total_commands"`
	TotalErrors      int64                  `json:"total_errors"`
	TotalBytes       int64                  `json:"total_bytes"`
	TotalObjects     int64                  `json:"total_objects"`
	AverageThroughput float64              `json:"average_throughput_mbps"`
	CommandStats     map[string]*CommandStat `json:"command_stats"`
	ErrorStats       map[string]int64       `json:"error_stats"`
	Performance      *PerformanceStats      `json:"performance"`
	System           *SystemStats           `json:"system"`
}

// CommandStat holds statistics for a specific command
type CommandStat struct {
	Count          int64         `json:"count"`
	TotalDuration  time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
	SuccessRate    float64       `json:"success_rate"`
	LastUsed       time.Time     `json:"last_used"`
	Errors         int64         `json:"errors"`
}

// PerformanceStats holds performance metrics
type PerformanceStats struct {
	PeakThroughput     float64           `json:"peak_throughput_mbps"`
	AverageMemoryUsage int64             `json:"average_memory_usage_bytes"`
	PeakMemoryUsage    int64             `json:"peak_memory_usage_bytes"`
	WorkerPoolSizes    map[int]int64     `json:"worker_pool_sizes"`
	ThroughputHistory  []ThroughputPoint `json:"throughput_history"`
}

// ThroughputPoint represents a point in throughput history
type ThroughputPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Throughput float64   `json:"throughput_mbps"`
	Command    string    `json:"command"`
}

// SystemStats holds system-level statistics
type SystemStats struct {
	OSDistribution       map[string]int64 `json:"os_distribution"`
	ArchDistribution     map[string]int64 `json:"arch_distribution"`
	ContainerUsage       int64            `json:"container_usage"`
	CloudProviderUsage   map[string]int64 `json:"cloud_provider_usage"`
	GoVersionDistribution map[string]int64 `json:"go_version_distribution"`
}

// NewDashboard creates a new analytics dashboard
func NewDashboard(port int) *Dashboard {
	dashboard := &Dashboard{
		stats: &Statistics{
			StartTime:    time.Now(),
			LastUpdate:   time.Now(),
			CommandStats: make(map[string]*CommandStat),
			ErrorStats:   make(map[string]int64),
			Performance: &PerformanceStats{
				WorkerPoolSizes:   make(map[int]int64),
				ThroughputHistory: make([]ThroughputPoint, 0),
			},
			System: &SystemStats{
				OSDistribution:        make(map[string]int64),
				ArchDistribution:      make(map[string]int64),
				CloudProviderUsage:    make(map[string]int64),
				GoVersionDistribution: make(map[string]int64),
			},
		},
		enabled: false,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", dashboard.handleIndex)
	mux.HandleFunc("/api/stats", dashboard.handleStats)
	mux.HandleFunc("/api/health", dashboard.handleHealth)
	mux.HandleFunc("/static/", dashboard.handleStatic)

	dashboard.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return dashboard
}

// Start starts the dashboard server
func (d *Dashboard) Start() error {
	d.mu.Lock()
	d.enabled = true
	d.mu.Unlock()

	fmt.Printf("📊 Starting analytics dashboard on http://localhost%s\n", d.server.Addr)
	return d.server.ListenAndServe()
}

// Stop stops the dashboard server
func (d *Dashboard) Stop() error {
	d.mu.Lock()
	d.enabled = false
	d.mu.Unlock()

	return d.server.Close()
}

// UpdateStats updates statistics with new data
func (d *Dashboard) UpdateStats(command string, duration time.Duration, success bool, bytesTransferred, objectsProcessed int64, throughput float64, memoryUsage int64, workerPoolSize int, osInfo, arch, goVersion, cloudProvider string, isContainer bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stats.LastUpdate = time.Now()
	d.stats.TotalCommands++
	d.stats.TotalBytes += bytesTransferred
	d.stats.TotalObjects += objectsProcessed

	// Update command statistics
	if d.stats.CommandStats[command] == nil {
		d.stats.CommandStats[command] = &CommandStat{}
	}
	cmdStat := d.stats.CommandStats[command]
	cmdStat.Count++
	cmdStat.TotalDuration += duration
	cmdStat.AverageDuration = cmdStat.TotalDuration / time.Duration(cmdStat.Count)
	cmdStat.LastUsed = time.Now()

	if success {
		cmdStat.SuccessRate = float64(cmdStat.Count-cmdStat.Errors) / float64(cmdStat.Count)
	} else {
		cmdStat.Errors++
		d.stats.TotalErrors++
		cmdStat.SuccessRate = float64(cmdStat.Count-cmdStat.Errors) / float64(cmdStat.Count)
	}

	// Update performance statistics
	if throughput > d.stats.Performance.PeakThroughput {
		d.stats.Performance.PeakThroughput = throughput
	}

	if memoryUsage > d.stats.Performance.PeakMemoryUsage {
		d.stats.Performance.PeakMemoryUsage = memoryUsage
	}

	// Update average throughput
	if d.stats.TotalCommands > 0 {
		totalThroughput := d.stats.AverageThroughput*float64(d.stats.TotalCommands-1) + throughput
		d.stats.AverageThroughput = totalThroughput / float64(d.stats.TotalCommands)
	}

	// Update average memory usage
	if d.stats.TotalCommands > 0 {
		totalMemory := d.stats.Performance.AverageMemoryUsage*int64(d.stats.TotalCommands-1) + memoryUsage
		d.stats.Performance.AverageMemoryUsage = totalMemory / int64(d.stats.TotalCommands)
	}

	// Track worker pool sizes
	d.stats.Performance.WorkerPoolSizes[workerPoolSize]++

	// Add to throughput history (keep last 100 points)
	d.stats.Performance.ThroughputHistory = append(d.stats.Performance.ThroughputHistory, ThroughputPoint{
		Timestamp:  time.Now(),
		Throughput: throughput,
		Command:    command,
	})
	if len(d.stats.Performance.ThroughputHistory) > 100 {
		d.stats.Performance.ThroughputHistory = d.stats.Performance.ThroughputHistory[1:]
	}

	// Update system statistics
	d.stats.System.OSDistribution[osInfo]++
	d.stats.System.ArchDistribution[arch]++
	d.stats.System.GoVersionDistribution[goVersion]++
	
	if cloudProvider != "" {
		d.stats.System.CloudProviderUsage[cloudProvider]++
	}
	
	if isContainer {
		d.stats.System.ContainerUsage++
	}
}

// UpdateError updates error statistics
func (d *Dashboard) UpdateError(errorType string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stats.ErrorStats[errorType]++
	d.stats.TotalErrors++
}

// GetStats returns current statistics
func (d *Dashboard) GetStats() *Statistics {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Create a deep copy to avoid race conditions
	statsCopy := *d.stats
	
	// Copy maps
	statsCopy.CommandStats = make(map[string]*CommandStat)
	for k, v := range d.stats.CommandStats {
		cmdStatCopy := *v
		statsCopy.CommandStats[k] = &cmdStatCopy
	}
	
	statsCopy.ErrorStats = make(map[string]int64)
	for k, v := range d.stats.ErrorStats {
		statsCopy.ErrorStats[k] = v
	}

	// Copy performance stats
	perfCopy := *d.stats.Performance
	perfCopy.WorkerPoolSizes = make(map[int]int64)
	for k, v := range d.stats.Performance.WorkerPoolSizes {
		perfCopy.WorkerPoolSizes[k] = v
	}
	
	perfCopy.ThroughputHistory = make([]ThroughputPoint, len(d.stats.Performance.ThroughputHistory))
	copy(perfCopy.ThroughputHistory, d.stats.Performance.ThroughputHistory)
	statsCopy.Performance = &perfCopy

	// Copy system stats
	sysCopy := *d.stats.System
	sysCopy.OSDistribution = make(map[string]int64)
	for k, v := range d.stats.System.OSDistribution {
		sysCopy.OSDistribution[k] = v
	}
	
	sysCopy.ArchDistribution = make(map[string]int64)
	for k, v := range d.stats.System.ArchDistribution {
		sysCopy.ArchDistribution[k] = v
	}
	
	sysCopy.CloudProviderUsage = make(map[string]int64)
	for k, v := range d.stats.System.CloudProviderUsage {
		sysCopy.CloudProviderUsage[k] = v
	}
	
	sysCopy.GoVersionDistribution = make(map[string]int64)
	for k, v := range d.stats.System.GoVersionDistribution {
		sysCopy.GoVersionDistribution[k] = v
	}
	
	statsCopy.System = &sysCopy

	return &statsCopy
}

// handleIndex serves the main dashboard page
func (d *Dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>S3ry Analytics Dashboard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 20px; border-radius: 10px; margin-bottom: 20px; }
        .header h1 { margin: 0; font-size: 2em; }
        .header p { margin: 5px 0 0 0; opacity: 0.9; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .card { background: white; padding: 20px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .card h3 { margin-top: 0; color: #333; border-bottom: 2px solid #eee; padding-bottom: 10px; }
        .metric { display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid #f0f0f0; }
        .metric:last-child { border-bottom: none; }
        .metric-value { font-weight: bold; color: #667eea; }
        .chart-container { position: relative; height: 300px; }
        .status-good { color: #28a745; }
        .status-warning { color: #ffc107; }
        .status-error { color: #dc3545; }
        .refresh-btn { position: fixed; bottom: 20px; right: 20px; background: #667eea; color: white; border: none; padding: 15px; border-radius: 50px; cursor: pointer; font-size: 16px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚀 S3ry Analytics Dashboard</h1>
        <p>Real-time performance and usage statistics</p>
    </div>

    <div class="grid">
        <div class="card">
            <h3>📊 Overview</h3>
            <div class="metric">
                <span>Total Commands</span>
                <span class="metric-value" id="total-commands">Loading...</span>
            </div>
            <div class="metric">
                <span>Success Rate</span>
                <span class="metric-value" id="success-rate">Loading...</span>
            </div>
            <div class="metric">
                <span>Total Data Transferred</span>
                <span class="metric-value" id="total-bytes">Loading...</span>
            </div>
            <div class="metric">
                <span>Average Throughput</span>
                <span class="metric-value" id="avg-throughput">Loading...</span>
            </div>
        </div>

        <div class="card">
            <h3>⚡ Performance</h3>
            <div class="chart-container">
                <canvas id="throughput-chart"></canvas>
            </div>
        </div>

        <div class="card">
            <h3>🎯 Commands</h3>
            <div id="command-stats">Loading...</div>
        </div>

        <div class="card">
            <h3>🔧 System Info</h3>
            <div id="system-stats">Loading...</div>
        </div>
    </div>

    <button class="refresh-btn" onclick="refreshData()">🔄</button>

    <script>
        let throughputChart;

        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }

        function formatDuration(ms) {
            if (ms < 1000) return ms + 'ms';
            if (ms < 60000) return (ms / 1000).toFixed(2) + 's';
            return (ms / 60000).toFixed(2) + 'm';
        }

        async function loadData() {
            try {
                const response = await fetch('/api/stats');
                const data = await response.json();
                updateDashboard(data);
            } catch (error) {
                console.error('Failed to load data:', error);
            }
        }

        function updateDashboard(stats) {
            // Update overview
            document.getElementById('total-commands').textContent = stats.total_commands.toLocaleString();
            const successRate = ((stats.total_commands - stats.total_errors) / stats.total_commands * 100).toFixed(1);
            document.getElementById('success-rate').textContent = successRate + '%';
            document.getElementById('total-bytes').textContent = formatBytes(stats.total_bytes);
            document.getElementById('avg-throughput').textContent = stats.average_throughput_mbps.toFixed(2) + ' MB/s';

            // Update command stats
            let commandHtml = '';
            Object.entries(stats.command_stats || {}).forEach(([cmd, stat]) => {
                commandHtml += '<div class="metric">';
                commandHtml += '<span>' + cmd + '</span>';
                commandHtml += '<span class="metric-value">' + stat.count + ' (' + (stat.success_rate * 100).toFixed(1) + '%)</span>';
                commandHtml += '</div>';
            });
            document.getElementById('command-stats').innerHTML = commandHtml || '<p>No data available</p>';

            // Update system stats
            let systemHtml = '';
            Object.entries(stats.system?.os_distribution || {}).forEach(([os, count]) => {
                systemHtml += '<div class="metric"><span>' + os + '</span><span class="metric-value">' + count + '</span></div>';
            });
            document.getElementById('system-stats').innerHTML = systemHtml || '<p>No data available</p>';

            // Update throughput chart
            updateThroughputChart(stats.performance?.throughput_history || []);
        }

        function updateThroughputChart(history) {
            const ctx = document.getElementById('throughput-chart').getContext('2d');
            
            if (throughputChart) {
                throughputChart.destroy();
            }

            const labels = history.map(point => new Date(point.timestamp).toLocaleTimeString());
            const data = history.map(point => point.throughput_mbps);

            throughputChart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: labels,
                    datasets: [{
                        label: 'Throughput (MB/s)',
                        data: data,
                        borderColor: '#667eea',
                        backgroundColor: 'rgba(102, 126, 234, 0.1)',
                        tension: 0.4,
                        fill: true
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        y: {
                            beginAtZero: true,
                            title: {
                                display: true,
                                text: 'Throughput (MB/s)'
                            }
                        }
                    },
                    plugins: {
                        legend: {
                            display: false
                        }
                    }
                }
            });
        }

        function refreshData() {
            loadData();
        }

        // Load initial data
        loadData();

        // Auto-refresh every 30 seconds
        setInterval(loadData, 30000);
    </script>
</body>
</html>
`

	t, err := template.New("dashboard").Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, nil)
}

// handleStats serves the statistics API endpoint
func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := d.GetStats()
	
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	json.NewEncoder(w).Encode(stats)
}

// handleHealth serves the health check endpoint
func (d *Dashboard) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"uptime":    time.Since(d.stats.StartTime).String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStatic serves static files (placeholder)
func (d *Dashboard) handleStatic(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}