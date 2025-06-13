import * as vscode from 'vscode';
import { S3Client, S3Bucket, S3Object } from '../services/s3Client';

export class S3Item extends vscode.TreeItem {
    constructor(
        public readonly label: string,
        public readonly collapsibleState: vscode.TreeItemCollapsibleState,
        public readonly type: 'bucket' | 'folder' | 'object',
        public readonly bucket?: string,
        public readonly key?: string,
        public readonly size?: number,
        public readonly lastModified?: string
    ) {
        super(label, collapsibleState);

        this.contextValue = type === 'bucket' ? 's3Bucket' : type === 'folder' ? 's3Folder' : 's3Object';
        this.tooltip = this.generateTooltip();
        this.iconPath = this.getIcon();
        
        if (type === 'object') {
            this.command = {
                command: 's3ry.previewObject',
                title: 'Preview',
                arguments: [this]
            };
        }
    }

    private generateTooltip(): string {
        switch (this.type) {
            case 'bucket':
                return `Bucket: ${this.label}`;
            case 'folder':
                return `Folder: ${this.key || this.label}`;
            case 'object':
                const sizeStr = this.size ? this.formatBytes(this.size) : 'Unknown size';
                const dateStr = this.lastModified ? new Date(this.lastModified).toLocaleString() : 'Unknown date';
                return `${this.key}\nSize: ${sizeStr}\nModified: ${dateStr}`;
            default:
                return this.label;
        }
    }

    private getIcon(): vscode.ThemeIcon {
        switch (this.type) {
            case 'bucket':
                return new vscode.ThemeIcon('cloud');
            case 'folder':
                return new vscode.ThemeIcon('folder');
            case 'object':
                return this.getFileIcon(this.key || this.label);
            default:
                return new vscode.ThemeIcon('file');
        }
    }

    private getFileIcon(filename: string): vscode.ThemeIcon {
        const ext = filename.split('.').pop()?.toLowerCase();
        const iconMap: { [key: string]: string } = {
            'js': 'file-javascript',
            'ts': 'file-typescript',
            'py': 'file-python',
            'go': 'file-go',
            'json': 'file-json',
            'xml': 'file-xml',
            'yaml': 'file-yaml',
            'yml': 'file-yaml',
            'md': 'file-markdown',
            'txt': 'file-text',
            'pdf': 'file-pdf',
            'png': 'file-image',
            'jpg': 'file-image',
            'jpeg': 'file-image',
            'gif': 'file-image',
            'zip': 'file-zip',
            'tar': 'file-zip',
            'gz': 'file-zip'
        };
        
        return new vscode.ThemeIcon(iconMap[ext || ''] || 'file');
    }

    private formatBytes(bytes: number): string {
        if (bytes === 0) return '0 B';
        
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }
}

export class S3BucketsProvider implements vscode.TreeDataProvider<S3Item> {
    private _onDidChangeTreeData: vscode.EventEmitter<S3Item | undefined | null | void> = new vscode.EventEmitter<S3Item | undefined | null | void>();
    readonly onDidChangeTreeData: vscode.Event<S3Item | undefined | null | void> = this._onDidChangeTreeData.event;

    constructor(private s3Client: S3Client) {
        // Listen for real-time updates
        this.s3Client.on('upload', () => this.refresh());
        this.s3Client.on('download', () => this.refresh());
        this.s3Client.on('delete', () => this.refresh());
    }

    refresh(): void {
        this._onDidChangeTreeData.fire();
    }

    getTreeItem(element: S3Item): vscode.TreeItem {
        return element;
    }

    async getChildren(element?: S3Item): Promise<S3Item[]> {
        if (!element) {
            // Root level - return buckets
            return this.getBuckets();
        } else if (element.type === 'bucket') {
            // Bucket level - return objects and folders
            return this.getObjects(element.label);
        } else if (element.type === 'folder') {
            // Folder level - return nested objects and folders
            return this.getObjects(element.bucket!, element.key!);
        }
        
        return [];
    }

    private async getBuckets(): Promise<S3Item[]> {
        try {
            const buckets = await this.s3Client.getBuckets();
            return buckets.map(bucket => new S3Item(
                bucket.name,
                vscode.TreeItemCollapsibleState.Collapsed,
                'bucket'
            ));
        } catch (error) {
            vscode.window.showErrorMessage(`Failed to load buckets: ${error}`);
            return [];
        }
    }

    private async getObjects(bucket: string, prefix?: string): Promise<S3Item[]> {
        try {
            const objects = await this.s3Client.getObjects(bucket, prefix, '/');
            const items: S3Item[] = [];

            // Group objects by their immediate path components
            const folders = new Map<string, S3Item>();
            const files: S3Item[] = [];

            for (const obj of objects) {
                const relativePath = prefix ? obj.key.substring(prefix.length) : obj.key;
                const pathParts = relativePath.split('/').filter(part => part.length > 0);

                if (pathParts.length === 0) {
                    continue;
                }

                if (pathParts.length === 1) {
                    // Direct file in current folder
                    if (!obj.key.endsWith('/')) {
                        files.push(new S3Item(
                            pathParts[0],
                            vscode.TreeItemCollapsibleState.None,
                            'object',
                            bucket,
                            obj.key,
                            obj.size,
                            obj.lastModified
                        ));
                    }
                } else {
                    // Nested folder
                    const folderName = pathParts[0];
                    const folderKey = prefix ? `${prefix}${folderName}/` : `${folderName}/`;
                    
                    if (!folders.has(folderName)) {
                        folders.set(folderName, new S3Item(
                            folderName,
                            vscode.TreeItemCollapsibleState.Collapsed,
                            'folder',
                            bucket,
                            folderKey
                        ));
                    }
                }
            }

            // Combine folders and files, with folders first
            items.push(...Array.from(folders.values()));
            items.push(...files);

            return items;
        } catch (error) {
            vscode.window.showErrorMessage(`Failed to load objects: ${error}`);
            return [];
        }
    }
}