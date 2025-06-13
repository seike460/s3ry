import * as vscode from 'vscode';
import { S3Client, Bookmark } from '../services/s3Client';

export class BookmarkItem extends vscode.TreeItem {
    constructor(
        public readonly bookmark: Bookmark
    ) {
        super(
            bookmark.name,
            vscode.TreeItemCollapsibleState.None
        );

        this.tooltip = this.generateTooltip();
        this.iconPath = this.getIcon();
        this.contextValue = 'bookmark';
        this.description = this.generateDescription();
        
        // Add command to navigate to the bookmarked location
        this.command = {
            command: 's3ry.refreshExplorer',
            title: 'Navigate to Bookmark',
            arguments: [this.bookmark]
        };
    }

    private generateDescription(): string {
        const location = this.bookmark.prefix 
            ? `${this.bookmark.bucket}/${this.bookmark.prefix}`
            : this.bookmark.bucket;
        
        return location;
    }

    private generateTooltip(): string {
        const lines = [
            `Name: ${this.bookmark.name}`,
            `Type: ${this.bookmark.type}`,
            `Location: ${this.bookmark.bucket}${this.bookmark.prefix ? '/' + this.bookmark.prefix : ''}`,
            `Created: ${this.bookmark.createdAt ? new Date(this.bookmark.createdAt).toLocaleString() : 'Unknown'}`
        ];

        if (this.bookmark.description) {
            lines.splice(1, 0, `Description: ${this.bookmark.description}`);
        }

        if (this.bookmark.useCount && this.bookmark.useCount > 0) {
            lines.push(`Used: ${this.bookmark.useCount} times`);
        }

        if (this.bookmark.lastUsed) {
            lines.push(`Last used: ${new Date(this.bookmark.lastUsed).toLocaleString()}`);
        }

        if (this.bookmark.tags && this.bookmark.tags.length > 0) {
            lines.push(`Tags: ${this.bookmark.tags.join(', ')}`);
        }

        return lines.join('\n');
    }

    private getIcon(): vscode.ThemeIcon {
        const iconMap: { [key: string]: string } = {
            'location': 'location',
            'operation': 'gear',
            'query': 'search'
        };

        const iconName = iconMap[this.bookmark.type] || 'bookmark';
        
        // Use different colors based on bookmark type
        const colorMap: { [key: string]: vscode.ThemeColor } = {
            'location': new vscode.ThemeColor('charts.blue'),
            'operation': new vscode.ThemeColor('charts.orange'),
            'query': new vscode.ThemeColor('charts.purple')
        };

        const color = colorMap[this.bookmark.type] || new vscode.ThemeColor('charts.yellow');
        
        return new vscode.ThemeIcon(iconName, color);
    }
}

export class S3BookmarksProvider implements vscode.TreeDataProvider<BookmarkItem> {
    private _onDidChangeTreeData: vscode.EventEmitter<BookmarkItem | undefined | null | void> = new vscode.EventEmitter<BookmarkItem | undefined | null | void>();
    readonly onDidChangeTreeData: vscode.Event<BookmarkItem | undefined | null | void> = this._onDidChangeTreeData.event;

    constructor(private s3Client: S3Client) {}

    refresh(): void {
        this._onDidChangeTreeData.fire();
    }

    getTreeItem(element: BookmarkItem): vscode.TreeItem {
        return element;
    }

    async getChildren(element?: BookmarkItem): Promise<BookmarkItem[]> {
        if (element) {
            return [];
        }

        try {
            const bookmarks = await this.s3Client.getBookmarks();
            
            // Sort bookmarks by usage (most used first), then by name
            bookmarks.sort((a, b) => {
                const usageA = a.useCount || 0;
                const usageB = b.useCount || 0;
                
                if (usageA !== usageB) {
                    return usageB - usageA;
                }
                
                return a.name.localeCompare(b.name);
            });
            
            return bookmarks.map(bookmark => new BookmarkItem(bookmark));
        } catch (error) {
            vscode.window.showErrorMessage(`Failed to load bookmarks: ${error}`);
            return [];
        }
    }
}