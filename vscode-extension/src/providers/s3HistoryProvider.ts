import * as vscode from 'vscode';
import { S3Client, HistoryEntry } from '../services/s3Client';

export class HistoryItem extends vscode.TreeItem {
    constructor(
        public readonly entry: HistoryEntry
    ) {
        super(
            HistoryItem.formatLabel(entry),
            vscode.TreeItemCollapsibleState.None
        );

        this.tooltip = this.generateTooltip();
        this.iconPath = this.getIcon();
        this.contextValue = 'historyEntry';
        
        // Add command to navigate to the bucket/object
        this.command = {
            command: 'vscode.open',
            title: 'Navigate',
            arguments: [vscode.Uri.parse(`s3://${entry.bucket}/${entry.key || ''}`)]
        };
    }

    private static formatLabel(entry: HistoryEntry): string {
        const timestamp = new Date(entry.timestamp).toLocaleTimeString();
        const action = entry.action.toUpperCase();
        const target = entry.key ? `${entry.bucket}/${entry.key}` : entry.bucket;
        const status = entry.success ? '✅' : '❌';
        
        return `${status} ${timestamp} ${action} ${target}`;
    }

    private generateTooltip(): string {
        const lines = [
            `Action: ${this.entry.action}`,
            `Bucket: ${this.entry.bucket}`,
            `Timestamp: ${new Date(this.entry.timestamp).toLocaleString()}`,
            `Status: ${this.entry.success ? 'Success' : 'Failed'}`
        ];

        if (this.entry.key) {
            lines.splice(2, 0, `Key: ${this.entry.key}`);
        }

        if (this.entry.duration) {
            lines.push(`Duration: ${this.entry.duration}ms`);
        }

        if (this.entry.size) {
            lines.push(`Size: ${this.formatBytes(this.entry.size)}`);
        }

        if (this.entry.error) {
            lines.push(`Error: ${this.entry.error}`);
        }

        return lines.join('\n');
    }

    private getIcon(): vscode.ThemeIcon {
        if (!this.entry.success) {
            return new vscode.ThemeIcon('error', new vscode.ThemeColor('errorForeground'));
        }

        const iconMap: { [key: string]: string } = {
            'download': 'cloud-download',
            'upload': 'cloud-upload',
            'delete': 'trash',
            'copy': 'files',
            'move': 'arrow-right',
            'list': 'list-unordered',
            'view': 'eye',
            'browse': 'folder-opened',
            'create_bucket': 'new-folder',
            'delete_bucket': 'trash'
        };

        const iconName = iconMap[this.entry.action] || 'circle-filled';
        return new vscode.ThemeIcon(iconName, new vscode.ThemeColor('charts.green'));
    }

    private formatBytes(bytes: number): string {
        if (bytes === 0) return '0 B';
        
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }
}

export class S3HistoryProvider implements vscode.TreeDataProvider<HistoryItem> {
    private _onDidChangeTreeData: vscode.EventEmitter<HistoryItem | undefined | null | void> = new vscode.EventEmitter<HistoryItem | undefined | null | void>();
    readonly onDidChangeTreeData: vscode.Event<HistoryItem | undefined | null | void> = this._onDidChangeTreeData.event;

    constructor(private s3Client: S3Client) {
        // Listen for real-time updates
        this.s3Client.on('upload', () => this.refresh());
        this.s3Client.on('download', () => this.refresh());
        this.s3Client.on('delete', () => this.refresh());
    }

    refresh(): void {
        this._onDidChangeTreeData.fire();
    }

    getTreeItem(element: HistoryItem): vscode.TreeItem {
        return element;
    }

    async getChildren(element?: HistoryItem): Promise<HistoryItem[]> {
        if (element) {
            return [];
        }

        try {
            const history = await this.s3Client.getHistory();
            return history.map(entry => new HistoryItem(entry));
        } catch (error) {
            vscode.window.showErrorMessage(`Failed to load history: ${error}`);
            return [];
        }
    }
}