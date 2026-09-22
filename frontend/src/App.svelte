<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import TitleBar from './lib/components/TitleBar.svelte';
  import NavRail from './lib/components/NavRail.svelte';
  import BigPictureShell from './lib/components/bigpicture/BigPictureShell.svelte';
  import MasterDetailCatalog from './lib/components/MasterDetailCatalog.svelte';
  import DownloadsView from './lib/components/DownloadsView.svelte';
  import SettingsView from './lib/components/SettingsView.svelte';
  import FavoritesView from './lib/components/FavoritesView.svelte';
  import CollectionsView from './lib/components/CollectionsView.svelte';
  import GamepadHUD from './lib/components/GamepadHUD.svelte';
  import { gamepad } from './lib/navigation/gamepad';

  import {
    GetCatalog,
    GetGameDetails,
    UpdateSteamAppID,
    StartDownload,
    PauseDownload,
    ResumeDownload,
    CancelDownload,
    GetDownloads,
    GetDownloadHistory,
    GetSettings,
    SaveSettings,
    ImportFileZillaXML,
    TestConnection,
    SelectDirectory,
    OpenLocalFolder,
    TriggerSteamOSKeyboard,
    GetTorrentCatalog,
    GetTorrentCatalogChunk,
    GetTorrentCatalogCount,
    GetTorrentSources,
    AddTorrentSource,
    RemoveTorrentSource,
    ToggleTorrentSource,
    SyncTorrentSources,
    StartTorrentDownload,
    GetFavorites,
    LogMessage
  } from '../wailsjs/go/main/App';

  import { EventsOn, EventsOff, Quit, WindowFullscreen, WindowUnfullscreen } from '../wailsjs/runtime/runtime';

  // Application State (Svelte 5 Runes)
  let displayMode = $state<'desktop' | 'bigpicture'>('desktop');
  let activeTab = $state<'home' | 'catalog' | 'torrents' | 'collections' | 'favorites' | 'downloads' | 'settings'>('torrents');

  function switchToBigPicture() {
    displayMode = 'bigpicture';
    activeTab = 'home';
    try {
      WindowFullscreen();
    } catch (e) {
      console.warn('Failed to enter fullscreen:', e);
    }
  }

  function switchToDesktop() {
    displayMode = 'desktop';
    if (activeTab === 'home') {
      activeTab = hasFtpServers ? 'catalog' : (hasTorrentSources ? 'torrents' : 'settings');
    }
    try {
      WindowUnfullscreen();
    } catch (e) {
      console.warn('Failed to exit fullscreen:', e);
    }
  }
  let searchQuery = $state<string>('');
  let games = $state.raw<any[]>([]);
  let torrentGames = $state.raw<any[]>([]);
  let activeDownloads = $state<any[]>([]);
  let downloadHistory = $state<any[]>([]);
  let settings = $state<any | null>(null);
  let isRefreshing = $state<boolean>(false);
  let isCatalogLoading = $state<boolean>(true);
  let isTorrentsLoading = $state<boolean>(true);
  let catalogStatusText = $state<string>('');
  let isGamepadConnected = $state<boolean>(false);
  let toastMessage = $state<string>('');
  let metadataProgress = $state<{
    isSyncing: boolean;
    current: number;
    total: number;
    currentGame?: string;
  }>({
    isSyncing: false,
    current: 0,
    total: 0,
    currentGame: ''
  });

  // Batch buffer for real-time metadata/review/icon updates to prevent constant 10k array recreation
  const pendingEnrichments = new Map<number | string, any>();
  const pendingReviews = new Map<number | string, any>();
  const pendingIcons = new Map<number | string, string>();
  let batchFlushTimer: any = null;

  function flushBatchedGameUpdates() {
    batchFlushTimer = null;
    if (isTorrentsLoading) {
      // Defer flush until initial torrent catalog load finishes
      scheduleBatchFlush();
      return;
    }
    if (pendingEnrichments.size === 0 && pendingReviews.size === 0 && pendingIcons.size === 0) return;

    const enrichments = new Map(pendingEnrichments);
    const reviews = new Map(pendingReviews);
    const icons = new Map(pendingIcons);
    pendingEnrichments.clear();
    pendingReviews.clear();
    pendingIcons.clear();

    const applyUpdates = (list: any[]) => {
      if (!list || list.length === 0) return list;

      let hasTarget = false;
      for (let i = 0; i < list.length; i++) {
        const item = list[i];
        if (!item) continue;
        if (enrichments.has(item.id) || reviews.has(item.id) || icons.has(item.id)) {
          hasTarget = true;
          break;
        }
        if (item.steamAppId) {
          const sKey = `steam:${item.steamAppId}`;
          if (enrichments.has(sKey) || reviews.has(sKey) || icons.has(sKey)) {
            hasTarget = true;
            break;
          }
        }
      }
      if (!hasTarget) return list;

      let modified = false;
      const next = list.map((g) => {
        if (!g) return g;
        let itemModified = false;
        let updated = g;

        // Check enrichments
        const enr = enrichments.get(g.id) || (g.steamAppId ? enrichments.get(`steam:${g.steamAppId}`) : null);
        if (enr) {
          itemModified = true;
          updated = {
            ...updated,
            steamTitle: enr.steamTitle || updated.steamTitle,
            iconUrl: enr.iconUrl || updated.iconUrl,
            shortDescription: enr.shortDescription || updated.shortDescription,
            headerImage: enr.headerImage || updated.headerImage,
            capsuleImage: enr.capsuleImage || updated.capsuleImage,
            backgroundImage: enr.backgroundImage || updated.backgroundImage,
            genres: (enr.genres && enr.genres.length > 0) ? enr.genres : updated.genres,
            tags: (enr.tags && enr.tags.length > 0) ? enr.tags : updated.tags,
            developers: (enr.developers && enr.developers.length > 0) ? enr.developers : updated.developers,
            publishers: (enr.publishers && enr.publishers.length > 0) ? enr.publishers : updated.publishers,
            releaseDate: enr.releaseDate || updated.releaseDate,
            controllerSupport: enr.controllerSupport || updated.controllerSupport,
            metacriticScore: enr.metacriticScore || updated.metacriticScore,
            reviewScoreDesc: enr.reviewScoreDesc || updated.reviewScoreDesc,
            reviewPercent: enr.reviewPercent || updated.reviewPercent,
            totalReviews: enr.totalReviews || updated.totalReviews,
          };
        }

        // Check reviews
        const rev = reviews.get(g.id) || (g.steamAppId ? reviews.get(`steam:${g.steamAppId}`) : null);
        if (rev) {
          itemModified = true;
          updated = {
            ...updated,
            reviewScoreDesc: rev.reviewScoreDesc,
            reviewPercent: rev.reviewPercent,
            totalReviews: rev.totalReviews,
          };
        }

        // Check icons
        const ic = icons.get(g.id) || (g.steamAppId ? icons.get(`steam:${g.steamAppId}`) : null);
        if (ic && ic !== updated.iconUrl) {
          itemModified = true;
          updated = { ...updated, iconUrl: ic };
        }

        if (itemModified) {
          modified = true;
          return updated;
        }
        return g;
      });

      return modified ? next : list;
    };

    games = applyUpdates(games);
    torrentGames = applyUpdates(torrentGames);
  }

  function scheduleBatchFlush() {
    if (!batchFlushTimer) {
      batchFlushTimer = setTimeout(flushBatchedGameUpdates, 1000);
    }
  }

  let isInitialLoadComplete = $state(false);

  let hasFtpServers = $derived.by(() => {
    if (!settings) return false;
    const hasSaved = Array.isArray(settings.savedServers) && settings.savedServers.some((s: any) => s && s.host && s.host.trim() !== '');
    const hasActive = !!(settings.activeServer && settings.activeServer.host && settings.activeServer.host.trim() !== '');
    return hasSaved || hasActive;
  });

  let hasTorrentSources = $derived.by(() => {
    const fromSettings = Array.isArray(settings?.torrentSources) && settings.torrentSources.some((s: any) => s && s.enabled);
    return fromSettings || (Array.isArray(torrentGames) && torrentGames.length > 0);
  });

  let activeDownloadProgress = $derived.by(() => {
    const list = activeDownloads || [];
    const downloading = list.find((d) => d && (d.status === 'downloading' || d.status === 'scanning'));
    if (downloading) {
      return {
        isDownloading: true,
        percent: Math.round(downloading.progressPercent || 0),
        title: downloading.gameTitle || '',
        speed: downloading.speedDisplay || ''
      };
    }
    const queued = list.find((d) => d && d.status === 'queued');
    if (queued) {
      return {
        isDownloading: true,
        percent: Math.round(queued.progressPercent || 0),
        title: queued.gameTitle || '',
        speed: 'В очереди...'
      };
    }
    return { isDownloading: false, percent: 0, title: '', speed: '' };
  });

  $effect(() => {
    if (!isInitialLoadComplete || isTorrentsLoading || isCatalogLoading) return;
    if (!hasFtpServers && activeTab === 'catalog') {
      activeTab = hasTorrentSources ? 'torrents' : 'settings';
    }
    if (!hasTorrentSources && activeTab === 'torrents') {
      activeTab = hasFtpServers ? 'catalog' : 'settings';
    }
  });

  function showToast(msg: string) {
    toastMessage = msg;
    setTimeout(() => {
      if (toastMessage === msg) toastMessage = '';
    }, 3000);
  }

  function logApp(level: 'INFO' | 'WARN' | 'ERROR' | 'DEBUG', message: string) {
    const prefix = `[Frontend] ${message}`;
    if (level === 'ERROR') {
      console.error(prefix);
    } else if (level === 'WARN') {
      console.warn(prefix);
    } else {
      console.log(prefix);
    }
    try {
      if (typeof LogMessage === 'function') {
        LogMessage(level, 'Frontend', message);
      }
    } catch {}
  }

  function withTimeout<T>(promise: Promise<T>, timeoutMs: number, fallbackValue: T, operationName: string): Promise<T> {
    return new Promise<T>((resolve) => {
      let settled = false;
      const timer = setTimeout(() => {
        if (!settled) {
          settled = true;
          logApp('WARN', `${operationName} timed out after ${timeoutMs}ms, using fallback`);
          resolve(fallbackValue);
        }
      }, timeoutMs);

      promise
        .then((val) => {
          if (!settled) {
            settled = true;
            clearTimeout(timer);
            resolve(val);
          }
        })
        .catch((err) => {
          if (!settled) {
            settled = true;
            clearTimeout(timer);
            logApp('ERROR', `${operationName} failed: ${err?.message || err}`);
            resolve(fallbackValue);
          }
        });
    });
  }

  let catalogLoadPromise: Promise<void> | null = null;

  async function handleLoadTorrentCatalog(forceRefresh: boolean = false): Promise<void> {
    if (catalogLoadPromise && !forceRefresh) {
      return catalogLoadPromise;
    }

    const tStart = Date.now();
    logApp('INFO', `Starting handleLoadTorrentCatalog(forceRefresh=${forceRefresh})`);

    catalogLoadPromise = (async () => {
      isTorrentsLoading = true;
      try {
        let total = 0;
        try {
          if (typeof GetTorrentCatalogCount === 'function') {
            total = await withTimeout(GetTorrentCatalogCount(), 10000, 0, 'GetTorrentCatalogCount');
          }
        } catch (e: any) {
          logApp('WARN', `GetTorrentCatalogCount exception: ${e?.message || e}`);
        }

        logApp('INFO', `Torrent catalog total games count: ${total} (took ${Date.now() - tStart}ms)`);

        if (total > 0) {
          const CHUNK_SIZE = 2000;
          const chunkStart = Date.now();
          // Step 1: Fetch first chunk and render IMMEDIATELY (< 200ms)
          const firstChunk = await withTimeout(
            GetTorrentCatalogChunk(0, CHUNK_SIZE),
            10000,
            [],
            'GetTorrentCatalogChunk(0)'
          );

          if (Array.isArray(firstChunk) && firstChunk.length > 0) {
            torrentGames = firstChunk;
            // Instantly clear loader so user sees games right away
            isTorrentsLoading = false;
            logApp('INFO', `First chunk displayed immediately: ${firstChunk.length} games (took ${Date.now() - chunkStart}ms)`);
          } else {
            logApp('WARN', `First chunk was empty or invalid (total was ${total})`);
          }

          // Step 2: Stream remaining chunks gently in the background without blocking the UI
          if (total > CHUNK_SIZE) {
            const allGames = Array.isArray(firstChunk) ? [...firstChunk] : [];
            const remainingOffsets: number[] = [];
            for (let offset = CHUNK_SIZE; offset < total; offset += CHUNK_SIZE) {
              remainingOffsets.push(offset);
            }

            // Background async loader - does not block initial load completion
            (async () => {
              const bgStart = Date.now();
              const BATCH_SIZE = 2; // Stream 2 chunks at a time to prevent WebView2 IPC pipe congestion
              const seenIds = new Set<number>(allGames.map((g) => g.id));

              for (let i = 0; i < remainingOffsets.length; i += BATCH_SIZE) {
                const batchOffsets = remainingOffsets.slice(i, i + BATCH_SIZE);
                const chunks = await Promise.all(
                  batchOffsets.map((off) =>
                    withTimeout(GetTorrentCatalogChunk(off, CHUNK_SIZE), 10000, [], `GetTorrentCatalogChunk(${off})`)
                  )
                );

                let batchNewCount = 0;
                for (const ch of chunks) {
                  if (Array.isArray(ch) && ch.length > 0) {
                    for (const g of ch) {
                      if (g && !seenIds.has(g.id)) {
                        seenIds.add(g.id);
                        allGames.push(g);
                        batchNewCount++;
                      }
                    }
                  }
                }

                if (batchNewCount > 0) {
                  torrentGames = allGames;
                }

                // Yield to browser event loop so UI stays fluid at 60fps
                await new Promise((r) => setTimeout(r, 20));
              }

              logApp('INFO', `Background chunk streaming completed: total ${allGames.length} games (took ${Date.now() - bgStart}ms)`);
            })();
          }
        } else {
          // Fallback legacy RPC if count is 0 or uninitialized
          logApp('INFO', `Attempting fallback GetTorrentCatalog(forceRefresh=${forceRefresh})...`);
          const res = await withTimeout(GetTorrentCatalog(forceRefresh), 12000, [], 'GetTorrentCatalog');
          if (Array.isArray(res) && res.length > 0) {
            torrentGames = res;
            logApp('INFO', `Fallback GetTorrentCatalog returned ${res.length} games`);
          } else {
            logApp('WARN', 'Fallback GetTorrentCatalog returned 0 games');
          }
        }
      } catch (e: any) {
        logApp('ERROR', `Failed to load torrent catalog: ${e?.message || e}`);
      } finally {
        isTorrentsLoading = false;
        catalogLoadPromise = null;
        logApp('INFO', `handleLoadTorrentCatalog complete: isTorrentsLoading=false, torrentGames=${torrentGames?.length || 0} (total ${Date.now() - tStart}ms)`);
      }
    })();

    return catalogLoadPromise;
  }

  async function ensureWailsReady(timeoutMs = 4000): Promise<boolean> {
    const start = Date.now();
    while (Date.now() - start < timeoutMs) {
      if (
        typeof (window as any)?.go?.main?.App?.GetSettings === 'function' &&
        typeof (window as any)?.runtime?.EventsOn === 'function'
      ) {
        return true;
      }
      await new Promise((r) => setTimeout(r, 25));
    }
    return false;
  }

  async function loadInitialData() {
    isCatalogLoading = true;
    isTorrentsLoading = true;
    const startInit = Date.now();
    logApp('INFO', 'Starting loadInitialData');
    try {
      const wailsReady = await ensureWailsReady();
      logApp('INFO', `Wails bridge ready: ${wailsReady} (took ${Date.now() - startInit}ms)`);

      // Retry fetching settings with backoff to ensure IPC bridge is fully initialized
      let fetchedSettings: any = null;
      for (let attempt = 0; attempt < 5; attempt++) {
        try {
          fetchedSettings = await withTimeout(GetSettings(), 3000, null, `GetSettings(attempt ${attempt + 1})`);
          if (fetchedSettings) break;
        } catch (err: any) {
          logApp('WARN', `GetSettings attempt ${attempt + 1} failed: ${err?.message || err}`);
          await new Promise((r) => setTimeout(r, 75 * (attempt + 1)));
        }
      }

      // 1. Fetch downloads, history, and torrent sources immediately
      const [fetchedDownloads, fetchedHistory, fetchedSources] = await Promise.all([
        withTimeout(GetDownloads(), 5000, [], 'GetDownloads'),
        withTimeout(GetDownloadHistory(), 5000, [], 'GetDownloadHistory'),
        withTimeout(GetTorrentSources(), 5000, [], 'GetTorrentSources')
      ]);

      if (fetchedSettings) {
        if ((!fetchedSettings.torrentSources || fetchedSettings.torrentSources.length === 0) && Array.isArray(fetchedSources) && fetchedSources.length > 0) {
          fetchedSettings.torrentSources = fetchedSources;
        }
        settings = fetchedSettings;
      } else if (Array.isArray(fetchedSources) && fetchedSources.length > 0) {
        settings = { torrentSources: fetchedSources, savedServers: [] };
      }

      activeDownloads = Array.isArray(fetchedDownloads) ? fetchedDownloads : [];
      downloadHistory = Array.isArray(fetchedHistory) ? fetchedHistory : [];

      logApp('INFO', `Downloads count: ${activeDownloads.length}, History count: ${downloadHistory.length}, Torrent sources: ${settings?.torrentSources?.length || 0}`);

      if (settings && settings.steamDeckMode) {
        switchToBigPicture();
      }

      // Automatically select initial active tab based on configured sources
      const ftpAvailable = (settings?.savedServers || []).some((s: any) => s && s.host && s.host.trim() !== '') ||
        !!(settings?.activeServer && settings.activeServer.host && settings.activeServer.host.trim() !== '');
      const torrentAvailable = (settings?.torrentSources || []).some((s: any) => s && s.enabled) ||
        (Array.isArray(fetchedSources) && fetchedSources.some((s: any) => s && s.enabled));

      if (!ftpAvailable && torrentAvailable) {
        activeTab = 'torrents';
      } else if (!ftpAvailable && !torrentAvailable) {
        activeTab = 'settings';
      } else {
        activeTab = 'catalog';
      }
      logApp('INFO', `Initial tab determined: "${activeTab}" (ftpAvailable=${ftpAvailable}, torrentAvailable=${torrentAvailable})`);

      // 2. Fetch game catalog and torrent catalog concurrently and independently
      if (ftpAvailable) {
        withTimeout(GetCatalog(false), 12000, [], 'GetCatalog')
          .then((res) => {
            games = Array.isArray(res) ? res : [];
            logApp('INFO', `Catalog loaded: ${games.length} games`);
          })
          .catch((err) => {
            logApp('ERROR', `Failed to load catalog: ${err?.message || err}`);
          })
          .finally(() => {
            isCatalogLoading = false;
          });
      } else {
        isCatalogLoading = false;
        games = [];
      }

      // Load torrent catalog via streaming/chunked loader and await first chunk display
      await handleLoadTorrentCatalog(false);
    } catch (e: any) {
      logApp('ERROR', `loadInitialData uncaught exception: ${e?.message || e}`);
      isCatalogLoading = false;
      isTorrentsLoading = false;
    } finally {
      isInitialLoadComplete = true;
      logApp('INFO', `loadInitialData completed (total ${Date.now() - startInit}ms)`);
    }
  }

  async function handleRefreshCatalog() {
    if (isRefreshing) return;
    isRefreshing = true;
    try {
      if (activeTab === 'torrents') {
        await SyncTorrentSources();
        await handleLoadTorrentCatalog(true);
        showToast('Каталог торрентов обновлен');
      } else {
        isCatalogLoading = true;
        const res = await GetCatalog(true);
        games = Array.isArray(res) ? res : [];
        showToast('Каталог обновлен');
      }
    } catch (e: any) {
      showToast('Ошибка обновления: ' + (e?.message || e));
    } finally {
      isRefreshing = false;
      isCatalogLoading = false;
      isTorrentsLoading = false;
    }
  }

  async function handleStartDownload(gameId: number, targetPath: string) {
    try {
      const dlId = await StartDownload(gameId, targetPath);
      activeTab = 'downloads';
      showToast('Загрузка добавлена в очередь');
      const dls = await GetDownloads();
      activeDownloads = Array.isArray(dls) ? dls : [];
    } catch (e: any) {
      showToast('Не удалось начать загрузку: ' + (e?.message || e));
    }
  }

  async function handlePauseDownload(id: string) {
    try {
      await PauseDownload(id);
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleResumeDownload(id: string) {
    try {
      await ResumeDownload(id);
      showToast('Загрузка возобновлена');
      const dls = await GetDownloads();
      activeDownloads = Array.isArray(dls) ? dls : [];
      const hist = await GetDownloadHistory();
      downloadHistory = Array.isArray(hist) ? hist : [];
    } catch (e: any) {
      showToast('Ошибка возобновления: ' + (e?.message || e));
    }
  }

  async function handleCancelDownload(id: string) {
    try {
      await CancelDownload(id);
      activeDownloads = (activeDownloads || []).filter((d) => d && d.downloadId !== id);
      const hist = await GetDownloadHistory();
      downloadHistory = Array.isArray(hist) ? hist : [];
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleOpenFolder(folderPath: string) {
    try {
      await OpenLocalFolder(folderPath);
    } catch (e: any) {
      showToast('Не удалось открыть папку: ' + (e?.message || e));
    }
  }

  async function handlePauseAll() {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.PauseAllDownloads) {
        await app.PauseAllDownloads();
      }
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleResumeAll() {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.ResumeAllDownloads) {
        await app.ResumeAllDownloads();
        showToast('Все загрузки возобновлены');
        const dls = await GetDownloads();
        activeDownloads = Array.isArray(dls) ? dls : [];
      }
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleClearCompleted() {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.ClearCompletedDownloads) {
        await app.ClearCompletedDownloads();
        activeDownloads = (activeDownloads || []).filter((d) => d && d.status !== 'completed' && d.status !== 'cancelled');
        const hist = await GetDownloadHistory();
        downloadHistory = Array.isArray(hist) ? hist : [];
        showToast('Завершенные загрузки очищены');
      }
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleDeleteRecord(id: string, removeFiles: boolean = false) {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.DeleteDownloadRecord) {
        await app.DeleteDownloadRecord(id, removeFiles);
        activeDownloads = (activeDownloads || []).filter((d) => d && d.downloadId !== id);
        const hist = await GetDownloadHistory();
        downloadHistory = Array.isArray(hist) ? hist : [];
      }
    } catch (e: any) {
      console.error(e);
    }
  }

  async function handleLaunchGame(path: string) {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.LaunchGame) {
        await app.LaunchGame(path);
      } else {
        handleOpenFolder(path);
      }
    } catch (e: any) {
      console.warn('Failed to launch game executable, falling back to folder:', e);
      handleOpenFolder(path);
    }
  }

  async function handleSaveSettings(newSettings: any) {
    try {
      await SaveSettings(newSettings);
      settings = newSettings;
      showToast('Настройки успешно сохранены');
    } catch (e: any) {
      showToast('Ошибка сохранения: ' + (e?.message || e));
    }
  }

  async function handleImportXML(xml: string) {
    try {
      const servers = await ImportFileZillaXML(xml);
      settings = await GetSettings();
      showToast(`Импортировано серверов: ${servers ? servers.length : 0}`);
      handleRefreshCatalog();
    } catch (e: any) {
      showToast('Ошибка импорта: ' + (e?.message || e));
    }
  }

  async function handleImportXMLFile() {
    try {
      let servers: any;
      if (typeof (window as any)?.go?.main?.App?.ImportFileZillaFile === 'function') {
        servers = await (window as any).go.main.App.ImportFileZillaFile();
      }
      if (servers && servers.length > 0) {
        settings = await GetSettings();
        showToast(`Импортировано серверов: ${servers.length}`);
        handleRefreshCatalog();
      }
    } catch (e: any) {
      if (e) showToast('Ошибка импорта: ' + (e?.message || e));
    }
  }

  async function handleOpenConfigFolder() {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.OpenConfigFolder) {
        await app.OpenConfigFolder();
      }
    } catch (e: any) {
      showToast('Не удалось открыть папку: ' + (e?.message || e));
    }
  }

  async function handleClearMetadataCache(): Promise<number> {
    try {
      const app = (window as any)?.go?.main?.App;
      if (app && app.ClearMetadataCache) {
        const count = await app.ClearMetadataCache();
        showToast(`Кэш очищен (${count} записей)`);
        handleRefreshCatalog();
        return count;
      }
      return 0;
    } catch (e: any) {
      showToast('Ошибка очистки кэша: ' + (e?.message || e));
      return 0;
    }
  }

  onMount(() => {
    // Listen to real-time settings updates
    EventsOn('settings:updated', (updated: any) => {
      if (updated) {
        settings = updated;
      }
    });

    // Listen to real-time torrent catalog updates
    EventsOn('torrents:updated', async () => {
      try {
        const freshSettings = await GetSettings();
        if (freshSettings) settings = freshSettings;
      } catch (e) {
        console.error('Failed to reload settings on torrents:updated:', e);
      }
      handleLoadTorrentCatalog(false);
    });

    loadInitialData();

    // Listen to real-time download progress events
    EventsOn('download:progress', async (event: any) => {
      if (!event) return;
      const list = activeDownloads || [];
      const existingIdx = list.findIndex((d) => d && d.downloadId === event.downloadId);
      if (existingIdx >= 0) {
        list[existingIdx] = event;
        activeDownloads = [...list];
      } else {
        activeDownloads = [...list, event];
      }

      if (event.status === 'completed' || event.status === 'cancelled') {
        try {
          const hist = await GetDownloadHistory();
          downloadHistory = Array.isArray(hist) ? hist : [];
        } catch (e) {
          console.error(e);
        }
      }
    });

    // Listen to progressive Steam enrichment events
    EventsOn('game:enriched', (enrichedGame: any) => {
      if (!enrichedGame) return;
      pendingEnrichments.set(enrichedGame.id, enrichedGame);
      if (enrichedGame.steamAppId) {
        pendingEnrichments.set(`steam:${enrichedGame.steamAppId}`, enrichedGame);
      }
      scheduleBatchFlush();
    });

    // Listen to background Steam metadata enrichment progress
    EventsOn('metadata:progress', (progress: any) => {
      if (!progress) return;
      metadataProgress = {
        isSyncing: !!progress.isSyncing,
        current: progress.current || 0,
        total: progress.total || 0,
        currentGame: progress.currentGame || ''
      };
    });

    // Listen to real-time Steam review summary updates
    EventsOn('game:reviews-updated', (event: any) => {
      if (!event) return;
      if (event.gameId) pendingReviews.set(event.gameId, event);
      if (event.steamAppId) pendingReviews.set(`steam:${event.steamAppId}`, event);
      scheduleBatchFlush();
    });

    // Listen to remote catalog scan status
    EventsOn('catalog:status', (data: any) => {
      if (!data) return;
      if (data.status === 'connecting' || data.status === 'scanning') {
        catalogStatusText = data.message || '';
      } else {
        catalogStatusText = '';
      }
    });

    // Listen to fast real-time icon updates
    EventsOn('game:icon-updated', (event: any) => {
      if (!event || (!event.appId && !event.gameId) || !event.iconUrl) return;
      if (event.gameId) pendingIcons.set(event.gameId, event.iconUrl);
      if (event.appId) pendingIcons.set(`steam:${event.appId}`, event.iconUrl);
      scheduleBatchFlush();
    });

    // Setup Gamepad Navigation
    gamepad.onTabChange = (dir) => {
      const tabs: ('home' | 'catalog' | 'torrents' | 'collections' | 'favorites' | 'downloads' | 'settings')[] = [];
      if (displayMode === 'bigpicture') {
        tabs.push('home');
      }
      if (hasFtpServers) tabs.push('catalog');
      if (hasTorrentSources) tabs.push('torrents');
      if (displayMode !== 'bigpicture') {
        tabs.push('collections');
      }
      tabs.push('favorites', 'downloads', 'settings');
      const curIdx = tabs.indexOf(activeTab);
      if (curIdx === -1) {
        activeTab = tabs[0] || 'settings';
      } else if (dir === 'NEXT') {
        activeTab = tabs[(curIdx + 1) % tabs.length];
      } else {
        activeTab = tabs[(curIdx - 1 + tabs.length) % tabs.length];
      }
      setTimeout(() => {
        gamepad.focusFirstInZone('grid') || gamepad.focusFirstInZone('list') || gamepad.focusFirstInZone('detail');
      }, 100);
    };

    gamepad.onSearch = () => {
      if (hasFtpServers) {
        activeTab = 'catalog';
      } else if (hasTorrentSources) {
        activeTab = 'torrents';
      }
      TriggerSteamOSKeyboard();
    };

    gamepad.onBack = () => {
      const defaultTab = displayMode === 'bigpicture' ? 'home' : (hasFtpServers ? 'catalog' : (hasTorrentSources ? 'torrents' : 'settings'));
      if (activeTab !== defaultTab) {
        activeTab = defaultTab;
        setTimeout(() => {
          gamepad.focusFirstInZone('grid') || gamepad.focusFirstInZone('list');
        }, 100);
      }
    };

    gamepad.onControllerStateChange = (connected) => {
      isGamepadConnected = connected;
    };

    gamepad.start();
  });

  onDestroy(() => {
    if (batchFlushTimer) {
      clearTimeout(batchFlushTimer);
      batchFlushTimer = null;
    }
    gamepad.stop();
    EventsOff('download:progress');
    EventsOff('game:enriched');
    EventsOff('metadata:progress');
    EventsOff('torrents:updated');
    EventsOff('catalog:status');
    EventsOff('game:icon-updated');
    EventsOff('game:reviews-updated');
  });
</script>

{#if displayMode === 'bigpicture'}
  <BigPictureShell
    bind:activeTab
    {games}
    {torrentGames}
    {hasFtpServers}
    {hasTorrentSources}
    {activeDownloads}
    {downloadHistory}
    {settings}
    {isGamepadConnected}
    {isCatalogLoading}
    {isTorrentsLoading}
    onStartDownload={handleStartDownload}
    onPauseDownload={handlePauseDownload}
    onResumeDownload={handleResumeDownload}
    onCancelDownload={handleCancelDownload}
    onPauseAll={handlePauseAll}
    onResumeAll={handleResumeAll}
    onClearCompleted={handleClearCompleted}
    onDeleteRecord={handleDeleteRecord}
    onSaveSettings={handleSaveSettings}
    onImportXML={handleImportXML}
    onImportFile={handleImportXMLFile}
    onTestConnection={TestConnection}
    onSelectFolder={SelectDirectory}
    onOpenFolder={handleOpenFolder}
    onOpenConfigFolder={handleOpenConfigFolder}
    onClearMetadataCache={handleClearMetadataCache}
    onSwitchToDesktop={switchToDesktop}
    onCloseApp={() => Quit()}
  />
{:else}
  <!-- Full-viewport Master-Detail Application Shell (Desktop View) -->
  <div class="h-screen w-screen flex flex-col overflow-hidden bg-[#07080a] text-white selection:bg-[#3b82f6] selection:text-white font-sans-ui">
    <!-- Custom Window Control Bar -->
    <TitleBar onToggleBigPicture={switchToBigPicture} />

    <!-- Main Application Body: Left NavRail + Content Stage -->
    <div class="flex-1 flex overflow-hidden min-h-0">
      <!-- Left Thin Navigation Rail (64px) -->
      <NavRail
        bind:activeTab
        {hasFtpServers}
        {hasTorrentSources}
        activeDownloadsCount={(activeDownloads || []).filter((d) => d && (d.status === 'downloading' || d.status === 'queued')).length}
        downloadProgress={activeDownloadProgress}
        {isGamepadConnected}
        isConnected={!!settings?.activeServer?.host}
        serverName={settings?.activeServer?.name || ''}
        onRefresh={handleRefreshCatalog}
        {isRefreshing}
        {metadataProgress}
        isCatalogLoading={isCatalogLoading || isTorrentsLoading}
        onToggleBigPicture={switchToBigPicture}
      />

      <!-- Main Content Stage -->
      <div class="flex-1 flex overflow-hidden">
      {#key activeTab}
      {#if activeTab === 'catalog'}
        <MasterDetailCatalog
          {games}
          isLoading={isCatalogLoading}
          loadingStatusText={catalogStatusText}
          bind:searchQuery
          downloadPath={settings?.downloadPath || 'C:\\Ducke'}
          onStartDownload={handleStartDownload}
          onSelectFolder={SelectDirectory}
        />
      {:else if activeTab === 'torrents'}
        <MasterDetailCatalog
          games={torrentGames}
          isLoading={isTorrentsLoading}
          loadingStatusText="Загрузка каталога торрентов..."
          bind:searchQuery
          downloadPath={settings?.downloadPath || 'C:\\Ducke'}
          onStartDownload={handleStartDownload}
          onSelectFolder={SelectDirectory}
        />
      {:else if activeTab === 'collections'}
        <CollectionsView
          downloadPath={settings?.downloadPath || 'C:\\Ducke'}
          onStartDownload={handleStartDownload}
          onSelectFolder={SelectDirectory}
          onSearchInCatalog={(query: string) => {
            searchQuery = query;
            activeTab = hasTorrentSources ? 'torrents' : (hasFtpServers ? 'catalog' : 'settings');
          }}
        />
      {:else if activeTab === 'favorites'}
        <FavoritesView
          downloadPath={settings?.downloadPath || 'C:\\Ducke'}
          onStartDownload={handleStartDownload}
          onSelectFolder={SelectDirectory}
          onOpenCatalog={() => (activeTab = hasFtpServers ? 'catalog' : (hasTorrentSources ? 'torrents' : 'settings'))}
        />
      {:else if activeTab === 'downloads'}
        <DownloadsView
          {games}
          {activeDownloads}
          {downloadHistory}
          {settings}
          downloadPath={settings?.downloadPath || ''}
          onPause={handlePauseDownload}
          onResume={handleResumeDownload}
          onCancel={handleCancelDownload}
          onPauseAll={handlePauseAll}
          onResumeAll={handleResumeAll}
          onClearCompleted={handleClearCompleted}
          onDeleteRecord={handleDeleteRecord}
          onOpenFolder={handleOpenFolder}
          onLaunchGame={handleLaunchGame}
          onGoToCatalog={() => (activeTab = hasFtpServers ? 'catalog' : (hasTorrentSources ? 'torrents' : 'settings'))}
          onGoToSettings={() => (activeTab = 'settings')}
          onUpdateSpeedLimit={async (kbps: number) => {
            try {
              const app = (window as any)?.go?.main?.App;
              if (app && typeof app.UpdateSpeedLimit === 'function') {
                await app.UpdateSpeedLimit(kbps);
              } else if (settings) {
                const updated = { ...settings, maxSpeedKBps: kbps };
                await handleSaveSettings(updated);
              }
            } catch (e: any) {
              console.error('Failed to update speed limit:', e);
            }
          }}
        />
      {:else if activeTab === 'settings'}
        <SettingsView
          {settings}
          onSaveSettings={handleSaveSettings}
          onImportFile={handleImportXMLFile}
          onTestConnection={TestConnection}
          onSelectFolder={SelectDirectory}
          onOpenFolder={handleOpenFolder}
          onOpenConfigFolder={handleOpenConfigFolder}
          onClearMetadataCache={handleClearMetadataCache}
        />
      {/if}
      {/key}
      </div>
    </div>

    <!-- Toast Notification (Steam Style) -->
    {#if toastMessage}
      <div class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 px-4 py-2 rounded bg-[#21252c] border border-white/[0.1] text-white text-xs font-semibold shadow-2xl animate-fade-in flex items-center gap-2">
        <span class="w-2 h-2 rounded-full bg-[#3b82f6]"></span>
        <span>{toastMessage}</span>
      </div>
    {/if}

    <!-- Gamepad HUD bar -->
    <GamepadHUD isVisible={isGamepadConnected} />
  </div>
{/if}
