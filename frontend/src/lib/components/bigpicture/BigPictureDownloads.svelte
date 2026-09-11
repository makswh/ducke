<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Pause,
    Play,
    X,
    FolderOpen,
    Trash2,
    HardDrive,
    Download,
    RefreshCw,
    Check,
    ArrowRight
  } from 'lucide-svelte';
  import { sound } from '../../navigation/audio';

  interface DownloadItem {
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
  }

  interface DownloadRecord {
    id: string;
    gameId: number;
    gameTitle: string;
    remotePath: string;
    localPath: string;
    totalBytes: number;
    downloadedBytes: number;
    status: string;
    errorMessage: string;
    createdAt: number;
    updatedAt: number;
  }

  interface DiskInfo {
    freeBytes: number;
    totalBytes: number;
    freeGB: string;
    totalGB: string;
  }

  let {
    games = [] as any[],
    activeDownloads = [] as DownloadItem[],
    downloadHistory = [] as DownloadRecord[],
    downloadPath = '',
    onPause = (id: string) => {},
    onResume = (id: string) => {},
    onCancel = (id: string) => {},
    onPauseAll = () => {},
    onResumeAll = () => {},
    onClearCompleted = () => {},
    onDeleteRecord = (id: string, removeFiles: boolean) => {},
    onOpenFolder = (path: string) => {},
    onGoToCatalog = () => {}
  } = $props();

  let isMounted = true;
  let resolvedCovers = $state<Record<string, string>>({});
  let resolvedLogos = $state<Record<string, string>>({});
  let imageLoadFailed = $state<Record<string, boolean>>({});

  onDestroy(() => {
    isMounted = false;
  });

  async function resolveMissingLogo(title: string, gameId?: number) {
    if (!title || !isMounted) return;
    const key = title.trim().toLowerCase();
    if (resolvedLogos[key] !== undefined) return;
    resolvedLogos[key] = '';

    try {
      const app = (window as any)?.go?.main?.App;
      if (app && typeof app.ResolveGameLogo === 'function') {
        const g = (games || []).find(
          (x) => x && ((gameId && x.id === gameId) || (x.cleanTitle && x.cleanTitle.toLowerCase() === key))
        );
        const appId = g?.steamAppId || 0;
        const gId = g?.id || gameId || 0;

        const searchTitle = title.replace(/\[.*?\]|\(.*?\)/g, '').trim() || title;
        const url = await app.ResolveGameLogo(gId, searchTitle, appId);
        if (url && isMounted) {
          resolvedLogos[key] = url;
        }
      }
    } catch {
      // Ignore if no logo available
    }
  }

  function getGameLogo(dl: DownloadItem): string {
    if (!dl) return '';
    const key = (dl.gameTitle || '').trim().toLowerCase();
    if (resolvedLogos[key] !== undefined) {
      return resolvedLogos[key] && !imageLoadFailed[resolvedLogos[key]] ? resolvedLogos[key] : '';
    }
    const g = (games || []).find(
      (x) => x && ((dl.gameId && x.id === dl.gameId) || (x.cleanTitle && x.cleanTitle.toLowerCase() === key))
    );
    if (g && g.steamAppId && g.steamAppId > 0) {
      const steamLogo = `https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/logo.png`;
      if (!imageLoadFailed[steamLogo]) {
        return steamLogo;
      }
    }
    resolveMissingLogo(dl.gameTitle, dl.gameId);
    return '';
  }

  async function resolveMissingCover(title: string, gameId?: number) {
    if (!title || !isMounted) return;
    const key = title.trim().toLowerCase();
    if (resolvedCovers[key] !== undefined) return;
    resolvedCovers[key] = '';

    try {
      const app = (window as any)?.go?.main?.App;
      if (app) {
        let url = '';
        const g = (games || []).find(
          (x) => x && ((gameId && x.id === gameId) || (x.cleanTitle && x.cleanTitle.toLowerCase() === key))
        );
        const appId = g?.steamAppId || 0;
        const gId = g?.id || gameId || 0;

        const searchTitle = title.replace(/\[.*?\]|\(.*?\)/g, '').trim() || title;
        if (typeof app.ResolveGameBanner === 'function') {
          url = await app.ResolveGameBanner(gId, searchTitle, appId);
        } else if (typeof app.ResolveGameCover === 'function') {
          url = await app.ResolveGameCover(gId, searchTitle, appId);
        }

        if (url && isMounted) {
          resolvedCovers[key] = url;
        }
      }
    } catch {
      // Normal fallback when cover is not available; resolvedCovers[key] remains '' to prevent repeat attempts
    }
  }

  function getGameBackground(dl: DownloadItem): string {
    if (!dl) return '';
    const key = (dl.gameTitle || '').trim().toLowerCase();
    const g = (games || []).find(
      (x) => x && ((dl.gameId && x.id === dl.gameId) || (x.cleanTitle && x.cleanTitle.toLowerCase() === key))
    );
    if (g) {
      if (g.backgroundImage && !imageLoadFailed[g.backgroundImage]) {
        return g.backgroundImage;
      }
      if (g.screenshots && g.screenshots.length > 0 && !imageLoadFailed[g.screenshots[0]]) {
        return g.screenshots[0];
      }
      if (g.steamAppId && g.steamAppId > 0) {
        const v6b = `https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/page_bg_generated_v6b.jpg`;
        if (!imageLoadFailed[v6b]) return v6b;
        const pageBg = `https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/page.bg.jpg`;
        if (!imageLoadFailed[pageBg]) return pageBg;
      }
    }
    if (resolvedCovers[key] !== undefined) {
      return resolvedCovers[key] && !imageLoadFailed[resolvedCovers[key]] ? resolvedCovers[key] : '';
    }
    if (dl.coverImage && !imageLoadFailed[dl.coverImage]) {
      return dl.coverImage;
    }
    resolveMissingCover(dl.gameTitle, dl.gameId);
    return '';
  }

  function getGameBanner(dl: DownloadItem): string {
    if (!dl) return '';
    const key = (dl.gameTitle || '').trim().toLowerCase();
    const g = (games || []).find(
      (x) => x && ((dl.gameId && x.id === dl.gameId) || (x.cleanTitle && x.cleanTitle.toLowerCase() === key))
    );
    if (g) {
      if (g.headerImage && !imageLoadFailed[g.headerImage]) {
        return g.headerImage;
      }
      if (g.steamAppId && g.steamAppId > 0) {
        const header = `https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/header.jpg`;
        if (!imageLoadFailed[header]) return header;
      }
      if (g.backgroundImage && !imageLoadFailed[g.backgroundImage]) {
        return g.backgroundImage;
      }
      if (g.capsuleImage && !imageLoadFailed[g.capsuleImage]) {
        return g.capsuleImage;
      }
    }
    if (dl.coverImage && !imageLoadFailed[dl.coverImage]) {
      return dl.coverImage;
    }
    if (resolvedCovers[key] !== undefined) {
      return resolvedCovers[key] && !imageLoadFailed[resolvedCovers[key]] ? resolvedCovers[key] : '';
    }
    resolveMissingCover(dl.gameTitle, dl.gameId);
    return '';
  }

  function getGameCover(dl: DownloadItem): string {
    if (!dl) return '';
    const key = (dl.gameTitle || '').trim().toLowerCase();
    const g = (games || []).find(
      (x) => x && ((dl.gameId && x.id === dl.gameId) || (x.cleanTitle && x.cleanTitle.toLowerCase() === key))
    );
    if (g) {
      if (g.capsuleImage && !imageLoadFailed[g.capsuleImage]) {
        return g.capsuleImage;
      }
      if (g.headerImage && !imageLoadFailed[g.headerImage]) {
        return g.headerImage;
      }
      if (g.backgroundImage && !imageLoadFailed[g.backgroundImage]) {
        return g.backgroundImage;
      }
      if (g.steamAppId && g.steamAppId > 0) {
        const header = `https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/header.jpg`;
        if (!imageLoadFailed[header]) return header;
      }
    }
    if (resolvedCovers[key] !== undefined) {
      return resolvedCovers[key] && !imageLoadFailed[resolvedCovers[key]] ? resolvedCovers[key] : '';
    }
    if (dl.coverImage && !imageLoadFailed[dl.coverImage]) {
      return dl.coverImage;
    }
    resolveMissingCover(dl.gameTitle, dl.gameId);
    return '';
  }

  function getRecordCover(record: DownloadRecord): string {
    if (!record) return '';
    const key = (record.gameTitle || '').trim().toLowerCase();
    const g = (games || []).find(
      (x) => x && ((record.gameId && x.id === record.gameId) || (x.cleanTitle && x.cleanTitle.toLowerCase() === key))
    );
    if (g) {
      if (g.capsuleImage && !imageLoadFailed[g.capsuleImage]) {
        return g.capsuleImage;
      }
      if (g.headerImage && !imageLoadFailed[g.headerImage]) {
        return g.headerImage;
      }
      if (g.backgroundImage && !imageLoadFailed[g.backgroundImage]) {
        return g.backgroundImage;
      }
      if (g.steamAppId && g.steamAppId > 0) {
        const header = `https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/${g.steamAppId}/header.jpg`;
        if (!imageLoadFailed[header]) return header;
      }
    }
    if (resolvedCovers[key] !== undefined) {
      return resolvedCovers[key] && !imageLoadFailed[resolvedCovers[key]] ? resolvedCovers[key] : '';
    }
    resolveMissingCover(record.gameTitle, record.gameId);
    return '';
  }

  let diskSpace = $state<DiskInfo | null>(null);
  let peakSpeedBytes = $state<number>(0);

  let safeActiveDownloads = $derived.by<DownloadItem[]>(() => {
    return (activeDownloads || []).filter((d) => d && d.downloadId && d.status !== 'completed' && d.status !== 'cancelled');
  });

  let currentDownload = $derived.by<DownloadItem | null>(() => {
    if (safeActiveDownloads.length === 0) return null;
    const active = safeActiveDownloads.find((d) => d.status === 'downloading' || d.status === 'scanning');
    return active || safeActiveDownloads[0];
  });

  let queuedDownloads = $derived.by<DownloadItem[]>(() => {
    if (!currentDownload) return [];
    return safeActiveDownloads.filter((d) => d.downloadId !== currentDownload?.downloadId);
  });

  let safeDownloadHistory = $derived.by<DownloadRecord[]>(() => {
    const list = downloadHistory || [];
    const activeIds = new Set(safeActiveDownloads.map((d) => d.downloadId));
    const activeGameIds = new Set(safeActiveDownloads.map((d) => d.gameId));
    return list.filter((r) => !activeIds.has(r.id) && !activeGameIds.has(r.gameId));
  });

  let totalSpeedBytes = $derived.by<number>(() => {
    let sum = 0;
    for (const dl of safeActiveDownloads) {
      if (dl.status === 'downloading' && dl.speedBytesPerSec > 0) {
        sum += dl.speedBytesPerSec;
      }
    }
    return sum;
  });

  $effect(() => {
    if (totalSpeedBytes > peakSpeedBytes) {
      peakSpeedBytes = totalSpeedBytes;
    }
  });

  $effect(() => {
    if (currentDownload?.gameTitle) {
      resolveMissingCover(currentDownload.gameTitle, currentDownload.gameId);
      resolveMissingLogo(currentDownload.gameTitle, currentDownload.gameId);
    }
    for (const item of queuedDownloads) {
      if (item.gameTitle) resolveMissingCover(item.gameTitle, item.gameId);
    }
    for (const item of safeDownloadHistory) {
      if (item.gameTitle) resolveMissingCover(item.gameTitle, item.gameId);
    }
  });

  let isLinux = $state<boolean>(false);
  let installerMap = $state<Record<string, string>>({});
  let installingId = $state<string | null>(null);

  async function checkInstallers() {
    const app = (window as any)?.go?.main?.App;
    if (!app) return;

    if (!isLinux) {
      try {
        if (typeof app.IsLinuxSystem === 'function') {
          isLinux = await app.IsLinuxSystem();
        }
      } catch {}
    }

    if (!isLinux) return;

    for (const record of safeDownloadHistory) {
      if (record.status === 'completed' && record.localPath && !installerMap[record.id]) {
        try {
          if (typeof app.FindLinuxInstaller === 'function') {
            const inst = await app.FindLinuxInstaller(record.localPath);
            if (inst) {
              installerMap[record.id] = inst;
            }
          }
        } catch {}
      }
    }
  }

  async function handleLaunchInstaller(installerPath: string, recordId?: string) {
    if (!installerPath) return;
    sound.playSelect();
    try {
      if (recordId) installingId = recordId;
      const app = (window as any)?.go?.main?.App;
      if (app && typeof app.LaunchLinuxInstaller === 'function') {
        await app.LaunchLinuxInstaller(installerPath);
      }
    } catch (e) {
      console.error('Failed to launch installer:', e);
    } finally {
      setTimeout(() => {
        if (installingId === recordId) installingId = null;
      }, 3000);
    }
  }

  $effect(() => {
    if (safeDownloadHistory.length > 0) {
      checkInstallers();
    }
  });

  let downloadingCount = $derived.by<number>(() => {
    return safeActiveDownloads.filter((d) => d.status === 'downloading').length;
  });

  let pausedCount = $derived.by<number>(() => {
    return safeActiveDownloads.filter((d) => d.status === 'paused' || d.status === 'failed').length;
  });

  function formatSpeedRu(bytesPerSec: number): string {
    if (bytesPerSec <= 0) return '0 байт/с';
    const mb = bytesPerSec / (1024 * 1024);
    if (mb >= 1.0) {
      return `${mb.toFixed(1).replace('.', ',')} Мбайт/с`;
    }
    const kb = bytesPerSec / 1024;
    return `${kb.toFixed(1).replace('.', ',')} Кбайт/с`;
  }

  function formatBytesRu(bytes: number): string {
    if (bytes <= 0) return '0 байт';
    const gb = bytes / (1024 * 1024 * 1024);
    if (gb >= 1.0) {
      return `${gb.toFixed(1).replace('.', ',')} ГБ`;
    }
    const mb = bytes / (1024 * 1024);
    if (mb >= 1.0) {
      return `${mb.toFixed(1).replace('.', ',')} МБ`;
    }
    const kb = bytes / 1024;
    return `${kb.toFixed(0)} КБ`;
  }

  function formatSteamDate(timestamp: number): string {
    if (!timestamp) return '';
    const d = new Date(timestamp * 1000);
    const now = new Date();
    const isToday = d.toDateString() === now.toDateString();
    const timeStr = d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
    if (isToday) {
      return `СЕГОДНЯ ${timeStr}`;
    }
    const yesterday = new Date(now);
    yesterday.setDate(yesterday.getDate() - 1);
    if (d.toDateString() === yesterday.toDateString()) {
      return `ВЧЕРА ${timeStr}`;
    }
    return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' }).toUpperCase() + `, ${timeStr}`;
  }

  function formatSteamETA(seconds: number): string {
    if (seconds <= 0) return '--';
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    const mStr = mins < 10 ? `0${mins}` : `${mins}`;
    const sStr = secs < 10 ? `0${secs}` : `${secs}`;
    return `${mStr}:${sStr}`;
  }

  async function loadDiskSpace() {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.GetDiskSpaceInfo) {
        const info = await app.GetDiskSpaceInfo(downloadPath);
        if (isMounted && info && info.freeBytes > 0) {
          diskSpace = info;
        }
      }
    } catch {
      // Ignore
    }
  }

  let activeBackgroundUrl = $derived.by<string>(() => {
    if (!currentDownload) return '';
    return getGameBackground(currentDownload);
  });

  let speedHistory = $state<number[]>(Array(90).fill(0));
  let diskHistory = $state<number[]>(Array(90).fill(0));

  function getHistBars(width = 800, height = 90) {
    const sampleCount = 90;
    const recent = speedHistory.slice(-sampleCount);
    const maxVal = Math.max(peakSpeedBytes, 1024 * 1024);
    const step = width / sampleCount;
    const barW = step * 0.5; // Exact 1:1 ratio between bar width and gap

    return recent.map((val, idx) => {
      const barH = val > 0 ? Math.max(3, (val / maxVal) * (height - 12)) : 0;
      return {
        x: (idx * step).toFixed(2),
        y: (height - barH).toFixed(2),
        w: barW.toFixed(2),
        h: barH.toFixed(2)
      };
    });
  }

  function getDiskLine(width = 800, height = 90): string {
    const sampleCount = 90;
    const recent = diskHistory.slice(-sampleCount);
    const maxVal = Math.max(peakSpeedBytes, 1024 * 1024);
    const step = width / (sampleCount - 1);
    const halfBar = (width / sampleCount) * 0.25;

    return recent
      .map((val, idx) => {
        const x = (idx * step + halfBar).toFixed(2);
        const y = val > 0 ? (height - (val / maxVal) * (height - 14) - 6).toFixed(2) : (height - 2).toFixed(2);
        return `${idx === 0 ? 'M' : 'L'} ${x} ${y}`;
      })
      .join(' ');
  }

  onMount(() => {
    isMounted = true;
    loadDiskSpace();
    const diskTimer = setInterval(() => {
      if (isMounted) loadDiskSpace();
    }, 5000);
    const speedTimer = setInterval(() => {
      if (!isMounted) return;
      const curNet = totalSpeedBytes;
      const curDisk = (currentDownload && currentDownload.status === 'downloading') ? Math.round(curNet * 0.95) : 0;
      speedHistory = [...speedHistory.slice(1), curNet];
      diskHistory = [...diskHistory.slice(1), curDisk];
    }, 1000);
    return () => {
      isMounted = false;
      clearInterval(diskTimer);
      clearInterval(speedTimer);
    };
  });
</script>

<!-- SteamOS 10-Foot Big Picture Downloads View -->
<div data-nav-zone="detail" class="flex-1 flex flex-col h-full overflow-y-auto relative select-none bg-[#07080a] text-[#ededed] font-sans">
  
  <!-- Atmospheric Steam Backdrop with dark scrim for crisp typography contrast -->
  {#if activeBackgroundUrl}
    <div
      class="absolute top-0 left-0 right-0 h-[600px] overflow-hidden pointer-events-none z-0 select-none"
      style="mask-image: linear-gradient(to bottom, rgba(0,0,0,0.9) 0%, rgba(0,0,0,0.7) 35%, rgba(0,0,0,0.2) 75%, rgba(0,0,0,0) 100%); -webkit-mask-image: linear-gradient(to bottom, rgba(0,0,0,0.9) 0%, rgba(0,0,0,0.7) 35%, rgba(0,0,0,0.2) 75%, rgba(0,0,0,0) 100%);"
    >
      <img
        src={activeBackgroundUrl}
        alt=""
        referrerpolicy="no-referrer"
        class="w-full h-full object-cover object-top brightness-60 contrast-105 opacity-70 transition-opacity duration-500"
        onerror={() => {
          imageLoadFailed[activeBackgroundUrl] = true;
        }}
      />
      <!-- Dark Scrim Gradients matching Ducke #07080a base -->
      <div class="absolute inset-0 bg-gradient-to-b from-[#07080a]/40 via-[#07080a]/75 to-[#07080a]"></div>
      <div class="absolute inset-0 bg-gradient-to-r from-[#07080a]/80 via-transparent to-[#07080a]/80"></div>
    </div>
  {/if}

  <!-- ============================================================ -->
  <!-- 1. TOP ACTIVE DOWNLOAD: FULL-WIDTH FLUSH COMMAND DECK        -->
  <!-- ============================================================ -->
  {#if currentDownload}
    {@const isDownloading = currentDownload.status === 'downloading'}
    {@const isPaused = currentDownload.status === 'paused'}
    {@const isScanning = currentDownload.status === 'scanning'}
    {@const isFailed = currentDownload.status === 'failed'}
    {@const percent = (currentDownload.progressPercent || 0).toFixed(1)}
    {@const logoUrl = getGameLogo(currentDownload)}
    {@const bannerUrl = getGameBanner(currentDownload)}

    <div class="relative z-10 w-full rounded-none bg-[#090d14] border-b border-[#1b222d] border-t-0 border-x-0 overflow-hidden shadow-xl flex flex-col md:flex-row items-stretch min-h-[170px] sm:min-h-[180px]">
      
      <!-- 1. LEFT GAME BANNER WITH OFFICIAL LOGO / TITLE OVERLAY -->
      <div class="relative w-full md:w-[400px] lg:w-[460px] shrink-0 h-[170px] sm:h-[180px] overflow-hidden bg-[#0d121a]">
        {#if bannerUrl && !imageLoadFailed[bannerUrl]}
          <img
            src={bannerUrl}
            alt={currentDownload.gameTitle}
            referrerpolicy="no-referrer"
            class="w-full h-full object-cover object-center brightness-90"
            onerror={() => {
              imageLoadFailed[bannerUrl] = true;
              resolveMissingCover(currentDownload.gameTitle, currentDownload.gameId);
            }}
          />
        {/if}
        <!-- Scrim gradient for contrast and seamless blend on right -->
        <div class="absolute inset-0 bg-gradient-to-t from-black/85 via-black/30 to-transparent"></div>
        <div class="absolute top-0 bottom-0 right-0 w-16 bg-gradient-to-r from-transparent to-[#090d14] hidden md:block"></div>

        <!-- Game Logo or Title overlaid at bottom left -->
        <div class="absolute bottom-3.5 left-5 right-5 flex items-end z-20">
          {#if logoUrl && !imageLoadFailed[logoUrl]}
            <img
              src={logoUrl}
              alt={currentDownload.gameTitle}
              class="max-h-12 sm:max-h-14 max-w-[85%] object-contain object-left filter drop-shadow-[0_2px_8px_rgba(0,0,0,0.95)]"
              onerror={() => {
                imageLoadFailed[logoUrl] = true;
                resolveMissingLogo(currentDownload.gameTitle, currentDownload.gameId);
              }}
            />
          {:else}
            <span class="text-lg sm:text-xl font-black text-white uppercase tracking-wider filter drop-shadow-[0_2px_6px_rgba(0,0,0,0.95)] line-clamp-2">
              {currentDownload.gameTitle}
            </span>
          {/if}
        </div>
      </div>

      <!-- 2. LIVE GRAPH SPANNING ACROSS BANNER RIGHT & MIDDLE SECTION (1:1 bar:gap) -->
      <div class="absolute bottom-0 left-[260px] sm:left-[300px] md:left-[340px] right-[400px] sm:right-[440px] md:right-[480px] h-[100px] z-10 pointer-events-none overflow-hidden hidden sm:flex items-end">
        <svg class="w-full h-[90px]" viewBox="0 0 800 90" preserveAspectRatio="none">
          {#each getHistBars(800, 90) as bar}
            <rect x={bar.x} y={bar.y} width={bar.w} height={bar.h} fill="#1a9fff" opacity="0.85" />
          {/each}
          <path d={getDiskLine(800, 90)} fill="none" stroke="#22c55e" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </div>

      <!-- 3. MIDDLE AREA (Gamepad actions: Folder, Cancel) -->
      <div class="hidden md:flex flex-col justify-center py-4 px-3 relative z-20 shrink-0 space-y-2">
        {#if currentDownload.localPath}
          <button
            data-nav-item
            type="button"
            class="w-10 h-10 rounded-xl bg-white/5 hover:bg-white/10 active:bg-white/15 focus:ring-2 focus:ring-white focus:outline-none flex items-center justify-center text-[#8f98a0] hover:text-white transition-colors cursor-pointer"
            onclick={() => {
              sound.playSelect();
              onOpenFolder(currentDownload.localPath);
            }}
            title="Открыть папку"
          >
            <FolderOpen class="w-4.5 h-4.5" />
          </button>
        {/if}
        <button
          data-nav-item
          type="button"
          class="w-10 h-10 rounded-xl bg-white/5 hover:bg-rose-500/20 active:bg-rose-500/30 focus:ring-2 focus:ring-rose-400 focus:outline-none flex items-center justify-center text-[#8f98a0] hover:text-rose-400 transition-colors cursor-pointer"
          onclick={() => {
            sound.playBack();
            onCancel(currentDownload.downloadId);
          }}
          title="Отменить загрузку"
        >
          <X class="w-4.5 h-4.5" />
        </button>
      </div>

      <!-- 4. RIGHT TELEMETRY & PROGRESS PANEL (10-Foot Console Layout) -->
      <div class="flex-1 min-w-0 md:max-w-[480px] ml-auto flex flex-col justify-between px-5 sm:px-7 py-4 bg-[#090d14] relative z-20 border-t md:border-t-0 md:border-l border-[#1b222d]">
        
        <!-- Top Metrics Row -->
        <div class="flex items-start justify-between gap-3 min-w-0">
          <div class="flex flex-col items-start min-w-0">
            <!-- Row 1: СЕТЬ / МАКС. / ИСП. ДИСКА -->
            <div class="flex items-center gap-5 text-xs font-mono">
              <!-- СЕТЬ -->
              <div class="flex flex-col">
                <div class="flex items-center gap-1 text-[10px] font-mono font-bold text-sky-400 tracking-wider uppercase">
                  <svg class="w-2.5 h-2.5 text-sky-400" viewBox="0 0 12 12" fill="currentColor">
                    <rect x="1" y="6" width="2" height="6" rx="0.5" />
                    <rect x="5" y="3" width="2" height="9" rx="0.5" />
                    <rect x="9" y="1" width="2" height="11" rx="0.5" />
                  </svg>
                  <span>СЕТЬ</span>
                </div>
                <span class="text-sm font-mono font-bold text-white tracking-tight">
                  {formatSpeedRu(totalSpeedBytes)}
                </span>
              </div>

              <!-- МАКС. -->
              <div class="flex flex-col">
                <div class="flex items-center gap-1 text-[10px] font-mono font-bold text-sky-400 tracking-wider uppercase">
                  <svg class="w-2.5 h-2.5 text-sky-400" viewBox="0 0 12 12" fill="currentColor">
                    <rect x="1" y="6" width="2" height="6" rx="0.5" />
                    <rect x="5" y="3" width="2" height="9" rx="0.5" />
                    <rect x="9" y="1" width="2" height="11" rx="0.5" />
                  </svg>
                  <span>МАКС.</span>
                </div>
                <span class="text-sm font-mono font-bold text-white tracking-tight">
                  {formatSpeedRu(peakSpeedBytes)}
                </span>
              </div>

              <!-- ИСП. ДИСКА -->
              <div class="flex flex-col">
                <div class="flex items-center gap-1 text-[10px] font-mono font-bold text-emerald-400 tracking-wider uppercase">
                  <span class="w-2.5 h-0.5 bg-emerald-400 inline-block"></span>
                  <span>ИСП. ДИСКА</span>
                </div>
                <span class="text-sm font-mono font-bold text-white tracking-tight">
                  {formatSpeedRu(isDownloading ? Math.round(totalSpeedBytes * 0.96) : 0)}
                </span>
              </div>
            </div>

            <!-- Row 2: DRIVE FREE SPACE -->
            {#if diskSpace}
              <div class="text-[11px] font-mono text-[#8f98a0] mt-1.5 flex items-center gap-1.5">
                <HardDrive class="w-3.5 h-3.5 text-[#64748b]" />
                <span>СВОБОДНО: <strong class="text-white font-normal">{diskSpace.freeGB}</strong></span>
              </div>
            {/if}
          </div>
        </div>

        <!-- Two Horizontal Progress Bars: Загрузка данных & Установка файлов -->
        <div class="space-y-2.5 my-2">
          <!-- 1: Загрузка данных (Blue) -->
          <div class="space-y-1">
            <div class="flex items-center justify-between text-xs font-mono">
              <span class="text-[#c6d4df] font-medium">Загрузка данных</span>
              <div class="flex items-center gap-1.5 text-xs font-mono text-white font-bold">
                <span>{formatBytesRu(currentDownload.downloadedBytes)} / {formatBytesRu(currentDownload.totalBytes)}</span>
                <Download class="w-3.5 h-3.5 text-[#8f98a0]" />
              </div>
            </div>
            <div class="h-1.5 w-full bg-[#1a2330] rounded-none overflow-hidden">
              <div
                class="h-full bg-[#1a9fff] transition-all duration-300"
                style="width: {percent}%"
              ></div>
            </div>
          </div>

          <!-- 2: Установка файлов (Green) -->
          <div class="space-y-1">
            <div class="flex items-center justify-between text-xs font-mono">
              <span class="text-[#c6d4df] font-medium">Установка файлов</span>
              <span class="text-xs font-mono text-[#8f98a0]">{percent}%</span>
            </div>
            <div class="h-1.5 w-full bg-[#1a2330] rounded-none overflow-hidden">
              <div
                class="h-full bg-[#22c55e] transition-all duration-300"
                style="width: {percent}%"
              ></div>
            </div>
          </div>
        </div>

        <!-- Bottom Line: ETA + Large Console Blue Action Button -->
        <div class="flex items-center justify-between gap-3 pt-1 min-w-0">
          <span class="text-xs font-mono text-[#8f98a0] truncate">
            {#if isPaused}
              Приостановлено
            {:else if isDownloading}
              {#if currentDownload.etaSeconds > 0}
                Осталось примерно {formatSteamETA(currentDownload.etaSeconds)}
              {:else}
                Загрузка данных...
              {/if}
            {:else if isScanning}
              Проверка файлов...
            {:else if isFailed}
              <span class="text-rose-400">Ошибка: {currentDownload.errorMessage || 'Сбой'}</span>
            {:else}
              В очереди
            {/if}
          </span>

          {#if isDownloading || isScanning}
            <button
              data-nav-item
              type="button"
              class="w-11 h-11 bg-[#1a9fff] hover:bg-[#28a8ff] active:bg-[#1388dc] focus:ring-2 focus:ring-white focus:outline-none text-white flex items-center justify-center rounded-xl transition-all cursor-pointer shrink-0 shadow-lg"
              onclick={() => {
                sound.playSelect();
                onPause(currentDownload.downloadId);
              }}
              title="Приостановить"
            >
              <Pause class="w-5 h-5 fill-white text-white" />
            </button>
          {:else}
            <button
              data-nav-item
              type="button"
              class="w-11 h-11 bg-[#1a9fff] hover:bg-[#28a8ff] active:bg-[#1388dc] focus:ring-2 focus:ring-white focus:outline-none text-white flex items-center justify-center rounded-xl transition-all cursor-pointer shrink-0 shadow-lg"
              onclick={() => {
                sound.playSelect();
                onResume(currentDownload.downloadId);
              }}
              title="Возобновить"
            >
              <Play class="w-5 h-5 fill-white text-white ml-0.5" />
            </button>
          {/if}
        </div>

      </div>

    </div>
  {:else}
    <!-- Clean Console Empty State when no active download -->
    <div class="relative z-10 w-full rounded-none bg-[#0c1017] border-b border-[#1e2633] border-t-0 border-x-0 px-8 py-8 flex flex-col sm:flex-row items-center justify-between gap-6">
      <div class="space-y-1 text-center sm:text-left">
        <h2 class="text-lg font-bold text-white uppercase tracking-wider font-mono">Все загрузки завершены</h2>
        <p class="text-xs text-[#8f98a0]">
          В очереди нет активных процессов.
          {#if diskSpace}
            Доступно на накопителе: <span class="text-white font-mono font-bold">{diskSpace.freeGB}</span>.
          {/if}
        </p>
      </div>
      <button
        data-nav-item
        class="px-5 py-2.5 rounded-xl bg-[#171d27] hover:bg-[#202937] focus:ring-2 focus:ring-white focus:outline-none text-[#c6d4df] hover:text-white text-xs font-mono uppercase border border-[#2b3648] cursor-pointer transition-all flex items-center gap-2"
        onclick={() => {
          sound.playSelect();
          onGoToCatalog();
        }}
      >
        <span>Библиотека игр</span>
        <ArrowRight class="w-3.5 h-3.5" />
      </button>
    </div>
  {/if}

  <!-- Lower Container for Queue and Completed (with 10-foot Big Picture padding) -->
  <div class="relative z-10 w-full flex-1 flex flex-col px-6 sm:px-10 lg:px-12 py-8 space-y-8 pb-32">

    <!-- Global HUD Actions (Пауза всех / Возобновить все / Очистить) -->
    <div class="flex items-center justify-between flex-wrap gap-4 border-b border-white/[0.06] pb-4">
      <div class="flex items-center gap-3 text-xs text-[#8e95a2] font-mono">
        <span>В очереди: <strong class="text-white font-bold">{safeActiveDownloads.length}</strong></span>
        <span>•</span>
        <span>В архиве: <strong class="text-white font-bold">{safeDownloadHistory.length}</strong></span>
      </div>

      <div class="flex items-center gap-3">
        {#if downloadingCount > 0}
          <button
            data-nav-item
            class="px-4 py-2 rounded-xl bg-white/10 hover:bg-white/20 active:bg-white/25 focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold flex items-center gap-2 cursor-pointer transition-colors"
            onclick={() => {
              sound.playSelect();
              onPauseAll();
            }}
          >
            <Pause class="w-3.5 h-3.5" />
            <span>Пауза всех</span>
          </button>
        {/if}

        {#if pausedCount > 0}
          <button
            data-nav-item
            class="px-4 py-2 rounded-xl bg-[#1a9fff] hover:bg-[#28a8ff] active:bg-[#1388dc] focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-mono font-bold flex items-center gap-2 cursor-pointer transition-all shadow-md"
            onclick={() => {
              sound.playSelect();
              onResumeAll();
            }}
          >
            <Play class="w-3.5 h-3.5 fill-current" />
            <span>Возобновить все</span>
          </button>
        {/if}

        {#if safeDownloadHistory.length > 0}
          <button
            data-nav-item
            class="px-4 py-2 rounded-xl bg-white/5 hover:bg-rose-500/20 active:bg-rose-500/30 focus:ring-2 focus:ring-rose-400 focus:outline-none text-[#8e95a2] hover:text-rose-300 text-xs font-mono font-bold flex items-center gap-2 cursor-pointer transition-colors"
            onclick={() => {
              sound.playBack();
              onClearCompleted();
            }}
          >
            <Trash2 class="w-3.5 h-3.5" />
            <span>Очистить историю</span>
          </button>
        {/if}
      </div>
    </div>

    <!-- ============================================================ -->
    <!-- 2. SECTION "ДАЛЕЕ (X)" (QUEUE)                              -->
    <!-- ============================================================ -->
    <section class="space-y-4">
      <div class="flex items-center gap-3">
        <span class="text-xs font-mono uppercase font-bold text-[#8f98a0] tracking-wider whitespace-nowrap">
          ДАЛЕЕ ({queuedDownloads.length})
        </span>
        <div class="flex-1 h-px bg-white/[0.06]"></div>
      </div>

      {#if queuedDownloads.length === 0}
        <p class="text-xs text-[#64748b] py-2 font-mono">В очереди нет ожидающих загрузок</p>
      {:else}
        <div class="space-y-2.5">
          {#each queuedDownloads as qItem, idx (qItem.downloadId)}
            {@const qCover = getGameCover(qItem)}
            <div class="w-full p-3.5 sm:p-4 rounded-2xl bg-[#0c1017] hover:bg-[#111722] border border-white/[0.04] hover:border-white/[0.08] transition-colors flex items-center justify-between gap-4">
              <div class="flex items-center gap-3.5 min-w-0 flex-1">
                <span class="text-xs font-mono text-[#616875] w-6 text-center font-bold">#{idx + 1}</span>
                <div class="w-24 h-12 rounded-xl bg-black/50 border border-white/[0.06] overflow-hidden flex-shrink-0 relative">
                  {#if qCover && !imageLoadFailed[qCover]}
                    <img
                      src={qCover}
                      alt=""
                      class="w-full h-full object-cover"
                      onerror={() => {
                        imageLoadFailed[qCover] = true;
                        resolveMissingCover(qItem.gameTitle, qItem.gameId);
                      }}
                    />
                  {:else}
                    <div class="w-full h-full flex items-center justify-center text-[10px] font-bold text-[#64748b] uppercase font-mono">
                      Ducke
                    </div>
                  {/if}
                </div>
                <div class="min-w-0">
                  <h4 class="text-sm font-bold text-white uppercase tracking-wide truncate">{qItem.gameTitle}</h4>
                  <span class="text-[11px] font-mono text-[#8f98a0] uppercase tracking-wider block mt-0.5">
                    РАЗМЕР: {formatBytesRu(qItem.totalBytes)} · В ОЧЕРЕДИ
                  </span>
                </div>
              </div>

              <div class="flex items-center gap-2.5 flex-shrink-0">
                <button
                  data-nav-item
                  class="h-9 px-3.5 rounded-xl bg-[#171d27] hover:bg-[#1a9fff] hover:text-white text-[#8f98a0] focus:ring-2 focus:ring-white focus:outline-none border border-white/[0.06] flex items-center gap-1.5 text-xs font-mono font-bold cursor-pointer transition-colors"
                  title="Начать сейчас"
                  onclick={() => {
                    sound.playSelect();
                    onResume(qItem.downloadId);
                  }}
                >
                  <Play class="w-3.5 h-3.5 fill-current ml-0.5" />
                  <span>Старт</span>
                </button>
                <button
                  data-nav-item
                  class="w-9 h-9 rounded-xl bg-[#171d27] hover:bg-rose-600/30 text-[#8f98a0] hover:text-rose-400 focus:ring-2 focus:ring-rose-400 focus:outline-none border border-white/[0.06] flex items-center justify-center cursor-pointer transition-colors"
                  title="Убрать из очереди"
                  onclick={() => {
                    sound.playBack();
                    onCancel(qItem.downloadId);
                  }}
                >
                  <X class="w-4 h-4" />
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </section>

    <!-- ============================================================ -->
    <!-- 3. SECTION "ЗАВЕРШЕНО (X)" (COMPLETED HISTORY)              -->
    <!-- ============================================================ -->
    <section class="space-y-4">
      <div class="flex items-center gap-3">
        <span class="text-xs font-mono uppercase font-bold text-[#8f98a0] tracking-wider whitespace-nowrap">
          ЗАВЕРШЕНО ({safeDownloadHistory.length})
        </span>
        <div class="flex-1 h-px bg-white/[0.06]"></div>
      </div>

      {#if safeDownloadHistory.length === 0}
        <p class="text-xs text-[#64748b] py-2 font-mono">Нет завершенных загрузок</p>
      {:else}
        <div class="space-y-2.5">
          {#each safeDownloadHistory as record (record.id)}
            {@const rCover = getRecordCover(record)}
            <div class="w-full p-3.5 sm:p-4 rounded-2xl bg-[#0c1017] hover:bg-[#111722] border border-white/[0.04] hover:border-white/[0.08] transition-colors flex items-center justify-between gap-4">
              
              <div class="flex items-center gap-3.5 min-w-0 flex-1">
                <div class="w-24 h-12 rounded-xl bg-black/50 border border-white/[0.06] overflow-hidden flex-shrink-0 relative">
                  {#if rCover && !imageLoadFailed[rCover]}
                    <img
                      src={rCover}
                      alt=""
                      class="w-full h-full object-cover"
                      onerror={() => {
                        imageLoadFailed[rCover] = true;
                        resolveMissingCover(record.gameTitle, record.gameId);
                      }}
                    />
                  {:else}
                    <div class="w-full h-full bg-[#141922] flex items-center justify-center text-[10px] font-bold text-[#64748b] uppercase font-mono">
                      Ducke
                    </div>
                  {/if}
                </div>

                <div class="min-w-0">
                  <h4 class="text-sm font-bold text-white uppercase tracking-wide truncate">{record.gameTitle}</h4>
                  <span class="text-[11px] font-mono text-[#8f98a0] uppercase tracking-wider block mt-0.5">
                    ЗАГРУЖЕНО: {formatBytesRu(record.downloadedBytes || record.totalBytes)} из {formatBytesRu(record.totalBytes || record.downloadedBytes)}
                  </span>
                </div>
              </div>

              <div class="text-[11px] font-mono text-[#8f98a0] uppercase tracking-wider hidden sm:block whitespace-nowrap">
                {formatSteamDate(record.updatedAt)}
              </div>

              <!-- Actions (STRICTLY NO PLAY BUTTON) -->
              <div class="flex items-center gap-2.5 flex-shrink-0">
                {#if record.status === 'completed'}
                  {#if isLinux && installerMap[record.id]}
                    <button
                      data-nav-item
                      class="bg-emerald-500 hover:bg-emerald-400 text-black font-black text-xs px-3.5 py-2 rounded-xl flex items-center gap-1.5 shadow-md cursor-pointer transition-colors font-mono focus:ring-2 focus:ring-white focus:outline-none active:scale-95"
                      onclick={() => handleLaunchInstaller(installerMap[record.id], record.id)}
                      title="Запустить установку игры (.run)"
                    >
                      <Play class="w-3.5 h-3.5 fill-black stroke-[2]" />
                      <span>{installingId === record.id ? 'ЗАПУСК...' : 'УСТАНОВИТЬ'}</span>
                    </button>
                  {/if}

                  {#if record.localPath}
                    <button
                      data-nav-item
                      class="bg-[#171d27] hover:bg-[#202937] focus:ring-2 focus:ring-white focus:outline-none text-[#c6d4df] hover:text-white text-xs px-3.5 py-2 rounded-xl border border-white/[0.06] flex items-center gap-1.5 transition-colors cursor-pointer font-mono"
                      onclick={() => {
                        sound.playSelect();
                        onOpenFolder(record.localPath);
                      }}
                      title="Открыть папку с файлами"
                    >
                      <FolderOpen class="w-3.5 h-3.5 text-[#8f98a0]" />
                      <span>Папка</span>
                    </button>
                  {/if}
                {:else}
                  <button
                    data-nav-item
                    class="bg-[#1a9fff] hover:bg-[#2cb2ff] focus:ring-2 focus:ring-white focus:outline-none text-white text-xs font-bold px-3.5 py-2 rounded-xl flex items-center gap-1.5 shadow-sm cursor-pointer transition-colors font-mono"
                    onclick={() => {
                      sound.playSelect();
                      onResume(record.id);
                    }}
                  >
                    <RefreshCw class="w-3.5 h-3.5" />
                    <span>ДОКАЧАТЬ</span>
                  </button>
                {/if}

                <button
                  data-nav-item
                  class="p-2 text-[#64748b] hover:text-rose-400 focus:ring-2 focus:ring-rose-400 focus:outline-none transition-all cursor-pointer rounded-xl hover:bg-white/[0.05]"
                  onclick={() => {
                    sound.playBack();
                    onDeleteRecord(record.id, false);
                  }}
                  title="Удалить из истории"
                >
                  <Trash2 class="w-4 h-4" />
                </button>
              </div>

            </div>
          {/each}
        </div>
      {/if}
    </section>

  </div>
</div>
