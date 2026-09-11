<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import TitleBar from './lib/components/TitleBar.svelte';
  import NavRail from './lib/components/NavRail.svelte';
  import BigPictureShell from './lib/components/bigpicture/BigPictureShell.svelte';
  import MasterDetailCatalog from './lib/components/MasterDetailCatalog.svelte';
  import DownloadsView from './lib/components/DownloadsView.svelte';
  import SettingsView from './lib/components/SettingsView.svelte';
  import FavoritesView from './lib/components/FavoritesView.svelte';
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
    GetTorrentSources,
    AddTorrentSource,
    RemoveTorrentSource,
    ToggleTorrentSource,
    SyncTorrentSources,
    StartTorrentDownload,
    GetFavorites
  } from '../wailsjs/go/main/App';

  import { EventsOn, EventsOff, Quit, WindowFullscreen, WindowUnfullscreen } from '../wailsjs/runtime/runtime';

  // Application State (Svelte 5 Runes)
  let displayMode = $state<'desktop' | 'bigpicture'>('desktop');
  let activeTab = $state<'catalog' | 'torrents' | 'favorites' | 'downloads' | 'settings'>('catalog');

  function switchToBigPicture() {
    displayMode = 'bigpicture';
    try {
      WindowFullscreen();
    } catch (e) {
      console.warn('Failed to enter fullscreen:', e);
    }
  }

  function switchToDesktop() {
    displayMode = 'desktop';
    try {
      WindowUnfullscreen();
    } catch (e) {
      console.warn('Failed to exit fullscreen:', e);
    }
  }
  let searchQuery = $state<string>('');
  let games = $state<any[]>([]);
  let torrentGames = $state<any[]>([]);
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

  let hasFtpServers = $derived.by(() => {
    if (!settings) return false;
    const hasSaved = Array.isArray(settings.savedServers) && settings.savedServers.some((s: any) => s && s.host && s.host.trim() !== '');
    const hasActive = !!(settings.activeServer && settings.activeServer.host && settings.activeServer.host.trim() !== '');
    return hasSaved || hasActive;
  });

  let hasTorrentSources = $derived.by(() => {
    if (!settings) return false;
    return Array.isArray(settings.torrentSources) && settings.torrentSources.some((s: any) => s && s.enabled);
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

  async function handleLoadTorrentCatalog(forceRefresh: boolean = false) {
    isTorrentsLoading = true;
    try {
      const res = await GetTorrentCatalog(forceRefresh);
      torrentGames = Array.isArray(res) ? res : [];
    } catch (e: any) {
      console.error('Failed to load torrent catalog:', e);
    } finally {
      isTorrentsLoading = false;
    }
  }

  async function loadInitialData() {
    isCatalogLoading = true;
    isTorrentsLoading = true;
    try {
      // 1. Fetch settings, downloads, and torrent sources immediately
      const [fetchedSettings, fetchedDownloads, fetchedHistory, fetchedSources] = await Promise.all([
        GetSettings().catch(() => null),
        GetDownloads().catch(() => []),
        GetDownloadHistory().catch(() => []),
        GetTorrentSources().catch(() => [])
      ]);

      if (fetchedSettings) {
        if ((!fetchedSettings.torrentSources || fetchedSettings.torrentSources.length === 0) && Array.isArray(fetchedSources) && fetchedSources.length > 0) {
          fetchedSettings.torrentSources = fetchedSources;
        }
        settings = fetchedSettings;
      }
      activeDownloads = Array.isArray(fetchedDownloads) ? fetchedDownloads : [];
      downloadHistory = Array.isArray(fetchedHistory) ? fetchedHistory : [];

      if (settings && settings.steamDeckMode) {
        switchToBigPicture();
      }

      // Automatically select initial active tab based on configured sources
      const ftpAvailable = (settings?.savedServers || []).some((s: any) => s && s.host && s.host.trim() !== '') ||
        !!(settings?.activeServer && settings.activeServer.host && settings.activeServer.host.trim() !== '');
      const torrentAvailable = (settings?.torrentSources || []).some((s: any) => s && s.enabled);

      if (!ftpAvailable && torrentAvailable) {
        activeTab = 'torrents';
      } else if (!ftpAvailable && !torrentAvailable) {
        activeTab = 'settings';
      } else {
        activeTab = 'catalog';
      }

      // 2. Fetch game catalog and torrent catalog concurrently and independently
      if (ftpAvailable) {
        GetCatalog(false)
          .then((res) => {
            games = Array.isArray(res) ? res : [];
          })
          .catch((err) => {
            console.error('Failed to load initial catalog:', err);
          })
          .finally(() => {
            isCatalogLoading = false;
          });
      } else {
        isCatalogLoading = false;
        games = [];
      }

      GetTorrentCatalog(false)
        .then((res) => {
          torrentGames = Array.isArray(res) ? res : [];
        })
        .catch((err) => {
          console.error('Failed to load torrent catalog:', err);
        })
        .finally(() => {
          isTorrentsLoading = false;
        });
    } catch (e: any) {
      console.error('Failed to load initial state:', e);
      isCatalogLoading = false;
      isTorrentsLoading = false;
    }
  }

  async function handleRefreshCatalog() {
    if (isRefreshing) return;
    isRefreshing = true;
    try {
      if (activeTab === 'torrents') {
        isTorrentsLoading = true;
        await SyncTorrentSources();
        const res = await GetTorrentCatalog(true);
        torrentGames = Array.isArray(res) ? res : [];
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
    loadInitialData();

    // Listen to real-time settings updates
    EventsOn('settings:updated', (updated: any) => {
      if (updated) {
        settings = updated;
      }
    });

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

    // Listen to progressive Steam enrichment events
    EventsOn('game:enriched', (enrichedGame: any) => {
      if (!enrichedGame) return;
      const match = (g: any) => {
        if (!g) return g;
        if (g.id === enrichedGame.id) return enrichedGame;
        if (enrichedGame.steamAppId && enrichedGame.steamAppId !== 0 && g.steamAppId === enrichedGame.steamAppId) {
          return {
            ...g,
            steamTitle: enrichedGame.steamTitle || g.steamTitle,
            iconUrl: enrichedGame.iconUrl || g.iconUrl,
            shortDescription: enrichedGame.shortDescription || g.shortDescription,
            detailedDescription: enrichedGame.detailedDescription || g.detailedDescription,
            headerImage: enrichedGame.headerImage || g.headerImage,
            capsuleImage: enrichedGame.capsuleImage || g.capsuleImage,
            backgroundImage: enrichedGame.backgroundImage || g.backgroundImage,
            screenshots: (enrichedGame.screenshots && enrichedGame.screenshots.length > 0) ? enrichedGame.screenshots : g.screenshots,
            movies: (enrichedGame.movies && enrichedGame.movies.length > 0) ? enrichedGame.movies : g.movies,
            genres: (enrichedGame.genres && enrichedGame.genres.length > 0) ? enrichedGame.genres : g.genres,
            developers: (enrichedGame.developers && enrichedGame.developers.length > 0) ? enrichedGame.developers : g.developers,
            publishers: (enrichedGame.publishers && enrichedGame.publishers.length > 0) ? enrichedGame.publishers : g.publishers,
            releaseDate: enrichedGame.releaseDate || g.releaseDate,
            controllerSupport: enrichedGame.controllerSupport || g.controllerSupport,
            pcRequirements: enrichedGame.pcRequirements || g.pcRequirements,
            metacriticScore: enrichedGame.metacriticScore || g.metacriticScore,
            reviewScoreDesc: enrichedGame.reviewScoreDesc || g.reviewScoreDesc,
            reviewPercent: enrichedGame.reviewPercent || g.reviewPercent,
            totalReviews: enrichedGame.totalReviews || g.totalReviews,
          };
        }
        return g;
      };
      games = (games || []).map(match);
      torrentGames = (torrentGames || []).map(match);
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
      const updateReview = (g: any) => {
        if (g && (g.id === event.gameId || (event.steamAppId > 0 && g.steamAppId === event.steamAppId))) {
          return {
            ...g,
            reviewScoreDesc: event.reviewScoreDesc,
            reviewPercent: event.reviewPercent,
            totalReviews: event.totalReviews,
          };
        }
        return g;
      };
      games = (games || []).map(updateReview);
      torrentGames = (torrentGames || []).map(updateReview);
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
      const updateIcon = (g: any) => {
        if (g && ((event.appId > 0 && g.steamAppId === event.appId) || g.id === event.gameId)) {
          return { ...g, iconUrl: event.iconUrl };
        }
        return g;
      };
      games = (games || []).map(updateIcon);
      torrentGames = (torrentGames || []).map(updateIcon);
    });

    // Setup Gamepad Navigation
    gamepad.onTabChange = (dir) => {
      const tabs: ('catalog' | 'torrents' | 'favorites' | 'downloads' | 'settings')[] = [];
      if (hasFtpServers) tabs.push('catalog');
      if (hasTorrentSources) tabs.push('torrents');
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
      const defaultTab = hasFtpServers ? 'catalog' : (hasTorrentSources ? 'torrents' : 'settings');
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
    gamepad.stop();
    EventsOff('download:progress');
    EventsOff('game:enriched');
    EventsOff('metadata:progress');
    EventsOff('torrents:updated');
    EventsOff('catalog:status');
    EventsOff('game:icon-updated');
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
      </div>
    </div>

    <!-- Toast Notification (Modern Pro Pill) -->
    {#if toastMessage}
      <div class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 px-4 py-2 rounded-lg bg-[#21252c] border border-white/[0.1] text-white text-xs font-semibold shadow-2xl animate-fade-in flex items-center gap-2">
        <span class="w-2 h-2 rounded-full bg-[#3b82f6]"></span>
        <span>{toastMessage}</span>
      </div>
    {/if}

    <!-- Gamepad HUD bar -->
    <GamepadHUD isVisible={isGamepadConnected} />
  </div>
{/if}
