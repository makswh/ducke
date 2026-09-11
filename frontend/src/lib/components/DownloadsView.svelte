<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Pause,
    Play,
    X,
    FolderOpen,
    Trash2,
    HardDrive,
    BarChart2,
    TrendingUp,
    Settings,
    Download,
    RefreshCw,
    Sliders,
    ChevronDown,
    Check,
    Menu,
    Layers
  } from 'lucide-svelte';

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
    isTorrent?: boolean;
    magnetUri?: string;
    torrentSeeds?: number;
    torrentPeers?: number;
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
    isTorrent?: boolean;
    magnetUri?: string;
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
    settings = null as any,
    onPause = (id: string) => {},
    onResume = (id: string) => {},
    onCancel = (id: string) => {},
    onPauseAll = () => {},
    onResumeAll = () => {},
    onClearCompleted = () => {},
    onDeleteRecord = (id: string, removeFiles: boolean) => {},
    onOpenFolder = (path: string) => {},
    onGoToCatalog = () => {},
    onGoToSettings = () => {},
    onUpdateSpeedLimit = (kbps: number) => {}
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
  let isSpeedMenuOpen = $state<boolean>(false);
  let isDeckMenuOpen = $state<boolean>(false);

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

  // Speed limit presets in KB/s (0 = unlimited)
  const speedPresets = [
    { label: 'Без ограничений', value: 0 },
    { label: '10 МБ/с', value: 10240 },
    { label: '25 МБ/с', value: 25600 },
    { label: '50 МБ/с', value: 51200 },
    { label: '100 МБ/с', value: 102400 }
  ];

  function handleSelectSpeedLimit(kbps: number) {
    isSpeedMenuOpen = false;
    onUpdateSpeedLimit(kbps);
  }

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

  // Steam-format speed (e.g. 74,2 Кбайт/с, 6,2 Мбайт/с)
  function formatSpeedRu(bytesPerSec: number): string {
    if (bytesPerSec <= 0) return '0 байт/с';
    const mb = bytesPerSec / (1024 * 1024);
    if (mb >= 1.0) {
      return `${mb.toFixed(1).replace('.', ',')} Мбайт/с`;
    }
    const kb = bytesPerSec / 1024;
    return `${kb.toFixed(1).replace('.', ',')} Кбайт/с`;
  }

  // Steam-format size (e.g. 4,5 МБ, 534,3 МБ, 2,3 ГБ)
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

  // Steam-format date (e.g. СЕГОДНЯ 10:00, ВЧЕРА 15:30, 4 СЕНТ., 14:17)
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

  // Steam ETA format (e.g. 01:08 or 45:12)
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

  function getSparklinePoints(width = 500, height = 54): string {
    const points = speedHistory;
    const maxVal = Math.max(peakSpeedBytes, 1024 * 1024);
    const step = width / (points.length - 1);

    return points
      .map((val, idx) => {
        const x = (idx * step).toFixed(1);
        const y = (height - (val / maxVal) * (height - 8) - 4).toFixed(1);
        return `${idx === 0 ? 'M' : 'L'} ${x} ${y}`;
      })
      .join(' ');
  }

  function getSparklineArea(width = 500, height = 54): string {
    const line = getSparklinePoints(width, height);
    if (!line) return '';
    return `${line} L ${width} ${height} L 0 ${height} Z`;
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
    {@const speedLimitKB = settings?.maxSpeedKBps || 0}
    {@const logoUrl = getGameLogo(currentDownload)}
    {@const bannerUrl = getGameBanner(currentDownload)}

    <div class="relative z-10 w-full rounded-none bg-[#090d14] border-b border-[#1b222d] border-t-0 border-x-0 overflow-hidden shadow-xl flex flex-col md:flex-row items-stretch min-h-[160px] sm:min-h-[170px]">
      
      <!-- 1. LEFT GAME BANNER WITH HALF-LIFE STYLE LOGO / TITLE OVERLAY -->
      <div class="relative w-full md:w-[380px] lg:w-[440px] shrink-0 h-[160px] sm:h-[170px] overflow-hidden bg-[#0d121a]">
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

        <!-- Game Logo or Title overlaid at bottom left (matching screenshot) -->
        <div class="absolute bottom-3 left-4 right-4 flex items-end justify-between gap-2 z-20">
          <div class="flex items-end gap-2 max-w-[85%]">
            {#if logoUrl && !imageLoadFailed[logoUrl]}
              <img
                src={logoUrl}
                alt={currentDownload.gameTitle}
                class="max-h-10 sm:max-h-12 object-contain object-left filter drop-shadow-[0_2px_8px_rgba(0,0,0,0.95)]"
                onerror={() => {
                  imageLoadFailed[logoUrl] = true;
                  resolveMissingLogo(currentDownload.gameTitle, currentDownload.gameId);
                }}
              />
            {:else}
              <span class="text-base sm:text-lg font-black text-white uppercase tracking-wider filter drop-shadow-[0_2px_6px_rgba(0,0,0,0.95)] line-clamp-2">
                {currentDownload.gameTitle}
              </span>
            {/if}
          </div>
          {#if currentDownload.isTorrent}
            <span class="px-2 py-0.5 rounded text-[10px] font-mono font-bold uppercase tracking-wider bg-white/10 text-white border border-white/15 shrink-0">
              TORRENT
            </span>
          {/if}
        </div>
      </div>

      <!-- 2. LIVE GRAPH SPANNING ACROSS BANNER RIGHT & MIDDLE SECTION (Matching Screenshot) -->
      <div class="absolute bottom-0 left-[240px] sm:left-[280px] md:left-[320px] right-[380px] sm:right-[420px] md:right-[460px] h-[95px] z-10 pointer-events-none overflow-hidden hidden sm:flex items-end">
        <svg class="w-full h-[90px]" viewBox="0 0 800 90" preserveAspectRatio="none">
          {#each getHistBars(800, 90) as bar}
            <rect x={bar.x} y={bar.y} width={bar.w} height={bar.h} fill="#1a9fff" opacity="0.85" />
          {/each}
          <path d={getDiskLine(800, 90)} fill="none" stroke="#22c55e" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </div>

      <!-- 3. MIDDLE AREA (Context Menu ☰) -->
      <div class="hidden md:flex flex-col justify-start py-3 px-3 relative z-20 shrink-0">
        <div class="relative">
          <button
            type="button"
            class="w-7 h-7 flex items-center justify-center text-[#8f98a0] hover:text-white transition-colors cursor-pointer"
            onclick={() => (isDeckMenuOpen = !isDeckMenuOpen)}
            title="Опции загрузки"
          >
            <Menu class="w-4 h-4" />
          </button>

          {#if isDeckMenuOpen}
            <div class="absolute left-0 top-8 z-30 w-48 rounded bg-[#161c26] border border-white/10 shadow-2xl py-1 text-xs font-sans">
              {#if currentDownload.localPath}
                <button
                  class="w-full text-left px-3 py-2 flex items-center gap-2 text-xs text-[#c6d4df] hover:bg-white/[0.08] hover:text-white transition-colors cursor-pointer"
                  onclick={() => {
                    isDeckMenuOpen = false;
                    onOpenFolder(currentDownload.localPath);
                  }}
                >
                  <FolderOpen class="w-3.5 h-3.5 text-[#8f98a0]" />
                  <span>Открыть папку</span>
                </button>
              {/if}
              <button
                class="w-full text-left px-3 py-2 flex items-center gap-2 text-xs text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
                onclick={() => {
                  isDeckMenuOpen = false;
                  onCancel(currentDownload.downloadId);
                }}
              >
                <X class="w-3.5 h-3.5" />
                <span>Отменить загрузку</span>
              </button>
            </div>
          {/if}
        </div>
      </div>

      <!-- 4. RIGHT TELEMETRY & PROGRESS PANEL (Matching Screenshot) -->
      <div class="flex-1 min-w-0 md:max-w-[460px] ml-auto flex flex-col justify-between px-4 sm:px-6 py-3 bg-[#090d14] relative z-20 border-t md:border-t-0 md:border-l border-[#1b222d]">
        
        <!-- Top Metrics Row & Settings Button -->
        <div class="flex items-start justify-between gap-3 min-w-0">
          <div class="flex flex-col items-start min-w-0">
            <!-- Row 1: СЕТЬ / МАКС. / ИСП. ДИСКА -->
            <div class="flex items-center gap-4 text-xs font-mono">
              <!-- СЕТЬ -->
              <div class="flex flex-col">
                <div class="flex items-center gap-1 text-[9px] font-mono font-bold text-sky-400 tracking-wider uppercase">
                  <svg class="w-2.5 h-2.5 text-sky-400" viewBox="0 0 12 12" fill="currentColor">
                    <rect x="1" y="6" width="2" height="6" rx="0.5" />
                    <rect x="5" y="3" width="2" height="9" rx="0.5" />
                    <rect x="9" y="1" width="2" height="11" rx="0.5" />
                  </svg>
                  <span>СЕТЬ</span>
                </div>
                <span class="text-xs font-mono font-bold text-white tracking-tight">
                  {formatSpeedRu(totalSpeedBytes)}
                </span>
              </div>

              <!-- МАКС. -->
              <div class="flex flex-col">
                <div class="flex items-center gap-1 text-[9px] font-mono font-bold text-sky-400 tracking-wider uppercase">
                  <svg class="w-2.5 h-2.5 text-sky-400" viewBox="0 0 12 12" fill="currentColor">
                    <rect x="1" y="6" width="2" height="6" rx="0.5" />
                    <rect x="5" y="3" width="2" height="9" rx="0.5" />
                    <rect x="9" y="1" width="2" height="11" rx="0.5" />
                  </svg>
                  <span>МАКС.</span>
                </div>
                <span class="text-xs font-mono font-bold text-white tracking-tight">
                  {formatSpeedRu(peakSpeedBytes)}
                </span>
              </div>

              <!-- ИСП. ДИСКА -->
              <div class="flex flex-col">
                <div class="flex items-center gap-1 text-[9px] font-mono font-bold text-emerald-400 tracking-wider uppercase">
                  <span class="w-2.5 h-0.5 bg-emerald-400 inline-block"></span>
                  <span>ИСП. ДИСКА</span>
                </div>
                <span class="text-xs font-mono font-bold text-white tracking-tight">
                  {formatSpeedRu(isDownloading ? Math.round(totalSpeedBytes * 0.96) : 0)}
                </span>
              </div>
            </div>

            <!-- Row 2: ОГРАНИЧЕНИЕ ЗАГРУЗОК -->
            <div class="text-[10px] font-mono uppercase tracking-wider text-[#8f98a0] mt-1 flex items-center gap-1 relative">
              <span>ОГРАНИЧЕНИЕ ЗАГРУЗОК:</span>
              <button
                type="button"
                class="text-[#c6d4df] hover:text-white cursor-pointer uppercase transition-colors"
                onclick={() => (isSpeedMenuOpen = !isSpeedMenuOpen)}
              >
                {speedLimitKB > 0 ? formatSpeedRu(speedLimitKB * 1024) : 'БЕЗ ОГРАНИЧЕНИЙ'}
              </button>

              {#if isSpeedMenuOpen}
                <div class="absolute left-0 top-5 z-30 w-48 rounded bg-[#161c26] border border-white/10 shadow-2xl py-1 text-xs font-sans normal-case">
                  <div class="px-3 py-1 text-[10px] font-mono text-[#8f98a0] uppercase tracking-wider border-b border-white/5">
                    Лимит скорости
                  </div>
                  {#each speedPresets as preset}
                    <button
                      class="w-full text-left px-3 py-1.5 flex items-center justify-between text-xs hover:bg-white/[0.08] transition-colors cursor-pointer {speedLimitKB === preset.value ? 'text-sky-400 font-bold' : 'text-[#c6d4df]'}"
                      onclick={() => handleSelectSpeedLimit(preset.value)}
                    >
                      <span>{preset.label}</span>
                      {#if speedLimitKB === preset.value}
                        <Check class="w-3.5 h-3.5 text-sky-400" />
                      {/if}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          </div>

          <!-- Gear Settings Button -->
          <button
            type="button"
            class="w-7 h-7 sm:w-8 sm:h-8 rounded-sm bg-[#171d27] hover:bg-[#202937] text-[#8f98a0] hover:text-white border border-[#2b3648] flex items-center justify-center transition-colors cursor-pointer shrink-0"
            title="Настройки Ducke"
            onclick={onGoToSettings}
          >
            <Settings class="w-3.5 h-3.5" />
          </button>
        </div>

        <!-- Two Horizontal Progress Bars: Загрузка данных & Установка файлов -->
        <div class="space-y-2 my-1">
          <!-- 1: Загрузка данных (Blue) -->
          <div class="space-y-1">
            <div class="flex items-center justify-between text-xs font-mono">
              <span class="text-[#c6d4df] font-medium">Загрузка данных</span>
              <div class="flex items-center gap-1 text-xs font-mono text-white font-bold">
                <span>{formatBytesRu(currentDownload.downloadedBytes)} / {formatBytesRu(currentDownload.totalBytes)}</span>
                <Download class="w-3 h-3 text-[#8f98a0]" />
              </div>
            </div>
            <div class="h-1 w-full bg-[#1a2330] rounded-none overflow-hidden">
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
            <div class="h-1 w-full bg-[#1a2330] rounded-none overflow-hidden">
              <div
                class="h-full bg-[#22c55e] transition-all duration-300"
                style="width: {percent}%"
              ></div>
            </div>
          </div>
        </div>

        <!-- Bottom Line: [Осталось примерно 05:09 / Приостановлено] ---------- [Blue Play/Pause Button] -->
        <div class="flex items-center justify-between gap-3 pt-1 min-w-0">
          <span class="text-xs font-mono text-[#8f98a0] truncate">
            {#if isPaused}
              Приостановлено
            {:else if isDownloading}
              {#if currentDownload.isTorrent}
                {#if currentDownload.etaSeconds > 0}
                  Осталось {formatSteamETA(currentDownload.etaSeconds)} • Пиры: {currentDownload.torrentPeers || 0} (сиды: {currentDownload.torrentSeeds || 0})
                {:else}
                  Загрузка данных... • Пиры: {currentDownload.torrentPeers || 0} (сиды: {currentDownload.torrentSeeds || 0})
                {/if}
              {:else if currentDownload.etaSeconds > 0}
                Осталось примерно {formatSteamETA(currentDownload.etaSeconds)}
              {:else}
                Загрузка данных...
              {/if}
            {:else if isScanning}
              {#if currentDownload.isTorrent}
                Поиск пиров / Получение метаданных...
              {:else}
                Проверка файлов...
              {/if}
            {:else if isFailed}
              <span class="text-rose-400">Ошибка: {currentDownload.errorMessage || 'Сбой'}</span>
            {:else}
              В очереди
            {/if}
          </span>

          {#if isDownloading || isScanning}
            <button
              type="button"
              class="w-9 h-9 sm:w-10 sm:h-10 bg-[#1a9fff] hover:bg-[#28a8ff] active:bg-[#1388dc] text-white flex items-center justify-center rounded-sm transition-colors cursor-pointer shrink-0 shadow-md"
              onclick={() => onPause(currentDownload.downloadId)}
              title="Приостановить"
            >
              <Pause class="w-4 h-4 fill-white text-white" />
            </button>
          {:else}
            <button
              type="button"
              class="w-9 h-9 sm:w-10 sm:h-10 bg-[#1a9fff] hover:bg-[#28a8ff] active:bg-[#1388dc] text-white flex items-center justify-center rounded-sm transition-colors cursor-pointer shrink-0 shadow-md"
              onclick={() => onResume(currentDownload.downloadId)}
              title="Возобновить"
            >
              <Play class="w-4 h-4 fill-white text-white ml-0.5" />
            </button>
          {/if}
        </div>

      </div>

    </div>
  {:else}
    <!-- Clean Empty State when no active download -->
    <div class="relative z-10 w-full rounded-none bg-[#0c1017] border-b border-[#1e2633] border-t-0 border-x-0 px-6 sm:px-8 lg:px-10 py-7 flex flex-col sm:flex-row items-center justify-between gap-6">
      <div class="space-y-1 text-center sm:text-left">
        <h2 class="text-base font-bold text-white uppercase tracking-wider font-mono">Все загрузки завершены</h2>
        <p class="text-xs text-[#8f98a0]">
          В очереди нет активных процессов.
          {#if diskSpace}
            Доступно на накопителе: <span class="text-white font-mono font-bold">{diskSpace.freeGB}</span>.
          {/if}
        </p>
      </div>
      <button
        data-nav-item
        class="px-4 py-2 rounded bg-[#171d27] hover:bg-[#202937] text-[#c6d4df] hover:text-white text-xs font-mono uppercase border border-[#2b3648] cursor-pointer transition-colors"
        onclick={onGoToCatalog}
      >
        Каталог игр →
      </button>
    </div>
  {/if}

  <!-- Lower Container for Queue and Completed (with standard padding) -->
  <div class="relative z-10 w-full flex-1 flex flex-col px-6 sm:px-8 lg:px-10 py-7 space-y-7 pb-32">

    <!-- ============================================================ -->
    <!-- 2. SECTION "ДАЛЕЕ (X)" (QUEUE)                              -->
    <!-- ============================================================ -->
    <section class="space-y-3">
      <div class="flex items-center gap-3">
        <span class="text-xs font-mono uppercase font-bold text-[#8f98a0] tracking-wider whitespace-nowrap">
          ДАЛЕЕ ({queuedDownloads.length})
        </span>
        <div class="flex-1 h-px bg-white/[0.06]"></div>

        {#if safeActiveDownloads.length > 1}
          <div class="flex items-center gap-2">
            {#if downloadingCount > 0}
              <button
                data-nav-item
                class="text-[11px] font-mono uppercase text-[#8f98a0] hover:text-white bg-[#141922] hover:bg-[#1c2330] px-3 py-1 rounded border border-white/[0.06] transition-colors cursor-pointer"
                onclick={onPauseAll}
              >
                Приостановить всё
              </button>
            {/if}
            {#if pausedCount > 0}
              <button
                data-nav-item
                class="text-[11px] font-mono uppercase text-[#8f98a0] hover:text-white bg-[#141922] hover:bg-[#1c2330] px-3 py-1 rounded border border-white/[0.06] transition-colors cursor-pointer"
                onclick={onResumeAll}
              >
                Возобновить всё
              </button>
            {/if}
          </div>
        {/if}
      </div>

      {#if queuedDownloads.length === 0}
        <p class="text-xs text-[#64748b] py-2 font-mono">В очереди нет ожидающих загрузок</p>
      {:else}
        <div class="space-y-1.5">
          {#each queuedDownloads as qItem (qItem.downloadId)}
            {@const qCover = getGameCover(qItem)}
            <div class="w-full p-2.5 sm:p-3 rounded bg-[#0c1017] hover:bg-[#111722] border border-white/[0.04] hover:border-white/[0.08] transition-colors flex items-center justify-between gap-4 group">
              <div class="flex items-center gap-3 min-w-0 flex-1">
                <div class="w-20 h-10 rounded bg-black/50 border border-white/[0.06] overflow-hidden flex-shrink-0 relative">
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
                    <div class="w-full h-full flex items-center justify-center text-[9px] font-bold text-[#64748b] uppercase font-mono">
                      Ducke
                    </div>
                  {/if}
                </div>
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <h4 class="text-sm font-bold text-white uppercase tracking-wide truncate">{qItem.gameTitle}</h4>
                    {#if qItem.isTorrent}
                      <span class="px-1.5 py-0.2 rounded text-[9px] font-mono font-bold bg-white/10 text-white border border-white/15 shrink-0">
                        TORRENT
                      </span>
                    {/if}
                  </div>
                  <span class="text-[10px] font-mono text-[#8f98a0] uppercase tracking-wider block mt-0.5">
                    РАЗМЕР: {formatBytesRu(qItem.totalBytes)} · В ОЧЕРЕДИ
                  </span>
                </div>
              </div>

              <div class="flex items-center gap-2 flex-shrink-0">
                <button
                  data-nav-item
                  class="h-7 px-2.5 rounded bg-[#171d27] hover:bg-[#1a9fff] hover:text-white text-[#8f98a0] border border-white/[0.06] flex items-center gap-1 text-xs cursor-pointer transition-colors"
                  title="Начать сейчас"
                  onclick={() => onResume(qItem.downloadId)}
                >
                  <Play class="w-3 h-3 fill-current ml-0.5" />
                  <span class="hidden sm:inline font-mono text-[11px]">Старт</span>
                </button>
                <button
                  data-nav-item
                  class="w-7 h-7 rounded bg-[#171d27] hover:bg-rose-600/30 text-[#8f98a0] hover:text-rose-400 border border-white/[0.06] flex items-center justify-center cursor-pointer transition-colors"
                  title="Убрать из очереди"
                  onclick={() => onCancel(qItem.downloadId)}
                >
                  <X class="w-3 h-3" />
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
    <section class="space-y-3">
      <div class="flex items-center gap-3">
        <span class="text-xs font-mono uppercase font-bold text-[#8f98a0] tracking-wider whitespace-nowrap">
          ЗАВЕРШЕНО ({safeDownloadHistory.length})
        </span>
        <div class="flex-1 h-px bg-white/[0.06]"></div>
        {#if safeDownloadHistory.length > 0}
          <button
            data-nav-item
            class="bg-[#141922] hover:bg-[#1c2330] text-[#8f98a0] hover:text-white text-[11px] font-mono uppercase px-3 py-1 rounded border border-white/[0.06] transition-colors cursor-pointer"
            onclick={onClearCompleted}
          >
            Очистить историю
          </button>
        {/if}
      </div>

      {#if safeDownloadHistory.length === 0}
        <p class="text-xs text-[#64748b] py-2 font-mono">Нет завершенных загрузок</p>
      {:else}
        <div class="space-y-1.5">
          {#each safeDownloadHistory as record (record.id)}
            {@const rCover = getRecordCover(record)}
            <div class="w-full p-2.5 sm:p-3 rounded bg-[#0c1017] hover:bg-[#111722] border border-white/[0.04] hover:border-white/[0.08] transition-colors flex items-center justify-between gap-4 group">
              
              <div class="flex items-center gap-3 min-w-0 flex-1">
                <div class="w-20 h-10 rounded bg-black/50 border border-white/[0.06] overflow-hidden flex-shrink-0 relative">
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
                    <div class="w-full h-full bg-[#141922] flex items-center justify-center text-[9px] font-bold text-[#64748b] uppercase font-mono">
                      Ducke
                    </div>
                  {/if}
                </div>

                <div class="min-w-0">
                  <h4 class="text-sm font-bold text-white uppercase tracking-wide truncate">{record.gameTitle}</h4>
                  <span class="text-[10px] font-mono text-[#8f98a0] uppercase tracking-wider block mt-0.5">
                    ЗАГРУЖЕНО: {formatBytesRu(record.downloadedBytes || record.totalBytes)} из {formatBytesRu(record.totalBytes || record.downloadedBytes)}
                  </span>
                </div>
              </div>

              <div class="text-[11px] font-mono text-[#8f98a0] uppercase tracking-wider hidden sm:block whitespace-nowrap">
                {formatSteamDate(record.updatedAt)}
              </div>

              <!-- Actions (STRICTLY NO PLAY BUTTON) -->
              <div class="flex items-center gap-2 flex-shrink-0">
                {#if record.status === 'completed'}
                  {#if isLinux && installerMap[record.id]}
                    <button
                      data-nav-item
                      class="bg-emerald-500 hover:bg-emerald-400 text-black font-bold text-xs px-3 py-1.5 rounded flex items-center gap-1.5 shadow-sm transition-colors cursor-pointer"
                      onclick={() => handleLaunchInstaller(installerMap[record.id], record.id)}
                      title="Запустить установку игры (.run)"
                    >
                      <Play class="w-3.5 h-3.5 fill-black stroke-[2]" />
                      <span class="font-mono text-[11px]">{installingId === record.id ? 'Запуск...' : 'Установить'}</span>
                    </button>
                  {/if}

                  {#if record.localPath}
                    <button
                      data-nav-item
                      class="bg-[#171d27] hover:bg-[#202937] text-[#c6d4df] hover:text-white text-xs px-3 py-1.5 rounded border border-white/[0.06] flex items-center gap-1.5 transition-colors cursor-pointer"
                      onclick={() => onOpenFolder(record.localPath)}
                      title="Открыть папку с файлами"
                    >
                      <FolderOpen class="w-3.5 h-3.5 text-[#8f98a0]" />
                      <span class="font-mono text-[11px]">Папка</span>
                    </button>
                  {/if}
                {:else}
                  <button
                    data-nav-item
                    class="bg-[#1a9fff] hover:bg-[#2cb2ff] text-white text-xs font-bold px-3 py-1.5 rounded flex items-center gap-1 shadow-sm cursor-pointer transition-colors"
                    onclick={() => onResume(record.id)}
                  >
                    <RefreshCw class="w-3 h-3" />
                    <span class="font-mono text-[11px]">ДОКАЧАТЬ</span>
                  </button>
                {/if}

                <button
                  data-nav-item
                  class="p-1.5 text-[#64748b] hover:text-rose-400 opacity-0 group-hover:opacity-100 transition-all cursor-pointer rounded hover:bg-white/[0.05]"
                  onclick={() => onDeleteRecord(record.id, false)}
                  title="Удалить из истории"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>

            </div>
          {/each}
        </div>
      {/if}
    </section>

  </div>
</div>

