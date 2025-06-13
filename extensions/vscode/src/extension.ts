import * as vscode from 'vscode';
import * as cp from 'child_process';
import * as path from 'path';
import * as fs from 'fs';

// Extension state
let statusBarItem: vscode.StatusBarItem;
let s3BucketProvider: S3BucketProvider;
let performanceMonitor: PerformanceMonitor;

export function activate(context: vscode.ExtensionContext) {
    console.log('S3ry extension is now active!');

    // Initialize components
    statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
    s3BucketProvider = new S3BucketProvider();
    performanceMonitor = new PerformanceMonitor();

    // Register tree data provider
    vscode.window.registerTreeDataProvider('s3ryBuckets', s3BucketProvider);

    // Register commands
    const commands = [
        vscode.commands.registerCommand('s3ry.listBuckets', listBuckets),
        vscode.commands.registerCommand('s3ry.openTUI', openTUI),
        vscode.commands.registerCommand('s3ry.uploadFile', uploadFile),
        vscode.commands.registerCommand('s3ry.downloadFile', downloadFile),
        vscode.commands.registerCommand('s3ry.syncFolder', syncFolder),
        vscode.commands.registerCommand('s3ry.showPerformanceStats', showPerformanceStats),
        vscode.commands.registerCommand('s3ry.configure', configure),
        vscode.commands.registerCommand('s3ry.checkForUpdates', checkForUpdates),
        vscode.commands.registerCommand('s3ry.refreshBuckets', () => s3BucketProvider.refresh())
    ];

    context.subscriptions.push(...commands, statusBarItem);

    // Initialize status bar
    updateStatusBar();
    statusBarItem.show();

    // Check if s3ry is installed
    checkS3ryInstallation();

    // Start performance monitoring if enabled
    const config = vscode.workspace.getConfiguration('s3ry');
    if (config.get('showPerformanceMetrics')) {
        performanceMonitor.start();
    }

    // Auto-refresh buckets if enabled
    if (config.get('autoRefresh')) {
        const interval = config.get('refreshInterval', 30) * 1000;
        setInterval(() => s3BucketProvider.refresh(), interval);
    }
}

export function deactivate() {
    performanceMonitor.stop();
}

// Command implementations

async function listBuckets() {
    try {
        const buckets = await executeS3ryCommand(['list', '--format', 'json']);
        const bucketList = JSON.parse(buckets);
        
        const quickPick = vscode.window.createQuickPick();
        quickPick.items = bucketList.map((bucket: any) => ({
            label: bucket.name,
            description: `${bucket.objects} objects, ${formatBytes(bucket.size)}`,
            detail: `Region: ${bucket.region}, Created: ${new Date(bucket.created).toLocaleDateString()}`
        }));
        
        quickPick.title = 'S3 Buckets';
        quickPick.placeholder = 'Select a bucket to explore';
        
        quickPick.onDidChangeSelection(selection => {
            if (selection[0]) {
                listObjects(selection[0].label);
            }
            quickPick.hide();
        });
        
        quickPick.show();
    } catch (error) {
        vscode.window.showErrorMessage(`Failed to list buckets: ${error}`);
    }
}

async function listObjects(bucket: string) {
    try {
        const objects = await executeS3ryCommand(['list', bucket, '--format', 'json']);
        const objectList = JSON.parse(objects);
        
        const quickPick = vscode.window.createQuickPick();
        quickPick.items = objectList.map((obj: any) => ({
            label: obj.key,
            description: formatBytes(obj.size),
            detail: `Modified: ${new Date(obj.lastModified).toLocaleDateString()}`
        }));
        
        quickPick.title = `Objects in ${bucket}`;
        quickPick.placeholder = 'Select an object to download';
        
        quickPick.onDidChangeSelection(selection => {
            if (selection[0]) {
                downloadSpecificFile(bucket, selection[0].label);
            }
            quickPick.hide();
        });
        
        quickPick.show();
    } catch (error) {
        vscode.window.showErrorMessage(`Failed to list objects: ${error}`);
    }
}

async function openTUI() {
    const config = vscode.workspace.getConfiguration('s3ry');
    const terminalType = config.get('tuiTerminal', 'integrated');
    
    if (terminalType === 'integrated') {
        const terminal = vscode.window.createTerminal('S3ry TUI');
        terminal.sendText('s3ry --tui');
        terminal.show();
    } else {
        // Open in external terminal
        const s3ryPath = getS3ryPath();
        cp.spawn(s3ryPath, ['--tui'], { 
            detached: true,
            stdio: 'ignore'
        });
    }
}

async function uploadFile(uri?: vscode.Uri) {
    try {
        const filePath = uri?.fsPath || await selectFile();
        if (!filePath) return;
        
        const bucket = await selectBucket();
        if (!bucket) return;
        
        const key = await vscode.window.showInputBox({
            prompt: 'Enter S3 key (path) for the file',
            value: path.basename(filePath)
        });
        
        if (!key) return;
        
        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: `Uploading ${path.basename(filePath)} to S3`,
            cancellable: false
        }, async (progress) => {
            const result = await executeS3ryCommand(['upload', filePath, `s3://${bucket}/${key}`]);
            
            if (result.includes('successfully')) {
                vscode.window.showInformationMessage(`File uploaded successfully to s3://${bucket}/${key}`);
            }
        });
        
    } catch (error) {
        vscode.window.showErrorMessage(`Upload failed: ${error}`);
    }
}

async function downloadFile() {
    try {
        const bucket = await selectBucket();
        if (!bucket) return;
        
        const objects = await executeS3ryCommand(['list', bucket, '--format', 'json']);
        const objectList = JSON.parse(objects);
        
        const selectedObject = await vscode.window.showQuickPick(
            objectList.map((obj: any) => ({
                label: obj.key,
                description: formatBytes(obj.size)
            })),
            { placeHolder: 'Select object to download' }
        );
        
        if (!selectedObject) return;
        
        const downloadPath = await vscode.window.showSaveDialog({
            defaultUri: vscode.Uri.file(selectedObject.label)
        });
        
        if (!downloadPath) return;
        
        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: `Downloading ${selectedObject.label}`,
            cancellable: false
        }, async () => {
            await executeS3ryCommand(['download', `s3://${bucket}/${selectedObject.label}`, downloadPath.fsPath]);
            vscode.window.showInformationMessage(`File downloaded to ${downloadPath.fsPath}`);
        });
        
    } catch (error) {
        vscode.window.showErrorMessage(`Download failed: ${error}`);
    }
}

async function downloadSpecificFile(bucket: string, key: string) {
    try {
        const downloadPath = await vscode.window.showSaveDialog({
            defaultUri: vscode.Uri.file(path.basename(key))
        });
        
        if (!downloadPath) return;
        
        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: `Downloading ${key}`,
            cancellable: false
        }, async () => {
            await executeS3ryCommand(['download', `s3://${bucket}/${key}`, downloadPath.fsPath]);
            vscode.window.showInformationMessage(`File downloaded to ${downloadPath.fsPath}`);
        });
        
    } catch (error) {
        vscode.window.showErrorMessage(`Download failed: ${error}`);
    }
}

async function syncFolder(uri?: vscode.Uri) {
    try {
        const folderPath = uri?.fsPath || await selectFolder();
        if (!folderPath) return;
        
        const bucket = await selectBucket();
        if (!bucket) return;
        
        const prefix = await vscode.window.showInputBox({
            prompt: 'Enter S3 prefix (optional)',
            value: ''
        });
        
        const target = prefix ? `s3://${bucket}/${prefix}` : `s3://${bucket}`;
        
        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: `Syncing ${path.basename(folderPath)} to S3`,
            cancellable: false
        }, async () => {
            await executeS3ryCommand(['sync', folderPath, target]);
            vscode.window.showInformationMessage(`Folder synced to ${target}`);
        });
        
    } catch (error) {
        vscode.window.showErrorMessage(`Sync failed: ${error}`);
    }
}

async function showPerformanceStats() {
    try {
        const stats = await executeS3ryCommand(['stats', '--format', 'json']);
        const statsData = JSON.parse(stats);
        
        const panel = vscode.window.createWebviewPanel(
            's3ryStats',
            'S3ry Performance Statistics',
            vscode.ViewColumn.One,
            {
                enableScripts: true
            }
        );
        
        panel.webview.html = generateStatsHTML(statsData);
        
    } catch (error) {
        vscode.window.showErrorMessage(`Failed to load performance stats: ${error}`);
    }
}

async function configure() {
    const config = vscode.workspace.getConfiguration('s3ry');
    
    const options = [
        'Default Region',
        'Worker Pool Size',
        'Chunk Size',
        'Auto Refresh',
        'Show Performance Metrics',
        'Debug Mode'
    ];
    
    const selected = await vscode.window.showQuickPick(options, {
        placeHolder: 'Select configuration option'
    });
    
    switch (selected) {
        case 'Default Region':
            const region = await vscode.window.showInputBox({
                prompt: 'Enter default AWS region',
                value: config.get('defaultRegion', 'us-west-2')
            });
            if (region) {
                await config.update('defaultRegion', region, vscode.ConfigurationTarget.Global);
            }
            break;
            
        case 'Worker Pool Size':
            const poolSize = await vscode.window.showInputBox({
                prompt: 'Enter worker pool size (1-100)',
                value: config.get('workerPoolSize', 10).toString()
            });
            if (poolSize && !isNaN(Number(poolSize))) {
                await config.update('workerPoolSize', Number(poolSize), vscode.ConfigurationTarget.Global);
            }
            break;
            
        // Add other configuration options...
    }
}

async function checkForUpdates() {
    try {
        const result = await executeS3ryCommand(['version', '--check-update']);
        
        if (result.includes('update available')) {
            const update = await vscode.window.showInformationMessage(
                'A new version of s3ry is available!',
                'Update Now',
                'View Release Notes',
                'Skip'
            );
            
            if (update === 'Update Now') {
                const terminal = vscode.window.createTerminal('S3ry Update');
                terminal.sendText('s3ry update install');
                terminal.show();
            } else if (update === 'View Release Notes') {
                vscode.env.openExternal(vscode.Uri.parse('https://github.com/seike460/s3ry/releases'));
            }
        } else {
            vscode.window.showInformationMessage('S3ry is up to date!');
        }
        
    } catch (error) {
        vscode.window.showErrorMessage(`Failed to check for updates: ${error}`);
    }
}

// Helper functions

function executeS3ryCommand(args: string[]): Promise<string> {
    return new Promise((resolve, reject) => {
        const s3ryPath = getS3ryPath();
        const child = cp.spawn(s3ryPath, args);
        
        let stdout = '';
        let stderr = '';
        
        child.stdout.on('data', (data) => {
            stdout += data.toString();
        });
        
        child.stderr.on('data', (data) => {
            stderr += data.toString();
        });
        
        child.on('close', (code) => {
            if (code === 0) {
                resolve(stdout.trim());
            } else {
                reject(new Error(stderr || `Process exited with code ${code}`));
            }
        });
        
        child.on('error', (error) => {
            reject(error);
        });
    });
}

function getS3ryPath(): string {
    const config = vscode.workspace.getConfiguration('s3ry');
    return config.get('binaryPath', 's3ry');
}

async function checkS3ryInstallation() {
    try {
        await executeS3ryCommand(['--version']);
        updateStatusBar('S3ry Ready', '$(check)');
    } catch (error) {
        updateStatusBar('S3ry Not Found', '$(error)');
        
        const install = await vscode.window.showErrorMessage(
            'S3ry binary not found. Please install s3ry first.',
            'Install Instructions',
            'Configure Path'
        );
        
        if (install === 'Install Instructions') {
            vscode.env.openExternal(vscode.Uri.parse('https://github.com/seike460/s3ry#installation'));
        } else if (install === 'Configure Path') {
            const path = await vscode.window.showInputBox({
                prompt: 'Enter path to s3ry binary'
            });
            if (path) {
                const config = vscode.workspace.getConfiguration('s3ry');
                await config.update('binaryPath', path, vscode.ConfigurationTarget.Global);
                checkS3ryInstallation(); // Recheck
            }
        }
    }
}

function updateStatusBar(text?: string, icon?: string) {
    statusBarItem.text = `${icon || '$(cloud)'} ${text || 'S3ry'}`;
    statusBarItem.command = 's3ry.listBuckets';
    statusBarItem.tooltip = 'Click to list S3 buckets';
}

async function selectBucket(): Promise<string | undefined> {
    try {
        const buckets = await executeS3ryCommand(['list', '--format', 'json']);
        const bucketList = JSON.parse(buckets);
        
        const selected = await vscode.window.showQuickPick(
            bucketList.map((bucket: any) => bucket.name),
            { placeHolder: 'Select S3 bucket' }
        );
        
        return selected;
    } catch (error) {
        vscode.window.showErrorMessage(`Failed to list buckets: ${error}`);
        return undefined;
    }
}

async function selectFile(): Promise<string | undefined> {
    const selected = await vscode.window.showOpenDialog({
        canSelectFiles: true,
        canSelectFolders: false,
        canSelectMany: false
    });
    
    return selected?.[0].fsPath;
}

async function selectFolder(): Promise<string | undefined> {
    const selected = await vscode.window.showOpenDialog({
        canSelectFiles: false,
        canSelectFolders: true,
        canSelectMany: false
    });
    
    return selected?.[0].fsPath;
}

function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function generateStatsHTML(stats: any): string {
    return `
        <!DOCTYPE html>
        <html>
        <head>
            <title>S3ry Performance Statistics</title>
            <style>
                body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; padding: 20px; }
                .metric { display: flex; justify-content: space-between; padding: 10px; border-bottom: 1px solid #eee; }
                .metric-value { font-weight: bold; color: #007ACC; }
                .section { margin-bottom: 30px; }
                h2 { color: #333; border-bottom: 2px solid #007ACC; padding-bottom: 10px; }
            </style>
        </head>
        <body>
            <h1>🚀 S3ry Performance Statistics</h1>
            
            <div class="section">
                <h2>Overall Performance</h2>
                <div class="metric">
                    <span>Total Operations</span>
                    <span class="metric-value">${stats.totalOperations || 0}</span>
                </div>
                <div class="metric">
                    <span>Average Throughput</span>
                    <span class="metric-value">${stats.averageThroughput || 0} MB/s</span>
                </div>
                <div class="metric">
                    <span>Success Rate</span>
                    <span class="metric-value">${((stats.successfulOperations / stats.totalOperations) * 100).toFixed(1)}%</span>
                </div>
            </div>
            
            <div class="section">
                <h2>Recent Activity</h2>
                <div class="metric">
                    <span>Last Operation</span>
                    <span class="metric-value">${stats.lastOperation || 'N/A'}</span>
                </div>
                <div class="metric">
                    <span>Data Transferred Today</span>
                    <span class="metric-value">${formatBytes(stats.dataTransferredToday || 0)}</span>
                </div>
            </div>
        </body>
        </html>
    `;
}

// Tree Data Provider for S3 Buckets
class S3BucketProvider implements vscode.TreeDataProvider<BucketItem> {
    private _onDidChangeTreeData: vscode.EventEmitter<BucketItem | undefined | null | void> = new vscode.EventEmitter<BucketItem | undefined | null | void>();
    readonly onDidChangeTreeData: vscode.Event<BucketItem | undefined | null | void> = this._onDidChangeTreeData.event;
    
    private buckets: BucketItem[] = [];
    
    refresh(): void {
        this.loadBuckets();
        this._onDidChangeTreeData.fire();
    }
    
    getTreeItem(element: BucketItem): vscode.TreeItem {
        return element;
    }
    
    getChildren(element?: BucketItem): Thenable<BucketItem[]> {
        if (!element) {
            return Promise.resolve(this.buckets);
        }
        return Promise.resolve([]);
    }
    
    private async loadBuckets() {
        try {
            const result = await executeS3ryCommand(['list', '--format', 'json']);
            const bucketList = JSON.parse(result);
            
            this.buckets = bucketList.map((bucket: any) => new BucketItem(
                bucket.name,
                `${bucket.objects} objects, ${formatBytes(bucket.size)}`,
                vscode.TreeItemCollapsibleState.None
            ));
        } catch (error) {
            this.buckets = [];
        }
    }
}

class BucketItem extends vscode.TreeItem {
    constructor(
        public readonly label: string,
        private readonly description: string,
        public readonly collapsibleState: vscode.TreeItemCollapsibleState
    ) {
        super(label, collapsibleState);
        this.tooltip = `${this.label}: ${this.description}`;
        this.description = this.description;
        this.iconPath = new vscode.ThemeIcon('database');
        this.contextValue = 'bucket';
    }
}

// Performance Monitor
class PerformanceMonitor {
    private interval?: NodeJS.Timeout;
    
    start() {
        this.interval = setInterval(async () => {
            try {
                const stats = await executeS3ryCommand(['stats', '--format', 'json']);
                const statsData = JSON.parse(stats);
                
                const config = vscode.workspace.getConfiguration('s3ry');
                if (config.get('showPerformanceMetrics')) {
                    updateStatusBar(
                        `${statsData.averageThroughput || 0} MB/s`, 
                        '$(graph)'
                    );
                }
            } catch (error) {
                // Ignore errors in background monitoring
            }
        }, 5000); // Update every 5 seconds
    }
    
    stop() {
        if (this.interval) {
            clearInterval(this.interval);
        }
    }
}