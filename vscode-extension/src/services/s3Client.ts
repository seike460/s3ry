import * as vscode from 'vscode';
import * as WebSocket from 'ws';

export interface S3Bucket {
    name: string;
    creationDate: string;
    region?: string;
}

export interface S3Object {
    key: string;
    size: number;
    lastModified: string;
    etag: string;
    storageClass?: string;
}

export interface HistoryEntry {
    id: string;
    timestamp: string;
    action: string;
    bucket: string;
    key?: string;
    success: boolean;
    duration?: number;
    size?: number;
    error?: string;
}

export interface Bookmark {
    id?: string;
    name: string;
    description: string;
    bucket: string;
    prefix: string;
    type: string;
    tags?: string[];
    createdAt?: string;
    lastUsed?: string;
    useCount?: number;
}

export class S3Client {
    private baseUrl: string = '';
    private ws: WebSocket | null = null;
    private listeners: { [key: string]: Function[] } = {};

    connect(port: number) {
        this.baseUrl = `http://localhost:${port}/api/vscode`;
        
        // Connect WebSocket for real-time updates
        const wsUrl = `ws://localhost:${port}/api/vscode/ws`;
        this.ws = new WebSocket(wsUrl);

        this.ws.on('open', () => {
            console.log('Connected to S3ry server');
        });

        this.ws.on('message', (data: WebSocket.Data) => {
            try {
                const message = JSON.parse(data.toString());
                this.emit(message.type, message.data);
            } catch (error) {
                console.error('Failed to parse WebSocket message:', error);
            }
        });

        this.ws.on('error', (error) => {
            console.error('WebSocket error:', error);
        });

        this.ws.on('close', () => {
            console.log('Disconnected from S3ry server');
            this.ws = null;
        });
    }

    disconnect() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
    }

    on(event: string, listener: Function) {
        if (!this.listeners[event]) {
            this.listeners[event] = [];
        }
        this.listeners[event].push(listener);
    }

    private emit(event: string, data: any) {
        const eventListeners = this.listeners[event];
        if (eventListeners) {
            eventListeners.forEach(listener => listener(data));
        }
    }

    private async request(path: string, options: RequestInit = {}): Promise<any> {
        if (!this.baseUrl) {
            throw new Error('S3ry server not connected');
        }

        const url = `${this.baseUrl}${path}`;
        const response = await fetch(url, {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`HTTP ${response.status}: ${errorText}`);
        }

        const contentType = response.headers.get('content-type');
        if (contentType && contentType.includes('application/json')) {
            return response.json();
        }

        return response.text();
    }

    // Bucket operations
    async getBuckets(): Promise<S3Bucket[]> {
        return this.request('/buckets');
    }

    async createBucket(name: string): Promise<void> {
        await this.request('/buckets', {
            method: 'POST',
            body: JSON.stringify({ name })
        });
    }

    async deleteBucket(name: string): Promise<void> {
        await this.request(`/buckets/${encodeURIComponent(name)}`, {
            method: 'DELETE'
        });
    }

    // Object operations
    async getObjects(bucket: string, prefix?: string, delimiter?: string): Promise<S3Object[]> {
        const params = new URLSearchParams();
        if (prefix) params.append('prefix', prefix);
        if (delimiter) params.append('delimiter', delimiter);
        
        const query = params.toString();
        const path = `/buckets/${encodeURIComponent(bucket)}/objects${query ? '?' + query : ''}`;
        
        return this.request(path);
    }

    async uploadFile(localPath: string, bucket: string, key: string): Promise<void> {
        await this.request('/workspace/upload', {
            method: 'POST',
            body: JSON.stringify({ localPath, bucket, key })
        });
    }

    async downloadFile(bucket: string, key: string, localPath: string): Promise<void> {
        await this.request('/workspace/download', {
            method: 'POST',
            body: JSON.stringify({ bucket, key, localPath })
        });
    }

    async deleteObject(bucket: string, key: string): Promise<void> {
        await this.request(`/buckets/${encodeURIComponent(bucket)}/objects/${encodeURIComponent(key)}`, {
            method: 'DELETE'
        });
    }

    async copyObject(sourceBucket: string, sourceKey: string, targetBucket: string, targetKey: string): Promise<void> {
        // TODO: Implement copy operation in the server
        throw new Error('Copy operation not yet implemented');
    }

    async getObjectContent(bucket: string, key: string): Promise<string> {
        // For preview functionality - download to temp file and read
        const tempPath = `/tmp/${key.replace(/[/\\]/g, '_')}_${Date.now()}`;
        await this.downloadFile(bucket, key, tempPath);
        
        // Read file content (this would need to be implemented properly)
        // For now, return placeholder
        return `Content of s3://${bucket}/${key}\n\n[Preview functionality coming soon]`;
    }

    // History operations
    async getHistory(): Promise<HistoryEntry[]> {
        return this.request('/history');
    }

    // Bookmark operations
    async getBookmarks(): Promise<Bookmark[]> {
        return this.request('/bookmarks');
    }

    async createBookmark(bookmark: Bookmark): Promise<void> {
        await this.request('/bookmarks', {
            method: 'POST',
            body: JSON.stringify(bookmark)
        });
    }

    async deleteBookmark(id: string): Promise<void> {
        await this.request(`/bookmarks/${encodeURIComponent(id)}`, {
            method: 'DELETE'
        });
    }

    // Configuration
    async getConfig(): Promise<any> {
        return this.request('/config');
    }

    async updateConfig(config: any): Promise<void> {
        await this.request('/config', {
            method: 'PUT',
            body: JSON.stringify(config)
        });
    }

    // Health check
    async checkHealth(): Promise<boolean> {
        try {
            const response = await fetch(`${this.baseUrl.replace('/api/vscode', '')}/health`);
            return response.ok;
        } catch {
            return false;
        }
    }
}