import {
  GetDownloads,
  GetDownloadHistory,
  StartDownload,
  PauseDownload,
  ResumeDownload,
  CancelDownload
} from '../../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime';
import type { GameEntity } from '../types/game';

export interface DownloadItem {
  downloadId: string;
  gameId: number;
  gameTitle: string;
  coverImage?: string;
  downloadedBytes: number;
  totalBytes: number;
  progressPercent: number;
  speedBytesPerSec: number;
  speedDisplay: string;
  etaSeconds: number;
  etaDisplay: string;
  status: 'queued' | 'scanning' | 'downloading' | 'paused' | 'completed' | 'failed' | 'cancelled';
  currentFile: string;
  fileIndex?: number;
  totalFiles?: number;
  localPath: string;
  errorMessage?: string;
  isTorrent?: boolean;
  magnetUri?: string;
  torrentSeeds?: number;
  torrentPeers?: number;
}

export interface DownloadRecord {
  id: string;
  gameId: number;
  gameTitle: string;
  remotePath: string;
  localPath: string;
  totalBytes: number;
  downloadedBytes: number;
  status: string;
  errorMessage: string;
  isTorrent?: boolean;
  magnetUri?: string;
  createdAt: number;
  updatedAt: number;
}

class DownloadsStore {
  // Reactive states
  activeDownloads = $state<DownloadItem[]>([]);
  downloadHistory = $state<DownloadRecord[]>([]);
  isLoading = $state<boolean>(false);
  peakSpeedBytes = $state<number>(0);

  private isInitialized = false;
  private unsubProgress: (() => void) | null = null;
  private unsubUpdated: (() => void) | null = null;
  private refreshDebounceTimer: any = null;

  // Derived properties
  get safeActiveDownloads(): DownloadItem[] {
    return (this.activeDownloads || []).filter(
      (d) => d && d.downloadId && d.status !== 'completed' && d.status !== 'cancelled'
    );
  }

  get currentDownload(): DownloadItem | null {
    const list = this.safeActiveDownloads;
    if (list.length === 0) return null;
    const active = list.find((d) => d.status === 'downloading' || d.status === 'scanning');
    return active || list[0];
  }

  get queuedDownloads(): DownloadItem[] {
    const current = this.currentDownload;
    if (!current) return [];
    return this.safeActiveDownloads.filter((d) => d.downloadId !== current.downloadId);
  }

  get safeDownloadHistory(): DownloadRecord[] {
    const list = this.downloadHistory || [];
    const activeIds = new Set(this.safeActiveDownloads.map((d) => d.downloadId));
    const activeGameIds = new Set(this.safeActiveDownloads.map((d) => d.gameId));
    return list.filter((r) => !activeIds.has(r.id) && !activeGameIds.has(r.gameId));
  }

  get completedHistory(): DownloadRecord[] {
    return this.safeDownloadHistory.filter((r) => r.status === 'completed');
  }

  get activeCount(): number {
    return this.safeActiveDownloads.filter(
      (d) => d.status === 'downloading' || d.status === 'scanning' || d.status === 'queued'
    ).length;
  }

  get downloadingCount(): number {
    return this.safeActiveDownloads.filter((d) => d.status === 'downloading').length;
  }

  get pausedCount(): number {
    return this.safeActiveDownloads.filter((d) => d.status === 'paused' || d.status === 'failed').length;
  }

  get totalSpeedBytes(): number {
    let sum = 0;
    for (const dl of this.safeActiveDownloads) {
      if (dl.status === 'downloading' && dl.speedBytesPerSec > 0) {
        sum += dl.speedBytesPerSec;
      }
    }
    return sum;
  }

  init() {
    if (this.isInitialized) return;
    this.isInitialized = true;

    // Listen to real-time speed / progress telemetry
    this.unsubProgress = EventsOn('download:progress', (event: any) => {
      if (!event || !event.downloadId) return;

      const list = this.activeDownloads;
      const idx = list.findIndex((d) => d && d.downloadId === event.downloadId);
      if (idx >= 0) {
        list[idx] = event as DownloadItem;
        this.activeDownloads = [...list];
      } else {
        this.activeDownloads = [...list, event as DownloadItem];
      }

      // Track peak speed
      const totalSpeed = this.totalSpeedBytes;
      if (totalSpeed > this.peakSpeedBytes) {
        this.peakSpeedBytes = totalSpeed;
      }

      // If task transitioned to completed or cancelled, trigger debounced history refresh
      if (event.status === 'completed' || event.status === 'cancelled') {
        this.debouncedRefresh();
      }
    });

    // Listen to queue / task changes from backend
    this.unsubUpdated = EventsOn('downloads:updated', () => {
      this.refresh();
    });

    // Initial load
    this.refresh();
  }

  async refresh() {
    try {
      this.isLoading = true;
      const [downloads, history] = await Promise.all([
        GetDownloads().catch((err) => {
          console.warn('[DownloadsStore] GetDownloads failed:', err);
          return [];
        }),
        GetDownloadHistory().catch((err) => {
          console.warn('[DownloadsStore] GetDownloadHistory failed:', err);
          return [];
        })
      ]);

      this.activeDownloads = Array.isArray(downloads) ? (downloads as DownloadItem[]) : [];
      this.downloadHistory = Array.isArray(history) ? (history as DownloadRecord[]) : [];
    } catch (err) {
      console.error('[DownloadsStore] refresh error:', err);
    } finally {
      this.isLoading = false;
    }
  }

  private debouncedRefresh() {
    if (this.refreshDebounceTimer) clearTimeout(this.refreshDebounceTimer);
    this.refreshDebounceTimer = setTimeout(() => {
      this.refresh();
    }, 300);
  }

  // Find download matching game or any of its variants
  getDownloadForGame(game: GameEntity | null): DownloadItem | null {
    if (!game) return null;
    const list = this.safeActiveDownloads;
    if (list.length === 0) return null;

    // 1. Direct game ID match
    const direct = list.find((d) => d.gameId === game.id);
    if (direct) return direct;

    // 2. Variant ID match
    if (game.variants && game.variants.length > 0) {
      const variantIds = new Set(game.variants.map((v) => v.id));
      const byVariant = list.find((d) => variantIds.has(d.gameId));
      if (byVariant) return byVariant;
    }

    // 3. Normalized title match fallback
    const normClean = (game.cleanTitle || game.rawName || '').trim().toLowerCase();
    if (normClean) {
      const byTitle = list.find((d) => (d.gameTitle || '').trim().toLowerCase() === normClean);
      if (byTitle) return byTitle;
    }

    return null;
  }

  // Find completed / historical record matching game or any of its variants
  getRecordForGame(game: GameEntity | null): DownloadRecord | null {
    if (!game) return null;
    const list = this.downloadHistory || [];
    if (list.length === 0) return null;

    const direct = list.find((r) => r.gameId === game.id);
    if (direct) return direct;

    if (game.variants && game.variants.length > 0) {
      const variantIds = new Set(game.variants.map((v) => v.id));
      const byVariant = list.find((r) => variantIds.has(r.gameId));
      if (byVariant) return byVariant;
    }

    const normClean = (game.cleanTitle || game.rawName || '').trim().toLowerCase();
    if (normClean) {
      const byTitle = list.find((r) => (r.gameTitle || '').trim().toLowerCase() === normClean);
      if (byTitle) return byTitle;
    }

    return null;
  }

  isGameDownloading(game: GameEntity | null): boolean {
    const dl = this.getDownloadForGame(game);
    if (!dl) return false;
    return dl.status === 'downloading' || dl.status === 'scanning' || dl.status === 'queued';
  }

  isGameInstalled(game: GameEntity | null): boolean {
    const rec = this.getRecordForGame(game);
    return rec?.status === 'completed';
  }

  // Lifecycle actions
  async start(gameId: number, targetPath: string): Promise<string> {
    const dlId = await StartDownload(gameId, targetPath);
    await this.refresh();
    return dlId;
  }

  async pause(downloadId: string) {
    // Optimistic UI update
    const idx = this.activeDownloads.findIndex((d) => d.downloadId === downloadId);
    if (idx >= 0) {
      this.activeDownloads[idx].status = 'paused';
      this.activeDownloads = [...this.activeDownloads];
    }
    await PauseDownload(downloadId);
    await this.refresh();
  }

  async resume(downloadId: string) {
    // Optimistic UI update
    const idx = this.activeDownloads.findIndex((d) => d.downloadId === downloadId);
    if (idx >= 0) {
      this.activeDownloads[idx].status = 'queued';
      this.activeDownloads = [...this.activeDownloads];
    }
    await ResumeDownload(downloadId);
    await this.refresh();
  }

  async cancel(downloadId: string) {
    this.activeDownloads = this.activeDownloads.filter((d) => d && d.downloadId !== downloadId);
    await CancelDownload(downloadId);
    await this.refresh();
  }

  async pauseAll() {
    const app = (window as any)?.go?.main?.App;
    if (app && app.PauseAllDownloads) {
      await app.PauseAllDownloads();
      await this.refresh();
    }
  }

  async resumeAll() {
    const app = (window as any)?.go?.main?.App;
    if (app && app.ResumeAllDownloads) {
      await app.ResumeAllDownloads();
      await this.refresh();
    }
  }

  async clearCompleted() {
    const app = (window as any)?.go?.main?.App;
    if (app && app.ClearCompletedDownloads) {
      await app.ClearCompletedDownloads();
      this.activeDownloads = this.activeDownloads.filter(
        (d) => d && d.status !== 'completed' && d.status !== 'cancelled'
      );
      await this.refresh();
    }
  }

  async deleteRecord(downloadId: string, removeFiles: boolean = false) {
    const app = (window as any)?.go?.main?.App;
    if (app && app.DeleteDownloadRecord) {
      await app.DeleteDownloadRecord(downloadId, removeFiles);
      this.activeDownloads = this.activeDownloads.filter((d) => d && d.downloadId !== downloadId);
      this.downloadHistory = this.downloadHistory.filter((r) => r && r.id !== downloadId);
      await this.refresh();
    }
  }

  destroy() {
    if (this.unsubProgress) {
      this.unsubProgress();
      this.unsubProgress = null;
    }
    if (this.unsubUpdated) {
      this.unsubUpdated();
      this.unsubUpdated = null;
    }
    if (this.refreshDebounceTimer) {
      clearTimeout(this.refreshDebounceTimer);
      this.refreshDebounceTimer = null;
    }
    this.isInitialized = false;
  }
}

export const downloadsStore = new DownloadsStore();
