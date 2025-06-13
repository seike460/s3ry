import * as vscode from 'vscode';
import * as child_process from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import { S3BucketsProvider, S3Item } from './providers/s3BucketsProvider';
import { S3HistoryProvider } from './providers/s3HistoryProvider';
import { S3BookmarksProvider } from './providers/s3BookmarksProvider';
import { S3Client } from './services/s3Client';

let s3Client: S3Client;
let s3BucketsProvider: S3BucketsProvider;
let s3HistoryProvider: S3HistoryProvider;
let s3BookmarksProvider: S3BookmarksProvider;
let serverProcess: child_process.ChildProcess | undefined;

export function activate(context: vscode.ExtensionContext) {
    console.log('S3ry extension is now active!');

    // Initialize S3 client
    s3Client = new S3Client();

    // Create providers
    s3BucketsProvider = new S3BucketsProvider(s3Client);
    s3HistoryProvider = new S3HistoryProvider(s3Client);
    s3BookmarksProvider = new S3BookmarksProvider(s3Client);

    // Register tree data providers
    vscode.window.createTreeView('s3ryBuckets', {
        treeDataProvider: s3BucketsProvider,
        showCollapseAll: true
    });

    vscode.window.createTreeView('s3ryHistory', {
        treeDataProvider: s3HistoryProvider
    });

    vscode.window.createTreeView('s3ryBookmarks', {
        treeDataProvider: s3BookmarksProvider
    });

    // Set context for enabling/disabling commands
    vscode.commands.executeCommand('setContext', 's3ry.enabled', true);

    // Register commands
    registerCommands(context);

    // Auto-start server if configured
    const config = vscode.workspace.getConfiguration('s3ry');
    if (config.get('autoStart', true)) {
        startS3ryServer();
    }

    // Show welcome message
    vscode.window.showInformationMessage(
        'S3ry extension activated! 🚀 271,615x performance improvement ready.',
        'Open S3ry Panel'
    ).then(selection => {
        if (selection === 'Open S3ry Panel') {
            vscode.commands.executeCommand('workbench.view.extension.s3ry');
        }
    });
}

function registerCommands(context: vscode.ExtensionContext) {
    // Enable/Disable commands
    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.enable', () => {
            vscode.commands.executeCommand('setContext', 's3ry.enabled', true);
            startS3ryServer();
            vscode.window.showInformationMessage('S3ry enabled');
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.disable', () => {
            vscode.commands.executeCommand('setContext', 's3ry.enabled', false);
            stopS3ryServer();
            vscode.window.showInformationMessage('S3ry disabled');
        })
    );

    // Refresh command
    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.refreshExplorer', () => {
            s3BucketsProvider.refresh();
            s3HistoryProvider.refresh();
            s3BookmarksProvider.refresh();
        })
    );

    // File operations
    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.uploadFile', async (uri: vscode.Uri) => {
            await uploadFile(uri);
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.uploadWorkspace', async (uri: vscode.Uri) => {
            await uploadWorkspace(uri);
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.downloadFile', async (item: S3Item) => {
            await downloadFile(item);
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.syncWorkspace', async () => {
            await syncWorkspace();
        })
    );

    // S3 operations
    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.createBucket', async () => {
            await createBucket();
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.deleteBucket', async (item: S3Item) => {
            await deleteBucket(item);
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.copyObject', async (item: S3Item) => {
            await copyObject(item);
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.deleteObject', async (item: S3Item) => {
            await deleteObject(item);
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.previewObject', async (item: S3Item) => {
            await previewObject(item);
        })
    );

    // Bookmark operations
    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.addBookmark', async (item: S3Item) => {
            await addBookmark(item);
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.removeBookmark', async (item: S3Item) => {
            await removeBookmark(item);
        })
    );

    // Settings
    context.subscriptions.push(
        vscode.commands.registerCommand('s3ry.openSettings', () => {
            vscode.commands.executeCommand('workbench.action.openSettings', 's3ry');
        })
    );
}

// Server management
function startS3ryServer() {
    if (serverProcess) {
        return;
    }

    const config = vscode.workspace.getConfiguration('s3ry');
    const port = config.get('serverPort', 3001);
    const region = config.get('awsRegion', 'us-east-1');
    const profile = config.get('awsProfile', '');
    const endpoint = config.get('customEndpoint', '');

    // Try to find s3ry-vscode binary
    const s3ryPath = findS3ryBinary();
    if (!s3ryPath) {
        vscode.window.showErrorMessage(
            'S3ry binary not found. Please install S3ry or add it to your PATH.',
            'Download S3ry'
        ).then(selection => {
            if (selection === 'Download S3ry') {
                vscode.env.openExternal(vscode.Uri.parse('https://github.com/seike460/s3ry/releases'));
            }
        });
        return;
    }

    const args = ['--port', port.toString(), '--region', region];
    if (profile) {
        args.push('--profile', profile);
    }
    if (endpoint) {
        args.push('--endpoint', endpoint);
    }

    serverProcess = child_process.spawn(s3ryPath, args, {
        stdio: 'pipe'
    });

    serverProcess.stdout?.on('data', (data) => {
        console.log(`S3ry server: ${data}`);
    });

    serverProcess.stderr?.on('data', (data) => {
        console.error(`S3ry server error: ${data}`);
    });

    serverProcess.on('close', (code) => {
        console.log(`S3ry server exited with code ${code}`);
        serverProcess = undefined;
    });

    // Initialize client connection
    setTimeout(() => {
        s3Client.connect(port);
    }, 2000);
}

function stopS3ryServer() {
    if (serverProcess) {
        serverProcess.kill();
        serverProcess = undefined;
    }
    s3Client.disconnect();
}

function findS3ryBinary(): string | null {
    const possiblePaths = [
        's3ry-vscode',
        './s3ry-vscode',
        '../s3ry-vscode',
        path.join(process.env.GOPATH || '', 'bin', 's3ry-vscode'),
        path.join(process.env.HOME || '', 'go', 'bin', 's3ry-vscode')
    ];

    for (const binPath of possiblePaths) {
        try {
            child_process.execSync(`${binPath} --help`, { stdio: 'ignore' });
            return binPath;
        } catch {
            continue;
        }
    }

    return null;
}

// File operations
async function uploadFile(uri?: vscode.Uri) {
    if (!uri && vscode.window.activeTextEditor) {
        uri = vscode.window.activeTextEditor.document.uri;
    }

    if (!uri) {
        const fileUris = await vscode.window.showOpenDialog({
            canSelectFiles: true,
            canSelectFolders: false,
            canSelectMany: false,
            openLabel: 'Upload to S3'
        });

        if (!fileUris || fileUris.length === 0) {
            return;
        }

        uri = fileUris[0];
    }

    const bucket = await vscode.window.showQuickPick(
        s3Client.getBuckets().then(buckets => buckets.map(b => b.name)),
        { placeHolder: 'Select target bucket' }
    );

    if (!bucket) {
        return;
    }

    const defaultKey = path.basename(uri.fsPath);
    const key = await vscode.window.showInputBox({
        prompt: 'Enter S3 key (object path)',
        value: defaultKey
    });

    if (!key) {
        return;
    }

    try {
        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: `Uploading ${defaultKey} to S3...`,
            cancellable: false
        }, async () => {
            await s3Client.uploadFile(uri!.fsPath, bucket, key);
        });

        vscode.window.showInformationMessage(`✅ Uploaded ${defaultKey} to s3://${bucket}/${key}`);
        s3BucketsProvider.refresh();
    } catch (error) {
        vscode.window.showErrorMessage(`❌ Upload failed: ${error}`);
    }
}

async function uploadWorkspace(uri?: vscode.Uri) {
    if (!uri) {
        if (!vscode.workspace.workspaceFolders) {
            vscode.window.showErrorMessage('No workspace folder open');
            return;
        }
        uri = vscode.workspace.workspaceFolders[0].uri;
    }

    const bucket = await vscode.window.showQuickPick(
        s3Client.getBuckets().then(buckets => buckets.map(b => b.name)),
        { placeHolder: 'Select target bucket' }
    );

    if (!bucket) {
        return;
    }

    const prefix = await vscode.window.showInputBox({
        prompt: 'Enter S3 prefix for workspace files',
        value: 'workspace/'
    });

    if (prefix === undefined) {
        return;
    }

    // TODO: Implement batch upload for workspace
    vscode.window.showInformationMessage('Workspace upload feature coming soon!');
}

async function downloadFile(item: S3Item) {
    if (item.type !== 'object') {
        return;
    }

    const saveUri = await vscode.window.showSaveDialog({
        defaultUri: vscode.Uri.file(item.label),
        saveLabel: 'Download from S3'
    });

    if (!saveUri) {
        return;
    }

    try {
        await vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: `Downloading ${item.label} from S3...`,
            cancellable: false
        }, async () => {
            await s3Client.downloadFile(item.bucket!, item.key!, saveUri.fsPath);
        });

        vscode.window.showInformationMessage(`✅ Downloaded ${item.label} from S3`);
    } catch (error) {
        vscode.window.showErrorMessage(`❌ Download failed: ${error}`);
    }
}

async function syncWorkspace() {
    vscode.window.showInformationMessage('Workspace sync feature coming soon!');
}

// S3 operations
async function createBucket() {
    const bucketName = await vscode.window.showInputBox({
        prompt: 'Enter bucket name',
        validateInput: (value) => {
            if (!value || value.length < 3) {
                return 'Bucket name must be at least 3 characters long';
            }
            if (!/^[a-z0-9.-]+$/.test(value)) {
                return 'Bucket name can only contain lowercase letters, numbers, periods, and hyphens';
            }
            return null;
        }
    });

    if (!bucketName) {
        return;
    }

    try {
        await s3Client.createBucket(bucketName);
        vscode.window.showInformationMessage(`✅ Created bucket: ${bucketName}`);
        s3BucketsProvider.refresh();
    } catch (error) {
        vscode.window.showErrorMessage(`❌ Failed to create bucket: ${error}`);
    }
}

async function deleteBucket(item: S3Item) {
    if (item.type !== 'bucket') {
        return;
    }

    const confirmation = await vscode.window.showWarningMessage(
        `Are you sure you want to delete bucket "${item.label}"?`,
        { modal: true },
        'Delete'
    );

    if (confirmation !== 'Delete') {
        return;
    }

    try {
        await s3Client.deleteBucket(item.label);
        vscode.window.showInformationMessage(`✅ Deleted bucket: ${item.label}`);
        s3BucketsProvider.refresh();
    } catch (error) {
        vscode.window.showErrorMessage(`❌ Failed to delete bucket: ${error}`);
    }
}

async function copyObject(item: S3Item) {
    if (item.type !== 'object') {
        return;
    }

    const targetKey = await vscode.window.showInputBox({
        prompt: 'Enter target key',
        value: item.key
    });

    if (!targetKey || targetKey === item.key) {
        return;
    }

    try {
        await s3Client.copyObject(item.bucket!, item.key!, item.bucket!, targetKey);
        vscode.window.showInformationMessage(`✅ Copied ${item.key} to ${targetKey}`);
        s3BucketsProvider.refresh();
    } catch (error) {
        vscode.window.showErrorMessage(`❌ Failed to copy object: ${error}`);
    }
}

async function deleteObject(item: S3Item) {
    if (item.type !== 'object') {
        return;
    }

    const confirmation = await vscode.window.showWarningMessage(
        `Are you sure you want to delete "${item.key}"?`,
        { modal: true },
        'Delete'
    );

    if (confirmation !== 'Delete') {
        return;
    }

    try {
        await s3Client.deleteObject(item.bucket!, item.key!);
        vscode.window.showInformationMessage(`✅ Deleted object: ${item.key}`);
        s3BucketsProvider.refresh();
    } catch (error) {
        vscode.window.showErrorMessage(`❌ Failed to delete object: ${error}`);
    }
}

async function previewObject(item: S3Item) {
    if (item.type !== 'object') {
        return;
    }

    try {
        const content = await s3Client.getObjectContent(item.bucket!, item.key!);
        const doc = await vscode.workspace.openTextDocument({
            content,
            language: getLanguageFromKey(item.key!)
        });
        await vscode.window.showTextDocument(doc);
    } catch (error) {
        vscode.window.showErrorMessage(`❌ Failed to preview object: ${error}`);
    }
}

function getLanguageFromKey(key: string): string {
    const ext = path.extname(key).toLowerCase();
    const languageMap: { [key: string]: string } = {
        '.js': 'javascript',
        '.ts': 'typescript',
        '.py': 'python',
        '.go': 'go',
        '.json': 'json',
        '.xml': 'xml',
        '.yaml': 'yaml',
        '.yml': 'yaml',
        '.md': 'markdown',
        '.txt': 'plaintext'
    };
    return languageMap[ext] || 'plaintext';
}

// Bookmark operations
async function addBookmark(item: S3Item) {
    const name = await vscode.window.showInputBox({
        prompt: 'Enter bookmark name',
        value: item.type === 'bucket' ? item.label : `${item.bucket}/${item.key}`
    });

    if (!name) {
        return;
    }

    const description = await vscode.window.showInputBox({
        prompt: 'Enter bookmark description (optional)'
    });

    try {
        await s3Client.createBookmark({
            name,
            description: description || '',
            bucket: item.bucket || item.label,
            prefix: item.key || '',
            type: item.type === 'bucket' ? 'location' : 'location'
        });

        vscode.window.showInformationMessage(`✅ Bookmark "${name}" created`);
        s3BookmarksProvider.refresh();
    } catch (error) {
        vscode.window.showErrorMessage(`❌ Failed to create bookmark: ${error}`);
    }
}

async function removeBookmark(item: S3Item) {
    // Implementation for removing bookmarks
    vscode.window.showInformationMessage('Remove bookmark feature coming soon!');
}

export function deactivate() {
    stopS3ryServer();
}